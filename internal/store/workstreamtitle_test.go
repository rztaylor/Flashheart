package store

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// titleTaken asserts err refuses a title that workstream other of alpha
// already has.
func titleTaken(t *testing.T, err error, other, title string) {
	t.Helper()
	var taken *WorkstreamTitleTakenError
	if !errors.As(err, &taken) {
		t.Fatalf("error = %v, want a *WorkstreamTitleTakenError", err)
	}
	if taken.Slug != other || taken.Title != title || taken.Project != "alpha" {
		t.Fatalf("taken = %+v, want %s %q", taken, other, title)
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error %v is not ErrInvalidInput", err)
	}
	if !strings.Contains(err.Error(), title) || !strings.Contains(err.Error(), "choose another title") {
		t.Fatalf("error text = %q", err)
	}
}

// TestCreateWorkstreamRefusesATakenTitle: a project's workstream titles
// are unique, compared ignoring case and spacing; a refused create writes
// nothing, another project may use the title, and a different title whose
// slug is taken still gets a fresh slug (FH-163, D76).
func TestCreateWorkstreamRefusesATakenTitle(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if _, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline mode"}); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	for _, tc := range []struct{ title, other, has string }{
		{"Offline mode", "offline-mode", "Offline mode"},
		{"  offline   MODE ", "offline-mode", "Offline mode"},
		{"board ui", "board-ui", "Board UI"},
	} {
		_, err := s.CreateWorkstream("alpha", NewWorkstream{Title: tc.title, Tickets: []string{"AL-5"}})
		titleTaken(t, err, tc.other, tc.has)
		if after := snapshot(t, root); !maps.Equal(before, after) {
			t.Fatalf("%q: a refused create wrote files", tc.title)
		}
	}
	if _, err := s.CreateWorkstream("beta", NewWorkstream{Title: "Offline mode"}); err != nil {
		t.Fatalf("same title in another project: %v", err)
	}
	again, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline mode!"})
	if err != nil || again.Slug != "offline-mode-2" {
		t.Fatalf("different title, same slug = %+v, %v", again, err)
	}
}

// TestEditWorkstreamRefusesATakenTitle: a new title another workstream
// has is refused and writes nothing, through the field edit and through a
// raw edit of the file; the workstream's own title in other case or
// spacing is not taken (FH-163, D76).
func TestEditWorkstreamRefusesATakenTitle(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if _, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline"}); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	_, err := s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{
		Changes: workstreamChanges(t, board.FieldChange{Key: "title", Value: "OFFLINE"}),
		Add:     []string{"AL-5"},
	})
	titleTaken(t, err, "offline", "Offline")
	_, err = s.UpdateWorkstream("alpha", "board-ui", Merge, func(data []byte) ([]byte, error) { return mdfile.SetTitle(data, "offline") })
	titleTaken(t, err, "offline", "Offline")
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Fatal("a refused edit wrote files")
	}

	if _, err := s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{
		Changes: workstreamChanges(t, board.FieldChange{Key: "title", Value: "Board  ui"}),
	}); err != nil {
		t.Fatalf("its own title in other case: %v", err)
	}
	if title := board.ParseWorkstream("board-ui", mustRead(t, filepath.Join(root, "alpha", "workstreams", "board-ui.md"))).Title; title != "Board ui" {
		t.Fatalf("title = %q", title)
	}
	if _, err := s.UpdateWorkstream("alpha", "board-ui", Merge, func(data []byte) ([]byte, error) { return mdfile.SetScalar(data, "priority", "low") }); err != nil {
		t.Fatalf("a raw edit keeping the title: %v", err)
	}
}

// TestDuplicateTitlesMadeEarlierStillLoadAndEdit: workstreams that already
// share a title (made before titles were unique, or by hand) load as they
// are, take every edit that keeps their title, and can be retitled apart;
// only a change to a taken title is refused (D76).
func TestDuplicateTitlesMadeEarlierStillLoadAndEdit(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	original := string(mustRead(t, filepath.Join(root, "alpha", "workstreams", "board-ui.md")))
	duplicate := strings.Replace(strings.Replace(original, "slug: board-ui", "slug: board-ui-2", 1), "  - AL-2\n  - AL-3\n  - AL-4\n", "", 1)
	if err := os.WriteFile(filepath.Join(root, "alpha", "workstreams", "board-ui-2.md"), []byte(duplicate), 0o644); err != nil {
		t.Fatal(err)
	}

	b, _, err := s.ReadBoard()
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, project := range b.Projects {
		if project.Name == "alpha" {
			for _, workstream := range project.Workstreams {
				titles = append(titles, workstream.Slug+"="+workstream.Title)
			}
		}
	}
	if slices.Sort(titles); strings.Join(titles, "|") != "board-ui-2=Board UI|board-ui=Board UI" {
		t.Fatalf("workstreams = %v", titles)
	}

	if _, err := s.EditWorkstream("alpha", "board-ui-2", Merge, WorkstreamEdit{
		Changes: workstreamChanges(t, board.FieldChange{Key: "priority", Value: "low"}),
		Add:     []string{"AL-5"},
	}); err != nil {
		t.Fatalf("an edit keeping the title: %v", err)
	}
	if _, err := s.EditWorkstream("alpha", "board-ui-2", Merge, WorkstreamEdit{
		Changes: workstreamChanges(t, board.FieldChange{Key: "title", Value: "Board ui"}),
	}); err != nil {
		t.Fatalf("its title in other case: %v", err)
	}
	if _, err := s.EditWorkstream("alpha", "board-ui-2", Merge, WorkstreamEdit{
		Changes: workstreamChanges(t, board.FieldChange{Key: "title", Value: "Card panel"}),
	}); err != nil {
		t.Fatalf("retitled apart: %v", err)
	}
	_, err = s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{
		Changes: workstreamChanges(t, board.FieldChange{Key: "title", Value: "card panel"}),
	})
	titleTaken(t, err, "board-ui-2", "Card panel")
}
