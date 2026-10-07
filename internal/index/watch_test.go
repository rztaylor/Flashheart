package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/store"
)

func TestWaitReturnsWhenTheRevisionAdvances(t *testing.T) {
	t.Parallel()

	source := &fakeSource{fingerprint: "a"}
	index, _ := newIndex(source)
	first, _ := index.Rebuild()

	done := make(chan uint64, 1)
	go func() { done <- index.Wait(context.Background(), first.Revision) }()
	select {
	case revision := <-done:
		t.Fatalf("Wait returned %d before any change", revision)
	case <-time.After(50 * time.Millisecond):
	}
	source.set("b", nil)
	if _, err := index.Rebuild(); err != nil {
		t.Fatal(err)
	}
	select {
	case revision := <-done:
		if revision != first.Revision+1 {
			t.Errorf("Wait = %d, want %d", revision, first.Revision+1)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after the change")
	}

	// An older revision returns at once; a cancelled wait returns the current one.
	if got := index.Wait(context.Background(), first.Revision); got != first.Revision+1 {
		t.Errorf("Wait(old) = %d", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if got := index.Wait(ctx, first.Revision+1); got != first.Revision+1 {
		t.Errorf("Wait(cancelled) = %d", got)
	}
}

// External edits reach waiting clients within a second (STO-7).
func TestWatchSeesExternalEdits(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "boards", "sample"))); err != nil {
		t.Fatal(err)
	}
	files, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	index := New(files, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	go index.Watch(ctx, root, ready)
	<-ready

	expectChange := func(label string, change func()) {
		t.Helper()
		snapshot, err := index.Current()
		if err != nil {
			t.Fatal(err)
		}
		change()
		wait, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		if got := index.Wait(wait, snapshot.Revision); got <= snapshot.Revision {
			t.Errorf("%s: no new revision within a second", label)
		}
	}
	ticket := filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "AL-4-drag-and-drop.md")
	expectChange("edit in place", func() {
		data, _ := os.ReadFile(ticket)
		_ = os.WriteFile(ticket, append(data, []byte("\nEdited.\n")...), 0o644)
	})
	expectChange("atomic replace", func() {
		data, _ := os.ReadFile(ticket)
		temporary := ticket + ".swp"
		_ = os.WriteFile(temporary, append(data, []byte("Again.\n")...), 0o644)
		_ = os.Rename(temporary, ticket)
	})
	folder := filepath.Join(root, "beta", "tickets", "BE-2-new")
	expectChange("new ticket", func() {
		_ = os.MkdirAll(folder, 0o755)
		_ = os.WriteFile(filepath.Join(folder, "BE-2-new.md"), []byte("---\nid: BE-2\nstatus: backlog\n---\n# New\n"), 0o644)
	})
	expectChange("edit the new ticket", func() {
		_ = os.WriteFile(filepath.Join(folder, "BE-2-new.md"), []byte("---\nid: BE-2\nstatus: done\n---\n# New\n"), 0o644)
	})
	expectChange("new project", func() {
		_ = os.MkdirAll(filepath.Join(root, "gamma", "tickets"), 0o755)
	})
	expectChange("workstream edit", func() {
		name := filepath.Join(root, "alpha", "workstreams", "board-ui.md")
		data, _ := os.ReadFile(name)
		_ = os.WriteFile(name, append(data, []byte("\nMore.\n")...), 0o644)
	})
}

// An edit made the moment a new ticket folder's revision appears is still
// seen: the folder is watched before that revision is published (FH-36).
func TestWatchSeesAnEditRightAfterANewFolderAppears(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "boards", "sample"))); err != nil {
		t.Fatal(err)
	}
	files, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	index := New(files, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	go index.Watch(ctx, root, ready)
	<-ready

	waitPast := func(revision uint64) uint64 {
		t.Helper()
		wait, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		return index.Wait(wait, revision)
	}
	for n := range 10 {
		snapshot, err := index.Current()
		if err != nil {
			t.Fatal(err)
		}
		folder := filepath.Join(root, "beta", "tickets", fmt.Sprintf("BE-%d-new", n+10))
		file := filepath.Join(folder, filepath.Base(folder)+".md")
		_ = os.MkdirAll(folder, 0o755)
		_ = os.WriteFile(file, []byte(fmt.Sprintf("---\nid: BE-%d\nstatus: backlog\n---\n# New\n", n+10)), 0o644)
		created := waitPast(snapshot.Revision)
		if created <= snapshot.Revision {
			t.Fatalf("folder %d: no revision for the new ticket", n)
		}
		_ = os.WriteFile(file, []byte(fmt.Sprintf("---\nid: BE-%d\nstatus: done\n---\n# New\n", n+10)), 0o644)
		if got := waitPast(created); got <= created {
			t.Fatalf("folder %d: the edit right after creation was missed", n)
		}
	}
}
