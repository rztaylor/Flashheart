package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A renamed file takes its old place among the staged writes, first when
// it was not staged, and its old name is gone only after every write, so
// a crash part-way leaves both names, never neither (FH-158).
func TestARenameWritesTheNewFileFirstAndRemovesTheOldLast(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	first := s.begin()
	first.put(al4, []byte("staged first\n"))
	if err := first.rename("alpha/workstreams/board-ui.md", "alpha/workstreams/board.md"); err != nil {
		t.Fatal(err)
	}
	if first.files[0].name != "alpha/workstreams/board.md" || first.files[1].name != al4 {
		t.Fatalf("staged order = %s, %s", first.files[0].name, first.files[1].name)
	}
	if _, err := first.read("alpha/workstreams/board-ui.md"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the old name reads %v within the transaction", err)
	}
	if err := first.commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "workstreams", "board-ui.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old file: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "alpha", "workstreams", "board.md")); err != nil || !strings.Contains(string(data), "slug: board-ui") {
		t.Fatalf("new file: %v\n%s", err, data)
	}

	// A file already staged keeps its place under its new name.
	second := s.begin()
	second.put(al4, []byte("first\n"))
	second.put("alpha/workstreams/board.md", []byte("second\n"))
	if err := second.rename("alpha/workstreams/board.md", "alpha/workstreams/ui.md"); err != nil {
		t.Fatal(err)
	}
	if len(second.files) != 2 || second.files[1].name != "alpha/workstreams/ui.md" || string(second.files[1].data) != "second\n" {
		t.Fatalf("staged = %+v", second.files)
	}
}
