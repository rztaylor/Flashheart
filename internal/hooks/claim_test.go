package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/store"
)

// D50, FH-132: a session that changed files while holding no ticket, by
// claim or by branch, is asked once to claim one. Where the project
// enforces handoffs the reminder holds its stop; otherwise it goes with
// the session's next prompt or tool result. It is never repeated, and
// subagents are never told.
func TestAnEditingSessionWithoutATicketIsAskedOnceToClaimOne(t *testing.T) {
	t.Parallel()

	const enforce = "settings:\n  enforce_handoff: true\n"
	// setup makes the project with a ticket on another branch, so nothing
	// links the session unless a test says so.
	setup := func(t *testing.T, cwd, settings string) string {
		t.Helper()
		root := t.TempDir()
		run(t, root, "SessionStart", "s1 "+cwd, fake{})
		project := "name: demo\nkey: DM\nnext_id: 2\nrepos:\n  - " + cwd + "\n" + settings
		if err := os.WriteFile(filepath.Join(root, "demo", "project.yaml"), []byte(project), 0o644); err != nil {
			t.Fatal(err)
		}
		writeTicket(t, root, "demo", "DM-1-card-panel", "---\nid: DM-1\nstatus: in-progress\ntype: feature\npriority: high\ncreated: 2026-10-01\nbranch: feature/other\n---\n# Card panel\n")
		return root
	}
	hook := func(t *testing.T, root, cwd, event string) string {
		t.Helper()
		return run(t, root, event, "s1 "+cwd, fake{})
	}
	reminded := func(out string) bool { return strings.Contains(out, protocol.ClaimReminder) }

	t.Run("without enforcement the next prompt carries it, once", func(t *testing.T) {
		t.Parallel()
		cwd := repo(t)
		root := setup(t, cwd, "")
		hook(t, root, cwd, "Prompt")
		hook(t, root, cwd, "Edit")
		if out := hook(t, root, cwd, "Stop"); out != "" {
			t.Fatalf("stop output = %q, want nothing", out)
		}
		if out := hook(t, root, cwd, "Prompt"); out != "context:"+protocol.ClaimReminder+"\n" {
			t.Fatalf("prompt output = %q, want the reminder", out)
		}
		hook(t, root, cwd, "Edit:src/b.ts")
		hook(t, root, cwd, "Stop")
		if out := hook(t, root, cwd, "Prompt"); out != "" {
			t.Fatalf("second prompt output = %q, want nothing", out)
		}
		if log := errorLog(t, root); log != "" {
			t.Fatalf("hook errors: %s", log)
		}
	})
	t.Run("a tool result of the session's next turn carries it as well", func(t *testing.T) {
		t.Parallel()
		cwd := repo(t)
		root := setup(t, cwd, "")
		hook(t, root, cwd, "Edit")
		hook(t, root, cwd, "Stop")
		// Another hook kept the turn going: no prompt comes first.
		if out := hook(t, root, cwd, "Read"); !reminded(out) {
			t.Fatalf("tool result output = %q, want the reminder", out)
		}
		if out := hook(t, root, cwd, "Prompt"); out != "" {
			t.Fatalf("prompt output = %q, want nothing", out)
		}
	})
	t.Run("a subagent's tool result does not carry it", func(t *testing.T) {
		t.Parallel()
		cwd := repo(t)
		root := setup(t, cwd, "")
		hook(t, root, cwd, "SubagentStart")
		hook(t, root, cwd, "Sub:Edit")
		hook(t, root, cwd, "Stop")
		if out := hook(t, root, cwd, "Sub:Read"); out != "" {
			t.Fatalf("subagent tool result output = %q, want nothing", out)
		}
		// The session itself is asked: its subagents' work is its own.
		if out := hook(t, root, cwd, "Prompt"); !reminded(out) {
			t.Fatalf("prompt output = %q, want the reminder", out)
		}
	})
	t.Run("with enforcement it holds the stop once", func(t *testing.T) {
		t.Parallel()
		cwd := repo(t)
		root := setup(t, cwd, enforce)
		hook(t, root, cwd, "Prompt")
		hook(t, root, cwd, "Edit")
		if out := hook(t, root, cwd, "Stop"); out != "block:"+protocol.ClaimReminder {
			t.Fatalf("stop output = %q, want the reminder as a block", out)
		}
		if out := hook(t, root, cwd, "Stop"); out != "" {
			t.Fatalf("second stop output = %q, want nothing", out)
		}
		hook(t, root, cwd, "Prompt")
		hook(t, root, cwd, "Edit:src/b.ts")
		if out := hook(t, root, cwd, "Stop"); out != "" {
			t.Fatalf("a later turn's stop output = %q, want nothing", out)
		}
		if out := hook(t, root, cwd, "Prompt"); out != "" {
			t.Fatalf("prompt output = %q, want nothing", out)
		}
		// The held stop is not a handoff block.
		for _, e := range readEvents(t, root, "demo") {
			var data events.TurnEndData
			if e.Kind == events.TurnEnd && e.Decode(&data) == nil && data.BlockedForHandoff {
				t.Fatalf("turn end recorded as blocked for handoff: %+v", data)
			}
		}
	})
	t.Run("a stop the agent is already continuing gets it as a note", func(t *testing.T) {
		t.Parallel()
		cwd := repo(t)
		root := setup(t, cwd, enforce)
		hook(t, root, cwd, "Edit")
		if out := hook(t, root, cwd, "StopActive"); out != "" {
			t.Fatalf("stop output = %q, want nothing", out)
		}
		if out := hook(t, root, cwd, "Prompt"); !reminded(out) {
			t.Fatalf("prompt output = %q, want the reminder", out)
		}
	})
	t.Run("a session that changed nothing is not asked", func(t *testing.T) {
		t.Parallel()
		cwd := repo(t)
		root := setup(t, cwd, enforce)
		hook(t, root, cwd, "Read")
		hook(t, root, cwd, "Bash")
		if out := hook(t, root, cwd, "Stop"); out != "" {
			t.Fatalf("stop output = %q, want nothing", out)
		}
		if out := hook(t, root, cwd, "Prompt"); out != "" {
			t.Fatalf("prompt output = %q, want nothing", out)
		}
	})
	for _, settings := range []string{"", enforce} {
		name := map[string]string{"": "", enforce: " (enforcing)"}[settings]
		t.Run("a claim spares it"+name, func(t *testing.T) {
			t.Parallel()
			cwd := repo(t)
			root := setup(t, cwd, settings)
			s, err := store.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			err = events.New(s).Append(events.Event{Time: now, Run: "fake:s1", Agent: "fake", Kind: events.Claim, Project: "demo", Data: events.TicketData{Ticket: "DM-1", Home: "demo"}})
			s.Close()
			if err != nil {
				t.Fatal(err)
			}
			hook(t, root, cwd, "Edit")
			if out := hook(t, root, cwd, "Stop"); reminded(out) {
				t.Fatalf("stop output = %q", out)
			}
			if out := hook(t, root, cwd, "Prompt"); out != "" {
				t.Fatalf("prompt output = %q, want nothing", out)
			}
		})
		t.Run("a branch match spares it"+name, func(t *testing.T) {
			t.Parallel()
			cwd := repo(t)
			root := setup(t, cwd, settings)
			writeTicket(t, root, "demo", "DM-2-runs-tab", "---\nid: DM-2\nstatus: in-progress\ntype: feature\npriority: high\ncreated: 2026-10-01\nbranch: feature/demo\n---\n# Runs tab\n")
			hook(t, root, cwd, "Edit")
			if out := hook(t, root, cwd, "Stop"); reminded(out) {
				t.Fatalf("stop output = %q", out)
			}
			if out := hook(t, root, cwd, "Prompt"); out != "" {
				t.Fatalf("prompt output = %q, want nothing", out)
			}
		})
	}
}

// A session that changed files only through shell commands is asked too:
// the turn end's worktree check (FH-53) finds the change.
func TestAShellOnlySessionWithoutATicketIsAskedToClaimOne(t *testing.T) {
	t.Parallel()

	for _, write := range []bool{true, false} {
		t.Run(map[bool]string{true: "changed", false: "read only"}[write], func(t *testing.T) {
			t.Parallel()
			root, cwd := t.TempDir(), gitRepo(t)
			play(t, root, cwd, "0 SessionStart")
			steps := []string{"1 Prompt", "3 Bash", "4 Stop", "5 Prompt"}
			if write {
				steps = []string{"1 Prompt", "2 touch b.txt", "3 Bash", "4 Stop", "5 Prompt"}
			}
			out := play(t, root, cwd, steps...)
			if got := strings.Contains(out, protocol.ClaimReminder); got != write {
				t.Fatalf("prompt output = %q, reminded want %v", out, write)
			}
		})
	}
}
