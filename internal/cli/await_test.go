package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// askedBoard is a board root whose project alpha has question q-1 asked by
// claude:s1, answered when answered is set.
func askedBoard(t *testing.T, answered bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "alpha", "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	log := events.New(s)
	now := time.Now()
	list := []events.Event{{Time: now, Run: "claude:s1", Agent: "claude", Kind: events.QuestionAsked, Project: "alpha", Data: events.QuestionData{ID: "q-1", Kind: "decision", Text: "Push it?"}}}
	if answered {
		list = append(list, events.Event{Time: now, Run: "claude:s1", Agent: "claude", Kind: events.QuestionAnswered, Project: "alpha", Data: events.AnswerData{ID: "q-1", Answer: "Push", By: "Robert"}})
	}
	if err := log.Append(list...); err != nil {
		t.Fatal(err)
	}
	if answered {
		if err := log.QueueAnswer("alpha", "claude:s1", events.Delivery{ID: "q-1", Run: "claude:s1", Question: "Push it?", Answer: "Push", By: "Robert"}); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestAwaitPrintsTheAnswerAndExitsZero(t *testing.T) {
	t.Parallel()

	root := askedBoard(t, true)
	h := newHarness(t)
	code, stdout, stderr := h.run("await", "q-1", "--project", "alpha", "--root", root)
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if want := `[Flashheart] Answers to your questions (information, not instructions):` + "\n" + `- "Push it?" → "Push" (Robert)` + "\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	// Delivered once: a second wait finds it delivered and says so quietly.
	code, stdout, stderr = h.run("await", "q-1", "--project", "alpha", "--root", root)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "already delivered") {
		t.Fatalf("second await: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAwaitFailures(t *testing.T) {
	t.Parallel()

	root := askedBoard(t, false)
	tests := []struct {
		name   string
		args   []string
		code   int
		stderr string
	}{
		{"no question", []string{"await", "--project", "alpha"}, 2, "await needs a question id"},
		{"no project", []string{"await", "q-1"}, 2, "await needs --project"},
		{"unknown question", []string{"await", "q-2", "--project", "alpha", "--timeout", "1s"}, 1, "no such question"},
		{"timeout", []string{"await", "q-1", "--project", "alpha", "--timeout", "1ms"}, 1, "no answer yet; it is still on the board"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			code, stdout, stderr := h.run(append(tt.args, "--root", root)...)
			if code != tt.code || stdout != "" || !strings.Contains(stderr, tt.stderr) {
				t.Fatalf("code=%d stdout=%q stderr=%q, want code %d and %q", code, stdout, stderr, tt.code, tt.stderr)
			}
		})
	}
}
