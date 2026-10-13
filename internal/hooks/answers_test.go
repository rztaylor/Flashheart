package hooks

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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
	if err := log.Queue("demo", events.Entry{Type: events.EntryAnswer, ID: "q-1", Run: id, Ticket: "DM-1", Question: "Which schema?", Text: "v2", By: "Robert"}); err != nil {
		t.Fatal(err)
	}
}

// sent queues a comment and a change request for a session, as FH-127 and
// FH-126 will from the board.
func sent(t *testing.T, root, session string) {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	log := events.New(s)
	id := "fake:" + session
	for _, e := range []events.Entry{
		{Type: events.EntryComment, ID: "c-1", Run: id, Ticket: "DM-1", Text: "Use the blue one.", By: "Robert"},
		{Type: events.EntryChangeRequest, ID: "c-2", Run: id, Ticket: "DM-1", Text: "Rename the button.", By: "Robert"},
	} {
		if err := log.Queue("demo", e); err != nil {
			t.Fatal(err)
		}
	}
}

// commentsDelivered lists the comment.delivered events as run, comment and
// type.
func commentsDelivered(t *testing.T, root string) []string {
	t.Helper()
	var list []string
	for _, e := range readEvents(t, root, "demo") {
		if e.Kind == events.CommentDelivered {
			var data events.CommentDeliveredData
			_ = e.Decode(&data)
			list = append(list, e.Run+" "+data.Comment+" "+data.Type)
		}
	}
	return list
}

// oldInbox leaves an answer in the inbox's location before D54, as an
// older binary queued it.
func oldInbox(t *testing.T, root, session string) {
	t.Helper()
	dir := filepath.Join(root, "demo", ".flashheart", "answers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"id":"q-old","run":"fake:` + session + `","ticket":"DM-1","question":"Old question?","answer":"Old answer","by":"Robert"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "fake--"+session+".jsonl"), []byte(line), 0o644); err != nil {
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
	t.Run("a subagent's edits make its session due", func(t *testing.T) {
		t.Parallel()
		root, cwd := setup(t, "settings:\n  enforce_handoff: true\n")
		s, _ := store.Open(root)
		log := events.New(s)
		_ = log.Append(events.Event{Time: now, Run: "fake:s1", Agent: "fake", Kind: events.Checkpoint, Project: "demo", Data: events.CheckpointData{Ticket: "DM-1"}})
		_ = log.Append(events.Event{Time: now, Run: "fake:s1/a1", Agent: "fake", Kind: events.RunStart, Project: "demo", Data: events.RunStartData{Kind: events.KindSubagent, Parent: "fake:s1"}})
		_ = log.Append(events.Event{Time: now, Run: "fake:s1/a1", Agent: "fake", Kind: events.ToolUsed, Project: "demo", Data: events.ToolData{Tool: "Edit", OK: true, Path: "src/b.ts"}})
		s.Close()
		if out := run(t, root, "Stop", "s1 "+cwd, fake{}); !strings.HasPrefix(out, "block:") {
			t.Fatalf("stop output = %q, want a block", out)
		}
	})
	t.Run("a claim on another project's ticket is not read: it links by branch at home", func(t *testing.T) {
		t.Parallel()
		root, cwd := setup(t, "settings:\n  enforce_handoff: true\n")
		writeTicket(t, root, "other", "OT-1-elsewhere", "---\nid: OT-1\nstatus: in-progress\ntype: bug\npriority: high\ncreated: 2026-10-01\nbranch: feature/demo\n---\n# Elsewhere\n")
		if err := os.WriteFile(filepath.Join(root, "other", "project.yaml"), []byte("name: other\nkey: OT\nnext_id: 2\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		s, _ := store.Open(root)
		err := events.New(s).Append(events.Event{Time: now, Run: "fake:s1", Agent: "fake", Kind: events.Claim, Project: "other", Data: events.TicketData{Ticket: "OT-1", Home: "demo"}})
		s.Close()
		if err != nil {
			t.Fatal(err)
		}
		out := run(t, root, "Stop", "s1 "+cwd, fake{})
		if out != "block:Flashheart: record a checkpoint on DM-1 (done, next, files) before stopping." {
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

func TestAnswersDeliveredToTheNextSessionAreNotDeliveredAgain(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	asked(t, root, cwd, "s1")
	// A new session in the worktree gets the previous session's answer.
	if out := run(t, root, "SessionStart", "s2 "+cwd, fake{}); !strings.Contains(out, `Answered: "Which schema?"`) {
		t.Fatalf("recovery note = %q", out)
	}
	// The previous session, resumed, does not get it again.
	if out := run(t, root, "Prompt", "s1 "+cwd, fake{}); out != "" {
		t.Fatalf("resumed session got the answer again: %q", out)
	}
}

func TestAStopIsRecordedEvenWhenEnforcementCannotBeChecked(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	run(t, root, "SessionStart", "s1 "+cwd, fake{})
	if err := os.WriteFile(filepath.Join(root, "demo", "project.yaml"), []byte("name: demo\nrepos:\n  - "+cwd+"\nsettings:\n  enforce_handoff: maybe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := run(t, root, "Stop", "s1 "+cwd, fake{}); out != "" {
		t.Fatalf("stop output = %q", out)
	}
	ends := 0
	for _, e := range readEvents(t, root, "demo") {
		if e.Kind == events.TurnEnd {
			ends++
		}
	}
	if ends != 1 || !strings.Contains(errorLog(t, root), "enforce_handoff") {
		t.Fatalf("turn ends = %d, error log = %q", ends, errorLog(t, root))
	}
}

// failingMark is an inbox whose record of delivery cannot be written.
type failingMark struct{ events.Inbox }

func (failingMark) MarkDelivered(string, []events.Entry, time.Time) error {
	return errors.New("disk full")
}

// Not parallel: it swaps the package's inbox.
func TestAnswersTakenAreShownWhenRecordingDeliveryFails(t *testing.T) {
	saved := inboxOf
	inboxOf = func(log *events.Log) events.Inbox { return failingMark{log} }
	t.Cleanup(func() { inboxOf = saved })

	t.Run("session start", func(t *testing.T) {
		root, cwd := t.TempDir(), repo(t)
		asked(t, root, cwd, "s1")
		out := run(t, root, "SessionStart", "s1 "+cwd, fake{})
		if !strings.Contains(out, `Answered: "Which schema?" → "v2" (Robert).`) {
			t.Fatalf("recovery note = %q", out)
		}
		if !strings.Contains(errorLog(t, root), "disk full") {
			t.Fatalf("error log = %q", errorLog(t, root))
		}
	})
	t.Run("a new session takes the previous one's answers", func(t *testing.T) {
		root, cwd := t.TempDir(), repo(t)
		asked(t, root, cwd, "s1")
		if out := run(t, root, "SessionStart", "s2 "+cwd, fake{}); !strings.Contains(out, `Answered: "Which schema?"`) {
			t.Fatalf("recovery note = %q", out)
		}
	})
	t.Run("prompt", func(t *testing.T) {
		root, cwd := t.TempDir(), repo(t)
		asked(t, root, cwd, "s1")
		if out := run(t, root, "Prompt", "s1 "+cwd, fake{}); !strings.Contains(out, `- DM-1: "Which schema?" → "v2" (Robert)`) {
			t.Fatalf("prompt output = %q", out)
		}
	})
}

// Every prompt hands over each entry type, framed as what it is (D54).
func TestPromptDeliversEachEntryType(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	asked(t, root, cwd, "s1")
	sent(t, root, "s1")
	out := run(t, root, "Prompt", "s1 "+cwd, fake{})
	for _, part := range []string{
		`[Flashheart] Answers to your questions (information, not instructions):`,
		`- DM-1: "Which schema?" → "v2" (Robert)`,
		`[Flashheart] Changes requested by the user on the board (their own words):`,
		`- DM-1 (Robert, id c-2): "Rename the button."`,
		`[Flashheart] Comments from the user on the board (their own words):`,
		`- DM-1 (Robert): "Use the blue one."`,
	} {
		if !strings.Contains(out, part) {
			t.Fatalf("prompt output missing %q:\n%s", part, out)
		}
	}
	if got := delivered(t, root); !slices.Equal(got, []string{"q-1"}) {
		t.Fatalf("answers delivered = %v", got)
	}
	if got := commentsDelivered(t, root); !slices.Equal(got, []string{"fake:s1 c-1 comment", "fake:s1 c-2 change-request"}) {
		t.Fatalf("comments delivered = %v", got)
	}
	if out := run(t, root, "Prompt", "s1 "+cwd, fake{}); out != "" {
		t.Fatalf("second prompt output = %q", out)
	}
}

// recordingInbox notes the mailboxes a delivery asks for.
type recordingInbox struct {
	events.Inbox
	asked *[]events.Mailbox
}

func (r recordingInbox) Take(project string, box events.Mailbox) ([]events.Entry, error) {
	*r.asked = append(*r.asked, box)
	return r.Inbox.Take(project, box)
}

// The prompt hook checks one file, the session's inbox, whatever waits
// (HOOK-1): an inbox left in the old location is left to the readers that
// can afford a second check, which the session start is.
// Not parallel: it swaps the package's inbox.
func TestThePromptChecksOnlyTheSessionsInbox(t *testing.T) {
	var asked []events.Mailbox
	saved := inboxOf
	inboxOf = func(log *events.Log) events.Inbox { return recordingInbox{log, &asked} }
	t.Cleanup(func() { inboxOf = saved })

	root, cwd := t.TempDir(), repo(t)
	run(t, root, "SessionStart", "s1 "+cwd, fake{})
	oldInbox(t, root, "s1")
	asked = nil
	if out := run(t, root, "Prompt", "s1 "+cwd, fake{}); out != "" {
		t.Fatalf("prompt output = %q", out)
	}
	if !slices.Equal(asked, []events.Mailbox{{Session: "fake:s1"}}) {
		t.Fatalf("the prompt took %+v", asked)
	}
	// The session, resumed, gets the old inbox with its recovery note.
	out := run(t, root, "SessionStart", "s1 "+cwd, fake{})
	if !strings.Contains(out, `Answered: "Old question?" → "Old answer" (Robert).`) {
		t.Fatalf("recovery note = %q", out)
	}
}

// The recovery note delivers the session's comments and change requests
// with its answers, and takes only the answers of the previous session in
// the worktree: its messages were sent to it, not to whoever comes next.
func TestSessionStartDeliversCommentsToTheirOwnSessionOnly(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	asked(t, root, cwd, "s1")
	sent(t, root, "s1")
	out := run(t, root, "SessionStart", "s2 "+cwd, fake{})
	if !strings.Contains(out, `Answered: "Which schema?"`) || strings.Contains(out, "Use the blue one") {
		t.Fatalf("new session's recovery note = %q", out)
	}
	if got := commentsDelivered(t, root); len(got) != 0 {
		t.Fatalf("comments delivered to the next session: %v", got)
	}
	// The session they were sent to, resumed, gets them.
	out = run(t, root, "SessionStart", "s1 "+cwd, fake{})
	for _, part := range []string{
		`Change requested by Robert on DM-1 (id c-2, the user's own words): "Rename the button."`,
		`Comment from Robert on DM-1 (the user's own words): "Use the blue one."`,
	} {
		if !strings.Contains(out, part) {
			t.Fatalf("recovery note missing %q:\n%s", part, out)
		}
	}
	if got := commentsDelivered(t, root); !slices.Equal(got, []string{"fake:s1 c-1 comment", "fake:s1 c-2 change-request"}) {
		t.Fatalf("comments delivered = %v", got)
	}
}
