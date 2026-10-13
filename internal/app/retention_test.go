package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/hooks"
	"github.com/rztaylor/flashheart/internal/hooks/claude"
	"github.com/rztaylor/flashheart/internal/store"
)

// FH-82: a session that never reports its end (a crash; Codex has no end
// hook) leaves no activity state once it is stale_hours old, nor an answers
// inbox once its day's events have expired, without any run.end.
func TestRetentionRemovesTheFilesOfASessionThatNeverEnded(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(base, "src", "demo")
	if err := os.MkdirAll(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hook := func(event string, fields map[string]any) {
		t.Helper()
		fields["session_id"], fields["cwd"] = "s1", cwd
		payload, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		hooks.Run(hooks.Options{Root: root, Event: event, Stdin: bytes.NewReader(payload), Stdout: &bytes.Buffer{}, Adapter: claude.Adapter{}})
	}
	hook("SessionStart", map[string]any{"source": "startup"})
	hook("UserPromptSubmit", map[string]any{"prompt": "go"})
	for range 3 {
		hook("PostToolUse", map[string]any{"tool_name": "Read", "tool_input": map[string]any{"file_path": "a.txt"}, "tool_response": map[string]any{}})
	}
	if data, err := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log")); err == nil {
		t.Fatalf("hook-errors.log = %s", data)
	}
	files := store.New(root)
	defer files.Close()
	if err := events.New(files).Queue("demo", events.Entry{Type: events.EntryAnswer, ID: "q1", Run: "claude:s1", Question: "which?", Text: "this", By: "human"}); err != nil {
		t.Fatal(err)
	}
	listing := func(dir string) []string {
		entries, _ := os.ReadDir(filepath.Join(root, "demo", ".flashheart", dir))
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		return names
	}
	if names := listing("activity"); !slices.Equal(names, []string{"claude--s1.json"}) {
		t.Fatalf("activity state before retention = %v", names)
	}
	// An inbox an older binary left in the old location (D54).
	old := filepath.Join(root, "demo", ".flashheart", "answers")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "claude--s2.jsonl"), []byte(`{"id":"q2","run":"claude:s2","question":"which?","answer":"that","by":"human"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var diagnostics bytes.Buffer
	pruneExpired(files, time.Now().Add(11*time.Hour), 90, &diagnostics)
	// Retention moves old inboxes into the inbox the prompt hook checks.
	if names := listing("inbox"); !slices.Equal(names, []string{"claude--s1.jsonl", "claude--s2.jsonl"}) {
		t.Fatalf("inboxes after retention = %v", names)
	}
	if names := listing("answers"); len(names) != 0 {
		t.Fatalf("old inboxes after retention = %v, want moved", names)
	}
	if names := listing("activity"); len(names) != 1 {
		t.Fatalf("activity state of a session silent for 11 hours = %v, want kept", names)
	}
	pruneExpired(files, time.Now().Add(13*time.Hour), 90, &diagnostics)
	if names := listing("activity"); len(names) != 0 {
		t.Fatalf("activity state of a stale session = %v, want removed", names)
	}
	if names := listing("inbox"); len(names) != 2 {
		t.Fatalf("inbox = %v, want kept until its day's events expire", names)
	}
	pruneExpired(files, time.Now().AddDate(0, 0, 92), 90, &diagnostics)
	if names := listing("inbox"); len(names) != 0 {
		t.Fatalf("inbox = %v, want removed with its day's events", names)
	}
	if strings.TrimSpace(diagnostics.String()) != "" {
		t.Fatalf("diagnostics = %s", diagnostics.String())
	}
}

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
	pruneExpired(files, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), 90, &diagnostics)
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
