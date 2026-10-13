package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func inboxFile(root, dir, name string) string {
	return filepath.Join(root, "alpha", ".flashheart", dir, name)
}

func writeInboxFile(t *testing.T, root, dir, name, content string) {
	t.Helper()
	full := inboxFile(root, dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readInboxFile(t *testing.T, root, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(inboxFile(root, dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return "<none>"
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A session's inbox lives in .flashheart/inbox/ (D54); appending to it
// first moves an inbox left in the old answers/ location in front, so it
// drains into the one file the prompt hook checks.
func TestAppendInboxWritesTheInboxAndDrainsTheOldLocation(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if err := s.AppendInbox("alpha", "claude:s1", []byte("new-1\n")); err != nil {
		t.Fatal(err)
	}
	if got := readInboxFile(t, root, "inbox", "claude--s1.jsonl"); got != "new-1\n" {
		t.Fatalf("inbox = %q", got)
	}
	writeInboxFile(t, root, "answers", "claude--s1.jsonl", "old-1\n")
	if err := s.AppendInbox("alpha", "claude:s1", []byte("new-2\n")); err != nil {
		t.Fatal(err)
	}
	if got := readInboxFile(t, root, "inbox", "claude--s1.jsonl"); got != "old-1\nnew-1\nnew-2\n" {
		t.Fatalf("inbox = %q", got)
	}
	if got := readInboxFile(t, root, "answers", "claude--s1.jsonl"); got != "<none>" {
		t.Fatalf("old inbox = %q, want it moved", got)
	}
	if err := s.AppendInbox("alpha", "claude:s1/a1", []byte("x\n")); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("a subagent's own inbox: %v", err)
	}
}

func TestTakeInboxTakesTheLinesAskedForAndKeepsTheRest(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if got, err := s.TakeInbox("alpha", "claude:s1", nil); err != nil || got != nil {
		t.Fatalf("no inbox: %q, %v", got, err)
	}
	writeInboxFile(t, root, "inbox", "claude--s1.jsonl", "keep-1\ntake-1\n\nkeep-2\ntake-2\n")
	isTake := func(line []byte) bool { return string(line[:4]) == "take" }
	got, err := s.TakeInbox("alpha", "claude:s1", isTake)
	if err != nil || string(got) != "take-1\ntake-2\n" {
		t.Fatalf("taken = %q, %v", got, err)
	}
	if left := readInboxFile(t, root, "inbox", "claude--s1.jsonl"); left != "keep-1\nkeep-2\n" {
		t.Fatalf("left = %q", left)
	}
	// Nothing to take leaves the file as it is.
	if got, err := s.TakeInbox("alpha", "claude:s1", isTake); err != nil || got != nil {
		t.Fatalf("second take = %q, %v", got, err)
	}
	// Taking everything removes the file.
	if got, err := s.TakeInbox("alpha", "claude:s1", nil); err != nil || string(got) != "keep-1\nkeep-2\n" {
		t.Fatalf("take all = %q, %v", got, err)
	}
	if left := readInboxFile(t, root, "inbox", "claude--s1.jsonl"); left != "<none>" {
		t.Fatalf("left = %q, want removed", left)
	}
}

// Readers that can afford a second check take the old location too, while
// inboxes an older binary wrote there drain (D54).
func TestTakeLegacyInboxReadsTheOldLocation(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	writeInboxFile(t, root, "answers", "claude--s1.jsonl", "old-1\n")
	writeInboxFile(t, root, "inbox", "claude--s1.jsonl", "new-1\n")
	got, err := s.TakeLegacyInbox("alpha", "claude:s1", nil)
	if err != nil || string(got) != "old-1\n" {
		t.Fatalf("taken = %q, %v", got, err)
	}
	if left := readInboxFile(t, root, "answers", "claude--s1.jsonl"); left != "<none>" {
		t.Fatalf("old inbox = %q, want removed", left)
	}
	if left := readInboxFile(t, root, "inbox", "claude--s1.jsonl"); left != "new-1\n" {
		t.Fatalf("new inbox = %q, want untouched", left)
	}
}

func TestDrainLegacyInboxesMovesEveryOldInbox(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if moved, err := s.DrainLegacyInboxes("alpha"); err != nil || moved != 0 {
		t.Fatalf("nothing to drain: %d, %v", moved, err)
	}
	writeInboxFile(t, root, "answers", "claude--s1.jsonl", "old-1\n")
	writeInboxFile(t, root, "answers", "codex--s2.jsonl", "old-2\n")
	writeInboxFile(t, root, "answers", "notes.txt", "not an inbox\n")
	writeInboxFile(t, root, "inbox", "claude--s1.jsonl", "new-1\n")
	moved, err := s.DrainLegacyInboxes("alpha")
	if err != nil || moved != 2 {
		t.Fatalf("drained %d, %v", moved, err)
	}
	if got := readInboxFile(t, root, "inbox", "claude--s1.jsonl"); got != "old-1\nnew-1\n" {
		t.Fatalf("claude inbox = %q", got)
	}
	if got := readInboxFile(t, root, "inbox", "codex--s2.jsonl"); got != "old-2\n" {
		t.Fatalf("codex inbox = %q", got)
	}
	for _, name := range []string{"claude--s1.jsonl", "codex--s2.jsonl"} {
		if got := readInboxFile(t, root, "answers", name); got != "<none>" {
			t.Fatalf("old %s = %q, want moved", name, got)
		}
	}
	if got := readInboxFile(t, root, "answers", "notes.txt"); got != "not an inbox\n" {
		t.Fatalf("an unknown file was touched: %q", got)
	}
	if _, err := s.DrainLegacyInboxes("../x"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("an unsafe project: %v", err)
	}
}
