package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// reopen is a new index over root's board and event logs.
func reopen(t *testing.T, root string, clock *fakeClock) *Index {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s, Options{MaxAge: time.Hour, Now: clock.Now, Events: events.New(s), Runs: runs.DefaultSettings()})
}

// The Overview's headline metrics count from the human's latest board
// activity in the scope, through files older than the runs' window, or over
// the last day when there is none; the human's own activity moves the
// anchor, and a new day's re-read counts nothing twice (FH-52).
func TestSnapshotCountsChangesSinceTheHumansLastChange(t *testing.T) {
	t.Parallel()

	index, log, clock, root := runsBoard(t)
	if err := os.MkdirAll(filepath.Join(root, "beta", "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "beta", "project.yaml"), []byte("key: BE\nnext_id: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := clock.now
	agent := func(at time.Time, project, kind string, data events.TicketData) events.Event {
		data.By = "claude:s1"
		return events.Event{Time: at, Run: "claude:s1", Agent: "claude", Kind: kind, Project: project, Data: data}
	}
	moved := func(at time.Time, project, ticket, to string) events.Event {
		return agent(at, project, events.TicketMoved, events.TicketData{Ticket: ticket, From: "in-progress", To: to})
	}
	ticked := func(at time.Time, project, ticket string) events.Event {
		return agent(at, project, events.TicketUpdated, events.TicketData{Ticket: ticket, Fields: []string{"criteria"}})
	}
	created := func(at time.Time, project, ticket string) events.Event {
		return agent(at, project, events.TicketCreated, events.TicketData{Ticket: ticket})
	}
	check := func(label, project string, want Metrics) {
		t.Helper()
		snapshot, err := index.Rebuild()
		if err != nil {
			t.Fatal(err)
		}
		if got := snapshot.Metrics(project); !got.Since.Equal(want.Since) || got.LastChange != want.LastChange || got.ChangeCounts != want.ChangeCounts {
			t.Errorf("%s: Metrics(%q) = %+v, want %+v", label, project, got, want)
		}
	}

	// No human activity: the last day counts.
	if err := log.Append(
		moved(now.Add(-30*time.Hour), "alpha", "AL-1", "review"),
		moved(now.Add(-2*time.Hour), "alpha", "AL-3", "review"),
		ticked(now.Add(-time.Hour), "beta", "BE-1"),
	); err != nil {
		t.Fatal(err)
	}
	day := now.Add(-24 * time.Hour)
	check("no activity", "", Metrics{Since: day, ChangeCounts: events.ChangeCounts{Review: 1, Criteria: 1}})
	check("no activity", "alpha", Metrics{Since: day, ChangeCounts: events.ChangeCounts{Review: 1}})

	// The human last changed alpha five days ago, before the runs' window:
	// alpha counts from then, through the older files; beta still the day.
	old := now.AddDate(0, 0, -5)
	if err := log.Append(
		moved(old.Add(-time.Hour), "alpha", "AL-0", "review"),
		events.ByHuman(old, "alpha", events.TicketMoved, events.TicketData{Ticket: "AL-0", From: "review", To: "done"}),
		created(old.Add(time.Hour), "alpha", "AL-2"),
		ticked(now.AddDate(0, 0, -4), "alpha", "AL-2"),
		ticked(now.AddDate(0, 0, -4).Add(time.Minute), "alpha", "AL-2"),
	); err != nil {
		t.Fatal(err)
	}
	// Files before the window are read once, when a project is first seen:
	// a new index (a restarted serve) finds what was written into them.
	index = reopen(t, root, clock)
	alpha := events.ChangeCounts{Review: 2, Created: 1, Criteria: 2}
	check("an old change", "alpha", Metrics{Since: old, LastChange: true, ChangeCounts: alpha})
	check("an old change", "beta", Metrics{Since: day, ChangeCounts: events.ChangeCounts{Criteria: 1}})
	all := events.ChangeCounts{Review: 2, Created: 1, Criteria: 3}
	check("an old change", "", Metrics{Since: old, LastChange: true, ChangeCounts: all})

	// A new day re-reads the window: nothing counts twice.
	clock.now = now.Add(12 * time.Hour)
	check("a new day", "", Metrics{Since: old, LastChange: true, ChangeCounts: all})

	// An agent's change arrives incrementally; the human's own move then
	// becomes the anchor and leaves nothing since.
	if err := log.Append(moved(clock.now.Add(-time.Minute), "alpha", "AL-3", "done")); err != nil {
		t.Fatal(err)
	}
	alpha.Done++
	check("a new change", "alpha", Metrics{Since: old, LastChange: true, ChangeCounts: alpha})
	answered := clock.now
	if err := log.Append(events.Event{Time: answered, Run: "claude:s2", Agent: "claude", Kind: events.QuestionAnswered, Project: "beta", Data: events.AnswerData{ID: "q-1", Answer: "Yes", By: "Robert"}}); err != nil {
		t.Fatal(err)
	}
	check("the human answered", "", Metrics{Since: answered, LastChange: true})
	check("the human answered", "alpha", Metrics{Since: old, LastChange: true, ChangeCounts: alpha})
}
