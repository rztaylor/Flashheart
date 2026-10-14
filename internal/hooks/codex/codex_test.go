package codex

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/hooks"
	"github.com/rztaylor/flashheart/internal/store"
)

var now = time.Date(2026, 10, 12, 21, 0, 0, 0, time.UTC)

// session is the recorded payloads' session, as a run.
const session = "codex:019a7c00-0000-7000-8000-000000000001"

type expected struct {
	Run  string         `json:"run"`
	Kind string         `json:"kind"`
	Data map[string]any `json:"data"`
}

func demoRepo(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "src", "demo")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/feature/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// send replays a recorded payload (<Event>/<case>) in repo and returns
// what the hook printed.
func send(t *testing.T, root, repo, name string, at time.Time) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtures, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	payload := strings.ReplaceAll(string(raw), placeholder, repo)
	var stdout bytes.Buffer
	hooks.Run(hooks.Options{Root: root, Event: strings.Split(name, "/")[0], Stdin: strings.NewReader(payload), Stdout: &stdout, Now: func() time.Time { return at }, Adapter: Adapter{}})
	return stdout.String()
}

func noErrors(t *testing.T, root string) {
	t.Helper()
	if data, _ := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log")); len(data) > 0 {
		t.Fatalf("hook-errors.log: %s", data)
	}
}

func readLog(t *testing.T, root string) []events.Event {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var list []events.Event
	if err := events.New(s).Read("demo", time.Time{}, func(e events.Event) { list = append(list, e) }); err != nil {
		t.Fatal(err)
	}
	return list
}

// Every recorded payload, on an empty board, records the events beside it
// (<case>.events.json) and prints nothing.
func TestGoldenPayloads(t *testing.T) {
	t.Parallel()

	cases, err := filepath.Glob(filepath.Join(fixtures, "*", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, payloadFile := range cases {
		if strings.HasSuffix(payloadFile, ".events.json") || filepath.Base(filepath.Dir(payloadFile)) == "mcp" {
			continue
		}
		event := filepath.Base(filepath.Dir(payloadFile))
		name := event + "/" + strings.TrimSuffix(filepath.Base(payloadFile), ".json")
		covered[event] = true
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			wantRaw, err := os.ReadFile(strings.TrimSuffix(payloadFile, ".json") + ".events.json")
			if err != nil {
				t.Fatalf("every payload needs an .events.json: %v", err)
			}
			var want []expected
			if err := json.Unmarshal(wantRaw, &want); err != nil {
				t.Fatal(err)
			}

			root, repo := t.TempDir(), demoRepo(t)
			if out := send(t, root, repo, name, now); out != "" {
				t.Fatalf("stdout on an empty board = %s", out)
			}
			noErrors(t, root)
			var got []expected
			if _, err := os.Stat(filepath.Join(root, "demo")); err == nil {
				for _, e := range readLog(t, root) {
					if e.Agent != Agent || e.Project != "demo" || !e.Time.Equal(now) {
						t.Errorf("envelope = %+v", e)
					}
					var data map[string]any
					_ = e.Decode(&data)
					encoded, _ := json.Marshal(data)
					_ = json.Unmarshal([]byte(strings.ReplaceAll(string(encoded), repo, placeholder)), &data)
					got = append(got, expected{Run: e.Run, Kind: string(e.Kind), Data: data})
				}
			}
			if len(got) == 0 && len(want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, want) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				t.Fatalf("events =\n%s\nwant\n%s", gotJSON, wantRaw)
			}
			// Nothing from prompts, replies or tool inputs beyond paths is
			// stored (HOOK-2).
			stored := string(eventFiles(t, root))
			for _, secret := range []string{"ls -la", "rm -rf", "cat missing", "rg --files", "hello", "wobble", "demo app", "gAAAAA", "list_files", "Create notes.txt", "Done:", "Files:", "Wait completed", "transcript"} {
				if strings.Contains(stored, secret) {
					t.Fatalf("event log stores %q", secret)
				}
			}
		})
	}
	for _, event := range recorded {
		if !covered[event] {
			t.Errorf("no golden payload for %s", event)
		}
	}
}

func eventFiles(t *testing.T, root string) []byte {
	t.Helper()
	var all []byte
	files, _ := filepath.Glob(filepath.Join(root, "demo", ".flashheart", "events", "*.jsonl"))
	for _, file := range files {
		data, _ := os.ReadFile(file)
		all = append(all, data...)
	}
	return all
}

func toolData(t *testing.T, input hooks.Input) events.ToolData {
	t.Helper()
	if len(input.Events) == 0 || input.Events[0].Kind != events.ToolUsed {
		t.Fatalf("events = %+v, want a tool.used first", input.Events)
	}
	return input.Events[0].Data.(events.ToolData)
}

// apply_patch's file headers are the edit's paths: added, updated, deleted
// and moved files, relative ones from the session's directory. Lines of a
// file's content never count, whatever they say.
func TestApplyPatchHeadersAreEditPaths(t *testing.T) {
	t.Parallel()

	patch := strings.Join([]string{
		"*** Begin Patch",
		"*** Add File: /w/demo/notes.txt",
		"+*** Update File: /w/demo/not-a-header.txt",
		"*** Update File: /w/demo/README.md",
		"*** Move to: /w/demo/docs/README.md",
		"@@",
		"-old",
		"+new",
		"*** Delete File: old.txt",
		"*** Update File: /w/demo/notes.txt",
		"*** End Patch",
	}, "\n")
	input, _ := json.Marshal(map[string]any{
		"session_id": "s1", "cwd": "/w/demo/sub", "tool_name": "apply_patch",
		"tool_input": map[string]string{"command": patch},
	})
	parsed, err := Adapter{}.Parse("PostToolUse", input)
	if err != nil {
		t.Fatal(err)
	}
	data := toolData(t, parsed)
	want := []string{"/w/demo/notes.txt", "/w/demo/README.md", "/w/demo/docs/README.md", "/w/demo/sub/old.txt"}
	if !slices.Equal(data.Paths, want) || data.Shell || !data.OK || data.Tool != "apply_patch" {
		t.Fatalf("tool data = %+v, want paths %v", data, want)
	}

	// A patch with no headers, or input that is not a patch, edits nothing.
	for _, toolInput := range []string{`{"command":"*** Begin Patch\n*** End Patch"}`, `"apply"`, `{}`} {
		parsed, err := Adapter{}.Parse("PostToolUse", []byte(`{"session_id":"s1","cwd":"/w","tool_name":"apply_patch","tool_input":`+toolInput+`}`))
		if err != nil {
			t.Fatal(err)
		}
		if data := toolData(t, parsed); len(data.Paths) != 0 {
			t.Errorf("%s: paths %v", toolInput, data.Paths)
		}
	}
}

// Codex's shell tool is a shell command for hooks; nothing else is.
func TestOnlyBashIsAShellCommand(t *testing.T) {
	t.Parallel()

	for tool, shell := range map[string]bool{"Bash": true, "apply_patch": false, "collaborationspawn_agent": false, "mcp__flashheart__claim": false} {
		parsed, err := Adapter{}.Parse("PostToolUse", []byte(`{"session_id":"s1","cwd":"/w","tool_name":"`+tool+`","tool_input":{"command":"ls"}}`))
		if err != nil {
			t.Fatal(err)
		}
		if data := toolData(t, parsed); data.Shell != shell {
			t.Errorf("%s: shell %v, want %v", tool, data.Shell, shell)
		}
	}
}

// update_plan is an ordinary tool use: plans are not recorded (D51), so
// its steps are never read or stored.
func TestUpdatePlanRecordsOnlyItsUse(t *testing.T) {
	t.Parallel()

	payload := `{"session_id":"s1","cwd":"/w","tool_name":"update_plan","tool_input":{"explanation":"Starting.","plan":[` +
		`{"step":"Read the code","status":"completed"},{"step":"Write the test","status":"in_progress"}]}}`
	parsed, err := Adapter{}.Parse("PostToolUse", []byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Events) != 1 || parsed.Events[0].Kind != events.ToolUsed {
		t.Fatalf("events = %+v, want the tool use alone", parsed.Events)
	}
	if data := parsed.Events[0].Data.(events.ToolData); data.Tool != "update_plan" || data.Shell || len(data.Paths) != 0 {
		t.Fatalf("tool use = %+v", data)
	}
}

// project makes the demo project with a ticket in progress on the repo's
// branch, optionally enforcing handoffs.
func project(t *testing.T, root, repo, settings string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "demo", "tickets", "DM-1-card-panel"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(root, "demo", "project.yaml"):                                     "name: demo\nkey: DM\nnext_id: 2\nrepos:\n  - " + repo + "\n" + settings,
		filepath.Join(root, "demo", "tickets", "DM-1-card-panel", "DM-1-card-panel.md"): "---\nid: DM-1\nstatus: in-progress\ntype: feature\npriority: high\ncreated: 2026-10-01\nbranch: feature/demo\n---\n# Card panel\n\n## Handoff\n\n**Next**\n\n- Wire the Runs tab\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// additionalContext reads hookSpecificOutput as Codex does, for the event.
func additionalContext(t *testing.T, out, event string) string {
	t.Helper()
	var decoded struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	if decoded.HookSpecificOutput.HookEventName != event {
		t.Fatalf("output %s names event %q, want %q", out, decoded.HookSpecificOutput.HookEventName, event)
	}
	return decoded.HookSpecificOutput.AdditionalContext
}

// A session start gets the recovery note and a prompt the waiting answers,
// both as additionalContext, which Codex adds as a developer message
// (HOOK-3, HOOK-5; MANIFEST.md).
func TestRecoveryNoteAndAnswersReachCodex(t *testing.T) {
	t.Parallel()

	root, repo := t.TempDir(), demoRepo(t)
	send(t, root, repo, "SessionStart/startup", now)
	project(t, root, repo, "")

	note := additionalContext(t, send(t, root, repo, "SessionStart/compact", now.Add(time.Minute)), "SessionStart")
	for _, part := range []string{"[Flashheart] run=codex:00000001 project=demo (key DM) branch=feature/demo", `Ticket DM-1 "Card panel"`, "Next: Wire the Runs tab."} {
		if !strings.Contains(note, part) {
			t.Fatalf("recovery note lacks %q:\n%s", part, note)
		}
	}

	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	log := events.New(s)
	if err := log.Append(
		events.Event{Time: now, Run: session, Agent: Agent, Kind: events.QuestionAsked, Project: "demo", Data: events.QuestionData{ID: "q-1", Kind: "decision", Text: "Which schema?", Ticket: "DM-1"}},
		events.Event{Time: now, Run: session, Agent: Agent, Kind: events.QuestionAnswered, Project: "demo", Data: events.AnswerData{ID: "q-1", Answer: "v2", By: "Robert"}},
	); err != nil {
		t.Fatal(err)
	}
	if err := log.Queue("demo", events.Entry{Type: events.EntryAnswer, ID: "q-1", Run: session, Ticket: "DM-1", Question: "Which schema?", Text: "v2", By: "Robert"}); err != nil {
		t.Fatal(err)
	}
	s.Close()

	answers := additionalContext(t, send(t, root, repo, "UserPromptSubmit/prompt", now.Add(2*time.Minute)), "UserPromptSubmit")
	if !strings.Contains(answers, `- DM-1: "Which schema?" → "v2" (Robert)`) {
		t.Fatalf("answers = %q", answers)
	}
	if out := send(t, root, repo, "UserPromptSubmit/prompt", now.Add(3*time.Minute)); out != "" {
		t.Fatalf("second prompt repeated the answers: %s", out)
	}
	noErrors(t, root)
}

func turnEnds(t *testing.T, root string) []bool {
	t.Helper()
	var blocked []bool
	for _, e := range readLog(t, root) {
		if e.Kind == events.TurnEnd {
			var data events.TurnEndData
			_ = e.Decode(&data)
			blocked = append(blocked, data.BlockedForHandoff)
		}
	}
	return blocked
}

// A Stop with edits since the last checkpoint is blocked once for a
// handoff when the project enforces them (HOOK-6). An Interrupt cannot be
// stopped, so it ends the turn unblocked, as does a Stop Codex is already
// continuing for a hook or one in a project that does not enforce.
func TestOnlyAStopBlocksForAHandoff(t *testing.T) {
	t.Parallel()

	const reason = "Flashheart: record a checkpoint on DM-1 (done, next, files) before stopping."
	edit := func(t *testing.T, settings string) (string, string) {
		root, repo := t.TempDir(), demoRepo(t)
		send(t, root, repo, "SessionStart/startup", now)
		project(t, root, repo, settings)
		send(t, root, repo, "UserPromptSubmit/prompt", now.Add(time.Minute))
		send(t, root, repo, "PostToolUse/apply-patch", now.Add(2*time.Minute))
		return root, repo
	}
	const enforce = "settings:\n  enforce_handoff: true\n"

	t.Run("a stop is blocked once", func(t *testing.T) {
		t.Parallel()
		root, repo := edit(t, enforce)
		out := strings.TrimSpace(send(t, root, repo, "Stop/stop", now.Add(3*time.Minute)))
		want, _ := json.Marshal(map[string]string{"decision": "block", "reason": reason})
		if out != string(want) {
			t.Fatalf("stop output = %s, want %s", out, want)
		}
		// Codex continues the turn and says so in the next Stop.
		active := strings.Replace(string(mustRead(t, "Stop/stop")), `"stop_hook_active": false`, `"stop_hook_active": true`, 1)
		var stdout bytes.Buffer
		hooks.Run(hooks.Options{Root: root, Event: "Stop", Stdin: strings.NewReader(strings.ReplaceAll(active, placeholder, repo)), Stdout: &stdout, Now: func() time.Time { return now.Add(4 * time.Minute) }, Adapter: Adapter{}})
		if stdout.Len() > 0 {
			t.Fatalf("an active stop hook was blocked again: %s", stdout.String())
		}
		if got := turnEnds(t, root); !slices.Equal(got, []bool{true, false}) {
			t.Fatalf("turn ends blocked = %v", got)
		}
		noErrors(t, root)
	})
	t.Run("an interrupt is never blocked", func(t *testing.T) {
		t.Parallel()
		root, repo := edit(t, enforce)
		for _, name := range []string{"Interrupt/esc", "Interrupt/permission-denied"} {
			if out := send(t, root, repo, name, now.Add(3*time.Minute)); out != "" {
				t.Fatalf("%s output = %s", name, out)
			}
		}
		if got := turnEnds(t, root); !slices.Equal(got, []bool{false, false}) {
			t.Fatalf("turn ends blocked = %v", got)
		}
		noErrors(t, root)
	})
	t.Run("a project that does not enforce lets it stop", func(t *testing.T) {
		t.Parallel()
		root, repo := edit(t, "")
		if out := send(t, root, repo, "Stop/stop", now.Add(3*time.Minute)); out != "" {
			t.Fatalf("stop output = %s", out)
		}
		noErrors(t, root)
	})
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtures, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// A payload the adapter cannot read is logged to hook-errors.log and
// records and prints nothing (HOOK-1); an event Flashheart does not map is
// accepted and records nothing.
func TestBadPayloadsAreLogged(t *testing.T) {
	t.Parallel()

	for name, payload := range map[string]string{
		"not json":           "{",
		"no session":         `{"cwd":"/x"}`,
		"unsafe session":     `{"session_id":"../../etc","cwd":"/x"}`,
		"unsafe subagent id": `{"session_id":"abc","cwd":"/x","agent_id":"a/b"}`,
	} {
		root := t.TempDir()
		var stdout bytes.Buffer
		hooks.Run(hooks.Options{Root: root, Event: "UserPromptSubmit", Stdin: strings.NewReader(payload), Stdout: &stdout, Adapter: Adapter{}})
		log, _ := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log"))
		if !strings.Contains(string(log), "codex UserPromptSubmit: ") || stdout.Len() > 0 {
			t.Errorf("%s: hook-errors.log %q, stdout %q", name, log, stdout.String())
		}
		if _, err := os.Stat(filepath.Join(root, "demo")); err == nil {
			t.Errorf("%s: recorded a project", name)
		}
	}
	input, err := Adapter{}.Parse("FutureEvent", []byte(`{"session_id":"abc","cwd":"/x"}`))
	if err != nil || len(input.Events) != 0 || input.Recovery || input.Reply != nil {
		t.Fatalf("unknown event = %+v, %v", input, err)
	}
}

// The checkpoint tool's result settles the session's edit paths (FH-55).
func TestTheCheckpointToolSettlesRecordedPaths(t *testing.T) {
	t.Parallel()

	for tool, want := range map[string]bool{"mcp__flashheart__checkpoint": true, "mcp__flashheart__claim": false, "mcp__other__checkpoint": false, "Bash": false} {
		input, err := Adapter{}.Parse("PostToolUse", []byte(`{"session_id":"s1","cwd":"/x","tool_name":"`+tool+`","tool_input":{}}`))
		if err != nil || input.Settled != want || len(input.Events) != 1 {
			t.Errorf("%s: settled %v, events %d, %v; want settled %v", tool, input.Settled, len(input.Events), err, want)
		}
	}
}

// Render prints nothing without something to say.
func TestRenderWithNothingToSayIsSilent(t *testing.T) {
	t.Parallel()

	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Stop", "Interrupt", "PreToolUse"} {
		if out := (Adapter{}).Render(event, hooks.Output{}); out != nil {
			t.Errorf("%s: output %q", event, out)
		}
	}
}
