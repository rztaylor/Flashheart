package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

func TestReadArchivedListsArchivedTickets(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	if err := s.Archive("alpha", "AL-2"); err != nil {
		t.Fatal(err)
	}
	archived, err := s.ReadArchived("alpha")
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(archived))
	for _, item := range archived {
		ids = append(ids, item.Ticket.ID)
	}
	if !slices.Equal(ids, []string{"AL-2", "AL-8"}) {
		t.Fatalf("archived = %v", ids)
	}
	al2 := archived[0]
	if al2.Ticket.Column != board.Review || !al2.Review || al2.Ticket.Title == "" {
		t.Errorf("AL-2 = %+v", al2)
	}
	// Archiving stamps updated, which dates the archive.
	if al2.Ticket.Updated != "2026-10-05T14:12:09Z" {
		t.Errorf("updated = %q", al2.Ticket.Updated)
	}
}

func TestDeleteArchivedRemovesTheFolderAndEveryReference(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if err := s.Archive("alpha", "AL-3"); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PlanDelete("alpha", "AL-3")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(plan.Files, "AL-3-card-panel.md") || plan.Token == "" {
		t.Errorf("plan files = %v token = %q", plan.Files, plan.Token)
	}
	// AL-4 depends on AL-3; BE-1 refers to it.
	if len(plan.Tickets) != 2 || plan.Tickets[0].ID != "AL-4" || plan.Tickets[1].Project != "beta" || plan.Tickets[1].ID != "BE-1" {
		t.Errorf("plan tickets = %+v", plan.Tickets)
	}
	if !slices.Equal(plan.Workstreams, []string{"board-ui"}) {
		t.Errorf("plan workstreams = %v", plan.Workstreams)
	}

	if _, err := s.DeleteArchived("alpha", "AL-3", plan.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", ".archive", "tickets", "AL-3-card-panel")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("folder still there: %v", err)
	}
	be1, _ := os.ReadFile(filepath.Join(root, "beta", "tickets", "BE-1-hello", "BE-1-hello.md"))
	if list := mdfile.Parse(be1).List("depends-on"); len(list) != 0 {
		t.Errorf("BE-1 depends-on = %v", list)
	}
	workstream, _ := os.ReadFile(filepath.Join(root, "alpha", "workstreams", "board-ui.md"))
	if list := mdfile.Parse(workstream).List("tickets"); !slices.Equal(list, []string{"AL-2", "AL-4"}) {
		t.Errorf("board-ui tickets = %v", list)
	}
	project, _ := os.ReadFile(filepath.Join(root, "alpha", "project.yaml"))
	if !strings.Contains(string(project), "retired: [AL-3]") {
		t.Errorf("project.yaml =\n%s", project)
	}

	// Nothing is blocked by the deleted id any more.
	b, _, err := s.ReadBoard()
	if err != nil {
		t.Fatal(err)
	}
	analysis := board.Analyze(b)
	for ref, reasons := range analysis.Blocked {
		for _, reason := range reasons {
			if strings.Contains(reason.Describe(), "AL-3") {
				t.Errorf("%v is still blocked by AL-3: %s", ref, reason.Describe())
			}
		}
	}
}

func TestDeletedIDsAreNeverReused(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	// AL-8 is the highest number, archived in the sample.
	plan, err := s.PlanDelete("alpha", "AL-8")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteArchived("alpha", "AL-8", plan.Token); err != nil {
		t.Fatal(err)
	}
	// Lose next_id: the retired id still counts.
	name := filepath.Join(root, "alpha", "project.yaml")
	data, _ := os.ReadFile(name)
	if err := os.WriteFile(name, []byte(strings.Replace(string(data), "next_id: 9\n", "", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	project, err := s.ReadProject("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if project.NextID != 9 {
		t.Errorf("next id = %d, want 9", project.NextID)
	}
	created, err := s.CreateTicket("alpha", NewTicket{Title: "After a delete", Type: "feature", Priority: "low"})
	if err != nil || created.ID != "AL-9" {
		t.Errorf("created %q, %v", created.ID, err)
	}
}

func TestDeleteArchivedRefusals(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	// A live ticket cannot be deleted, nor an unknown one.
	if _, err := s.PlanDelete("alpha", "AL-3"); !errors.Is(err, ErrNotFound) {
		t.Errorf("plan live ticket: %v", err)
	}
	if _, err := s.DeleteArchived("alpha", "AL-3", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete live ticket: %v", err)
	}
	if _, err := s.DeleteArchived("alpha", "AL-404", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete unknown ticket: %v", err)
	}
	if _, err := s.DeleteArchived("alpha", "../tickets", "x"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("delete a path: %v", err)
	}

	// A symlinked archive folder is refused and its target left alone.
	link := filepath.Join(root, "alpha", ".archive", "tickets", "AL-20-link")
	if err := os.Symlink(filepath.Join(root, "alpha", "tickets", "AL-3-card-panel"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PlanDelete("alpha", "AL-20"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("plan symlink: %v", err)
	}
	if _, err := s.DeleteArchived("alpha", "AL-20", "x"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("delete symlink: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "tickets", "AL-3-card-panel", "AL-3-card-panel.md")); err != nil {
		t.Errorf("symlink target damaged: %v", err)
	}

	// A referencing ticket edited since the plan makes the token stale:
	// nothing is deleted or rewritten.
	if err := s.Archive("alpha", "AL-3"); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PlanDelete("alpha", "AL-3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateTicket("beta", "BE-1", "", setStatus("in-progress")); err != nil {
		t.Fatal(err)
	}
	_, err = s.DeleteArchived("alpha", "AL-3", plan.Token)
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("stale token: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", ".archive", "tickets", "AL-3-card-panel")); err != nil {
		t.Errorf("folder removed despite the conflict: %v", err)
	}
	be1, _ := os.ReadFile(filepath.Join(root, "beta", "tickets", "BE-1-hello", "BE-1-hello.md"))
	if !strings.Contains(string(be1), "depends-on: [AL-3]") {
		t.Errorf("BE-1 rewritten despite the conflict:\n%s", be1)
	}
}
