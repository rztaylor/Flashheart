package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

const al4 = "alpha/tickets/AL-4-drag-and-drop/AL-4-drag-and-drop.md"

// A write that replaces a ticket's content must say what it replaces: a
// precondition with no hash is refused and nothing is written (FH-86).
func TestAReplaceWithoutAHashIsRefused(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	before, _ := os.ReadFile(filepath.Join(root, al4))
	replace := func([]byte) ([]byte, error) { return []byte("---\nid: AL-4\n---\n# Gone\n"), nil }
	for _, pre := range []Precondition{Unchanged(""), {}} {
		if _, err := s.UpdateTicket("alpha", "AL-4", pre, replace); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("precondition %+v: %v, want ErrInvalidInput", pre, err)
		}
		if _, _, err := s.PlaceTicket("alpha", "AL-4", pre, "", replace); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("place with %+v: %v", pre, err)
		}
		if _, err := s.SetTicketWorkstream("alpha", "AL-4", pre, "", replace); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("workstream with %+v: %v", pre, err)
		}
	}
	if after, _ := os.ReadFile(filepath.Join(root, al4)); string(after) != string(before) {
		t.Fatalf("the ticket changed:\n%s", after)
	}
}

// Every ticket write stamps `updated` by one rule: a result whose
// frontmatter parses is stamped; one whose frontmatter is broken is written
// as the edit left it (FH-86).
func TestOneStampingRule(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	_, hash, err := s.ReadTicket("alpha", "AL-4")
	if err != nil {
		t.Fatal(err)
	}
	broken := "---\nid: AL-4\nstatus: [unclosed\n---\n# Drag and drop\n"
	if _, err := s.UpdateTicket("alpha", "AL-4", Unchanged(hash), func([]byte) ([]byte, error) { return []byte(broken), nil }); err != nil {
		t.Fatal(err)
	}
	if written, _ := os.ReadFile(filepath.Join(root, al4)); string(written) != broken {
		t.Fatalf("broken frontmatter was changed:\n%s", written)
	}

	// The same rule holds in a multi-file write: a ticket joining a
	// workstream is stamped.
	if _, err := s.SetTicketWorkstream("alpha", "AL-5", Merge, "board-ui", nil); err != nil {
		t.Fatal(err)
	}
	data, _, err := s.ReadTicket("alpha", "AL-5")
	if err != nil {
		t.Fatal(err)
	}
	if updated, _ := mdfile.Parse(data).String("updated"); updated != "2026-10-05T14:12:09Z" {
		t.Fatalf("updated = %q", updated)
	}
	if !strings.Contains(string(data), "workstream: board-ui") {
		t.Fatalf("ticket:\n%s", data)
	}
}

// A transaction that fails part-way writes nothing: placing a ticket whose
// edit fails leaves the tickets it would have ranked alone (FH-86; before,
// they were written one by one).
func TestAFailedTransactionWritesNothing(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	snapshot := func() map[string]string {
		files := map[string]string{}
		_ = filepath.WalkDir(filepath.Join(root, "alpha"), func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				data, _ := os.ReadFile(path)
				files[path] = string(data)
			}
			return nil
		})
		return files
	}
	before := snapshot()
	failing := errors.New("refused")
	calls := 0
	// Placing AL-4 after AL-6 at the end of Backlog ranks the unranked AL-5
	// and AL-6 above it first.
	_, _, err := s.PlaceTicket("alpha", "AL-4", Merge, "AL-6", func(data []byte) ([]byte, error) {
		// The first call plans the placement; the second, staging the
		// ticket after its neighbours, fails.
		if calls++; calls > 1 {
			return nil, failing
		}
		return mdfile.SetScalar(data, "status", "backlog")
	})
	if !errors.Is(err, failing) {
		t.Fatalf("err = %v", err)
	}
	after := snapshot()
	for path, data := range before {
		if after[path] != data {
			t.Errorf("%s changed", path)
		}
	}
}
