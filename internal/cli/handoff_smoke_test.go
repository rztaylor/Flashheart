package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

// TestCrossAgentHandoff extends the protocol smoke (agent-protocol §13)
// across agents, through the real hook and mcp commands: a session of
// one agent creates and claims a ticket, edits, is stopped once for a
// handoff, checkpoints and ends; a session of the other agent then starts
// in the same worktree, gets the recovery note naming the ticket and its
// last handoff, claims the ticket and continues to its own checkpoint.
// The ticket's runs show both sessions and its handoff names each in
// turn. Codex's hooks and MCP calls are its recorded payloads
// (testdata/hooks/codex) with the session and working directory swapped.
func TestCrossAgentHandoff(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, first, second string }{
		{"claude to codex", "claude", "codex"},
		{"codex to claude", "codex", "claude"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			root, repo := filepath.Join(base, "board"), filepath.Join(base, "src", "demo")
			mustWrite(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/feature/demo\n")
			mustWrite(t, filepath.Join(root, "demo", "project.yaml"), "name: demo\nkey: DM\nrepos:\n  - "+repo+"\nsettings:\n  enforce_handoff: true\n")
			// Each direction's sessions have their own ids, unlike the
			// recordings' one, so the two directions never share a run.
			sessions := map[string]string{
				"claude to codex": "33333333-cccc",
				"codex to claude": "44444444-dddd",
			}
			codexSessions := map[string]string{
				"claude to codex": "019a7c10-0000-7000-8000-0000000000a1",
				"codex to claude": "019a7c20-0000-7000-8000-0000000000b2",
			}
			agents := map[string]*agentSession{
				"claude": newClaudeSession(t, root, repo, sessions[tc.name]),
				"codex":  newCodexSession(t, root, repo, codexSessions[tc.name]),
			}
			first, second := agents[tc.first], agents[tc.second]
			const firstNext, secondNext = "Wire the Runs tab", "Write the review"

			// The first session starts the work and hands it off.
			if note := first.start(t); note != "" {
				t.Fatalf("first session got a note: %q", note)
			}
			first.prompt(t)
			if out := first.tool(t, "create_ticket", map[string]any{"type": "feature", "title": "Card panel", "description": "Open a ticket beside the board.", "criteria": []string{"Opens"}, "priority": "high"}); !strings.Contains(out, "ok ticket=DM-1") {
				t.Fatalf("create_ticket: %s", out)
			}
			first.work(t, firstNext)
			first.end(t)
			if handoff := readHandoff(t, root); !strings.HasPrefix(handoff.By, first.agent+":") || !slices.Equal(handoff.Next, []string{firstNext}) {
				t.Fatalf("handoff after the first session = %+v, want it by %s with Next %q", handoff, first.run, firstNext)
			}

			// The second session in the same worktree is told where the
			// work stands, takes the claim over and carries on.
			note := second.start(t)
			for _, part := range []string{
				"[Flashheart] run=" + second.agent + ":",
				" project=demo (key DM) branch=feature/demo\n",
				`Ticket DM-1 "Card panel" (in-progress, linked by branch).`,
				"Last handoff — Next: " + firstNext + ".",
				"Use the flashheart MCP tools: claim to continue",
			} {
				if !strings.Contains(note, part) {
					t.Fatalf("recovery note missing %q:\n%s", part, note)
				}
			}
			second.prompt(t)
			second.work(t, secondNext)
			second.end(t)
			if handoff := readHandoff(t, root); !strings.HasPrefix(handoff.By, second.agent+":") || !slices.Equal(handoff.Next, []string{secondNext}) {
				t.Fatalf("handoff after the second session = %+v, want it by %s with Next %q", handoff, second.run, secondNext)
			}

			// The log names each session, by its full run id, for its own
			// claim and checkpoint, in turn.
			claims, checkpoints := ticketRunsOf(t, root, events.Claim), ticketRunsOf(t, root, events.Checkpoint)
			want := []string{first.run, second.run}
			if !slices.Equal(claims, want) || !slices.Equal(checkpoints, want) {
				t.Fatalf("claims by %v and checkpoints by %v, want both %v", claims, checkpoints, want)
			}
			// The ticket's runs, as its Runs tab lists them, are both
			// sessions.
			files := store.New(root)
			defer files.Close()
			snapshot, err := index.New(files, index.Options{Events: events.New(files)}).Rebuild()
			if err != nil {
				t.Fatal(err)
			}
			var linked []string
			for _, run := range snapshot.TicketRuns("DM-1") {
				linked = append(linked, run.ID)
			}
			slices.Sort(linked)
			slices.Sort(want)
			if !slices.Equal(linked, want) {
				t.Fatalf("ticket runs = %v, want %v", linked, want)
			}
			if log, _ := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log")); len(log) > 0 {
				t.Fatalf("hook errors:\n%s", log)
			}
		})
	}
}

// agentSession is one agent session driven through the real hook and mcp
// commands. hook runs a hook event and returns its output; tool calls a
// Flashheart tool as the agent does; edit records one edit of a file.
type agentSession struct {
	agent, run string
	hook       func(t *testing.T, event string, extra map[string]any) string
	tool       func(t *testing.T, name string, args map[string]any) string
	edit       func(t *testing.T)
}

// start opens the session and returns its recovery note, if any.
func (s *agentSession) start(t *testing.T) string {
	t.Helper()
	out := s.hook(t, "SessionStart", map[string]any{"source": "startup"})
	if out == "" {
		return ""
	}
	var context struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &context); err != nil || context.HookSpecificOutput.HookEventName != "SessionStart" {
		t.Fatalf("%s SessionStart output %q: %v", s.agent, out, err)
	}
	return context.HookSpecificOutput.AdditionalContext
}

// prompt submits a prompt from the user.
func (s *agentSession) prompt(t *testing.T) {
	t.Helper()
	s.hook(t, "UserPromptSubmit", map[string]any{"prompt": "carry on with the card panel"})
}

// work claims DM-1, edits, is stopped once for a handoff, checkpoints with
// next as its next step and stops freely: the stop hook's block continues
// the turn, and the next stop says a stop hook is active (§9).
func (s *agentSession) work(t *testing.T, next string) {
	t.Helper()
	if out := s.tool(t, "claim", map[string]any{"ticket": "DM-1"}); !strings.Contains(out, "ok ticket=DM-1 column=in-progress") {
		t.Fatalf("%s claim: %s", s.agent, out)
	}
	s.edit(t)
	if out := s.hook(t, "Stop", map[string]any{"stop_hook_active": false}); !strings.Contains(out, `"decision":"block"`) || !strings.Contains(out, "record a checkpoint on DM-1") {
		t.Fatalf("%s stop was not blocked: %q", s.agent, out)
	}
	if out := s.tool(t, "checkpoint", map[string]any{"ticket": "DM-1", "done": []string{"Worked as " + s.agent}, "next": []string{next}, "files": []string{"src/Panel.tsx"}}); !strings.Contains(out, "ok ticket=DM-1 checkpoint") {
		t.Fatalf("%s checkpoint: %s", s.agent, out)
	}
	if out := s.hook(t, "Stop", map[string]any{"stop_hook_active": true}); out != "" {
		t.Fatalf("%s stop after the checkpoint: %q", s.agent, out)
	}
}

// end closes the session.
func (s *agentSession) end(t *testing.T) {
	t.Helper()
	s.hook(t, "SessionEnd", nil)
}

// agentHook runs flashheart hook for agent and event with payload on stdin,
// failing the test on any error output, and returns its output.
func agentHook(t *testing.T, deps Dependencies, root, agent, event string, payload map[string]any) string {
	t.Helper()
	data, _ := json.Marshal(payload)
	deps.Stdin = bytes.NewReader(data)
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"hook", agent, event, "--root", root}, &stdout, &stderr, deps); code != 0 || stderr.Len() > 0 {
		t.Fatalf("hook %s %s: code %d, stderr %q", agent, event, code, stderr.String())
	}
	return stdout.String()
}

// newClaudeSession is a Claude Code session in repo, with the payloads of
// TestProtocolSmoke: its PreToolUse hook stamps the run into each
// Flashheart call's input, and its MCP server knows the project from
// CLAUDE_PROJECT_DIR.
func newClaudeSession(t *testing.T, root, repo, session string) *agentSession {
	h := newHarness(t)
	h.env["CLAUDE_PROJECT_DIR"] = repo
	s := &agentSession{agent: "claude", run: "claude:" + session}
	s.hook = func(t *testing.T, event string, extra map[string]any) string {
		t.Helper()
		payload := map[string]any{"session_id": session, "cwd": repo, "hook_event_name": event}
		if event == "SessionEnd" {
			payload["reason"] = "prompt_input_exit"
		}
		for key, value := range extra {
			payload[key] = value
		}
		return agentHook(t, h.deps(), root, "claude", event, payload)
	}
	s.tool = func(t *testing.T, name string, args map[string]any) string {
		t.Helper()
		var stamped struct {
			HookSpecificOutput struct {
				UpdatedInput map[string]any `json:"updatedInput"`
			} `json:"hookSpecificOutput"`
		}
		out := s.hook(t, "PreToolUse", map[string]any{"tool_name": "mcp__flashheart__" + name, "tool_input": args})
		if err := json.Unmarshal([]byte(out), &stamped); err != nil || stamped.HookSpecificOutput.UpdatedInput["run"] != s.run {
			t.Fatalf("PreToolUse did not stamp the run: %q", out)
		}
		return mcpCallAs(t, h, root, "claude-code", name, stamped.HookSpecificOutput.UpdatedInput)
	}
	s.edit = func(t *testing.T) {
		t.Helper()
		s.hook(t, "PostToolUse", map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(repo, "src", "Panel.tsx")}, "tool_response": map[string]any{}})
	}
	return s
}

// Codex's recordings (testdata/hooks/codex/MANIFEST.md) name one session
// and one working directory throughout.
const (
	recordedCodexSession = "019a7c00-0000-7000-8000-000000000001"
	recordedCodexCwd     = "/Users/example/src/demo"
)

// codexCases are the recorded payloads a Codex session sends here, by event.
var codexCases = map[string]string{
	"SessionStart":     "startup",
	"UserPromptSubmit": "prompt",
	"PostToolUse":      "apply-patch",
	"Stop":             "stop",
	"SessionEnd":       "other",
}

// newCodexSession is a Codex session in repo, sending its recorded
// payloads as session: Flashheart's PreToolUse is not registered for
// Codex (§5.3), so its calls carry no run and the server attributes them
// by the session each names in its _meta (§7.1 step 2). Codex starts the
// MCP server in the session's working directory and sets no
// CLAUDE_PROJECT_DIR.
func newCodexSession(t *testing.T, root, repo, session string) *agentSession {
	h := newHarness(t)
	deps := func() Dependencies {
		deps := h.deps()
		deps.Getwd = func() (string, error) { return repo, nil }
		return deps
	}
	// recorded reads a recorded payload or call as this session would
	// send it in repo.
	recorded := func(t *testing.T, name string) map[string]any {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "hooks", "codex", name))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ReplaceAll(strings.ReplaceAll(string(data), recordedCodexSession, session), recordedCodexCwd, repo)
		var value map[string]any
		if err := json.Unmarshal([]byte(text), &value); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return value
	}
	s := &agentSession{agent: "codex", run: "codex:" + session}
	s.hook = func(t *testing.T, event string, extra map[string]any) string {
		t.Helper()
		payload := recorded(t, filepath.Join(event, codexCases[event]+".json"))
		for key, value := range extra {
			if _, ok := payload[key]; ok {
				payload[key] = value
			}
		}
		return agentHook(t, deps(), root, "codex", event, payload)
	}
	s.tool = func(t *testing.T, name string, args map[string]any) string {
		t.Helper()
		params := recorded(t, filepath.Join("mcp", "tools-call-flashheart.json"))["params"].(map[string]any)
		params["name"], params["arguments"] = name, args
		return mcpCallWith(t, deps(), root, "codex-mcp-client", params)
	}
	// The recorded apply_patch adds notes.txt and edits README.md.
	s.edit = func(t *testing.T) {
		t.Helper()
		s.hook(t, "PostToolUse", nil)
	}
	return s
}

// readHandoff reads DM-1's handoff from the board.
func readHandoff(t *testing.T, root string) (handoff struct {
	By   string
	Next []string
}) {
	t.Helper()
	files := store.New(root)
	defer files.Close()
	details, err := files.ReadProject("demo")
	if err != nil {
		t.Fatal(err)
	}
	for _, ticket := range details.Tickets {
		if ticket.ID == "DM-1" && ticket.Handoff != nil {
			handoff.By, handoff.Next = ticket.Handoff.By, ticket.Handoff.Next
		}
	}
	return handoff
}

// ticketRunsOf lists the full run ids of DM-1's events of kind, in order.
func ticketRunsOf(t *testing.T, root string, kind events.Kind) []string {
	t.Helper()
	files := store.New(root)
	defer files.Close()
	var list []string
	if err := events.New(files).Read("demo", time.Time{}, func(e events.Event) {
		var data struct {
			Ticket string `json:"ticket"`
		}
		if e.Kind == kind && e.Decode(&data) == nil && data.Ticket == "DM-1" {
			list = append(list, e.Run)
		}
	}); err != nil {
		t.Fatal(err)
	}
	return list
}
