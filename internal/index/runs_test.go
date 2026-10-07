package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// runsBoard is a temporary root with project alpha (key AL), an in-progress
// ticket on branch feature/x, and an index over it with a fake clock.
func runsBoard(t *testing.T) (*Index, *events.Log, *fakeClock, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "alpha", "tickets", "AL-3-card-panel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "alpha", "project.yaml"), []byte("key: AL\nnext_id: 4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ticket := "---\nid: AL-3\nstatus: in-progress\ntype: feature\npriority: high\ncreated: 2026-10-01\nbranch: feature/x\n---\n# Card panel\n"
	if err := os.WriteFile(filepath.Join(dir, "AL-3-card-panel.md"), []byte(ticket), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	log := events.New(s)
	clock := &fakeClock{now: time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)}
	return New(s, Options{MaxAge: time.Hour, Now: clock.Now, Events: log, Runs: runs.DefaultSettings()}), log, clock, root
}

func appendEvent(t *testing.T, log *events.Log, at time.Time, kind string, data any) {
	t.Helper()
	if err := log.Append(events.Event{Time: at, Run: "claude:s1", Agent: "claude", Kind: kind, Project: "alpha", Data: data}); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotDerivesRunsFromTheEventLog(t *testing.T) {
	t.Parallel()

	index, log, clock, _ := runsBoard(t)
	empty, err := index.Rebuild()
	if err != nil || len(empty.Runs) != 0 {
		t.Fatalf("empty = %+v, %v", empty.Runs, err)
	}

	now := clock.now
	appendEvent(t, log, now.Add(-time.Minute), events.RunStart, events.RunStartData{Kind: events.KindSession, Branch: "feature/x", Worktree: "/src/alpha"})
	appendEvent(t, log, now.Add(-time.Minute), events.TurnStart, events.TurnStartData{})
	working, _ := index.Rebuild()
	if working.Revision == empty.Revision || len(working.Runs) != 1 {
		t.Fatalf("after events: revision %d → %d, runs %d", empty.Revision, working.Revision, len(working.Runs))
	}
	run := working.Runs[0]
	if run.State != runs.Working || run.Link != (runs.Link{Ticket: "AL-3", By: runs.LinkBranch}) || run.Project != "alpha" {
		t.Fatalf("run = %+v", run)
	}
	if live, ok := working.TicketRun("AL-3"); !ok || live.ID != "claude:s1" {
		t.Fatalf("TicketRun = %+v, %v", live, ok)
	}
	if got := working.TicketRuns("AL-3"); len(got) != 1 {
		t.Fatalf("TicketRuns = %d", len(got))
	}

	// The clock alone moves a run to Quiet, and the revision with it.
	clock.now = now.Add(15 * time.Minute)
	quiet, _ := index.Rebuild()
	if quiet.Revision == working.Revision || quiet.Runs[0].State != runs.Quiet {
		t.Fatalf("after quiet: revision %d, state %s", quiet.Revision, quiet.Runs[0].State)
	}
	same, _ := index.Rebuild()
	if same.Revision != quiet.Revision {
		t.Fatal("revision moved with nothing changed")
	}

	// Only new lines are read on later rebuilds.
	appendEvent(t, log, clock.now, events.PermissionRequested, events.PermissionData{Tool: "Bash"})
	needs, _ := index.Rebuild()
	if needs.Runs[0].State != runs.NeedsYou || needs.Runs[0].Permission != "Bash" {
		t.Fatalf("needs you = %+v", needs.Runs[0])
	}
	if counts := needs.RunCounts("alpha"); counts.NeedsYou != 1 || counts.Live != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	if counts := needs.RunCounts(""); counts.NeedsYou != 1 {
		t.Fatalf("all counts = %+v", counts)
	}
	// An ended run is no ticket's live run.
	appendEvent(t, log, clock.now, events.RunEnd, events.RunEndData{})
	ended, _ := index.Rebuild()
	if _, ok := ended.TicketRun("AL-3"); ok {
		t.Fatal("ended run is still live on the ticket")
	}
	if len(ended.TicketRuns("AL-3")) != 1 {
		t.Fatal("ended run left the ticket's history")
	}
}

func TestOldEventFilesAreNotLoaded(t *testing.T) {
	t.Parallel()

	index, log, clock, _ := runsBoard(t)
	appendEvent(t, log, clock.now.Add(-4*24*time.Hour), events.RunStart, events.RunStartData{Kind: events.KindSession})
	appendEvent(t, log, clock.now.Add(-time.Hour), events.TurnStart, events.TurnStartData{})
	snapshot, _ := index.Rebuild()
	if len(snapshot.Runs) != 1 || !snapshot.Runs[0].Started.Equal(clock.now.Add(-time.Hour)) {
		t.Fatalf("runs = %+v", snapshot.Runs)
	}
}

func TestWatchSeesAppendedEvents(t *testing.T) {
	t.Parallel()

	index, log, clock, root := runsBoard(t)
	appendEvent(t, log, clock.now, events.RunStart, events.RunStartData{Kind: events.KindSession})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	go index.Watch(ctx, root, ready)
	<-ready
	before, _ := index.Current()

	appendEvent(t, log, clock.now, events.TurnStart, events.TurnStartData{})
	waitCtx, stop := context.WithTimeout(ctx, 5*time.Second)
	defer stop()
	if revision := index.Wait(waitCtx, before.Revision); revision <= before.Revision {
		t.Fatal("appending an event did not move the revision")
	}
	snapshot, _ := index.Current()
	if snapshot.Runs[0].State != runs.Working {
		t.Fatalf("state = %s", snapshot.Runs[0].State)
	}
}

// A new UTC day starts the fold again from the window: recent runs stay,
// and nothing is counted twice.
func TestTrackerRollsOverAtMidnight(t *testing.T) {
	t.Parallel()

	index, log, clock, _ := runsBoard(t)
	appendEvent(t, log, clock.now, events.RunStart, events.RunStartData{Kind: events.KindSession})
	appendEvent(t, log, clock.now, events.ToolUsed, events.ToolData{Tool: "Edit", OK: true, Path: "a.ts"})
	first, _ := index.Rebuild()
	if len(first.Runs) != 1 || first.Runs[0].Tools != 1 {
		t.Fatalf("day one runs = %+v", first.Runs)
	}
	clock.now = clock.now.Add(11 * time.Hour) // past midnight UTC
	appendEvent(t, log, clock.now, events.ToolUsed, events.ToolData{Tool: "Read", OK: true})
	next, _ := index.Rebuild()
	if len(next.Runs) != 1 || next.Runs[0].Tools != 2 || next.Runs[0].Edits != 1 {
		t.Fatalf("after midnight runs = %+v", next.Runs)
	}
}

// A session working in alpha that claims a ticket in beta is one run: its
// activity at home keeps the beta ticket's live badge current (FH-8).
func TestARunSpanningProjectsIsOneRun(t *testing.T) {
	t.Parallel()

	index, log, clock, root := runsBoard(t)
	dir := filepath.Join(root, "beta", "tickets", "BE-1-hello")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "beta", "project.yaml"), []byte("key: BE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "BE-1-hello.md"), []byte("---\nid: BE-1\nstatus: in-progress\ntype: feature\npriority: low\ncreated: 2026-10-01\n---\n# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := clock.now
	appendEvent(t, log, now.Add(-50*time.Minute), events.RunStart, events.RunStartData{Kind: events.KindSession, Worktree: "/src/alpha"})
	if err := log.Append(events.Event{Time: now.Add(-45 * time.Minute), Run: "claude:s1", Agent: "claude", Kind: events.Claim, Project: "beta", Data: events.TicketData{Ticket: "BE-1"}}); err != nil {
		t.Fatal(err)
	}
	appendEvent(t, log, now.Add(-time.Minute), events.TurnStart, events.TurnStartData{})
	snapshot, err := index.Rebuild()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Runs) != 1 {
		t.Fatalf("runs = %d, want the one session: %+v", len(snapshot.Runs), snapshot.Runs)
	}
	live, ok := snapshot.TicketRun("BE-1")
	if !ok || live.State != runs.Working || live.Project != "alpha" || !live.LastActivity.Equal(now.Add(-time.Minute)) {
		t.Fatalf("BE-1's run = %+v, %v", live, ok)
	}
}

// A question from a run that has ended stays open and answerable on its
// ticket, marked as waiting for the session to resume (FH-8, RUN-8).
func TestQuestionsOfEndedRunsWaitForTheSession(t *testing.T) {
	t.Parallel()

	index, log, clock, _ := runsBoard(t)
	now := clock.now
	appendEvent(t, log, now.Add(-5*time.Minute), events.RunStart, events.RunStartData{Kind: events.KindSession})
	appendEvent(t, log, now.Add(-4*time.Minute), events.QuestionAsked, events.QuestionData{ID: "q-1", Ticket: "AL-3", Kind: "question", Text: "Which?"})
	live, _ := index.Rebuild()
	if got := live.Questions("AL-3"); len(got) != 1 || got[0].SessionEnded {
		t.Fatalf("live questions = %+v", got)
	}
	appendEvent(t, log, now.Add(-time.Minute), events.RunEnd, events.RunEndData{Reason: "other"})
	ended, _ := index.Rebuild()
	got := ended.Questions("AL-3")
	if len(got) != 1 || !got[0].SessionEnded {
		t.Fatalf("questions of an ended run = %+v", got)
	}
	if q, run, ok := ended.Question("q-1"); !ok || run.State != runs.Ended || q.ID != "q-1" {
		t.Fatalf("Question = %+v %+v %v", q, run, ok)
	}
}
