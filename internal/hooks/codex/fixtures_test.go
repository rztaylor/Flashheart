package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// placeholder is the working directory every fixture is recorded in.
const placeholder = "/Users/example/src/demo"

var fixtures = filepath.Join("..", "..", "..", "testdata", "hooks", "codex")

// recorded lists the hook events Codex fired in the recordings
// (testdata/hooks/codex/MANIFEST.md).
var recorded = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PostToolUse",
	"SubagentStart", "SubagentStop", "PreCompact", "PostCompact", "Stop", "SessionEnd", "Interrupt",
}

type payload struct {
	SessionID string         `json:"session_id"`
	TurnID    string         `json:"turn_id"`
	Cwd       string         `json:"cwd"`
	Event     string         `json:"hook_event_name"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	ToolUseID string         `json:"tool_use_id"`
	AgentID   string         `json:"agent_id"`
}

func readPayload(t *testing.T, path string) payload {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var p payload
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return p
}

func TestEveryRecordedPayloadNamesItsEvent(t *testing.T) {
	events, err := os.ReadDir(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	for _, e := range events {
		if !e.IsDir() || e.Name() == "mcp" {
			continue
		}
		seen = append(seen, e.Name())
		cases, err := filepath.Glob(filepath.Join(fixtures, e.Name(), "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(cases) == 0 {
			t.Errorf("%s has no payloads", e.Name())
		}
		for _, path := range cases {
			if strings.HasSuffix(path, ".events.json") || strings.HasSuffix(path, ".output.json") {
				continue
			}
			p := readPayload(t, path)
			if p.Event != e.Name() {
				t.Errorf("%s: hook_event_name %q, want %q", path, p.Event, e.Name())
			}
			if p.SessionID == "" {
				t.Errorf("%s: no session_id", path)
			}
			if p.Cwd != placeholder {
				t.Errorf("%s: cwd %q, want the scrubbed %q", path, p.Cwd, placeholder)
			}
		}
	}
	slices.Sort(seen)
	want := slices.Clone(recorded)
	slices.Sort(want)
	if !slices.Equal(seen, want) {
		t.Errorf("fixture events %v, want the recorded %v", seen, want)
	}
}

// A subagent's hooks carry the parent's session_id with the subagent's own
// agent_id and turn_id.
func TestSubagentPayloadsCarryTheParentSession(t *testing.T) {
	parent := readPayload(t, filepath.Join(fixtures, "PreToolUse", "bash.json"))
	child := readPayload(t, filepath.Join(fixtures, "PreToolUse", "subagent-bash.json"))
	if child.SessionID != parent.SessionID {
		t.Errorf("subagent session_id %q, want the parent's %q", child.SessionID, parent.SessionID)
	}
	if child.AgentID == "" || child.TurnID == parent.TurnID {
		t.Errorf("subagent agent_id %q turn_id %q: want its own agent and turn", child.AgentID, child.TurnID)
	}
	start := readPayload(t, filepath.Join(fixtures, "SubagentStart", "default.json"))
	if start.AgentID != child.AgentID {
		t.Errorf("SubagentStart agent_id %q, want %q", start.AgentID, child.AgentID)
	}
}

// Codex sends the hook session's id and turn with every MCP tool call, its
// callId is the hook's tool_use_id, and it
// keeps input a PreToolUse hook rewrote (the recording stamped run), so
// either can tie a flashheart call to its session (agent-protocol §5.3).
// Both payloads come from one recorded session.
func TestFlashheartToolCallsCarryTheHookSession(t *testing.T) {
	hook := readPayload(t, filepath.Join(fixtures, "PreToolUse", "flashheart-tool.json"))
	if !strings.HasPrefix(hook.ToolName, "mcp__flashheart__") {
		t.Fatalf("tool_name %q, want a flashheart MCP tool", hook.ToolName)
	}
	b, err := os.ReadFile(filepath.Join(fixtures, "mcp", "tools-call-flashheart.json"))
	if err != nil {
		t.Fatal(err)
	}
	var call struct {
		Method string `json:"method"`
		Params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
			Meta      struct {
				CallID    string `json:"callId"`
				SessionID string `json:"sessionId"`
				Turn      struct {
					SessionID string `json:"session_id"`
					TurnID    string `json:"turn_id"`
				} `json:"x-codex-turn-metadata"`
			} `json:"_meta"`
		} `json:"params"`
	}
	if err := json.Unmarshal(b, &call); err != nil {
		t.Fatal(err)
	}
	if call.Params.Meta.CallID == "" || call.Params.Meta.CallID != hook.ToolUseID {
		t.Errorf("_meta.callId %q, want the hook's tool_use_id %q", call.Params.Meta.CallID, hook.ToolUseID)
	}
	if call.Method != "tools/call" || "mcp__flashheart__"+call.Params.Name != hook.ToolName {
		t.Errorf("call %s %q, want tools/call of %q", call.Method, call.Params.Name, hook.ToolName)
	}
	meta := call.Params.Meta
	if meta.SessionID != hook.SessionID || meta.Turn.SessionID != hook.SessionID || meta.Turn.TurnID != hook.TurnID {
		t.Errorf("_meta session %q / %q turn %q, want the hook's session %q turn %q",
			meta.SessionID, meta.Turn.SessionID, meta.Turn.TurnID, hook.SessionID, hook.TurnID)
	}
	if call.Params.Arguments["run"] != "codex:recorded-stamp" {
		t.Errorf("arguments %v, want the run the PreToolUse hook stamped", call.Params.Arguments)
	}
}

// The ChatGPT app's calls carry their session the same way, with no hooks.
func TestAppToolCallsCarryTheirSession(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(fixtures, "mcp", "tools-call-app.json"))
	if err != nil {
		t.Fatal(err)
	}
	var call struct {
		Params struct {
			Meta struct {
				SessionID string `json:"sessionId"`
				Turn      struct {
					SessionID string `json:"session_id"`
					TurnID    string `json:"turn_id"`
					Trigger   string `json:"turn_trigger"`
				} `json:"x-codex-turn-metadata"`
			} `json:"_meta"`
		} `json:"params"`
	}
	if err := json.Unmarshal(b, &call); err != nil {
		t.Fatal(err)
	}
	meta := call.Params.Meta
	if meta.SessionID == "" || meta.Turn.SessionID != meta.SessionID || meta.Turn.TurnID == "" {
		t.Errorf("_meta session %q / %q turn %q, want one session and a turn", meta.SessionID, meta.Turn.SessionID, meta.Turn.TurnID)
	}
	if meta.Turn.Trigger != "composer" {
		t.Errorf("turn_trigger %q, want the app's composer", meta.Turn.Trigger)
	}
}
