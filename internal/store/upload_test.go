package store

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const pngBytes = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

// countingReader counts what was read from it, to show an upload stops
// reading at the limit rather than taking the whole body first.
type countingReader struct {
	r    io.Reader
	read int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read += int64(n)
	return n, err
}

// A file from the browser (FH-18) is stored by what its bytes are, under a
// sanitised name in the ticket's files/, with an index entry that names
// no source and the person who added it.
func TestUploadStoresTheSniffedTypeUnderASafeName(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	s = s.As("Robert")
	dir := filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "files")
	upload := func(name, data, kind string) (StoredFile, error) {
		return s.UploadAttachment("alpha", "AL-4", Upload{Name: name, Body: strings.NewReader(data), Kind: kind})
	}

	// A PNG named .txt is a PNG; traversal in the name stays a name.
	stored, err := upload("../../../etc/Bug Report.txt", pngBytes, "")
	if err != nil {
		t.Fatal(err)
	}
	if stored.File != "20261005T1412-bug-report.png" {
		t.Errorf("stored as %q", stored.File)
	}
	if data, err := os.ReadFile(filepath.Join(dir, stored.File)); err != nil || string(data) != pngBytes {
		t.Errorf("stored %q, %v", data, err)
	}
	// The same name again takes a suffix; the same bytes again are the
	// same file.
	second, err := upload("bug report.png", pngBytes+"2", "screenshot")
	if err != nil || second.File != "20261005T1412-bug-report-2.png" {
		t.Errorf("second = %+v, %v", second, err)
	}
	again, err := upload("other.png", pngBytes, "")
	if err != nil || !again.Reused || again.File != stored.File {
		t.Errorf("again = %+v, %v", again, err)
	}
	pasted, err := upload("screenshot.png", pngBytes+"3", "screenshot")
	if err != nil || pasted.File != "20261005T1412-screenshot.png" {
		t.Errorf("pasted = %+v, %v", pasted, err)
	}

	project, err := s.ReadProject("alpha")
	if err != nil {
		t.Fatal(err)
	}
	entries := project.Attachments["AL-4"]
	if len(entries) != 3 {
		t.Fatalf("index = %+v", entries)
	}
	first := entries[0]
	if first.File != stored.File || first.Kind != "screenshot" || first.Caption != "Bug Report.txt" || first.Source != "" || first.Run != "Robert" {
		t.Errorf("entry = %+v", first)
	}
	// The upload is not a ticket write: the ticket's file and history
	// stay as they were.
	if _, err := os.Stat(filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "activity.md")); !os.IsNotExist(err) {
		t.Errorf("an upload wrote the ticket's activity: %v", err)
	}
}

// Refused uploads (a disallowed type by content, over the limit, a ticket
// that does not exist) leave nothing behind: no file, no temporary file, no
// index. Reading stops at the limit.
func TestRefusedUploadsLeaveNothingBehind(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	dir := filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "files")
	for _, tc := range []struct {
		name, ticket, data string
		limit              int64
		err                error
	}{
		{name: "drawing.png", ticket: "AL-4", data: `<svg xmlns="http://www.w3.org/2000/svg"/>`, err: ErrTypeNotAllowed},
		{name: "page.txt", ticket: "AL-4", data: "<!doctype html><p>hi", err: ErrTypeNotAllowed},
		{name: "page.html", ticket: "AL-4", data: "words", err: ErrTypeNotAllowed},
		{name: "tool.exe", ticket: "AL-4", data: "MZ\x90\x00", err: ErrTypeNotAllowed},
		{name: "big.png", ticket: "AL-4", data: pngBytes + strings.Repeat("x", 100), limit: 64, err: ErrTooLarge},
		{name: "shot.png", ticket: "AL-99", data: pngBytes, err: ErrNotFound},
	} {
		_, err := s.UploadAttachment("alpha", tc.ticket, Upload{Name: tc.name, Body: strings.NewReader(tc.data), MaxBytes: tc.limit})
		if !errors.Is(err, tc.err) {
			t.Errorf("%s: %v, want %v", tc.name, err, tc.err)
		}
		if err != nil && strings.Contains(err.Error(), tc.data) {
			t.Errorf("%s: the refusal repeats the content: %v", tc.name, err)
		}
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		entries, _ := os.ReadDir(dir)
		t.Errorf("refused uploads left files/: %v", entries)
	}

	// 20 MB is the default; a body far over it is read only to the limit.
	body := &countingReader{r: io.LimitReader(zeroes{}, 30<<20)}
	_, err := s.UploadAttachment("alpha", "AL-4", Upload{Name: "huge.png", Body: io.MultiReader(strings.NewReader(pngBytes), body)})
	if !errors.Is(err, ErrTooLarge) || !strings.Contains(err.Error(), "20 MB") {
		t.Fatalf("huge upload: %v", err)
	}
	if body.read > DefaultMaxFileCopy+64<<10 {
		t.Errorf("read %d bytes of a body over the limit", body.read)
	}
	// Exactly the limit is allowed.
	exact := bytes.Repeat([]byte("a"), 100)
	if _, err := s.UploadAttachment("alpha", "AL-4", Upload{Name: "exact.txt", Body: bytes.NewReader(exact), MaxBytes: 100}); err != nil {
		t.Errorf("an upload of exactly the limit: %v", err)
	}
}

type zeroes struct{}

func (zeroes) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
