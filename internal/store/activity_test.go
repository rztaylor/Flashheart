package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A session's activity state is read, appended with and replaced under the
// project lock; nil removes it, and a failed update changes nothing.
func TestUpdateActivityKeepsStateWithTheLog(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	state := filepath.Join(root, "alpha", ".flashheart", "activity", "claude--s1.json")
	events := filepath.Join(root, "alpha", ".flashheart", "events", "2026-10-05.jsonl")
	update := func(want string, next []byte, lines map[string][]byte, fail error) error {
		return s.UpdateActivity("alpha", "claude:s1", func(current []byte) ([]byte, map[string][]byte, error) {
			if string(current) != want {
				t.Errorf("state = %q, want %q", current, want)
			}
			return next, lines, fail
		})
	}
	if err := update("", []byte(`{"n":1}`), map[string][]byte{"2026-10-05.jsonl": []byte("{\"a\":1}\n")}, nil); err != nil {
		t.Fatal(err)
	}
	if err := update(`{"n":1}`, []byte(`{"n":2}`), nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := update(`{"n":2}`, []byte(`{"n":3}`), map[string][]byte{"2026-10-05.jsonl": []byte("{\"a\":2}\n")}, errors.New("refused")); err == nil {
		t.Fatal("a failed update succeeded")
	}
	if err := update(`{"n":2}`, []byte(`{"n":3}`), map[string][]byte{"../escape": []byte("x\n")}, nil); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("an unsafe event file: %v", err)
	}
	if data, _ := os.ReadFile(events); string(data) != "{\"a\":1}\n" {
		t.Fatalf("event file = %q", data)
	}
	if err := update(`{"n":2}`, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("state after removal: %v", err)
	}
	for _, session := range []string{"claude:s1/a1", "../x", ""} {
		if err := s.UpdateActivity("alpha", session, nil); !errors.Is(err, ErrInvalidName) {
			t.Errorf("session %q: %v", session, err)
		}
	}
}

// Retention removes the session files nothing has written since before,
// each checked again under the project lock (FH-82): activity state and
// session inboxes left by sessions that never reported their end.
func TestPruneSessionFilesRemovesOnlyThoseWrittenBefore(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	now := time.Now()
	old, fresh := now.Add(-13*time.Hour), now.Add(-time.Hour)
	write := func(name string, at time.Time) string {
		t.Helper()
		full := filepath.Join(root, "alpha", ".flashheart", name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(full, at, at); err != nil {
			t.Fatal(err)
		}
		return full
	}
	gone := []string{write("activity/claude--old.json", old), write("inbox/claude--old.jsonl", old), write("answers/claude--old.jsonl", old)}
	kept := []string{
		write("activity/claude--fresh.json", fresh), write("inbox/claude--fresh.jsonl", fresh), write("answers/claude--fresh.jsonl", fresh),
		write("activity/notes.txt", old), write("answers/notes.txt", old),
	}
	if err := os.MkdirAll(filepath.Join(root, "alpha", ".flashheart", "activity", "claude--dir.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	cutoff := now.Add(-12 * time.Hour)
	if removed, err := s.PruneActivity("alpha", cutoff); err != nil || removed != 1 {
		t.Fatalf("PruneActivity = %d, %v", removed, err)
	}
	if removed, err := s.PruneInboxes("alpha", cutoff); err != nil || removed != 2 {
		t.Fatalf("PruneInboxes = %d, %v", removed, err)
	}
	for _, name := range gone {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Errorf("%s: %v, want removed", name, err)
		}
	}
	for _, name := range kept {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("%s: %v, want kept", name, err)
		}
	}
	// A project with neither folder has nothing to prune.
	if err := os.MkdirAll(filepath.Join(root, "beta", "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if removed, err := s.PruneActivity("beta", now); err != nil || removed != 0 {
		t.Fatalf("PruneActivity(beta) = %d, %v", removed, err)
	}
	if _, err := s.PruneInboxes("../x", now); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("an unsafe project: %v", err)
	}
}
