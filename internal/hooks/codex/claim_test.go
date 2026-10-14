package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/protocol"
)

// Codex is known to show its model the additionalContext of a prompt
// only (MANIFEST.md), so a due claim reminder waits for the next prompt
// (D50).
func TestOnlyPromptsCarryNotes(t *testing.T) {
	t.Parallel()

	for _, event := range []string{"UserPromptSubmit", "PostToolUse", "SessionStart", "Stop", "Interrupt", "SessionEnd"} {
		input, err := Adapter{}.Parse(event, []byte(`{"session_id":"s1","cwd":"/x","tool_name":"Bash"}`))
		if want := event == "UserPromptSubmit"; err != nil || input.Notes != want {
			t.Errorf("%s: notes %v, %v; want %v", event, input.Notes, err, want)
		}
	}
}

// A Codex session that patches files without a ticket is told to claim
// one with its next prompt, once; a project that enforces handoffs holds
// its stop with the reminder instead.
func TestAnUnclaimedCodexSessionIsRemindedToClaim(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T, settings string) (string, string) {
		root, repo := t.TempDir(), demoRepo(t)
		send(t, root, repo, "SessionStart/startup", now)
		if err := os.WriteFile(filepath.Join(root, "demo", "project.yaml"), []byte("name: demo\nkey: DM\nnext_id: 1\nrepos:\n  - "+repo+"\n"+settings), 0o644); err != nil {
			t.Fatal(err)
		}
		send(t, root, repo, "UserPromptSubmit/prompt", now.Add(time.Minute))
		send(t, root, repo, "PostToolUse/apply-patch", now.Add(2*time.Minute))
		return root, repo
	}

	t.Run("with the next prompt", func(t *testing.T) {
		t.Parallel()
		root, repo := setup(t, "")
		if out := send(t, root, repo, "Stop/stop", now.Add(3*time.Minute)); out != "" {
			t.Fatalf("stop output = %s", out)
		}
		if out := send(t, root, repo, "PostToolUse/bash", now.Add(4*time.Minute)); out != "" {
			t.Fatalf("tool result output = %s", out)
		}
		note := additionalContext(t, send(t, root, repo, "UserPromptSubmit/prompt", now.Add(5*time.Minute)), "UserPromptSubmit")
		if note != protocol.ClaimReminder+"\n" {
			t.Fatalf("prompt context = %q", note)
		}
		if out := send(t, root, repo, "UserPromptSubmit/prompt", now.Add(6*time.Minute)); out != "" {
			t.Fatalf("second prompt output = %s", out)
		}
		noErrors(t, root)
	})
	t.Run("as the stop's reason", func(t *testing.T) {
		t.Parallel()
		root, repo := setup(t, "settings:\n  enforce_handoff: true\n")
		out := strings.TrimSpace(send(t, root, repo, "Stop/stop", now.Add(3*time.Minute)))
		want, _ := json.Marshal(map[string]string{"decision": "block", "reason": protocol.ClaimReminder})
		if out != string(want) {
			t.Fatalf("stop output = %s, want %s", out, want)
		}
		if out := send(t, root, repo, "UserPromptSubmit/prompt", now.Add(4*time.Minute)); out != "" {
			t.Fatalf("prompt output = %s", out)
		}
		noErrors(t, root)
	})
}
