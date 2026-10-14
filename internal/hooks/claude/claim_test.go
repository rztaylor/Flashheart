package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/hooks"
	"github.com/rztaylor/flashheart/internal/protocol"
)

// Claude Code shows its model the additionalContext of a prompt and of a
// tool result, so a claim reminder that is due goes with either (D50).
func TestPromptsAndToolResultsCarryNotes(t *testing.T) {
	t.Parallel()

	for event, want := range map[string]bool{
		"UserPromptSubmit": true, "PostToolUse": true, "PostToolUseFailure": true,
		"SessionStart": false, "Stop": false, "SessionEnd": false, "SubagentStop": false, "PermissionRequest": false,
	} {
		input, err := Adapter{}.Parse(event, []byte(`{"session_id":"s1","cwd":"/x","tool_name":"Read","agent_id":"","prompt":"go"}`))
		if err != nil || input.Notes != want {
			t.Errorf("%s: notes %v, %v; want %v", event, input.Notes, err, want)
		}
	}
}

// A Claude Code session that edits without a ticket is told to claim one
// with its next tool result or prompt, once; a project that enforces
// handoffs holds its stop with the reminder instead.
func TestAnUnclaimedClaudeSessionIsRemindedToClaim(t *testing.T) {
	t.Parallel()

	hook := func(t *testing.T, root, repo, event string, fields map[string]any, minute int) string {
		t.Helper()
		payload := map[string]any{"session_id": "s1", "cwd": repo}
		for k, v := range fields {
			payload[k] = v
		}
		data, _ := json.Marshal(payload)
		var stdout bytes.Buffer
		hooks.Run(hooks.Options{Root: root, Event: event, Stdin: bytes.NewReader(data), Stdout: &stdout, Now: func() time.Time { return now.Add(time.Duration(minute) * time.Minute) }, Adapter: Adapter{}, ChangeTimeout: 10 * time.Second})
		return stdout.String()
	}
	setup := func(t *testing.T, settings string) (string, string) {
		root, repo := t.TempDir(), demoRepo(t)
		hook(t, root, repo, "SessionStart", map[string]any{"source": "startup"}, 0)
		if err := os.WriteFile(filepath.Join(root, "demo", "project.yaml"), []byte("name: demo\nkey: DM\nnext_id: 1\nrepos:\n  - "+repo+"\n"+settings), 0o644); err != nil {
			t.Fatal(err)
		}
		hook(t, root, repo, "UserPromptSubmit", map[string]any{"prompt": "fix it"}, 1)
		hook(t, root, repo, "PostToolUse", map[string]any{"tool_name": "Edit", "tool_input": map[string]string{"file_path": filepath.Join(repo, "a.go")}}, 2)
		return root, repo
	}
	read := map[string]any{"tool_name": "Read", "tool_input": map[string]string{"file_path": "a.go"}}

	t.Run("as context", func(t *testing.T) {
		t.Parallel()
		root, repo := setup(t, "")
		if out := hook(t, root, repo, "Stop", nil, 3); out != "" {
			t.Fatalf("stop output = %s", out)
		}
		out := hook(t, root, repo, "PostToolUse", read, 4)
		want, _ := json.Marshal(map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": "PostToolUse", "additionalContext": protocol.ClaimReminder + "\n"}})
		if strings.TrimSpace(out) != string(want) {
			t.Fatalf("tool result output = %s, want %s", out, want)
		}
		if out := hook(t, root, repo, "UserPromptSubmit", map[string]any{"prompt": "more"}, 5); out != "" {
			t.Fatalf("prompt output = %s", out)
		}
	})
	t.Run("as the stop's reason", func(t *testing.T) {
		t.Parallel()
		root, repo := setup(t, "settings:\n  enforce_handoff: true\n")
		out := hook(t, root, repo, "Stop", map[string]any{"stop_hook_active": false}, 3)
		want, _ := json.Marshal(map[string]string{"decision": "block", "reason": protocol.ClaimReminder})
		if strings.TrimSpace(out) != string(want) {
			t.Fatalf("stop output = %s, want %s", out, want)
		}
		if out := hook(t, root, repo, "Stop", map[string]any{"stop_hook_active": true}, 4); out != "" {
			t.Fatalf("second stop output = %s", out)
		}
		if out := hook(t, root, repo, "PostToolUse", read, 5); out != "" {
			t.Fatalf("tool result output = %s", out)
		}
	})
}
