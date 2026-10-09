package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
