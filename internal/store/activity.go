package store

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"time"
)

// Activity state: a session's tool activity the hooks have not recorded yet
// and the edit paths they recorded since its last checkpoint (FH-55), one
// JSON file per session in <project>/.flashheart/activity/. Only hooks read
// and write it, always under the project lock, together with the event
// lines it turns into; retention removes a file no hook has written for
// stale_hours (PruneActivity). It is a disposable cache, never board state:
// it is written in place without a sync, and the format belongs to hooks.

func activityName(project, session string) (string, error) {
	name := strings.ReplaceAll(session, ":", "--")
	if !ValidProject(project) || !ValidName(name+".json") || strings.Contains(session, "/") {
		return "", fmt.Errorf("activity state %s/%s: %w", project, session, ErrInvalidName)
	}
	return path.Join(project, ".flashheart", "activity", name+".json"), nil
}

// UpdateActivity calls update with a session's activity state (nil when
// there is none) under the project lock, appends the event lines it returns
// (by event file name) and then writes the state it returns, or removes the
// state when it returns nil. The state is left as it was when update or an
// append fails.
func (s *Store) UpdateActivity(project, session string, update func(state []byte) (next []byte, lines map[string][]byte, err error)) error {
	name, err := activityName(project, session)
	if err != nil {
		return err
	}
	if err := s.checkProject(project); err != nil {
		return err
	}
	release, err := s.lock(project)
	if err != nil {
		return err
	}
	defer release()
	return s.updateActivityLocked(project, name, update)
}

// PruneActivity removes a project's activity state files last written
// before before (FH-82): retention's way to clear the state of a session
// that never reported its end. It reports how many it removed.
func (s *Store) PruneActivity(project string, before time.Time) (int, error) {
	return s.pruneSessionFiles(project, "activity", ".json", before)
}

// pruneSessionFiles removes the per-session files named <name><ext> in
// <project>/.flashheart/<dir>/ that were last written before before. Each is
// checked again under the project lock, which every writer of these files
// holds, so a file a hook or an answer rewrote meanwhile stays. The lock is
// taken only when something is old enough.
func (s *Store) pruneSessionFiles(project, dir, ext string, before time.Time) (int, error) {
	if err := s.checkProject(project); err != nil {
		return 0, err
	}
	folder := path.Join(project, ".flashheart", dir)
	entries, err := s.ReadDir(folder)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	old := func(info fs.FileInfo) bool {
		return info.Mode().IsRegular() && info.ModTime().Before(before)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || !strings.HasSuffix(name, ext) || !ValidName(name) {
			continue
		}
		if info, err := entry.Info(); err == nil && old(info) {
			names = append(names, path.Join(folder, name))
		}
	}
	if len(names) == 0 {
		return 0, nil
	}
	release, err := s.lock(project)
	if err != nil {
		return 0, err
	}
	defer release()
	root, _, err := s.handle()
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, name := range names {
		info, err := root.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) || err == nil && !old(info) {
			continue
		}
		if err == nil {
			err = root.Remove(name)
		}
		if err != nil {
			return removed, fmt.Errorf("expire %s: %w", name, err)
		}
		removed++
	}
	return removed, nil
}

func (s *Store) updateActivityLocked(project, name string, update func([]byte) ([]byte, map[string][]byte, error)) error {
	current, err := s.ReadFile(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	next, lines, err := update(current)
	if err != nil {
		return err
	}
	files := slices.Sorted(maps.Keys(lines))
	for _, file := range files {
		if !eventFileName.MatchString(file) {
			return fmt.Errorf("event file %q: %w", file, ErrInvalidName)
		}
	}
	for _, file := range files {
		if err := s.appendEventLinesLocked(project, file, lines[file]); err != nil {
			return err
		}
	}
	root, _, err := s.handle()
	if err != nil {
		return err
	}
	switch {
	case next == nil:
		if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove activity state: %w", err)
		}
		return nil
	case string(next) == string(current):
		return nil
	}
	if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
		return fmt.Errorf("write activity state: %w", err)
	}
	if err := root.WriteFile(name, next, 0o644); err != nil {
		return fmt.Errorf("write activity state: %w", err)
	}
	return nil
}
