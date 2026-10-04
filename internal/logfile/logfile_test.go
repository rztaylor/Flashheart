package logfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixed = time.Date(2026, 10, 4, 14, 12, 9, 0, time.UTC)

func newWriter(path string, maxBytes int64) *Writer {
	w := New(path, maxBytes)
	w.now = func() time.Time { return fixed }
	return w
}

func TestNothingIsCreatedUntilTheFirstWrite(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "board")
	w := newWriter(filepath.Join(root, ".flashheart", "serve.log"), 1<<20)
	defer w.Close()
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("New created %s (stat err = %v)", root, err)
	}
}

func TestWritesAreTimestampedAndAppended(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "board", ".flashheart", "serve.log")
	w := newWriter(path, 1<<20)
	if _, err := w.Write([]byte("first\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := w.Write([]byte("second")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	w.Close()
	// A new writer appends to the existing file.
	again := newWriter(path, 1<<20)
	if _, err := again.Write([]byte("third\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	again.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "2026-10-04T14:12:09Z first\n2026-10-04T14:12:09Z second\n2026-10-04T14:12:09Z third\n"
	if string(data) != want {
		t.Errorf("log = %q, want %q", data, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("log permissions = %o, want 600", perm)
	}
}

func TestRotatesToOneOlderGeneration(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "serve.log")
	w := newWriter(path, 64)
	defer w.Close()
	line := strings.Repeat("x", 30) + "\n" // 51 bytes with the timestamp prefix
	for range 3 {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	older, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatalf("no rotated generation: %v", err)
	}
	if strings.Count(string(current), "\n") != 1 || strings.Count(string(older), "\n") != 1 {
		t.Errorf("current=%q older=%q, want one line each", current, older)
	}
}

func TestWriteFailureIsReturnedNotPanicked(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	w := newWriter(filepath.Join(blocker, "serve.log"), 1<<20)
	defer w.Close()
	if _, err := w.Write([]byte("lost\n")); err == nil {
		t.Fatal("Write succeeded under a regular file")
	}
}
