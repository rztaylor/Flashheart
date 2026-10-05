package index

import (
	"errors"
	"io/fs"
	"strings"
	"sync"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/runs"
)

// DefaultMaxAge is how old a snapshot may be before Current rebuilds it.
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

// Index caches the latest snapshot.
type Index struct {
	source Source
	maxAge time.Duration
	now    func() time.Time
	runs   runs.Settings
	events *tracker

	mu          sync.Mutex
	current     *Snapshot
	fingerprint string
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

// Current returns the snapshot, rebuilding it first if it is older than
// MaxAge. If a rebuild fails the previous snapshot is returned with the error.
func (i *Index) Current() (*Snapshot, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.current != nil && i.now().Sub(i.current.BuiltAt) < i.maxAge {
		return i.current, nil
	}
	return i.rebuildLocked()
}

// Rebuild reads the board now.
func (i *Index) Rebuild() (*Snapshot, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.rebuildLocked()
}

const missingFingerprint = "\x00root-missing"

func (i *Index) rebuildLocked() (*Snapshot, error) {
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
	var views []runs.View
	if i.events != nil && !missing {
		names := make([]string, 0, len(b.Projects))
		for _, project := range b.Projects {
			names = append(names, project.Name)
		}
		read := i.events.update(names, now)
		views = i.events.views(b, now, i.runs)
		fingerprint += "\x00runs:" + read + "\x00" + signature(views)
	}
	revision := uint64(1)
	if i.current != nil {
		revision = i.current.Revision
		if fingerprint != i.fingerprint {
			revision++
		}
	}
	snapshot := &Snapshot{
		Revision:    revision,
		BuiltAt:     now,
		RootMissing: missing,
		V1Projects:  v1,
		Board:       b,
		Analysis:    board.Analyze(b),
		Runs:        views,
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
	advanced := i.current == nil || snapshot.Revision != i.current.Revision
	i.current, i.fingerprint = snapshot, fingerprint
	if advanced && i.changed != nil {
		close(i.changed)
		i.changed = nil
	}
	return snapshot, nil
}
