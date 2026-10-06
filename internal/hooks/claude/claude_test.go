package claude

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
