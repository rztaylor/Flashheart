package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Bytes from the browser (FH-18) get the checks a copied file gets: the
// allow-list, the size limit and secret scrubbing (FH-88, SEC-2, SEC-3).
func TestAddAttachmentChecksBytesLikeFiles(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	stored, err := s.AddAttachment("alpha", "AL-4", NewAttachment{
		Name: "build output.log", Data: []byte("key AKIAIOSFODNN7EXAMPLE\nok\n"), Caption: "CI", Run: "human",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "files", stored.File))
	if err != nil || strings.Contains(string(data), "AKIA") || !strings.Contains(string(data), "ok") {
		t.Fatalf("stored %q, %v", data, err)
	}
	if !strings.HasSuffix(stored.File, "-build-output.log") {
		t.Fatalf("stored as %s", stored.File)
	}
	project, err := s.ReadProject("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if entries := project.Attachments["AL-4"]; len(entries) != 1 || entries[0].Kind != "log" || entries[0].Caption != "CI" || entries[0].Source != "" {
		t.Fatalf("index = %+v", entries)
	}

	for _, tc := range []struct {
		input NewAttachment
		err   error
	}{
		{NewAttachment{Name: "page.html", Data: []byte("<script>")}, ErrTypeNotAllowed},
		{NewAttachment{Name: "drawing.svg", Data: []byte("<svg/>")}, ErrTypeNotAllowed},
		{NewAttachment{Name: "big.png", Data: make([]byte, 11), MaxBytes: 10}, ErrTooLarge},
		{NewAttachment{Name: "", Data: []byte("x")}, ErrTypeNotAllowed},
	} {
		if _, err := s.AddAttachment("alpha", "AL-4", tc.input); !errors.Is(err, tc.err) {
			t.Errorf("%s: %v, want %v", tc.input.Name, err, tc.err)
		}
	}
}

// The files index keeps what Flashheart does not write, unknown keys and
// comments, when a file is added (FH-88).
func TestAnIndexKeepsUnknownKeysWhenAFileIsAdded(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	dir := filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "files")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	index := "# Evidence for AL-4\n- file: old.png\n  caption: Before\n  kind: screenshot\n  reviewer: robert # checked\n"
	if err := os.WriteFile(filepath.Join(dir, "index.yaml"), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddAttachment("alpha", "AL-4", NewAttachment{Name: "after.png", Data: []byte("\x89PNG")}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "index.yaml"))
	for _, want := range []string{"# Evidence for AL-4", "reviewer: robert", "# checked", "file: old.png", "-after.png"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("index lost %q:\n%s", want, data)
		}
	}
	project, err := s.ReadProject("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if entries := project.Attachments["AL-4"]; len(entries) != 2 || entries[0].File != "old.png" || entries[1].Kind != "screenshot" {
		t.Fatalf("index = %+v", entries)
	}
}
