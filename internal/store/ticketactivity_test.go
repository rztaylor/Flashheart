package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

const al4Folder = "AL-4-drag-and-drop"

func activityOf(t *testing.T, root, dir, folder string) board.Activity {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "alpha", dir, folder, board.ActivityFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return board.ParseActivity(data)
}

func TestATicketWithoutActivityReadsAsEmpty(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	activity, hash, err := s.ReadTicketActivity("alpha", al4Folder)
	if err != nil || hash != "" || len(activity.Comments)+len(activity.History)+activity.Unknown != 0 {
		t.Errorf("activity = %+v, %q, %v", activity, hash, err)
	}
	if _, _, err := s.ReadTicketActivity("alpha", "../beta"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("unsafe folder: %v", err)
	}
}

func TestCommentsAreAddedAndEditedBesideTheTicket(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	ticket := filepath.Join(root, "alpha", "tickets", al4Folder, al4Folder+".md")
	before, _ := os.ReadFile(ticket)
	statBefore, _ := os.Stat(ticket)
	robert := s.As("Robert")

	comment, hash, err := robert.AddComment("alpha", "AL-4", Merge, NewComment{Body: "Keep the place.\nsk-abcdefghijklmnopqrstuvwxyz0123456789", Quote: "Second criterion", QuoteFrom: "Acceptance Criteria 2"})
	if err != nil {
		t.Fatal(err)
	}
	if comment.ID == "" || comment.By != "Robert" || comment.State != board.CommentOpen || !comment.Time.Equal(fixedNow) || comment.QuoteFrom != "Acceptance Criteria 2" {
		t.Errorf("comment = %+v", comment)
	}
	if strings.Contains(comment.Body, "sk-abcdefghijklmnop") {
		t.Errorf("a secret was kept: %q", comment.Body)
	}
	second, hash2, err := robert.AddComment("alpha", "AL-4", Unchanged(hash), NewComment{Body: "Also Done."})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == comment.ID {
		t.Errorf("two comments share id %s", second.ID)
	}
	activity, read, err := s.ReadTicketActivity("alpha", al4Folder)
	if err != nil || read != hash2 || len(activity.Comments) != 2 || activity.Comments[0].Quote != "Second criterion" || len(activity.History) != 0 {
		t.Fatalf("activity = %+v, hash %q (want %q), %v", activity, read, hash2, err)
	}

	// The ticket file is never touched.
	after, _ := os.ReadFile(ticket)
	statAfter, _ := os.Stat(ticket)
	if string(after) != string(before) || !statAfter.ModTime().Equal(statBefore.ModTime()) {
		t.Error("adding a comment changed the ticket file")
	}

	later := fixedNow.Add(time.Hour)
	s.SetClock(func() time.Time { return later })
	edited, hash3, err := robert.EditComment("alpha", "AL-4", comment.ID, Unchanged(hash2), "Keep the place in every column.")
	if err != nil {
		t.Fatal(err)
	}
	if edited.Body != "Keep the place in every column." || !edited.Edited.Equal(later) || edited.Quote != "Second criterion" {
		t.Errorf("edited = %+v", edited)
	}
	if after, _ := os.ReadFile(ticket); string(after) != string(before) {
		t.Error("editing a comment changed the ticket file")
	}

	// A stale hash is refused and nothing is written.
	name := filepath.Join(root, "alpha", "tickets", al4Folder, board.ActivityFile)
	current, _ := os.ReadFile(name)
	var conflict *ConflictError
	if _, _, err := robert.EditComment("alpha", "AL-4", second.ID, Unchanged(hash2), "Stale."); !errors.As(err, &conflict) || conflict.Hash != hash3 {
		t.Errorf("stale edit: %v", err)
	}
	if _, _, err := robert.AddComment("alpha", "AL-4", Unchanged(hash2), NewComment{Body: "Stale."}); !errors.As(err, &conflict) {
		t.Errorf("stale add: %v", err)
	}
	if unchanged, _ := os.ReadFile(name); string(unchanged) != string(current) {
		t.Error("a refused write changed activity.md")
	}

	for name, call := range map[string]func() error{
		"missing comment": func() error {
			_, _, err := robert.EditComment("alpha", "AL-4", "deadbeef", Unchanged(hash3), "x")
			return err
		},
		"empty comment": func() error {
			_, _, err := robert.AddComment("alpha", "AL-4", Merge, NewComment{Body: "  "})
			return err
		},
		"no author": func() error {
			_, _, err := s.AddComment("alpha", "AL-4", Merge, NewComment{Body: "Who?"})
			return err
		},
		"an agent": func() error {
			_, _, err := s.As("claude:5b0c7e2a").AddComment("alpha", "AL-4", Merge, NewComment{Body: "Agents keep notes."})
			return err
		},
		"no hash": func() error {
			_, _, err := robert.EditComment("alpha", "AL-4", comment.ID, Precondition{}, "x")
			return err
		},
		"missing ticket": func() error {
			_, _, err := robert.AddComment("alpha", "AL-99", Merge, NewComment{Body: "x"})
			return err
		},
	} {
		err := call()
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
		if name == "missing comment" || name == "missing ticket" {
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("%s: %v, want ErrNotFound", name, err)
			}
		} else if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: %v, want ErrInvalidInput", name, err)
		}
	}
	if unchanged, _ := os.ReadFile(name); string(unchanged) != string(current) {
		t.Error("a refused write changed activity.md")
	}
}

func TestAFullActivityFileRefusesCommentsAndSkipsHistory(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	name := filepath.Join(root, "alpha", "tickets", al4Folder, board.ActivityFile)
	full := "# Activity: AL-4\n\n" + strings.Repeat("x", MaxFileBytes-100) + "\n"
	write(t, name, full)
	if _, _, err := s.As("Robert").AddComment("alpha", "AL-4", Merge, NewComment{Body: strings.Repeat("y", 200)}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("comment on a full file: %v", err)
	}
	if _, err := s.As("Robert").UpdateTicket("alpha", "AL-4", Merge, setStatus("in-progress")); err != nil {
		t.Errorf("a ticket write with a full activity file: %v", err)
	}
	if data, _ := os.ReadFile(name); string(data) != full {
		t.Error("the full activity file changed")
	}
}

func TestAHistoryThatCannotBeWrittenFailsTheWrite(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	ticket := filepath.Join(root, "alpha", "tickets", al4Folder, al4Folder+".md")
	before, _ := os.ReadFile(ticket)
	// A directory where activity.md belongs cannot be read or replaced.
	if err := os.Mkdir(filepath.Join(root, "alpha", "tickets", al4Folder, board.ActivityFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := s.As("Robert").UpdateTicket("alpha", "AL-4", Merge, setStatus("in-progress")); err == nil {
		t.Fatal("the write succeeded without its history")
	}
	if after, _ := os.ReadFile(ticket); string(after) != string(before) {
		t.Error("the ticket was written without its history")
	}
}

// Every ticket write a person or an agent makes through the store appends
// exactly one history entry naming them, what changed and when, never the
// ticket's text; the plain store (serve's own link copies, migrate) and
// writes that change only notes or the handoff record none.
func TestEveryTicketWriteAppendsOneHistoryEntry(t *testing.T) {
	t.Parallel()

	tick := func(data []byte) ([]byte, error) {
		return mdfile.SetCheckbox(data, board.SectionCriteria, 1, true)
	}
	note := func(data []byte) ([]byte, error) {
		return mdfile.AppendToSection(data, board.SectionNotes, board.NoteLine(fixedNow, "", "Secret note"))
	}
	cases := []struct {
		name   string
		by     string
		write  func(s *Store) error
		dir    string
		folder string
		want   []board.Change
	}{
		{"move", "Robert", func(s *Store) error {
			_, err := s.UpdateTicket("alpha", "AL-4", Merge, setStatus("in-progress"))
			return err
		}, "tickets", al4Folder, []board.Change{{What: board.ChangeMoved, From: "up-next", To: "in-progress"}}},
		{"place in another column", "Robert", func(s *Store) error {
			_, _, err := s.PlaceTicket("alpha", "AL-4", Merge, "", setStatus("backlog"))
			return err
		}, "tickets", al4Folder, []board.Change{{What: board.ChangeMoved, From: "up-next", To: "backlog"}}},
		{"fields, a tick and a note in one write", "claude:5b0c7e2a", func(s *Store) error {
			changes, err := board.NewFieldChanges([]board.FieldChange{{Key: "priority", Value: "high"}, {Key: "title", Value: "Secret title"}})
			if err != nil {
				return err
			}
			_, err = s.EditTicket("alpha", "AL-4", Merge, changes, func(data []byte) ([]byte, error) {
				data, err := tick(data)
				if err != nil {
					return nil, err
				}
				return note(data)
			})
			return err
		}, "tickets", al4Folder, []board.Change{{What: board.ChangeEdited, Fields: []string{"priority", "title"}}, {What: board.ChangeTicked, Criteria: []int{2}}}},
		{"workstream", "claude:5b0c7e2a", func(s *Store) error {
			changes, err := board.NewFieldChanges([]board.FieldChange{{Key: "workstream", Value: ""}})
			if err != nil {
				return err
			}
			_, err = s.EditTicket("alpha", "AL-4", Merge, changes, nil)
			return err
		}, "tickets", al4Folder, []board.Change{{What: board.ChangeEdited, Fields: []string{"workstream"}}}},
		{"raw edit", "Robert", func(s *Store) error {
			_, err := s.UpdateTicket("alpha", "AL-4", Merge, func(data []byte) ([]byte, error) {
				return []byte(strings.Replace(string(data), "Fixture ticket.", "Secret description.", 1)), nil
			})
			return err
		}, "tickets", al4Folder, []board.Change{{What: board.ChangeEdited, Fields: []string{"description"}}}},
		{"create", "claude:5b0c7e2a", func(s *Store) error {
			_, err := s.CreateTicket("alpha", NewTicket{Title: "Secret ticket", Status: board.UpNext})
			return err
		}, "tickets", "AL-9-secret-ticket", []board.Change{{What: board.ChangeCreated, To: "up-next"}}},
		{"workstream created with the ticket", "claude:5b0c7e2a", func(s *Store) error {
			_, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline", Goal: "Work offline", Tickets: []string{"AL-6"}})
			return err
		}, "tickets", "AL-6-offline-mode", []board.Change{{What: board.ChangeEdited, Fields: []string{"workstream"}}}},
		{"review step", "Robert", func(s *Store) error {
			_, err := s.UpdateReview("alpha", "AL-2", Merge, func(data []byte) ([]byte, error) {
				next, err := board.SetReviewStep(string(data), 1, true)
				return []byte(next), err
			})
			return err
		}, "tickets", "AL-2-board-columns", []board.Change{{What: board.ChangeTicked, Steps: []int{2}}}},
		{"archive", "Robert", func(s *Store) error { return s.Archive("alpha", "AL-4") },
			".archive/tickets", al4Folder, []board.Change{{What: board.ChangeArchived}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, root := writable(t)
			if err := tc.write(s.As(tc.by)); err != nil {
				t.Fatal(err)
			}
			activity := activityOf(t, root, tc.dir, tc.folder)
			if len(activity.History) != 1 {
				t.Fatalf("history = %+v, want one entry", activity.History)
			}
			entry := activity.History[0]
			if entry.By != tc.by || !entry.Time.Equal(fixedNow) || entry.ID == "" || !slices.EqualFunc(entry.Changes, tc.want, sameChange) {
				t.Errorf("entry = %+v, want by %s changes %+v", entry, tc.by, tc.want)
			}
			data, _ := os.ReadFile(filepath.Join(root, "alpha", tc.dir, tc.folder, board.ActivityFile))
			if strings.Contains(string(data), "Secret") {
				t.Errorf("history holds ticket text:\n%s", data)
			}
			// Exactly one: no other ticket gained history.
			total := 0
			for _, dir := range []string{"tickets", ".archive/tickets"} {
				folders, _ := os.ReadDir(filepath.Join(root, "alpha", dir))
				for _, folder := range folders {
					total += len(activityOf(t, root, dir, folder.Name()).History)
				}
			}
			if total != 1 {
				t.Errorf("%d history entries across the project, want 1", total)
			}

			// The plain store writes the same change without history.
			plain, plainRoot := writable(t)
			if err := tc.write(plain); err != nil {
				t.Fatal(err)
			}
			if history := activityOf(t, plainRoot, tc.dir, tc.folder).History; len(history) != 0 {
				t.Errorf("the plain store wrote history %+v", history)
			}
		})
	}
}

func sameChange(a, b board.Change) bool {
	return a.What == b.What && a.From == b.From && a.To == b.To &&
		slices.Equal(a.Fields, b.Fields) && slices.Equal(a.Criteria, b.Criteria) && slices.Equal(a.Steps, b.Steps) &&
		slices.Equal(a.Comments, b.Comments)
}

func TestWritesThatChangeNoHistoryAppendNone(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	robert := s.As("Robert")
	steps := []func() error{
		func() error { // a note
			_, err := robert.UpdateTicket("alpha", "AL-4", Merge, func(data []byte) ([]byte, error) {
				return mdfile.AppendToSection(data, board.SectionNotes, "- 2026-10-05 · noted")
			})
			return err
		},
		func() error { // the handoff
			_, err := robert.UpdateTicket("alpha", "AL-4", Merge, func(data []byte) ([]byte, error) {
				return mdfile.ReplaceSection(data, board.SectionHandoff, "_Updated 2026-10-05._")
			})
			return err
		},
		func() error { // a move to the column it is in
			_, err := robert.UpdateTicket("alpha", "AL-4", Merge, setStatus("up-next"))
			return err
		},
		func() error { // a reorder within its column
			_, _, err := robert.PlaceTicket("alpha", "AL-4", Merge, "", setStatus("up-next"))
			return err
		},
	}
	for index, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", index, err)
		}
	}
	if history := activityOf(t, root, "tickets", al4Folder).History; len(history) != 0 {
		t.Errorf("history = %+v", history)
	}
}

func TestRestoreAppendsHistoryInTheLiveFolder(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	robert := s.As("Robert")
	if err := robert.Archive("alpha", "AL-6"); err != nil {
		t.Fatal(err)
	}
	if err := robert.Unarchive("alpha", "AL-6"); err != nil {
		t.Fatal(err)
	}
	history := activityOf(t, root, "tickets", "AL-6-offline-mode").History
	if len(history) != 2 || history[0].Changes[0].What != board.ChangeArchived || history[1].Changes[0].What != board.ChangeRestored {
		t.Errorf("history = %+v", history)
	}
	// The archive's own folder is gone; activity.md went with the ticket.
	if _, err := os.Stat(filepath.Join(root, "alpha", ".archive", "tickets", "AL-6-offline-mode")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("archived folder: %v", err)
	}
}

func TestPermanentDeleteListsAndRemovesTheActivityFile(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	robert := s.As("Robert")
	if _, _, err := robert.AddComment("alpha", "AL-6", Merge, NewComment{Body: "Not needed."}); err != nil {
		t.Fatal(err)
	}
	if err := robert.Archive("alpha", "AL-6"); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PlanDelete("alpha", "AL-6")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(plan.Files, board.ActivityFile) {
		t.Errorf("the delete does not list activity.md: %+v", plan.Files)
	}
	if _, err := s.DeleteArchived("alpha", "AL-6", plan.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", ".archive", "tickets", "AL-6-offline-mode")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("folder after delete: %v", err)
	}
}

func TestConcurrentCommentAndHistoryWritersAllLand(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	outputs := runWriters(t, "activity",
		[]string{root, "comment", "One."}, []string{root, "comment", "Two."}, []string{root, "move", "in-progress"})
	if !slices.Equal(outputs, []string{"ok", "ok", "ok"}) {
		t.Fatalf("writers = %v", outputs)
	}
	data, _ := os.ReadFile(filepath.Join(root, "alpha", "tickets", al4Folder, board.ActivityFile))
	activity := board.ParseActivity(data)
	var bodies []string
	for _, comment := range activity.Comments {
		bodies = append(bodies, comment.Body)
	}
	slices.Sort(bodies)
	if !slices.Equal(bodies, []string{"One.", "Two."}) || len(activity.History) != 1 || activity.Unknown != 0 {
		t.Errorf("activity = %+v\n%s", activity, data)
	}
}

func TestConcurrentEditsOfOneCommentOneWins(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	s := open(t, root).As("Robert")
	comment, hash, err := s.AddComment("alpha", "AL-4", Merge, NewComment{Body: "First."})
	if err != nil {
		t.Fatal(err)
	}
	outputs := runWriters(t, "activity",
		[]string{root, "edit", comment.ID, hash, "Edit one."}, []string{root, "edit", comment.ID, hash, "Edit two."})
	if !slices.Equal(outputs, []string{"conflict", "ok"}) {
		t.Errorf("writers = %v, want one ok and one conflict", outputs)
	}
}
