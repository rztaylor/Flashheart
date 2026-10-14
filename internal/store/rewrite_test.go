package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

const rewriter = "claude:2ff6eb1c-9261-47a2-abdd-2233822a2b1b"

// rewrite is an agent's rewrite of AL-5's description and criteria.
func rewrite(t *testing.T, data []byte) Edit {
	t.Helper()
	description, err := board.NewAgentSectionChange(data, "Description", "The scope changed.")
	if err != nil {
		t.Fatal(err)
	}
	criteria, err := board.NewCriteriaEdit(2, []board.CriterionOp{{Kind: board.CriterionReword, Item: 0, Text: "First, reworded"}})
	if err != nil {
		t.Fatal(err)
	}
	return func(data []byte) ([]byte, error) {
		data, err := description.Apply(data)
		if err != nil {
			return nil, err
		}
		return criteria.Apply(data)
	}
}

// An agent's rewrite keeps the text it replaced in the ticket's files, in
// the same write, with a note linking it; the history names who changed
// which sections, and the kept text restores the ticket (FH-69, D71).
func TestApplyTicketEditKeepsTheReplacedText(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	before, hash, err := s.ReadTicket("alpha", "AL-5")
	if err != nil {
		t.Fatal(err)
	}
	var noted string
	next, err := s.As(rewriter).ApplyTicketEdit("alpha", "AL-5", Unchanged(hash), TicketEdit{
		Rewrite: rewrite(t, before),
		Note: func(kept string) string {
			noted = kept
			return "- 2026-10-05 · claude:2ff6eb1c: Rewrote Description; earlier text in [" + kept + "](" + kept + ")"
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	after, current, err := s.ReadTicket("alpha", "AL-5")
	if err != nil || current != next {
		t.Fatalf("hash %s, want %s (%v)", current, next, err)
	}
	folder := filepath.Join(root, "alpha", "tickets", "AL-5-column-overflow")
	if !strings.HasPrefix(noted, "files/") || !strings.HasSuffix(noted, "-al-5-earlier-text.md") {
		t.Fatalf("kept file = %q", noted)
	}
	kept, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(noted)))
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"# AL-5: text replaced by " + rewriter, "## Description\n\nFixture ticket.\n", "## Acceptance Criteria\n\n- [x] First criterion\n- [ ] Second criterion\n"} {
		if !strings.Contains(string(kept), part) {
			t.Errorf("kept text is missing %q:\n%s", part, kept)
		}
	}
	if strings.Contains(string(kept), "Created for tests") || strings.Contains(string(kept), "## Notes") {
		t.Errorf("kept text holds sections the rewrite did not replace:\n%s", kept)
	}
	index, err := os.ReadFile(filepath.Join(folder, "files", "index.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"file: " + strings.TrimPrefix(noted, "files/"), "caption: Text replaced by claude:2ff6eb1c (Description, Acceptance Criteria)", "kind: other", "run: " + rewriter, "sha256: "} {
		if !strings.Contains(string(index), part) {
			t.Errorf("files index is missing %q:\n%s", part, index)
		}
	}
	if !strings.Contains(string(after), "- [x] First, reworded\n- [ ] Second criterion") ||
		!strings.Contains(string(after), "earlier text in ["+noted+"]("+noted+")") {
		t.Errorf("ticket after the rewrite:\n%s", after)
	}
	history := activityOf(t, root, "tickets", "AL-5-column-overflow").History
	if len(history) != 1 || history[0].By != rewriter || len(history[0].Changes) != 1 ||
		strings.Join(history[0].Changes[0].Fields, ",") != "description,acceptance-criteria" {
		t.Errorf("history = %+v", history)
	}

	// A person restores a section from the kept text.
	section, ok := mdfile.FindSection(mdfile.Parse(kept).Body, "Description")
	if !ok {
		t.Fatal("no Description in the kept text")
	}
	restore, err := board.NewSectionChange("Description", section.Content)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EditTicket("alpha", "AL-5", Unchanged(next), board.FieldChanges{}, restore.Apply); err != nil {
		t.Fatal(err)
	}
	restored, _, _ := s.ReadTicket("alpha", "AL-5")
	if got, _ := mdfile.FindSection(mdfile.Parse(restored).Body, "Description"); strings.TrimSpace(got.Content) != "Fixture ticket." {
		t.Errorf("restored description = %q", got.Content)
	}
}

// A rewrite to the text the ticket already has writes nothing, and one
// that only adds a section keeps no file but is still noted.
func TestApplyTicketEditKeepsOnlyWhatItReplaced(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	data, hash, err := s.ReadTicket("alpha", "AL-5")
	if err != nil {
		t.Fatal(err)
	}
	same, err := board.NewAgentSectionChange(data, "Description", "Fixture ticket.")
	if err != nil {
		t.Fatal(err)
	}
	called := false
	next, err := s.As(rewriter).ApplyTicketEdit("alpha", "AL-5", Unchanged(hash), TicketEdit{
		Rewrite: same.Apply, Note: func(string) string { called = true; return "- note" },
	})
	if err != nil || next != hash || called {
		t.Fatalf("same text: hash changed %v, noted %v, %v", next != hash, called, err)
	}

	added, err := board.NewAgentSectionChange(data, "Test Plan", "A new plan.")
	if err != nil {
		t.Fatal(err)
	}
	kept := "unset"
	if _, err := s.As(rewriter).ApplyTicketEdit("alpha", "AL-5", Unchanged(hash), TicketEdit{
		Rewrite: added.Apply, Note: func(stored string) string { kept = stored; return "- 2026-10-05 · added a test plan" },
	}); err != nil {
		t.Fatal(err)
	}
	if kept != "" {
		t.Errorf("kept = %q, want none for an added section", kept)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "tickets", "AL-5-column-overflow", "files")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("files/ was written for an added section: %v", err)
	}
	after, _, _ := s.ReadTicket("alpha", "AL-5")
	if !strings.Contains(string(after), "## Test Plan\n\nA new plan.") || !strings.Contains(string(after), "- 2026-10-05 · added a test plan") {
		t.Errorf("ticket:\n%s", after)
	}
}

// A rewrite based on an older read is a conflict and writes nothing: not
// the ticket, the kept text or the files index.
func TestApplyTicketEditConflictWritesNothing(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	data, _, err := s.ReadTicket("alpha", "AL-5")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.As(rewriter).ApplyTicketEdit("alpha", "AL-5", Unchanged(Hash([]byte("older"))), TicketEdit{
		Rewrite: rewrite(t, data), Note: func(string) string { return "- note" },
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want a conflict", err)
	}
	after, _, _ := s.ReadTicket("alpha", "AL-5")
	if string(after) != string(data) {
		t.Error("the ticket changed")
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "tickets", "AL-5-column-overflow", "files")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("files/ was written: %v", err)
	}
}
