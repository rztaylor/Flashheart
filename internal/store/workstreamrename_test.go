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

// renameFixture gives the sample board references to board-ui of every
// kind: members (AL-2, AL-3, AL-4), another workstream that waits for it
// (offline), a live ticket that waits for it (AL-5) and an archived
// ticket that joined it and waits for it (AL-8).
func renameFixture(t *testing.T) (*Store, string) {
	t.Helper()
	s, root := writable(t)
	if _, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline", Tickets: []string{"AL-6"}, DependsOnWorkstreams: []string{"board-ui"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateTicket("alpha", "AL-5", Merge, func(data []byte) ([]byte, error) {
		return mdfile.SetList(data, "depends-on-workstreams", []string{"board-ui", "offline"})
	}); err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(root, "alpha", ".archive", "tickets", "AL-8-old-idea", "AL-8-old-idea.md")
	data := strings.Replace(string(mustRead(t, archived)), "created: 2026-09-01\n", "created: 2026-09-01\nworkstream: board-ui\ndepends-on-workstreams: [board-ui]\n", 1)
	if err := os.WriteFile(archived, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return s, root
}

// TestEditWorkstreamRenamesItsSlug: a new slug renames the file and
// rewrites every reference to it, members' workstream fields and every
// depends-on-workstreams, live and archived, in the same write as the
// edit's fields and tickets (FH-158).
func TestEditWorkstreamRenamesItsSlug(t *testing.T) {
	t.Parallel()

	s, root := renameFixture(t)
	_, hash, err := s.ReadWorkstream("alpha", "board-ui")
	if err != nil {
		t.Fatal(err)
	}
	edited, err := s.EditWorkstream("alpha", "board-ui", Unchanged(hash), WorkstreamEdit{
		Changes: workstreamChanges(t, board.FieldChange{Key: "title", Value: "Board"}),
		Add:     []string{"AL-1"},
		Rename:  "board",
	})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Slug != "board" {
		t.Errorf("slug = %q, want board", edited.Slug)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "workstreams", "board-ui.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the old file is still there: %v", err)
	}
	data := mustRead(t, filepath.Join(root, "alpha", "workstreams", "board.md"))
	if Hash(data) != edited.Hash {
		t.Error("returned hash does not match the renamed file")
	}
	doc := mdfile.Parse(data)
	if slug, _ := doc.String("slug"); slug != "board" {
		t.Errorf("slug field = %q", slug)
	}
	if !strings.Contains(string(data), "\n# Board\n") {
		t.Errorf("title not set:\n%s", data)
	}
	if got := doc.List("tickets"); !slices.Equal(got, []string{"AL-2", "AL-3", "AL-4", "AL-1"}) {
		t.Errorf("tickets = %v", got)
	}
	for _, id := range []string{"AL-1", "AL-2", "AL-3", "AL-4"} {
		if got := workstreamField(t, s, id); got != "board" {
			t.Errorf("%s workstream = %q, want board", id, got)
		}
		noMembershipWarnings(t, s, id)
	}
	if got := mdfile.Parse(mustRead(t, filepath.Join(root, "alpha", "workstreams", "offline.md"))).List("depends-on-workstreams"); !slices.Equal(got, []string{"board"}) {
		t.Errorf("offline depends-on-workstreams = %v", got)
	}
	al5, _, _ := s.ReadTicket("alpha", "AL-5")
	if got := mdfile.Parse(al5).List("depends-on-workstreams"); !slices.Equal(got, []string{"board", "offline"}) {
		t.Errorf("AL-5 depends-on-workstreams = %v", got)
	}
	archived := mdfile.Parse(mustRead(t, filepath.Join(root, "alpha", ".archive", "tickets", "AL-8-old-idea", "AL-8-old-idea.md")))
	if got, _ := archived.String("workstream"); got != "board" {
		t.Errorf("archived AL-8 workstream = %q", got)
	}
	if got := archived.List("depends-on-workstreams"); !slices.Equal(got, []string{"board"}) {
		t.Errorf("archived AL-8 depends-on-workstreams = %v", got)
	}
	// A ticket that named neither is left alone.
	if got := workstreamField(t, s, "AL-6"); got != "offline" {
		t.Errorf("AL-6 workstream = %q", got)
	}
	rewritten := map[string]string{}
	for _, ticket := range edited.Rewritten {
		rewritten[ticket.ID] = strings.Join(ticket.Fields, ",")
	}
	if want := map[string]string{
		"AL-1": "workstream", "AL-2": "workstream", "AL-3": "workstream", "AL-4": "workstream",
		"AL-5": "depends-on-workstreams", "AL-8": "workstream,depends-on-workstreams",
	}; !maps.Equal(rewritten, want) {
		t.Errorf("rewritten = %v, want %v", rewritten, want)
	}
	b, _, err := s.ReadBoard()
	if err != nil {
		t.Fatal(err)
	}
	analysis := board.Analyze(b)
	var reasons []string
	for _, reason := range analysis.Blocked[board.Ref{Project: "alpha", ID: "AL-5"}] {
		reasons = append(reasons, reason.Describe())
	}
	for _, reason := range reasons {
		if strings.Contains(reason, "board-ui") {
			t.Errorf("AL-5 still blocked by the old slug: %v", reasons)
		}
	}
}

// TestEditWorkstreamRenameAlone: a rename with nothing else changes only
// the slug; renaming to the current slug writes nothing.
func TestEditWorkstreamRenameAlone(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	before := string(mustRead(t, filepath.Join(root, "alpha", "workstreams", "board-ui.md")))
	edited, err := s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{Rename: "ui"})
	if err != nil {
		t.Fatal(err)
	}
	after := string(mustRead(t, filepath.Join(root, "alpha", "workstreams", "ui.md")))
	if want := strings.Replace(before, "slug: board-ui\n", "slug: ui\n", 1); after != want {
		t.Fatalf("renamed file:\n%s\nwant:\n%s", after, want)
	}
	if edited.Slug != "ui" || edited.Hash != Hash([]byte(after)) {
		t.Errorf("edited = %+v", edited)
	}

	unchanged := snapshot(t, root)
	same, err := s.EditWorkstream("alpha", "ui", Merge, WorkstreamEdit{Rename: "ui"})
	if err != nil || same.Slug != "ui" || len(same.Rewritten) != 0 {
		t.Fatalf("same slug = %+v, %v", same, err)
	}
	if !maps.Equal(unchanged, snapshot(t, root)) {
		t.Fatal("renaming to the current slug wrote files")
	}
}

// TestEditWorkstreamRenameRefusesAndWritesNothing: a slug already taken,
// an invalid slug and a stale precondition each refuse the rename, with no
// file changed (FH-158).
func TestEditWorkstreamRenameRefusesAndWritesNothing(t *testing.T) {
	t.Parallel()

	s, root := renameFixture(t)
	_, hash, err := s.ReadWorkstream("alpha", "board-ui")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateWorkstream("alpha", "board-ui", Merge, func(data []byte) ([]byte, error) { return mdfile.SetScalar(data, "priority", "low") }); err != nil {
		t.Fatal(err)
	}
	// A file that is not a workstream still takes its name.
	if err := os.WriteFile(filepath.Join(root, "alpha", "workstreams", "notes.md"), []byte("not a workstream\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	for name, tc := range map[string]struct {
		pre    Precondition
		rename string
		is     error
	}{
		"taken":          {Merge, "offline", ErrExists},
		"taken by file":  {Merge, "notes", ErrExists},
		"upper case":     {Merge, "Board", ErrInvalidName},
		"path":           {Merge, "../board", ErrInvalidName},
		"spaces":         {Merge, "board ui", ErrInvalidName},
		"stale":          {Unchanged(hash), "board", ErrConflict},
		"too long":       {Merge, strings.Repeat("a", 101), ErrInvalidName},
		"leading hyphen": {Merge, "-board", ErrInvalidName},
	} {
		_, err := s.EditWorkstream("alpha", "board-ui", tc.pre, WorkstreamEdit{Add: []string{"AL-1"}, Rename: tc.rename})
		if !errors.Is(err, tc.is) {
			t.Errorf("%s: error = %v, want %v", name, err, tc.is)
		}
		if after := snapshot(t, root); !maps.Equal(before, after) {
			t.Fatalf("%s: a refused rename wrote files", name)
		}
	}
}

// TestEditWorkstreamRenameSkipsBrokenFiles: a ticket or workstream whose
// frontmatter does not parse is left as it is, since its references
// cannot be read; the renamed workstream's own refuses the rename.
func TestEditWorkstreamRenameSkipsBrokenFiles(t *testing.T) {
	t.Parallel()

	s, root := renameFixture(t)
	broken := filepath.Join(root, "alpha", "tickets", "AL-7-broken-frontmatter", "AL-7-broken-frontmatter.md")
	before := string(mustRead(t, broken))
	if _, err := s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{Rename: "board"}); err != nil {
		t.Fatal(err)
	}
	if after := string(mustRead(t, broken)); after != before {
		t.Error("a ticket that does not parse was rewritten")
	}

	file := filepath.Join(root, "alpha", "workstreams", "offline.md")
	if err := os.WriteFile(file, []byte("---\nslug: [offline\n---\n\n# Offline\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unchanged := snapshot(t, root)
	if _, err := s.EditWorkstream("alpha", "offline", Merge, WorkstreamEdit{Rename: "away"}); err == nil || !strings.Contains(err.Error(), "does not parse") {
		t.Fatalf("broken workstream rename error = %v", err)
	}
	if !maps.Equal(unchanged, snapshot(t, root)) {
		t.Fatal("a refused rename wrote files")
	}
}
