package store

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
)

// Activity state: a session's tool activity the hooks have not recorded yet
// and the edit paths they recorded since its last checkpoint (FH-55), one
// JSON file per session in <project>/.flashheart/activity/. Only hooks read
// and write it, always under the project lock, together with the event
// lines it turns into. It is a disposable cache, never board state: it is
// written in place without a sync, and the format belongs to hooks.

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
