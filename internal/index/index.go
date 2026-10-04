package index

import (
	"errors"
	"io/fs"
	"sync"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
)

// DefaultMaxAge is how old a snapshot may be before Current rebuilds it.
const DefaultMaxAge = 2 * time.Second

// Source reads the whole board with a fingerprint of the files read.
type Source interface {
	ReadBoard() (board.Board, string, error)
}

// Options configures an Index.
type Options struct {
	MaxAge time.Duration
	Now    func() time.Time
}

// Snapshot is an immutable view of the board at one revision.
type Snapshot struct {
	Revision    uint64
	BuiltAt     time.Time
	RootMissing bool
	Board       board.Board
	Analysis    board.Analysis

	projects map[string]int
}

// Project returns a project by directory name.
func (s *Snapshot) Project(name string) (*board.Project, bool) {
	index, ok := s.projects[name]
	if !ok {
		return nil, false
	}
	return &s.Board.Projects[index], true
}

// Ticket returns the first copy of a ticket in workflow order.
func (s *Snapshot) Ticket(project, slug string) (board.Ticket, bool) {
	p, ok := s.Project(project)
	if !ok {
		return board.Ticket{}, false
	}
	for _, ticket := range p.Tickets {
		if ticket.Slug == slug {
			return ticket, true
		}
	}
	return board.Ticket{}, false
}

// Index caches the latest snapshot.
type Index struct {
	source Source
	maxAge time.Duration
	now    func() time.Time

	mu          sync.Mutex
	current     *Snapshot
	fingerprint string
}

// New returns an index over source. Nothing is read until first use.
func New(source Source, options Options) *Index {
	if options.MaxAge <= 0 {
		options.MaxAge = DefaultMaxAge
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Index{source: source, maxAge: options.MaxAge, now: options.Now}
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
	if missing {
		b, fingerprint = board.Board{}, missingFingerprint
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
		BuiltAt:     i.now(),
		RootMissing: missing,
		Board:       b,
		Analysis:    board.Analyze(b),
		projects:    make(map[string]int, len(b.Projects)),
	}
	for index, project := range b.Projects {
		snapshot.projects[project.Name] = index
	}
	i.current, i.fingerprint = snapshot, fingerprint
	return snapshot, nil
}
