package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// A session that claimed its ticket 30 hours ago, early enough in the day
// that the claim sits two event files back, is still linked by its claim
// at a stop and at a resume, as the board and the MCP server see it
// (runs.Lookback).
func TestAThirtyHourOldClaimStillLinksTheSession(t *testing.T) {
	t.Parallel()

	clock := time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC)
	claimed := clock.Add(-30 * time.Hour)
	if events.FileName(claimed) == events.FileName(clock.Add(-24*time.Hour)) {
		t.Fatal("the claim must sit in an older file than the last day's")
	}
	at := func(t time.Time) func() time.Time { return func() time.Time { return t } }
	hook := func(t *testing.T, root, cwd, event, session string, now time.Time) string {
		t.Helper()
		var stdout strings.Builder
		Run(Options{Root: root, Event: event, Stdin: strings.NewReader(session + " " + cwd), Stdout: &stdout, Now: at(now), Adapter: fake{}, ChangeTimeout: 10 * time.Second})
		return stdout.String()
	}
	setup := func(t *testing.T) (string, string) {
		root, cwd := t.TempDir(), repo(t)
		// Another session creates the project.
		hook(t, root, cwd, "SessionStart", "s0", claimed.Add(-time.Minute))
		project := "name: demo\nkey: DM\nnext_id: 2\nrepos:\n  - " + cwd + "\nsettings:\n  enforce_handoff: true\n"
		if err := os.WriteFile(filepath.Join(root, "demo", "project.yaml"), []byte(project), 0o644); err != nil {
			t.Fatal(err)
		}
		// No branch: the session is linked by its claim alone.
		writeTicket(t, root, "demo", "DM-1-card-panel", "---\nid: DM-1\nstatus: in-progress\ntype: feature\npriority: high\ncreated: 2026-10-01\n---\n# Card panel\n")
		s, err := store.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		if err := events.New(s).Append(
			events.Event{Time: claimed, Run: "fake:s1", Agent: "fake", Kind: events.RunStart, Project: "demo", Data: events.RunStartData{Kind: events.KindSession, Cwd: cwd, Branch: "feature/demo", Worktree: cwd, Source: "startup"}},
			events.Event{Time: claimed, Run: "fake:s1", Agent: "fake", Kind: events.Claim, Project: "demo", Data: events.TicketData{Ticket: "DM-1", Home: "demo"}},
		); err != nil {
			t.Fatal(err)
		}
		return root, cwd
	}

	t.Run("handoff enforcement blocks the stop", func(t *testing.T) {
		t.Parallel()
		root, cwd := setup(t)
		hook(t, root, cwd, "Prompt", "s1", clock.Add(-5*time.Minute))
		hook(t, root, cwd, "Edit", "s1", clock.Add(-4*time.Minute))
		out := hook(t, root, cwd, "Stop", "s1", clock)
		if out != "block:Flashheart: record a checkpoint on DM-1 (done, next, files) before stopping." {
			t.Fatalf("stop output = %q", out)
		}
		if log := errorLog(t, root); log != "" {
			t.Fatal(log)
		}
	})
	t.Run("the recovery note links the claim", func(t *testing.T) {
		t.Parallel()
		root, cwd := setup(t)
		out := hook(t, root, cwd, "SessionStart", "s1", clock)
		if !strings.Contains(out, `Ticket DM-1 "Card panel" (in-progress, linked by claim).`) {
			t.Fatalf("recovery note = %q", out)
		}
	})
}
