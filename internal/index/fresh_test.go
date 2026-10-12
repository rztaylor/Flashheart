package index

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
)

// countingSource counts the board reads behind an index (FH-96).
type countingSource struct {
	Source
	reads atomic.Int64
}

func (c *countingSource) ReadBoard() (board.Board, string, error) {
	c.reads.Add(1)
	return c.Source.ReadBoard()
}

// eventsBoard is runsBoard's root with an index over a counting source, on
// the real clock so a watcher folds what is appended now.
func eventsBoard(t *testing.T, maxAge time.Duration) (*Index, *countingSource, *events.Log, string) {
	t.Helper()
	root, files := runsRoot(t)
	source := &countingSource{Source: files}
	log := events.New(files)
	return New(source, Options{MaxAge: maxAge, Events: log, Runs: runs.DefaultSettings()}), source, log, root
}

func appendNow(t *testing.T, log *events.Log, kind events.Kind, data any) {
	t.Helper()
	appendEvent(t, log, time.Now(), kind, data)
}

// waitPast waits up to a second for a revision newer than since.
func waitPast(t *testing.T, index *Index, since uint64) uint64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	revision := index.Wait(ctx, since)
	if revision <= since {
		t.Fatalf("no revision past %d within a second", since)
	}
	return revision
}

// Hook-cadence event appends refold the runs over the board already read:
// the watcher reads no ticket for them (FH-96).
func TestWatchFoldsAppendedEventsWithoutReadingTheBoard(t *testing.T) {
	t.Parallel()

	index, source, log, root := eventsBoard(t, time.Hour)
	watch(t, index, root)
	before, err := index.Current()
	if err != nil {
		t.Fatal(err)
	}
	reads := source.reads.Load()

	// The first append also creates the project's event directory.
	appendNow(t, log, events.RunStart, events.RunStartData{Kind: events.KindSession, Branch: "feature/x"})
	started := waitPast(t, index, before.Revision)
	appendNow(t, log, events.TurnStart, events.TurnStartData{})
	// Past the debounce, so the two appends may fold in separate windows,
	// as they can on a loaded machine (FH-111).
	time.Sleep(3 * debounce)
	appendNow(t, log, events.ToolUsed, events.ToolData{Tool: "Edit", OK: true, Path: "a.go"})

	// Each window publishes a revision, so wait for the run, not for one.
	folded := func(snapshot *Snapshot) bool {
		live, ok := snapshot.TicketRun("AL-3")
		return len(snapshot.Runs) == 1 && snapshot.Runs[0].State == runs.Working && ok && live.Tools == 1
	}
	snapshot, _ := index.Current()
	for revision := started; !folded(snapshot); snapshot, _ = index.Current() {
		revision = waitPast(t, index, revision)
	}
	if got := source.reads.Load() - reads; got != 0 {
		t.Errorf("appending events read the board %d times, want 0", got)
	}
}

// An event that records a ticket write still reads the board: the ticket's
// folder may be beyond the watch budget (FH-96).
func TestWatchReadsTheBoardForATicketEvent(t *testing.T) {
	t.Parallel()

	index, source, log, root := eventsBoard(t, time.Hour)
	appendNow(t, log, events.RunStart, events.RunStartData{Kind: events.KindSession, Branch: "feature/x"})
	watch(t, index, root)
	reads := source.reads.Load()

	appendNow(t, log, events.TicketUpdated, events.TicketData{Ticket: "AL-3", Fields: []string{"priority"}})
	deadline := time.Now().Add(time.Second)
	for source.reads.Load() == reads {
		if time.Now().After(deadline) {
			t.Fatal("a ticket event did not read the board within a second")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// While the watcher runs it keeps the snapshot fresh, so reads serve the
// snapshot as it is; once it stops, MaxAge applies again (FH-96).
func TestCurrentDoesNotRebuildWhileWatching(t *testing.T) {
	t.Parallel()

	index, source, _, root := eventsBoard(t, time.Nanosecond)
	stop := watch(t, index, root)
	reads := source.reads.Load()
	for range 3 {
		if snapshot, err := index.Current(); err != nil || snapshot == nil {
			t.Fatalf("Current = %v, %v", snapshot, err)
		}
	}
	if got := source.reads.Load() - reads; got != 0 {
		t.Errorf("Current read the board %d times while watching, want 0", got)
	}

	stop()
	reads = source.reads.Load()
	if _, err := index.Current(); err != nil {
		t.Fatal(err)
	}
	if got := source.reads.Load() - reads; got != 1 {
		t.Errorf("Current read the board %d times after watching stopped, want 1", got)
	}
}

// An edit to a ticket the watcher already sees still moves the revision,
// and with it the board is read again (STO-7).
func TestWatchReadsTheBoardForATicketEdit(t *testing.T) {
	t.Parallel()

	index, source, _, root := eventsBoard(t, time.Hour)
	watch(t, index, root)
	before, _ := index.Current()
	reads := source.reads.Load()
	name := filepath.Join(root, "alpha", "tickets", "AL-3-card-panel", "AL-3-card-panel.md")
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, append(data, []byte("\nEdited.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	waitPast(t, index, before.Revision)
	if source.reads.Load() == reads {
		t.Error("the edit was not read")
	}
}

// RefreshRuns folds new events over the board already read (FH-96).
func TestRefreshRunsReadsNoTicket(t *testing.T) {
	t.Parallel()

	root, files := runsRoot(t)
	source := &countingSource{Source: files}
	log := events.New(files)
	clock := &fakeClock{now: time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)}
	index := New(source, Options{MaxAge: time.Hour, Now: clock.Now, Events: log, Runs: runs.DefaultSettings()})
	first, err := index.RefreshRuns()
	if err != nil || source.reads.Load() != 1 {
		t.Fatalf("first refresh: %v, %d reads, want the board read once", err, source.reads.Load())
	}

	appendEvent(t, log, clock.now.Add(-time.Minute), events.RunStart, events.RunStartData{Kind: events.KindSession, Branch: "feature/x"})
	appendEvent(t, log, clock.now.Add(-time.Minute), events.TurnStart, events.TurnStartData{})
	refreshed, err := index.RefreshRuns()
	if err != nil || source.reads.Load() != 1 {
		t.Fatalf("refresh: %v, %d reads, want still 1", err, source.reads.Load())
	}
	if refreshed.Revision != first.Revision+1 || len(refreshed.Runs) != 1 || refreshed.Runs[0].Link.Ticket != "AL-3" {
		t.Fatalf("revision %d → %d, runs %+v", first.Revision, refreshed.Revision, refreshed.Runs)
	}
	if _, _, ok := refreshed.FindTicket("AL-3"); !ok {
		t.Fatal("the refreshed snapshot lost the board")
	}
	if same, _ := index.RefreshRuns(); same.Revision != refreshed.Revision {
		t.Fatal("revision moved with nothing new")
	}
	// A board change alone still moves the revision.
	name := filepath.Join(root, "alpha", "tickets", "AL-3-card-panel", "AL-3-card-panel.md")
	data, _ := os.ReadFile(name)
	if err := os.WriteFile(name, append(data, []byte("\nEdited.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if rebuilt, _ := index.Rebuild(); rebuilt.Revision != refreshed.Revision+1 {
		t.Fatalf("board edit: revision %d → %d", refreshed.Revision, rebuilt.Revision)
	}

	// Without a watcher, a stale board is read as Current would.
	clock.now = clock.now.Add(2 * time.Hour)
	if _, err := index.RefreshRuns(); err != nil || source.reads.Load() != 3 {
		t.Fatalf("stale refresh: %v, %d reads, want 3", err, source.reads.Load())
	}
}

// A board read started after a change was seen covers it; a failed one
// does not (FH-96).
func TestCoveredNeedsASuccessfulReadAfterTheMark(t *testing.T) {
	t.Parallel()

	source := &fakeSource{fingerprint: "a"}
	index, _ := newIndex(source)
	mark := index.reads.Load()
	if index.covered(mark) {
		t.Fatal("covered before any read")
	}
	if _, err := index.Rebuild(); err != nil {
		t.Fatal(err)
	}
	if !index.covered(mark) {
		t.Fatal("a read after the mark does not cover it")
	}
	mark = index.reads.Load()
	if index.covered(mark) {
		t.Fatal("a read before the mark covers it")
	}
	source.set("", errors.New("permission denied"))
	_, _ = index.Rebuild()
	if index.covered(mark) {
		t.Fatal("a failed read covers the mark")
	}
}

func TestClassifyWatchEvents(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]change{
		"/b/alpha/tickets/AL-1-a/AL-1-a.md":            boardChanged,
		"/b/alpha/workstreams/ui.md":                   boardChanged,
		"/b/alpha/.flashheart/events/2026-10-05.jsonl": runsChanged,
		"/b/alpha/.flashheart/events":                  logMoved,
		"/b/alpha/.flashheart":                         logMoved,
		"/b/alpha/.flashheart/lock":                    unchanged,
		"/b/alpha/.flashheart/activity/s1.json":        unchanged,
		"/b/.flashheart/retired.yaml":                  unchanged,
	} {
		if got := classify(name); got != want {
			t.Errorf("classify(%q) = %d, want %d", name, got, want)
		}
	}
}

// The watcher reads the board only for a board change no read has seen,
// or one in a folder it has just started watching; event appends refold.
func TestPendingChangesPlanTheLeastWork(t *testing.T) {
	t.Parallel()

	note := func(changes ...change) pending {
		var p pending
		for _, c := range changes {
			p.note(c, 7)
		}
		return p
	}
	cases := []struct {
		name           string
		pending        pending
		covered, added bool
		want           action
	}{
		{"nothing", pending{}, false, false, nothing},
		{"ignored files", note(unchanged), false, false, nothing},
		{"an event append", note(runsChanged), false, false, refoldRuns},
		{"a new event log", note(logMoved), false, true, refoldRuns},
		{"a board edit", note(boardChanged), false, false, readBoard},
		{"a board edit a write already read", note(boardChanged), true, false, nothing},
		{"a write and its event", note(boardChanged, runsChanged), true, false, refoldRuns},
		{"a new folder a write already read", note(boardChanged), true, true, readBoard},
		{"lost events", pending{force: true}, true, false, readBoard},
	}
	for _, tc := range cases {
		if got := tc.pending.plan(tc.covered, tc.added); got != tc.want {
			t.Errorf("%s: plan = %d, want %d", tc.name, got, tc.want)
		}
	}
	if p := note(boardChanged); p.mark != 7 {
		t.Errorf("mark = %d, want the reads when the change arrived", p.mark)
	}
}

// Event logs are watched within the folder budget, ahead of ticket
// folders; .flashheart stands in for a log not yet created (FH-96).
func TestEventLogsCountAgainstTheFolderBudget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir := func(parts ...string) string {
		t.Helper()
		path := filepath.Join(append([]string{root}, parts...)...)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	alphaLog := mkdir("alpha", ".flashheart", "events")
	betaDot := mkdir("beta", ".flashheart")
	mkdir("gamma", "tickets")
	base := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	var folders []string
	for n := range 3 {
		folder := mkdir("alpha", "tickets", fmt.Sprintf("AL-%d-x", n+1))
		if err := os.Chtimes(folder, base, base.Add(time.Duration(n)*time.Minute)); err != nil {
			t.Fatal(err)
		}
		folders = append(folders, folder)
	}

	want := watchSet(root, 3)
	for _, path := range []string{root, filepath.Join(root, "alpha"), filepath.Join(root, "alpha", "tickets"), alphaLog, betaDot, folders[2]} {
		if !want[path] {
			t.Errorf("%s is not watched", path)
		}
	}
	for _, path := range []string{folders[0], folders[1], filepath.Dir(alphaLog)} {
		if want[path] {
			t.Errorf("%s is watched beyond the budget", path)
		}
	}
	if got := len(watchSet(root, 1)); got != len(want)-2 {
		t.Errorf("a budget of 1 watches %d paths, want %d", got, len(want)-2)
	}
}

// Runs are found by id, and a run's subagents by their parent (FH-96).
func TestSnapshotFindsRunsAndSubagents(t *testing.T) {
	t.Parallel()

	snapshot := foldSnapshot(concat(sessionStart(30, "claude:s1", "feature/x"), sessionStart(20, "claude:s2", "main"), []events.Event{
		subagentStart(10, "claude:s1/a1"),
		subagentStart(8, "claude:s1/a2"),
	}))
	if run, ok := snapshot.Run("claude:s2"); !ok || run.ID != "claude:s2" {
		t.Fatalf("Run(claude:s2) = %+v, %v", run, ok)
	}
	if _, ok := snapshot.Run("claude:s9"); ok {
		t.Fatal("found a run that is not there")
	}
	var ids []string
	for _, run := range snapshot.Subagents("claude:s1") {
		ids = append(ids, run.ID)
	}
	if fmt.Sprint(ids) != "[claude:s1/a1 claude:s1/a2]" {
		t.Errorf("Subagents = %v", ids)
	}
	if family := snapshot.Family("claude:s1"); len(family) != 3 || family[0].ID != "claude:s1" {
		t.Errorf("Family = %+v", family)
	}
	if len(snapshot.Subagents("claude:s2")) != 0 || len(snapshot.TicketRuns("")) != 0 {
		t.Error("runs found for no parent or no ticket")
	}
}
