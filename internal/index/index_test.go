package index

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/store"
)

type fakeSource struct {
	mu          sync.Mutex
	board       board.Board
	fingerprint string
	err         error
	reads       int
}

func (f *fakeSource) ReadBoard() (board.Board, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	return f.board, f.fingerprint, f.err
}

func (f *fakeSource) set(fingerprint string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fingerprint, f.err = fingerprint, err
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func newIndex(source Source) (*Index, *fakeClock) {
	clock := &fakeClock{now: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)}
	return New(source, Options{MaxAge: 2 * time.Second, Now: clock.Now}), clock
}

func TestRevisionChangesOnlyWhenTheBoardChanges(t *testing.T) {
	t.Parallel()

	source := &fakeSource{fingerprint: "a"}
	index, _ := newIndex(source)
	first, err := index.Rebuild()
	if err != nil || first.Revision != 1 {
		t.Fatalf("first = %+v, %v", first, err)
	}
	same, _ := index.Rebuild()
	if same.Revision != 1 {
		t.Errorf("unchanged rebuild revision = %d, want 1", same.Revision)
	}
	source.set("b", nil)
	changed, _ := index.Rebuild()
	if changed.Revision != 2 {
		t.Errorf("changed rebuild revision = %d, want 2", changed.Revision)
	}
}

func TestCurrentRebuildsOnlyWhenStale(t *testing.T) {
	t.Parallel()

	source := &fakeSource{fingerprint: "a"}
	index, clock := newIndex(source)
	if _, err := index.Current(); err != nil {
		t.Fatal(err)
	}
	if _, err := index.Current(); err != nil {
		t.Fatal(err)
	}
	if source.reads != 1 {
		t.Errorf("reads = %d within MaxAge, want 1", source.reads)
	}
	clock.now = clock.now.Add(3 * time.Second)
	if _, err := index.Current(); err != nil {
		t.Fatal(err)
	}
	if source.reads != 2 {
		t.Errorf("reads = %d after MaxAge, want 2", source.reads)
	}
}

func TestMissingRootIsASnapshotNotAnError(t *testing.T) {
	t.Parallel()

	source := &fakeSource{err: fmt.Errorf("open board root: %w", os.ErrNotExist)}
	index, clock := newIndex(source)
	snapshot, err := index.Current()
	if err != nil || !snapshot.RootMissing || snapshot.Revision != 1 {
		t.Fatalf("snapshot = %+v, err = %v", snapshot, err)
	}
	source.set("a", nil)
	clock.now = clock.now.Add(time.Minute)
	snapshot, _ = index.Current()
	if snapshot.RootMissing || snapshot.Revision != 2 {
		t.Errorf("after the root appears: %+v", snapshot)
	}
}

func TestReadFailureKeepsThePreviousSnapshot(t *testing.T) {
	t.Parallel()

	source := &fakeSource{fingerprint: "a"}
	index, clock := newIndex(source)
	good, _ := index.Current()
	source.set("", errors.New("permission denied"))
	clock.now = clock.now.Add(time.Minute)
	snapshot, err := index.Current()
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("err = %v", err)
	}
	if snapshot != good {
		t.Error("a failed rebuild replaced the last good snapshot")
	}
}

func TestSnapshotLookups(t *testing.T) {
	t.Parallel()

	alpha := board.Project{Name: "alpha", Tickets: []board.Ticket{
		board.ParseTicket("feat--a", board.Todo, []byte("---\ndepends-on: [feat--b]\n---\n# A\n")),
		board.ParseTicket("feat--b", board.InProgress, []byte("# B\n")),
	}}
	index, _ := newIndex(&fakeSource{board: board.Board{Projects: []board.Project{alpha}}, fingerprint: "x"})
	snapshot, _ := index.Current()
	if project, ok := snapshot.Project("alpha"); !ok || project.Name != "alpha" {
		t.Errorf("Project(alpha) = %v", ok)
	}
	if _, ok := snapshot.Project("beta"); ok {
		t.Error("Project(beta) found")
	}
	if ticket, ok := snapshot.Ticket("alpha", "feat--a"); !ok || ticket.Title != "A" {
		t.Errorf("Ticket(alpha, feat--a) = %+v, %v", ticket, ok)
	}
	if reasons := snapshot.Analysis.Blocked[board.Ref{Project: "alpha", Slug: "feat--a"}]; len(reasons) != 1 {
		t.Errorf("analysis not computed: %+v", reasons)
	}
}

// TestIndexesFiveThousandTicketsQuickly is NFR-1: 5,000 tickets across 10
// projects index in under a second.
func TestIndexesFiveThousandTicketsQuickly(t *testing.T) {
	if testing.Short() {
		t.Skip("generates 5,000 files")
	}
	root := t.TempDir()
	columns := []board.Column{board.Todo, board.InProgress, board.ReadyToReview, board.Done}
	for p := range 10 {
		project := fmt.Sprintf("project-%02d", p)
		var workstream strings.Builder
		workstream.WriteString("---\ntickets:\n")
		for n := range 500 {
			slug := fmt.Sprintf("feat--ticket-%03d", n)
			column := columns[n%len(columns)]
			deps := ""
			if n > 0 {
				deps = fmt.Sprintf("depends-on: [feat--ticket-%03d]\n", n-1)
			}
			if n < 20 {
				fmt.Fprintf(&workstream, "  - %s\n", slug)
			}
			content := fmt.Sprintf("---\ntype: feature\nproject: %s\ncreated: 2026-10-04\npriority: medium\n%sworkstream: main\ntags: [generated]\n---\n\n# Ticket %d\n\n## Description\n\nGenerated ticket %d for the index benchmark.\n\n## Acceptance Criteria\n\n- [x] One\n- [ ] Two\n\n## Notes\n\nNone.\n", project, deps, n, n)
			path := filepath.Join(root, project, string(column), slug+".md")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		workstream.WriteString("---\n# Main\n")
		path := filepath.Join(root, project, "workstreams", "main.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(workstream.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	source := store.New(root)
	defer source.Close()
	index := New(source, Options{})
	started := time.Now()
	snapshot, err := index.Rebuild()
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	tickets := 0
	for _, project := range snapshot.Board.Projects {
		tickets += len(project.Tickets)
	}
	if tickets != 5000 {
		t.Fatalf("indexed %d tickets, want 5000", tickets)
	}
	limit := time.Second
	if raceEnabled {
		limit = 10 * time.Second // the race detector slows this several-fold
	}
	t.Logf("indexed %d tickets in %s", tickets, elapsed)
	if elapsed > limit {
		t.Errorf("indexing took %s, want under %s (NFR-1)", elapsed, limit)
	}
}
