package index

import (
	"errors"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
)

// DefaultMaxAge is how old a snapshot's board may be before Current
// rebuilds it, when Watch is not keeping it fresh.
const DefaultMaxAge = 2 * time.Second

// Source reads the whole board with a fingerprint of the files read, and
// lists projects still in format v1.
type Source interface {
	ReadBoard() (board.Board, string, error)
	V1Projects() ([]string, error)
}

// Options configures an Index.
type Options struct {
	MaxAge time.Duration
	Now    func() time.Time
	// Events, when set, adds agent runs to snapshots, derived with Runs.
	Events EventSource
	Runs   runs.Settings
}

// Snapshot is an immutable view of the board at one revision.
type Snapshot struct {
	Revision    uint64
	BuiltAt     time.Time
	RootMissing bool
	// V1Projects lists projects that need `flashheart migrate` (MIG-1); while
	// any exist the board is not shown.
	V1Projects []string
	Board      board.Board
	Analysis   board.Analysis
	// Runs are the agent runs of the last two days' event logs at BuiltAt.
	Runs []runs.View

	// human is each project's latest human board activity (LatestHuman).
	human map[string]events.HumanActivity
	// lease is how long a claim outlives its run's last activity (Holder).
	lease time.Duration
	// runIndex finds Runs by ticket, id and parent (FH-96).
	runIndex runIndex

	projects map[string]int
	tickets  map[string][2]int // id → project index, ticket index (first copy)
	archived map[string]bool
}

// Project returns a project by directory name.
func (s *Snapshot) Project(name string) (*board.Project, bool) {
	index, ok := s.projects[name]
	if !ok {
		return nil, false
	}
	return &s.Board.Projects[index], true
}

// FindTicket returns a ticket by id with its project (the first copy in
// workflow order when an id is duplicated).
func (s *Snapshot) FindTicket(id string) (*board.Project, board.Ticket, bool) {
	position, ok := s.tickets[id]
	if !ok {
		return nil, board.Ticket{}, false
	}
	project := &s.Board.Projects[position[0]]
	return project, project.Tickets[position[1]], true
}

// Archived reports whether id names an archived ticket.
func (s *Snapshot) Archived(id string) bool { return s.archived[id] }

// LatestHuman returns the human's latest activity on the board in a project,
// or across every project when project is "": a ticket written on the board
// or an agent's question answered there (agent-protocol §3), from the whole
// retained event log. ok is false when there is none.
func (s *Snapshot) LatestHuman(project string) (activity events.HumanActivity, ok bool) {
	if project != "" {
		activity, ok = s.human[project]
		return activity, ok
	}
	for _, candidate := range s.human {
		if !ok || candidate.Time.After(activity.Time) {
			activity, ok = candidate, true
		}
	}
	return activity, ok
}

// Index caches the latest snapshot.
type Index struct {
	source Source
	maxAge time.Duration
	now    func() time.Time
	runs   runs.Settings
	events *tracker

	// reads counts board reads started, so the watcher can tell whether one
	// began after a change it saw (FH-96).
	reads atomic.Uint64

	mu      sync.Mutex
	current *Snapshot
	// boardPrint and runsPrint fingerprint current's board files and its
	// runs; the revision advances when either changes.
	boardPrint, runsPrint string
	// boardAt is when current's board was read, and boardRead which read
	// that was (a value of reads).
	boardAt   time.Time
	boardRead uint64
	// watching is set while Watch keeps the snapshot fresh: Current then
	// serves it without checking its age.
	watching bool
	// changed is closed and replaced whenever the revision advances.
	changed chan struct{}
}

// New returns an index over source. Nothing is read until first use.
func New(source Source, options Options) *Index {
	if options.MaxAge <= 0 {
		options.MaxAge = DefaultMaxAge
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	index := &Index{source: source, maxAge: options.MaxAge, now: options.Now, runs: options.Runs}
	if options.Events != nil {
		index.events = &tracker{source: options.Events}
		if index.runs == (runs.Settings{}) {
			index.runs = runs.DefaultSettings()
		}
	}
	return index
}

// Current returns the snapshot. While Watch runs it is served as it is: the
// watcher keeps it fresh. Otherwise it is rebuilt first if its board is
// older than MaxAge. If a rebuild fails the previous snapshot is returned
// with the error.
func (i *Index) Current() (*Snapshot, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.current != nil && (i.watching || i.now().Sub(i.boardAt) < i.maxAge) {
		return i.current, nil
	}
	return i.rebuildLocked()
}

// Rebuild reads the board and the new events now.
func (i *Index) Rebuild() (*Snapshot, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.rebuildLocked()
}

// RefreshRuns folds the events appended since the last read into the runs,
// over the board already read: no ticket is read. The board is read as
// Current would, when there is none yet or it is stale and Watch is not
// running.
func (i *Index) RefreshRuns() (*Snapshot, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.current == nil || i.current.RootMissing || (!i.watching && i.now().Sub(i.boardAt) >= i.maxAge) {
		return i.rebuildLocked()
	}
	snapshot, _ := i.refoldLocked()
	return snapshot, nil
}

// refold is the watcher's RefreshRuns. It reports whether a new event
// records a write to the board, which the watcher then reads: the write
// may be in a folder it does not watch.
func (i *Index) refold() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.current == nil || i.current.RootMissing {
		_, _ = i.rebuildLocked()
		return false
	}
	_, board := i.refoldLocked()
	return board
}

// covered reports whether the current board was read by a read that began
// after mark, a value of reads.
func (i *Index) covered(mark uint64) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.current != nil && i.boardRead > mark
}

func (i *Index) setWatching(watching bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.watching = watching
}

const missingFingerprint = "\x00root-missing"

func (i *Index) rebuildLocked() (*Snapshot, error) {
	read := i.reads.Add(1)
	b, fingerprint, err := i.source.ReadBoard()
	missing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !missing {
		return i.current, err
	}
	var v1 []string
	if missing {
		b, fingerprint = board.Board{}, missingFingerprint
	} else {
		if v1, err = i.source.V1Projects(); err != nil {
			return i.current, err
		}
		if len(v1) > 0 {
			fingerprint += "\x00v1:" + strings.Join(v1, ",")
		}
	}
	now := i.now()
	snapshot := &Snapshot{
		RootMissing: missing,
		V1Projects:  v1,
		Board:       b,
		Analysis:    board.Analyze(b),
		projects:    make(map[string]int, len(b.Projects)),
		tickets:     map[string][2]int{},
		archived:    map[string]bool{},
	}
	for index, project := range b.Projects {
		snapshot.projects[project.Name] = index
		for position, ticket := range project.Tickets {
			if _, seen := snapshot.tickets[ticket.ID]; !seen && ticket.ID != "" {
				snapshot.tickets[ticket.ID] = [2]int{index, position}
			}
		}
		for _, id := range project.Archived {
			snapshot.archived[id] = true
		}
	}
	i.boardAt, i.boardRead = now, read
	snapshot, _ = i.publishLocked(snapshot, fingerprint, now)
	return snapshot, nil
}

// refoldLocked publishes the current board with its runs refolded from the
// new events, and reports whether one records a write to the board.
func (i *Index) refoldLocked() (*Snapshot, bool) {
	if i.events == nil {
		return i.current, false
	}
	next := *i.current
	return i.publishLocked(&next, i.boardPrint, i.now())
}

// publishLocked derives next's runs at now over its board, advances the
// revision if the board or the runs changed, and makes next current.
func (i *Index) publishLocked(next *Snapshot, boardPrint string, now time.Time) (*Snapshot, bool) {
	next.BuiltAt, next.Runs, next.human = now, nil, nil
	var runsPrint string
	var boardEvents bool
	if i.events != nil && !next.RootMissing {
		names := make([]string, 0, len(next.Board.Projects))
		for _, project := range next.Board.Projects {
			names = append(names, project.Name)
		}
		var read string
		read, boardEvents = i.events.update(names, now)
		next.Runs = i.events.views(next.Board, now, i.runs)
		next.human = i.events.human()
		runsPrint = read + "\x00" + signature(next.Runs)
	}
	next.runIndex, next.lease = indexRuns(next.Runs), i.runs.Lease
	next.Revision = 1
	if i.current != nil {
		next.Revision = i.current.Revision
		if boardPrint != i.boardPrint || runsPrint != i.runsPrint {
			next.Revision++
		}
	}
	advanced := i.current == nil || next.Revision != i.current.Revision
	i.current, i.boardPrint, i.runsPrint = next, boardPrint, runsPrint
	if advanced && i.changed != nil {
		close(i.changed)
		i.changed = nil
	}
	return next, boardEvents
}
