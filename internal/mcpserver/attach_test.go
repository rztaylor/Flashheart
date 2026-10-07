package mcpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/store"
)

// png is a minimal PNG: a signature and an empty IHDR-less body is enough
// for the allow-list, which goes by type.
var png = []byte("\x89PNG\r\n\x1a\n0000")

func TestAttachCopiesAFileIntoTheTicket(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Board"})
	e.startSession(session)
	dir := t.TempDir()
	shot := filepath.Join(dir, "board desktop.png")
	if err := os.WriteFile(shot, png, 0o644); err != nil {
		t.Fatal(err)
	}
	out := e.ok("attach", map[string]any{"ticket": id, "path": shot, "caption": "Board at desktop"})
	contains(t, out, "Attached files/", "![Board at desktop](files/", "ok ticket="+id)
	index := e.readFile("demo/tickets/" + id + "-board/files/index.yaml")
	sum := sha256.Sum256(png)
	for _, part := range []string{"caption: Board at desktop", "kind: screenshot", "sha256: " + hex.EncodeToString(sum[:]), "source: " + shot} {
		if !strings.Contains(index, part) {
			t.Errorf("index.yaml missing %q:\n%s", part, index)
		}
	}
	// The original stays where it was: a copy, never a move.
	if _, err := os.Stat(shot); err != nil {
		t.Errorf("original moved: %v", err)
	}
	// A log is attached as a link with its kind.
	logFile := filepath.Join(dir, "run.log")
	if err := os.WriteFile(logFile, []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	contains(t, e.ok("attach", map[string]any{"ticket": id, "path": logFile, "caption": "Test run", "kind": "log"}), "[Test run](files/")

	// The event names the file and its kind (agent-protocol §3).
	logs, _ := filepath.Glob(filepath.Join(e.root, "demo", ".flashheart", "events", "*.jsonl"))
	var recorded string
	for _, name := range logs {
		data, _ := os.ReadFile(name)
		recorded += string(data)
	}
	if !strings.Contains(recorded, `"kind":"attachment.added","project":"demo","data":{"ticket":"`+id+`","file":"`) || !strings.Contains(recorded, `"kind":"screenshot"}`) {
		t.Errorf("events:\n%s", recorded)
	}

	// Brackets in a caption do not break the snippet.
	other := filepath.Join(dir, "other.png")
	if err := os.WriteFile(other, append(png, '1'), 0o644); err != nil {
		t.Fatal(err)
	}
	contains(t, e.ok("attach", map[string]any{"ticket": id, "path": other, "caption": "Board [after]"}), `![Board \[after\]](files/`)
}

func TestAttachRefusals(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	write(t, e.root+"/.flashheart/config.yaml", "attachments:\n  max_bytes: 8\n")
	id := e.ticket(store.NewTicket{Title: "Board"})
	e.startSession(session)
	dir := t.TempDir()
	file := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	e.fails("attach", map[string]any{"ticket": id, "caption": "x", "path": file("logo.svg", "<svg/>")}, "type_not_allowed")
	e.fails("attach", map[string]any{"ticket": id, "caption": "x", "path": file("page.html", "<p>")}, "type_not_allowed")
	e.fails("attach", map[string]any{"ticket": id, "caption": "x", "path": file("big.png", string(png)+"more than eight bytes")}, "too_large")
	folder := filepath.Join(dir, "shots.png")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	e.fails("attach", map[string]any{"ticket": id, "caption": "x", "path": folder}, "invalid_input")
	e.fails("attach", map[string]any{"ticket": id, "caption": "x", "path": "relative.png"}, "invalid_input")
	e.fails("attach", map[string]any{"ticket": id, "caption": "x", "path": file("ok.png", "x"), "kind": "video"}, "invalid_input")
	e.rivalHolds(id)
	e.fails("attach", map[string]any{"ticket": id, "caption": "x", "path": file("held.png", "x")}, "claimed")
}
