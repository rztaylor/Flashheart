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
	"github.com/rztaylor/flashheart/internal/store"
)

// TestProtocolSmoke is the end-to-end smoke of agent-protocol §13: one
// Claude Code session, driven through the real hook and mcp commands,
// starts, creates and claims a ticket, edits, is stopped once for a
// handoff, checkpoints and ends; the next session gets the recovery note.
func TestProtocolSmoke(t *testing.T) {
	t.Parallel()

	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, repo := filepath.Join(base, "board"), filepath.Join(base, "src", "demo")
	mustWrite(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/feature/demo\n")
	mustWrite(t, filepath.Join(root, "demo", "project.yaml"), "name: demo\nkey: DM\nrepos:\n  - "+repo+"\nsettings:\n  enforce_handoff: true\n")
	h := newHarness(t)
	h.env["CLAUDE_PROJECT_DIR"] = repo

	hook := func(event, session string, extra map[string]any) string {
		t.Helper()
		payload := map[string]any{"session_id": session, "cwd": repo, "hook_event_name": event}
		for key, value := range extra {
			payload[key] = value
		}
		data, _ := json.Marshal(payload)
		deps := h.deps()
		deps.Stdin = bytes.NewReader(data)
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), []string{"hook", "claude", event, "--root", root}, &stdout, &stderr, deps); code != 0 || stderr.Len() > 0 {
			t.Fatalf("hook %s: code %d, stderr %q", event, code, stderr.String())
		}
		return stdout.String()
	}
	// tool calls a Flashheart tool as Claude Code does: PreToolUse stamps
	// the run into the input, then the MCP server runs it.
	tool := func(session, name string, args map[string]any) string {
		t.Helper()
		var stamped struct {
			HookSpecificOutput struct {
				UpdatedInput map[string]any `json:"updatedInput"`
			} `json:"hookSpecificOutput"`
		}
		out := hook("PreToolUse", session, map[string]any{"tool_name": "mcp__flashheart__" + name, "tool_input": args})
		if err := json.Unmarshal([]byte(out), &stamped); err != nil || stamped.HookSpecificOutput.UpdatedInput["run"] != "claude:"+session {
			t.Fatalf("PreToolUse did not stamp the run: %q", out)
		}
		return mcpCall(t, h, root, name, stamped.HookSpecificOutput.UpdatedInput)
	}

	const first, second = "11111111-aaaa", "22222222-bbbb"
	if note := hook("SessionStart", first, map[string]any{"source": "startup"}); note != "" {
		t.Fatalf("first session got a note: %q", note)
	}
	hook("UserPromptSubmit", first, map[string]any{"prompt": "build the card panel"})
	created := tool(first, "create_ticket", map[string]any{"type": "feature", "title": "Card panel", "description": "Open a ticket beside the board.", "criteria": []string{"Opens"}, "priority": "high"})
	if !strings.Contains(created, "ok ticket=DM-1") {
		t.Fatalf("create_ticket: %s", created)
	}
	if out := tool(first, "claim", map[string]any{"ticket": "DM-1"}); !strings.Contains(out, "ok ticket=DM-1 column=in-progress") {
		t.Fatalf("claim: %s", out)
	}
	hook("PostToolUse", first, map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(repo, "src", "Panel.tsx")}, "tool_response": map[string]any{}})

	// Stopping with edits and no checkpoint is blocked, once (HOOK-6).
	if out := hook("Stop", first, map[string]any{"stop_hook_active": false}); !strings.Contains(out, `"decision":"block"`) || !strings.Contains(out, "record a checkpoint on DM-1") {
		t.Fatalf("stop was not blocked: %q", out)
	}
	if out := tool(first, "checkpoint", map[string]any{"ticket": "DM-1", "done": []string{"Panel shell"}, "next": []string{"Wire the Runs tab"}, "files": []string{"src/Panel.tsx"}}); !strings.Contains(out, "ok ticket=DM-1 checkpoint") {
		t.Fatalf("checkpoint: %s", out)
	}
	// A new turn with nothing edited since the checkpoint stops freely.
	hook("UserPromptSubmit", first, map[string]any{"prompt": "thanks"})
	if out := hook("Stop", first, map[string]any{"stop_hook_active": false}); out != "" {
		t.Fatalf("stop after the checkpoint: %q", out)
	}
	hook("SessionEnd", first, map[string]any{"reason": "prompt_input_exit"})

	// The next session in the worktree is told where the work stands.
	note := hook("SessionStart", second, map[string]any{"source": "startup"})
	for _, part := range []string{
		"[Flashheart] run=claude:22222222 project=demo (key DM) branch=feature/demo",
		`Ticket DM-1 \"Card panel\" (in-progress, linked by branch).`,
		"Last handoff — Next: Wire the Runs tab.",
		"Use the flashheart MCP tools",
	} {
		if !strings.Contains(note, part) {
			t.Fatalf("recovery note missing %q:\n%s", part, note)
		}
	}
	ticket, _ := os.ReadFile(filepath.Join(root, "demo", "tickets", "DM-1-card-panel", "DM-1-card-panel.md"))
	for _, part := range []string{"status: in-progress", "branch: feature/demo", "**Next**\n\n- Wire the Runs tab", "- src/Panel.tsx"} {
		if !strings.Contains(string(ticket), part) {
			t.Fatalf("ticket missing %q:\n%s", part, ticket)
		}
	}
	if log, _ := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log")); len(log) > 0 {
		t.Fatalf("hook errors:\n%s", log)
	}
}

func mustWrite(t *testing.T, name, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mcpCall runs flashheart mcp for one tool call and returns its text.
func mcpCall(t *testing.T, h *harness, root, name string, args map[string]any) string {
	t.Helper()
	return mcpCallAs(t, h, root, "smoke", name, args)
}

// mcpCallAs is mcpCall from a client with the given name, as one session:
// the server's input ends after the call.
func mcpCallAs(t *testing.T, h *harness, root, client, name string, args map[string]any) string {
	t.Helper()
	call, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	requests := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"` + client + `","version":"1"}}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" + string(call) + "\n"
	out := &lockedBuffer{}
	deps := h.deps()
	deps.Stdin = &holdingReader{data: []byte(requests), done: func() bool { return strings.Count(out.String(), "\n") >= 2 }}
	var stderr bytes.Buffer
	if code := Run(context.Background(), []string{"mcp", "--root", root}, out, &stderr, deps); code != 0 || stderr.Len() > 0 {
		t.Fatalf("mcp %s: code %d, stderr %q", name, code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var response struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &response); err != nil {
		t.Fatalf("mcp %s response %q: %v", name, out.String(), err)
	}
	var text strings.Builder
	for _, content := range response.Result.Content {
		text.WriteString(content.Text)
	}
	if response.Result.IsError {
		t.Fatalf("mcp %s failed: %s", name, text.String())
	}
	return text.String()
}

// TestHooklessSessionOverStdio drives the real mcp command as Codex without
// Flashheart's hooks: its claim starts a run (agent-protocol §7.1), and
// the end of its input, the session closing, ends the run before the
// command returns.
func TestHooklessSessionOverStdio(t *testing.T) {
	t.Parallel()

	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, repo := filepath.Join(base, "board"), filepath.Join(base, "src", "demo")
	mustWrite(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
	mustWrite(t, filepath.Join(root, "demo", "project.yaml"), "name: demo\nkey: DM\nrepos:\n  - "+repo+"\n")
	mustWrite(t, filepath.Join(root, "demo", "tickets", "DM-1-panel", "DM-1-panel.md"), "---\nid: DM-1\nstatus: backlog\ntype: feature\npriority: high\n---\n# Panel\n")
	h := newHarness(t)
	h.env["CLAUDE_PROJECT_DIR"] = repo
	if out := mcpCallAs(t, h, root, "codex-mcp-client", "claim", map[string]any{"ticket": "DM-1"}); !strings.Contains(out, "ok ticket=DM-1 column=in-progress") {
		t.Fatalf("claim: %s", out)
	}
	var kinds []string
	run := ""
	if err := events.New(store.New(root)).Read("demo", time.Time{}, func(e events.Event) {
		if run == "" {
			run = e.Run
		}
		if e.Run == run {
			kinds = append(kinds, e.Kind)
		}
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{events.RunStart, events.Claim, events.TicketMoved, events.ToolUsed, events.RunEnd}
	if !strings.HasPrefix(run, "codex:mcp-") || !slices.Equal(kinds, want) {
		t.Fatalf("run %q events = %v, want %v", run, kinds, want)
	}
}
