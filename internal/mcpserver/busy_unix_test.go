//go:build unix

package mcpserver

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/store"
)

// Every write tool answers busy, with a fix, while another writer holds the
// project's lock past LockWait (agent-protocol §7.2). Not parallel: it
// shortens the package-wide LockWait, and parallel tests run after it.
func TestEveryWriteToolReportsBusy(t *testing.T) {
	old := store.LockWait
	store.LockWait = 100 * time.Millisecond
	t.Cleanup(func() { store.LockWait = old })

	e := newEnv(t, "DM")
	held := e.ticket(store.NewTicket{Title: "Mine", Criteria: []string{"one"}})
	other := e.ticket(store.NewTicket{Title: "Other"})
	e.startSession(session)
	e.ok("claim", map[string]any{"ticket": held})

	lock, err := os.OpenFile(e.root+"/demo/.flashheart/lock", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	for _, call := range []struct {
		tool string
		args map[string]any
	}{
		{"claim", map[string]any{"ticket": other}},
		{"release", map[string]any{"ticket": held}},
		{"checkpoint", map[string]any{"ticket": held, "done": []string{"x"}}},
		{"update_ticket", map[string]any{"ticket": held, "check": []string{"1"}}},
		{"move", map[string]any{"ticket": held, "to": "review"}},
		{"set_project_key", map[string]any{"key": "DM"}},
		{"create_ticket", map[string]any{"type": "bug", "title": "X", "description": "x", "criteria": []string{"y"}, "priority": "low"}},
		{"create_workstream", map[string]any{"title": "Line", "goal": "g"}},
		{"write_review", map[string]any{"ticket": held, "markdown": "# Review: Mine\n"}},
		{"ask_human", map[string]any{"kind": "question", "text": "Well?"}},
	} {
		e.fails(call.tool, call.args, "busy")
	}
}
