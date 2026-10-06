package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// columnOrder reads a project's tickets in one column in board order.
func columnOrder(t *testing.T, s *Store, project string, column board.Column) []string {
	t.Helper()
	info, err := s.ReadProject(project)
	if err != nil {
		t.Fatal(err)
	}
	tickets := slices.DeleteFunc(slices.Clone(info.Tickets), func(ticket board.Ticket) bool { return ticket.Column != column })
	slices.SortStableFunc(tickets, board.CompareOrder)
	var out []string
	for _, ticket := range tickets {
		out = append(out, ticket.ID)
	}
	return out
}

func TestPlaceTicketMovesAndRanksItInOneLockedWrite(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	al5 := filepath.Join(root, "alpha", "tickets", "AL-5-column-overflow", "AL-5-column-overflow.md")
	before, _ := os.ReadFile(al5)

	// Up next → top of Backlog.
	hash, placed, err := s.PlaceTicket("alpha", "AL-4", "", "", setStatus("backlog"))
	if err != nil || !placed {
		t.Fatalf("place: placed=%v err=%v", placed, err)
	}
	data, current, _ := s.ReadTicket("alpha", "AL-4")
	if hash != current {
		t.Error("returned hash does not match the file")
	}
	doc := mdfile.Parse(data)
	if status, _ := doc.String("status"); status != "backlog" {
		t.Errorf("status = %q", status)
	}
	if rank, _ := doc.String("rank"); !board.ValidRank(rank) {
		t.Errorf("rank = %q", rank)
	}
	if got := columnOrder(t, s, "alpha", board.Backlog); !slices.Equal(got[:3], []string{"AL-4", "AL-5", "AL-6"}) {
		t.Errorf("backlog = %v", got)
	}
	// Only the moving ticket was written.
	if after, _ := os.ReadFile(al5); string(after) != string(before) {
		t.Error("AL-5 should not have been rewritten")
	}

	// Reorder within the column: AL-6 directly after AL-4.
	if _, placed, err := s.PlaceTicket("alpha", "AL-6", "", "AL-4", setStatus("backlog")); err != nil || !placed {
		t.Fatalf("reorder: placed=%v err=%v", placed, err)
	}
	if got := columnOrder(t, s, "alpha", board.Backlog); !slices.Equal(got[:3], []string{"AL-4", "AL-6", "AL-5"}) {
		t.Errorf("backlog = %v", got)
	}

	// The broken ticket cannot hold a rank; placing after it still works
	// and leaves its file alone.
	broken := filepath.Join(root, "alpha", "tickets", "AL-7-broken-frontmatter", "AL-7-broken-frontmatter.md")
	brokenBefore, _ := os.ReadFile(broken)
	if _, _, err := s.PlaceTicket("alpha", "AL-4", "", "AL-5", setStatus("backlog")); err != nil {
		t.Fatalf("place after AL-5: %v", err)
	}
	if got := columnOrder(t, s, "alpha", board.Backlog); !slices.Equal(got[:3], []string{"AL-6", "AL-5", "AL-4"}) {
		t.Errorf("backlog = %v", got)
	}
	if after, _ := os.ReadFile(broken); string(after) != string(brokenBefore) {
		t.Error("the broken ticket was rewritten")
	}
}

func TestPlaceTicketChecksTheHashBeforeWritingAnything(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	snapshot := func() map[string]string {
		files := map[string]string{}
		_ = filepath.WalkDir(filepath.Join(root, "alpha", "tickets"), func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				data, _ := os.ReadFile(path)
				files[path] = string(data)
			}
			return nil
		})
		return files
	}
	before := snapshot()
	_, _, err := s.PlaceTicket("alpha", "AL-6", "stale", "AL-5", setStatus("backlog"))
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want a conflict", err)
	}
	after := snapshot()
	for path, content := range before {
		if after[path] != content {
			t.Errorf("%s changed despite the conflict", path)
		}
	}
}

func TestPlaceTicketAfterATicketThatLeftTheColumnKeepsItsRank(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	_, placed, err := s.PlaceTicket("alpha", "AL-4", "", "AL-3", setStatus("backlog"))
	if err != nil || placed {
		t.Fatalf("placed=%v err=%v; AL-3 is in progress, not Backlog", placed, err)
	}
	data, _, _ := s.ReadTicket("alpha", "AL-4")
	doc := mdfile.Parse(data)
	if status, _ := doc.String("status"); status != "backlog" {
		t.Errorf("the move should still happen: status = %q", status)
	}
	if rank, ok := doc.String("rank"); ok {
		t.Errorf("rank = %q, want none", rank)
	}
	if _, _, err := s.PlaceTicket("alpha", "AL-404", "", "", setStatus("backlog")); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown ticket: %v", err)
	}
}

func TestConcurrentPlacementsKeepOneOrder(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for _, id := range []string{"AL-4", "AL-5", "AL-6"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := open(t, root)
			if _, _, err := s.PlaceTicket("alpha", id, "", "", setStatus("up-next")); err != nil {
				errs <- fmt.Errorf("%s: %w", id, err)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	s := open(t, root)
	info, _ := s.ReadProject("alpha")
	ranks := map[string]bool{}
	for _, ticket := range info.Tickets {
		if ticket.Column == board.UpNext {
			if ticket.Rank == "" || ranks[ticket.Rank] {
				t.Errorf("%s rank %q is missing or shared", ticket.ID, ticket.Rank)
			}
			ranks[ticket.Rank] = true
		}
	}
	if len(ranks) != 3 {
		t.Errorf("ranked %d tickets, want 3", len(ranks))
	}
}
