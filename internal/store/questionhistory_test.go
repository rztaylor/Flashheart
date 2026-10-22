package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// A question about a ticket and what became of it are history in its
// activity file (FH-190), written once per question and outcome, at the
// time given, as the store's author; the ticket file is never touched.
func TestRecordQuestionAppendsHistoryOnce(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	ticket := filepath.Join(root, "alpha", "tickets", al4Folder, al4Folder+".md")
	before, _ := os.ReadFile(ticket)
	run := s.As("claude:5b0c7e2a-1f3d")
	asked := fixedNow.Add(-time.Hour)
	question := board.QuestionRecord{ID: "q-ABCDEFGHIJKLMNOP", Kind: "decision", Text: "Use the known objectives? sk-abcdefghijklmnopqrstuvwxyz0123456789", Options: []string{"Yes", "No"}}

	wrote, err := run.RecordQuestion("alpha", "AL-4", asked, board.Change{What: board.ChangeAsked, Question: question})
	if err != nil || !wrote {
		t.Fatalf("record = %v, %v", wrote, err)
	}
	wrote, err = run.RecordQuestion("alpha", "AL-4", fixedNow, board.Change{What: board.ChangeAsked, Question: question})
	if err != nil || wrote {
		t.Fatalf("recording it again = %v, %v", wrote, err)
	}
	if _, err := run.RecordQuestion("alpha", "AL-4", time.Time{}, board.Change{What: board.ChangeAnsweredInSession, Question: board.QuestionRecord{ID: question.ID, Text: "Use the known objectives?"}}); err != nil {
		t.Fatal(err)
	}

	history := activityOf(t, root, "tickets", al4Folder).History
	if len(history) != 2 {
		t.Fatalf("history = %+v", history)
	}
	first, second := history[0], history[1]
	if first.By != "claude:5b0c7e2a-1f3d" || !first.Time.Equal(asked) || first.Changes[0].What != board.ChangeAsked ||
		first.Changes[0].Question.ID != question.ID || len(first.Changes[0].Question.Options) != 2 {
		t.Errorf("asked = %+v", first)
	}
	if strings.Contains(first.Changes[0].Question.Text, "sk-abcdefghijklmnop") {
		t.Errorf("a secret was kept: %q", first.Changes[0].Question.Text)
	}
	if !second.Time.Equal(fixedNow) || second.Changes[0].What != board.ChangeAnsweredInSession {
		t.Errorf("answered in session = %+v", second)
	}
	if after, _ := os.ReadFile(ticket); string(after) != string(before) {
		t.Error("recording a question changed the ticket file")
	}
}

func TestRecordQuestionRefusesWhatItCannotRecord(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	change := board.Change{What: board.ChangeAsked, Question: board.QuestionRecord{ID: "q-1", Text: "?"}}
	if _, err := s.RecordQuestion("alpha", "AL-4", fixedNow, change); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("a store writing as no one: %v", err)
	}
	run := s.As("claude:s1")
	for name, bad := range map[string]board.Change{
		"not a question change": {What: board.ChangeMoved, From: "backlog", To: "up-next"},
		"no id":                 {What: board.ChangeAsked, Question: board.QuestionRecord{Text: "?"}},
	} {
		if _, err := run.RecordQuestion("alpha", "AL-4", fixedNow, bad); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := run.RecordQuestion("alpha", "AL-404", fixedNow, change); !errors.Is(err, ErrNotFound) {
		t.Errorf("a missing ticket: %v", err)
	}
	if history := activityOf(t, root, "tickets", al4Folder).History; len(history) != 0 {
		t.Errorf("history = %+v", history)
	}
}

// A board answer's note and its history are one write (FH-190): the
// ticket's ## Notes gains the bullet and its activity the answered entry,
// by the person; a second write of the same answer adds no second entry.
func TestUpdateTicketNotingWritesTheAnswerWithItsNote(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	robert := s.As("Robert")
	answered := board.Change{What: board.ChangeAnswered, Question: board.QuestionRecord{ID: "q-1", Text: "Which port?", Answer: "8080"}}
	note := func(data []byte) ([]byte, error) {
		return mdfile.AppendToSection(data, board.SectionNotes, board.NoteLine(fixedNow, "", "Answered."))
	}
	if _, err := robert.UpdateTicketNoting("alpha", "AL-4", Merge, note, answered); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "alpha", "tickets", al4Folder, al4Folder+".md"))
	if !strings.Contains(string(data), "Answered.") {
		t.Errorf("ticket =\n%s", data)
	}
	history := activityOf(t, root, "tickets", al4Folder).History
	if len(history) != 1 || history[0].By != "Robert" || len(history[0].Changes) != 1 || history[0].Changes[0].Question.Answer != "8080" {
		t.Fatalf("history = %+v", history)
	}
	if _, err := robert.UpdateTicketNoting("alpha", "AL-4", Merge, nil, answered); err != nil {
		t.Fatal(err)
	}
	if history := activityOf(t, root, "tickets", al4Folder).History; len(history) != 1 {
		t.Errorf("the answer was recorded twice: %+v", history)
	}
}
