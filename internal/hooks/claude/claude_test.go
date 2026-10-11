package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
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

// placeholder is the working directory every fixture is recorded in.
const placeholder = "/Users/example/src/demo"

var fixtures = filepath.Join("..", "..", "..", "testdata", "hooks", "claude")

var now = time.Date(2026, 10, 5, 14, 12, 9, 0, time.UTC)

// mapped lists every Claude Code hook event in agent-protocol §5.2.
var mapped = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PostToolUseFailure",
	"PermissionRequest", "PermissionDenied", "Notification", "TaskCreated", "TaskCompleted",
	"SubagentStart", "SubagentStop", "PreCompact", "PostCompact", "Stop", "SessionEnd",
}

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

func TestGoldenPayloads(t *testing.T) {
	t.Parallel()

	cases, err := filepath.Glob(filepath.Join(fixtures, "*", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, payloadFile := range cases {
		if strings.HasSuffix(payloadFile, ".events.json") || strings.HasSuffix(payloadFile, ".output.json") {
			continue
		}
		event := filepath.Base(filepath.Dir(payloadFile))
		name := event + "/" + strings.TrimSuffix(filepath.Base(payloadFile), ".json")
		covered[event] = true
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(payloadFile)
			if err != nil {
				t.Fatal(err)
			}
			wantRaw, err := os.ReadFile(strings.TrimSuffix(payloadFile, ".json") + ".events.json")
			if err != nil {
				t.Fatalf("every payload needs an .events.json: %v", err)
			}
			var want []expected
			if err := json.Unmarshal(wantRaw, &want); err != nil {
				t.Fatal(err)
			}

			root := t.TempDir()
			repo := demoRepo(t)
			payload := strings.ReplaceAll(string(raw), placeholder, repo)
			var stdout bytes.Buffer
			hooks.Run(hooks.Options{Root: root, Event: event, Stdin: strings.NewReader(payload), Stdout: &stdout, Now: func() time.Time { return now }, Adapter: Adapter{}})

			if data, _ := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log")); len(data) > 0 {
				t.Fatalf("hook-errors.log: %s", data)
			}
			// A case may expect output (<case>.output.json); otherwise a hook on
			// an empty board prints nothing.
			if wantOut, err := os.ReadFile(strings.TrimSuffix(payloadFile, ".json") + ".output.json"); err == nil {
				var gotJSON, wantJSON any
				if err := json.Unmarshal(stdout.Bytes(), &gotJSON); err != nil {
					t.Fatalf("output %q: %v", stdout.String(), err)
				}
				_ = json.Unmarshal(wantOut, &wantJSON)
				if !reflect.DeepEqual(gotJSON, wantJSON) {
					t.Fatalf("output =\n%s\nwant\n%s", stdout.String(), wantOut)
				}
			} else if stdout.Len() > 0 {
				t.Fatalf("stdout on an empty board = %s", stdout.String())
			}
			var got []expected
			if s, err := store.Open(root); err == nil {
				_ = events.New(s).Read("demo", time.Time{}, func(e events.Event) {
					if e.Agent != "claude" || e.Project != "demo" || !e.Time.Equal(now) {
						t.Errorf("envelope = %+v", e)
					}
					var data map[string]any
					_ = e.Decode(&data)
					encoded, _ := json.Marshal(data)
					_ = json.Unmarshal([]byte(strings.ReplaceAll(string(encoded), repo, placeholder)), &data)
					got = append(got, expected{Run: e.Run, Kind: e.Kind, Data: data})
				})
				s.Close()
			}
			if len(got) == 0 && len(want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, want) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				t.Fatalf("events =\n%s\nwant\n%s", gotJSON, wantRaw)
			}
			// Nothing from tool inputs beyond paths and plan items is stored.
			for _, secret := range []string{"sk-ant-", "rm -rf", "curl", "old_string", "hello", "signed in", "Signed in", "Header overlaps", "Found 3 files", "Done.", "Add a line"} {
				if strings.Contains(string(mustRead(t, root)), secret) {
					t.Fatalf("event log stores %q", secret)
				}
			}
		})
	}
	for _, event := range mapped {
		if !covered[event] {
			t.Errorf("no golden payload for %s", event)
		}
	}
}

func mustRead(t *testing.T, root string) []byte {
	t.Helper()
	var all []byte
	files, _ := filepath.Glob(filepath.Join(root, "demo", ".flashheart", "events", "*.jsonl"))
	for _, file := range files {
		data, _ := os.ReadFile(file)
		all = append(all, data...)
	}
	return all
}

func TestRenderSessionStartContext(t *testing.T) {
	t.Parallel()

	out := Adapter{}.Render("SessionStart", hooks.Output{Context: "[Flashheart] note\n"})
	var decoded struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	if decoded.HookSpecificOutput.HookEventName != "SessionStart" || decoded.HookSpecificOutput.AdditionalContext != "[Flashheart] note\n" {
		t.Fatalf("output = %+v", decoded)
	}
	if out := (Adapter{}).Render("SessionStart", hooks.Output{}); out != nil {
		t.Fatalf("empty output = %q", out)
	}
}

func TestParseRejectsBadPayloads(t *testing.T) {
	t.Parallel()

	for name, payload := range map[string]string{
		"not json":           "{",
		"no session":         `{"cwd":"/x"}`,
		"unsafe session":     `{"session_id":"../../etc","cwd":"/x"}`,
		"unsafe subagent id": `{"session_id":"abc","cwd":"/x","agent_id":"a/b"}`,
	} {
		if _, err := (Adapter{}).Parse("UserPromptSubmit", []byte(payload)); err == nil {
			t.Errorf("%s: Parse succeeded", name)
		}
	}
	// Events Flashheart does not map are accepted and record nothing.
	input, err := Adapter{}.Parse("FutureEvent", []byte(`{"session_id":"abc","cwd":"/x"}`))
	if err != nil || len(input.Events) != 0 || input.Recovery {
		t.Fatalf("unknown event = %+v, %v", input, err)
	}
	if !slices.Contains(mapped, "SessionEnd") {
		t.Fatal("mapped list is incomplete")
	}
}

func TestStampingAddsTheFullRunToFlashheartTools(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, payload, want string
	}{
		{"session, keeping the input", `{"session_id":"s1","cwd":"/x","tool_name":"mcp__flashheart__claim","tool_input":{"ticket":"FH-1","run":"claude:short"}}`,
			`{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{"run":"claude:s1","ticket":"FH-1"}}}`},
		{"subagent", `{"session_id":"s1","agent_id":"a7","cwd":"/x","tool_name":"mcp__flashheart__checkpoint","tool_input":{"ticket":"FH-1"}}`,
			`{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{"run":"claude:s1/a7","ticket":"FH-1"}}}`},
		{"another server's tool", `{"session_id":"s1","cwd":"/x","tool_name":"mcp__other__claim","tool_input":{}}`, ""},
		{"a built-in tool", `{"session_id":"s1","cwd":"/x","tool_name":"Bash","tool_input":{"command":"ls"}}`, ""},
		{"input that is not an object", `{"session_id":"s1","cwd":"/x","tool_name":"mcp__flashheart__claim","tool_input":"FH-1"}`, ""},
	}
	for _, tc := range cases {
		input, err := Adapter{}.Parse("PreToolUse", []byte(tc.payload))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(input.Events) > 0 || input.Recovery {
			t.Fatalf("%s: stamping recorded %+v", tc.name, input)
		}
		got := ""
		if input.Reply != nil {
			got = strings.TrimSpace(string(Adapter{}.Render("PreToolUse", *input.Reply)))
		}
		if got != tc.want {
			t.Errorf("%s: output = %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestStopAndPromptAskForTheirChecks(t *testing.T) {
	t.Parallel()

	stop, err := Adapter{}.Parse("Stop", []byte(`{"session_id":"s1","cwd":"/x","stop_hook_active":true}`))
	if err != nil || !stop.Stop || !stop.StopActive {
		t.Fatalf("Stop input = %+v, %v", stop, err)
	}
	prompt, err := Adapter{}.Parse("UserPromptSubmit", []byte(`{"session_id":"s1","cwd":"/x","prompt":"go"}`))
	if err != nil || prompt.Answers != "claude:s1" {
		t.Fatalf("prompt input = %+v, %v", prompt, err)
	}
	out := Adapter{}.Render("Stop", hooks.Output{Block: "record a checkpoint"})
	if strings.TrimSpace(string(out)) != `{"decision":"block","reason":"record a checkpoint"}` {
		t.Fatalf("block output = %s", out)
	}
	out = Adapter{}.Render("UserPromptSubmit", hooks.Output{Context: "answers"})
	if !strings.Contains(string(out), `"hookEventName":"UserPromptSubmit","additionalContext":"answers"`) && !strings.Contains(string(out), `"additionalContext":"answers","hookEventName":"UserPromptSubmit"`) {
		t.Fatalf("prompt output = %s", out)
	}
}

// gitDemoRepo is demoRepo as a real git checkout, for hooks that ask git
// whether the worktree changed.
func gitDemoRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "src", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "-q", "-b", "feature/demo")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return dir
}

// FH-55: a recorded session's tool results, replayed through the adapter as
// 200 calls in ten minutes, append about ten activity records (one a
// minute, one for each newly edited path, one at the stop) with every tool
// use and failure counted, instead of 200 tool.used events.
func TestManyToolResultsAreRecordedAsAboutTenActivityRecords(t *testing.T) {
	t.Parallel()

	root, repo := t.TempDir(), gitDemoRepo(t)
	const session = "340b083f-5d70-41b2-8cff-ec700908097a"
	load := func(name string) string {
		raw, err := os.ReadFile(filepath.Join(fixtures, name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		fields["session_id"] = session
		data, _ := json.Marshal(fields)
		return strings.ReplaceAll(string(data), placeholder, repo)
	}
	send := func(name string, at time.Time) {
		hooks.Run(hooks.Options{Root: root, Event: strings.Split(name, "/")[0], Stdin: strings.NewReader(load(name)), Now: func() time.Time { return at }, Adapter: Adapter{}, ChangeTimeout: 10 * time.Second})
	}

	send("UserPromptSubmit/prompt", now)
	mix := []string{"PostToolUse/read", "PostToolUse/bash", "PostToolUse/edit", "PostToolUseFailure/read-missing", "PostToolUse/read", "PostToolUse/write", "PostToolUse/agent", "PostToolUse/bash"}
	failures := 0
	for i := range 200 {
		name := mix[i%len(mix)]
		if strings.HasPrefix(name, "PostToolUseFailure") {
			failures++
		}
		send(name, now.Add(time.Duration(i+1)*3*time.Second))
	}
	send("Stop/stop", now.Add(10*time.Minute+time.Second))
	if data, _ := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log")); len(data) > 0 {
		t.Fatalf("hook-errors.log: %s", data)
	}

	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	records, tools, failed, shell := 0, 0, 0, 0
	var paths []string
	err = events.New(s).Read("demo", time.Time{}, func(e events.Event) {
		switch e.Kind {
		case events.ToolUsed:
			t.Errorf("a tool.used was recorded: %+v", e)
		case events.Activity:
			var data events.ActivityData
			_ = e.Decode(&data)
			records++
			tools, failed, shell = tools+data.Tools, failed+data.Failed, shell+len(data.Shell)
			paths = append(paths, data.Paths...)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if tools != 200 || failed != failures {
		t.Fatalf("records count %d tools and %d failures, want 200 and %d", tools, failed, failures)
	}
	if !slices.Equal(paths, []string{"README.md", "notes.txt"}) {
		t.Fatalf("paths = %v, want each edited path once", paths)
	}
	t.Logf("%d activity records for 200 tool results in ten minutes", records)
	if most := 10 + len(paths) + 1; records > most {
		t.Fatalf("%d activity records, want at most %d", records, most)
	}
	if shell == 0 {
		t.Fatal("the shell commands' windows were not recorded")
	}
}

// The checkpoint tool's own result tells hooks the session's edit paths are
// settled, so the next edit of each is recorded again (FH-55).
func TestTheCheckpointToolSettlesRecordedPaths(t *testing.T) {
	t.Parallel()

	for tool, want := range map[string]bool{"mcp__flashheart__checkpoint": true, "mcp__flashheart__claim": false, "mcp__other__checkpoint": false, "Bash": false} {
		for _, event := range []string{"PostToolUse", "PostToolUseFailure"} {
			input, err := Adapter{}.Parse(event, []byte(`{"session_id":"s1","cwd":"/x","tool_name":"`+tool+`","tool_input":{}}`))
			if err != nil || input.Settled != want || len(input.Events) != 1 {
				t.Errorf("%s %s: settled %v, events %d, %v; want settled %v", event, tool, input.Settled, len(input.Events), err, want)
			}
		}
	}
}
