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
// root, each project, tickets/, workstreams/, .archive/tickets/, and the
// event log in .flashheart/events/ for agent runs) plus the
// most recently changed ticket folders up to FolderBudget. Any event
// rebuilds the snapshot after a short debounce, so the revision moves within
// a second of an edit. A slow full rebuild catches edits in folders beyond
// the budget.

const (
	// FolderBudget bounds how many ticket folders are watched individually.
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
	sync := func() {
		want := watchSet(root)
		for path := range watched {
			if !want[path] {
				_ = watcher.Remove(path)
				delete(watched, path)
			}
		}
		for path := range want {
			if !watched[path] && watcher.Add(path) == nil {
				watched[path] = true
			}
		}
	}
	// Watches go in before each rebuild: an edit after a watch is added raises
	// an event, and one before it is in the rebuild. Rebuilding first would
	// publish a new folder's revision before the folder is watched, and an
	// edit in that gap would wait for the sweep (FH-36).
	sync()
	_, _ = i.Rebuild()
	signal()

	timer := time.NewTimer(time.Hour)
	timer.Stop()
	ticker := time.NewTicker(sweep)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op == fsnotify.Chmod || ignored(event.Name) {
				continue
			}
			timer.Reset(debounce)
		case <-watcher.Errors:
			timer.Reset(debounce)
		case <-timer.C:
			sync()
			_, _ = i.Rebuild()
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

// ignored reports events Flashheart itself causes that never change the
// board: lock files, logs and caches under .flashheart/. Event logs are the
// exception: appends change agent runs.
func ignored(name string) bool {
	slashed := filepath.ToSlash(name)
	if strings.Contains(slashed, "/.flashheart/events") {
		return false
	}
	return strings.Contains(slashed, "/.flashheart/") || strings.HasSuffix(slashed, "/.flashheart")
}

// watchSet lists the directories to watch: the root, each project and its
// tickets, workstreams and archive folders, and the most recently modified
// ticket folders up to FolderBudget.
func watchSet(root string) map[string]bool {
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
	var folders []folder
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		project := filepath.Join(root, name)
		want[project] = true
		for _, sub := range []string{"tickets", "workstreams", filepath.Join(".archive", "tickets"), ".flashheart", filepath.Join(".flashheart", "events")} {
			dir := filepath.Join(project, sub)
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				want[dir] = true
			}
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
	sort.Slice(folders, func(a, b int) bool { return folders[a].modified.After(folders[b].modified) })
	for _, f := range folders[:min(len(folders), FolderBudget)] {
		want[f.path] = true
	}
	return want
}
