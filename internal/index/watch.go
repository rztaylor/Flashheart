package index

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watching (STO-7). fsnotify watches are per directory (and, with kqueue on
// macOS, per file in it), so the watcher covers the board's structure (the
// root, each project, tickets/, workstreams/, .archive/tickets/) plus, up to
// FolderBudget, each project's event log (.flashheart/events/, or
// .flashheart/ until the log exists) and the most recently changed ticket
// folders. After a short debounce a board change reads the board, and an
// event append only refolds the runs over the board already read (FH-96),
// so the revision moves within a second of an edit. While it runs the
// watcher is the snapshot's freshness: Current does not rebuild on age. A
// slow full rebuild catches edits in folders beyond the budget.

const (
	// FolderBudget bounds how many ticket folders and event logs are
	// watched individually.
	FolderBudget = 1500
	// debounce coalesces the burst of events one save produces.
	debounce = 60 * time.Millisecond
	// sweep is how often unwatched folders are checked by a full rebuild.
	sweep = 10 * time.Second
)

// Wait returns the current revision once it is newer than since, or when ctx
// ends (long-polling, LIFE-3).
func (i *Index) Wait(ctx context.Context, since uint64) uint64 {
	for {
		i.mu.Lock()
		if i.changed == nil {
			i.changed = make(chan struct{})
		}
		changed := i.changed
		revision := uint64(0)
		if i.current != nil {
			revision = i.current.Revision
		}
		i.mu.Unlock()
		if revision > since {
			return revision
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return revision
		}
	}
}

// Watch keeps the snapshot current from file-system events until ctx ends.
// ready, when non-nil, is closed once the first watches are in place. If
// watching cannot start, it falls back to the sweep alone.
func (i *Index) Watch(ctx context.Context, root string, ready chan<- struct{}) {
	signal := func() {
		if ready != nil {
			close(ready)
			ready = nil
		}
	}
	defer signal()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		signal()
		i.sweepOnly(ctx)
		return
	}
	defer watcher.Close()
	watched := map[string]bool{}
	// sync brings the watches to the wanted set and reports whether it
	// added one.
	sync := func() bool {
		want := watchSet(root, FolderBudget)
		added := false
		for path := range watched {
			if !want[path] {
				_ = watcher.Remove(path)
				delete(watched, path)
			}
		}
		for path := range want {
			if !watched[path] && watcher.Add(path) == nil {
				watched[path] = true
				added = true
			}
		}
		return added
	}
	// Watches go in before each read: an edit after a watch is added raises
	// an event, and one before it is in the read. Reading first would
	// publish a new folder's revision before the folder is watched, and an
	// edit in that gap would wait for the sweep (FH-36).
	sync()
	_, _ = i.Rebuild()
	i.setWatching(true)
	defer i.setWatching(false)
	signal()

	timer := time.NewTimer(time.Hour)
	timer.Stop()
	ticker := time.NewTicker(sweep)
	defer ticker.Stop()
	var waiting pending
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op == fsnotify.Chmod || !waiting.note(classify(event.Name), i.reads.Load()) {
				continue
			}
			timer.Reset(debounce)
		case <-watcher.Errors:
			// Events may have been lost: read everything.
			waiting.force = true
			timer.Reset(debounce)
		case <-timer.C:
			changes := waiting
			waiting = pending{}
			added := (changes.sync || changes.force) && sync()
			switch changes.plan(i.covered(changes.mark), added) {
			case readBoard:
				_, _ = i.Rebuild()
			case refoldRuns:
				if i.refold() {
					sync()
					_, _ = i.Rebuild()
				}
			}
		case <-ticker.C:
			sync()
			_, _ = i.Rebuild()
		}
	}
}

func (i *Index) sweepOnly(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = i.Rebuild()
		}
	}
}

// change is what a file-system event may have changed.
type change int

const (
	// unchanged: lock files, logs and caches under .flashheart/, which
	// Flashheart itself writes and which never change the board.
	unchanged change = iota
	// runsChanged: an event log was appended to.
	runsChanged
	// logMoved: a .flashheart/ or .flashheart/events/ directory appeared or
	// went, so the watches move with it.
	logMoved
	// boardChanged: anything else under the root.
	boardChanged
)

func classify(name string) change {
	slashed := filepath.ToSlash(name)
	switch {
	case strings.HasSuffix(slashed, "/.flashheart") || strings.HasSuffix(slashed, "/.flashheart/events"):
		return logMoved
	case strings.Contains(slashed, "/.flashheart/events/"):
		return runsChanged
	case strings.Contains(slashed, "/.flashheart/"):
		return unchanged
	}
	return boardChanged
}

// pending gathers the changes seen within one debounce.
type pending struct {
	board, runs, sync, force bool
	// mark is the count of board reads started when the latest board change
	// arrived: a read started after it has seen the change.
	mark uint64
}

// note adds a change seen when reads board reads had started, and reports
// whether it needs any work.
func (p *pending) note(c change, reads uint64) bool {
	switch c {
	case boardChanged:
		p.board, p.sync, p.mark = true, true, reads
	case logMoved:
		p.runs, p.sync = true, true
	case runsChanged:
		p.runs = true
	default:
		return false
	}
	return true
}

type action int

const (
	nothing action = iota
	refoldRuns
	readBoard
)

// plan decides the work for the pending changes. covered is whether the
// board was read by a read started after the latest board change arrived
// (an API write's own rebuild, FH-96); added whether syncing added a watch,
// whose folder must then be read after it (FH-36). Board changes read the
// board unless covered; event appends only refold the runs.
func (p pending) plan(covered, added bool) action {
	switch {
	case p.force || (p.board && (added || !covered)):
		return readBoard
	case p.runs:
		return refoldRuns
	}
	return nothing
}

// watchSet lists the directories to watch: the root, each project and its
// tickets, workstreams and archive folders, then, within budget, each
// project's event log (.flashheart/events, or .flashheart until it exists)
// and the most recently modified ticket folders.
func watchSet(root string, budget int) map[string]bool {
	want := map[string]bool{}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		// Watch the parent so the root appearing is noticed.
		want[filepath.Dir(root)] = true
		return want
	}
	want[root] = true
	type folder struct {
		path     string
		modified time.Time
	}
	var logs []string
	var folders []folder
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		project := filepath.Join(root, name)
		want[project] = true
		for _, sub := range []string{"tickets", "workstreams", filepath.Join(".archive", "tickets")} {
			if isDir(filepath.Join(project, sub)) {
				want[filepath.Join(project, sub)] = true
			}
		}
		// Without .flashheart the project's own watch sees it appear.
		if log := filepath.Join(project, ".flashheart", "events"); isDir(log) {
			logs = append(logs, log)
		} else if isDir(filepath.Dir(log)) {
			logs = append(logs, filepath.Dir(log))
		}
		tickets, _ := os.ReadDir(filepath.Join(project, "tickets"))
		for _, ticket := range tickets {
			if !ticket.IsDir() || strings.HasPrefix(ticket.Name(), ".") {
				continue
			}
			info, err := ticket.Info()
			if err != nil {
				continue
			}
			folders = append(folders, folder{filepath.Join(project, "tickets", ticket.Name()), info.ModTime()})
		}
	}
	for _, log := range logs[:min(len(logs), budget)] {
		want[log] = true
	}
	budget = max(budget-len(logs), 0)
	sort.Slice(folders, func(a, b int) bool { return folders[a].modified.After(folders[b].modified) })
	for _, f := range folders[:min(len(folders), budget)] {
		want[f.path] = true
	}
	return want
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
