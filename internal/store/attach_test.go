package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func localFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestCopyIntoTicketStoresACopyAndIndexesIt(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	data := []byte("\x89PNG fake screenshot")
	source := localFile(t, "Board Desktop.png", data)
	stored, err := s.CopyIntoTicket("alpha", "AL-4", FileCopy{Source: source, Caption: "Board, dark", Kind: "screenshot", Run: "claude:s1"})
	if err != nil {
		t.Fatal(err)
	}
	if stored.File != "20261005T1412-board-desktop.png" || stored.Reused {
		t.Fatalf("stored = %+v", stored)
	}
	folder := filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop")
	copied, err := os.ReadFile(filepath.Join(folder, "files", stored.File))
	if err != nil || string(copied) != string(data) {
		t.Fatalf("copy = %q, %v", copied, err)
	}
	// The original can go; the copy stays (REV-5).
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	project, err := s.ReadProject("alpha")
	if err != nil {
		t.Fatal(err)
	}
	entries := project.Attachments["AL-4"]
	sum := sha256.Sum256(data)
	if len(entries) != 1 || entries[0].File != stored.File || entries[0].Caption != "Board, dark" || entries[0].Kind != "screenshot" ||
		entries[0].Source != source || entries[0].Run != "claude:s1" || entries[0].Added != "2026-10-05T14:12:09Z" || entries[0].SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("index = %+v", entries)
	}
}

func TestCopyIntoTicketReusesAnIdenticalCopy(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	source := localFile(t, "log.txt", []byte("build ok"))
	first, err := s.CopyIntoTicket("alpha", "AL-4", FileCopy{Source: source, Kind: "log"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CopyIntoTicket("alpha", "AL-4", FileCopy{Source: source, Kind: "log"})
	if err != nil {
		t.Fatal(err)
	}
	if second.File != first.File || !second.Reused {
		t.Fatalf("second copy = %+v, first %+v", second, first)
	}
	// Different content under the same name gets its own stored name.
	if err := os.WriteFile(source, []byte("build failed"), 0o644); err != nil {
		t.Fatal(err)
	}
	third, err := s.CopyIntoTicket("alpha", "AL-4", FileCopy{Source: source, Kind: "log"})
	if err != nil {
		t.Fatal(err)
	}
	if third.File == first.File || third.Reused || !strings.HasSuffix(third.File, "log-2.txt") {
		t.Fatalf("third copy = %+v", third)
	}
}

func TestCopyIntoTicketRefusesUnsafeSources(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	dir := t.TempDir()
	cases := []struct {
		name   string
		source string
		limit  int64
		want   error
	}{
		{"svg", localFile(t, "logo.svg", []byte("<svg/>")), 0, ErrTypeNotAllowed},
		{"html", localFile(t, "page.html", []byte("<p>")), 0, ErrTypeNotAllowed},
		{"no extension", localFile(t, "credentials", []byte("x")), 0, ErrTypeNotAllowed},
		{"too large", localFile(t, "big.log", []byte(strings.Repeat("x", 11))), 10, ErrTooLarge},
		{"directory", filepath.Join(dir, "shots.png"), 0, ErrNotRegular},
		{"missing", filepath.Join(dir, "gone.png"), 0, ErrNotFound},
		{"relative", "shot.png", 0, ErrInvalidInput},
	}
	if err := os.Mkdir(filepath.Join(dir, "shots.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		_, err := s.CopyIntoTicket("alpha", "AL-4", FileCopy{Source: tc.source, MaxBytes: tc.limit})
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
	if _, err := s.CopyIntoTicket("alpha", "AL-99", FileCopy{Source: localFile(t, "a.png", []byte("x"))}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing ticket: err = %v", err)
	}
}

func TestWriteReviewReplacesTheReviewFile(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	for _, text := range []string{"# Review: one\n", "# Review: two\n"} {
		if err := s.WriteReview("alpha", "AL-4", []byte(text)); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "review.md"))
		if err != nil || string(got) != text {
			t.Fatalf("review = %q, %v", got, err)
		}
	}
	if err := s.WriteReview("alpha", "AL-4", make([]byte, MaxFileBytes+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversized review: err = %v", err)
	}
}

func TestCopyIntoTicketJudgesTheLinkTargetAndScrubsText(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	dir := t.TempDir()
	secret := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(secret, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "notes.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CopyIntoTicket("alpha", "AL-4", FileCopy{Source: link}); !errors.Is(err, ErrTypeNotAllowed) {
		t.Fatalf("link to a key: err = %v", err)
	}
	log := localFile(t, "build.log", []byte("ok\nGITHUB_TOKEN=ghp_abcdefghijklmnopqrstuvwxyz0123456789\n"))
	stored, err := s.CopyIntoTicket("alpha", "AL-4", FileCopy{Source: log, Kind: "log"})
	if err != nil {
		t.Fatal(err)
	}
	copied, _ := os.ReadFile(filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "files", stored.File))
	if strings.Contains(string(copied), "ghp_") || !strings.Contains(string(copied), "ok\n") {
		t.Fatalf("copied log = %q", copied)
	}
}
