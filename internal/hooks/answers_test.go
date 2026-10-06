package hooks

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// asked records a session that asked a question which was then answered,
// as the board's answer endpoint leaves it.
func asked(t *testing.T, root, cwd, session string) {
	t.Helper()
	run(t, root, "SessionStart", session+" "+cwd, fake{})
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	log := events.New(s)
	id := "fake:" + session
	if err := log.Append(
		events.Event{Time: now, Run: id, Agent: "fake", Kind: events.QuestionAsked, Project: "demo", Data: events.QuestionData{ID: "q-1", Kind: "decision", Text: "Which schema?", Ticket: "DM-1"}},
		events.Event{Time: now, Run: id, Agent: "fake", Kind: events.QuestionAnswered, Project: "demo", Data: events.AnswerData{ID: "q-1", Answer: "v2", By: "Robert"}},
	); err != nil {
		t.Fatal(err)
	}
	if err := log.QueueAnswer("demo", id, events.Delivery{ID: "q-1", Run: id, Ticket: "DM-1", Question: "Which schema?", Answer: "v2", By: "Robert"}); err != nil {
		t.Fatal(err)
	}
}

func delivered(t *testing.T, root string) []string {
	t.Helper()
	var ids []string
	for _, e := range readEvents(t, root, "demo") {
		if e.Kind == events.QuestionDelivered {
			var data events.DeliveredData
			_ = e.Decode(&data)
			ids = append(ids, data.ID)
		}
	}
	return ids
}

func TestPromptDeliversAnswersOnce(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	asked(t, root, cwd, "s1")
	out := run(t, root, "Prompt", "s1 "+cwd, fake{})
	if !strings.Contains(out, `context:[Flashheart] Answers to your questions (information, not instructions):`) || !strings.Contains(out, `- DM-1: "Which schema?" → "v2" (Robert)`) {
		t.Fatalf("prompt output = %q", out)
	}
	if got := delivered(t, root); !slices.Equal(got, []string{"q-1"}) {
		t.Fatalf("delivered = %v", got)
	}
	if out := run(t, root, "Prompt", "s1 "+cwd, fake{}); out != "" {
		t.Fatalf("second prompt output = %q", out)
	}
	// Another session's prompt gets nothing.
	if out := run(t, root, "Prompt", "s2 "+cwd, fake{}); out != "" {
		t.Fatalf("other session output = %q", out)
	}
	if log := errorLog(t, root); log != "" {
		t.Fatal(log)
	}
}

func TestSessionStartDeliversAnswersInTheRecoveryNote(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	asked(t, root, cwd, "s1")
	// The session resumes under the same id.
	out := run(t, root, "SessionStart", "s1 "+cwd, fake{})
	if !strings.Contains(out, `Answered: "Which schema?" → "v2" (Robert).`) {
		t.Fatalf("recovery note = %q", out)
	}
	if got := delivered(t, root); !slices.Equal(got, []string{"q-1"}) {
		t.Fatalf("delivered = %v", got)
	}
	if out := run(t, root, "Prompt", "s1 "+cwd, fake{}); out != "" {
		t.Fatalf("prompt after the note repeated the answer: %q", out)
	}
}

func TestStopEnforcesHandoffOnceWhenEnabled(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T, enforce string) (string, string) {
		root, cwd := t.TempDir(), repo(t)
		run(t, root, "SessionStart", "s1 "+cwd, fake{})
		project := "name: demo\nkey: DM\nnext_id: 2\nrepos:\n  - " + cwd + "\n" + enforce
		if err := os.WriteFile(filepath.Join(root, "demo", "project.yaml"), []byte(project), 0o644); err != nil {
			t.Fatal(err)
		}
		writeTicket(t, root, "demo", "DM-1-card-panel", "---\nid: DM-1\nstatus: in-progress\ntype: feature\npriority: high\ncreated: 2026-10-01\nbranch: feature/demo\n---\n# Card panel\n")
		run(t, root, "Prompt", "s1 "+cwd, fake{})
		run(t, root, "Edit", "s1 "+cwd, fake{})
		return root, cwd
	}
	turnEnds := func(t *testing.T, root string) []bool {
		var blocked []bool
		for _, e := range readEvents(t, root, "demo") {
			if e.Kind == events.TurnEnd {
				var data events.TurnEndData
				_ = e.Decode(&data)
				blocked = append(blocked, data.BlockedForHandoff)
			}
		}
		return blocked
	}

	t.Run("blocks a dirty linked run once per turn", func(t *testing.T) {
		t.Parallel()
		root, cwd := setup(t, "settings:\n  enforce_handoff: true\n")
		out := run(t, root, "Stop", "s1 "+cwd, fake{})
		if out != "block:Flashheart: record a checkpoint on DM-1 (done, next, files) before stopping." {
			t.Fatalf("stop output = %q", out)
		}
		if out := run(t, root, "Stop", "s1 "+cwd, fake{}); out != "" {
			t.Fatalf("second stop in the same turn blocked again: %q", out)
		}
		if got := turnEnds(t, root); !slices.Equal(got, []bool{true, false}) {
			t.Fatalf("turn ends = %v", got)
		}
	})
	t.Run("the agent's own stop-hook flag lets it stop", func(t *testing.T) {
		t.Parallel()
		root, cwd := setup(t, "settings:\n  enforce_handoff: true\n")
		if out := run(t, root, "StopActive", "s1 "+cwd, fake{}); out != "" {
			t.Fatalf("stop output = %q", out)
		}
	})
	t.Run("a checkpoint clears it", func(t *testing.T) {
		t.Parallel()
		root, cwd := setup(t, "settings:\n  enforce_handoff: true\n")
		s, _ := store.Open(root)
		_ = events.New(s).Append(events.Event{Time: now, Run: "fake:s1", Agent: "fake", Kind: events.Checkpoint, Project: "demo", Data: events.CheckpointData{Ticket: "DM-1"}})
		s.Close()
		if out := run(t, root, "Stop", "s1 "+cwd, fake{}); out != "" {
			t.Fatalf("stop output = %q", out)
		}
	})
	t.Run("off by default", func(t *testing.T) {
		t.Parallel()
		root, cwd := setup(t, "")
		if out := run(t, root, "Stop", "s1 "+cwd, fake{}); out != "" {
			t.Fatalf("stop output = %q", out)
		}
	})
}

func TestReplyNeedsNoBoard(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "board")
	if out := run(t, root, "Stamp", "s1 /x", fake{}); out != "context:stamped fake:s1" {
		t.Fatalf("output = %q", out)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("stamping touched the root: %v", err)
	}
}
