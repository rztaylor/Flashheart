package app

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/store"
)

func TestPruneEventsRemovesExpiredFilesInEveryProject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, project := range []string{"alpha", "beta"} {
		dir := filepath.Join(root, project, ".flashheart", "events")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, project, "project.yaml"), []byte("name: "+project+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"2026-06-01.jsonl", "2026-10-01.jsonl"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	files := store.New(root)
	defer files.Close()
	var diagnostics bytes.Buffer
	pruneEvents(files, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), 90, &diagnostics)
	for _, project := range []string{"alpha", "beta"} {
		entries, _ := os.ReadDir(filepath.Join(root, project, ".flashheart", "events"))
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		if !slices.Equal(names, []string{"2026-10-01.jsonl"}) {
			t.Fatalf("%s events = %v", project, names)
		}
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("diagnostics = %s", diagnostics.String())
	}
}
