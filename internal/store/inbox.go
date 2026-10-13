package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"
)

// Session inbox (D54): what waits for a session's next prompt or tool
// result (answers, comments, change requests; HOOK-5), one JSONL file per
// session in <project>/.flashheart/inbox/. It is a delivery queue, so the
// prompt hook needs one stat and no log read; what it carries lives in the
// log and the ticket (STO-6). The line format and which lines a delivery
// takes belong to events.
//
// Inboxes written before D54 sit in <project>/.flashheart/answers/. They
// drain: appending to a session's inbox moves its old one in first,
// DrainLegacyInboxes moves them all, and TakeLegacyInbox lets readers that
// can afford a second check deliver one directly.

const (
	inboxDir       = "inbox"
	legacyInboxDir = "answers"
	inboxExt       = ".jsonl"
)

func inboxName(dir, project, session string) (string, error) {
	name := strings.ReplaceAll(session, ":", "--")
	if !ValidProject(project) || !ValidName(name+inboxExt) || strings.Contains(session, "/") {
		return "", fmt.Errorf("inbox %s/%s: %w", project, session, ErrInvalidName)
	}
	return path.Join(project, ".flashheart", dir, name+inboxExt), nil
}

// AppendInbox adds lines to a session's inbox under the project lock,
// after the session's inbox in the old location, if any, which it removes.
func (s *Store) AppendInbox(project, session string, lines []byte) error {
	name, err := inboxName(inboxDir, project, session)
	if err != nil {
		return err
	}
	legacy, _ := inboxName(legacyInboxDir, project, session)
	if err := s.checkProject(project); err != nil {
		return err
	}
	release, err := s.lock(project)
	if err != nil {
		return err
	}
	defer release()
	return s.moveInboxLocked(legacy, name, lines)
}

// moveInboxLocked writes from's lines, then to's, then extra into to and
// removes from. The caller holds the project lock.
func (s *Store) moveInboxLocked(from, to string, extra []byte) error {
	old, err := s.ReadFile(from)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	current, err := s.ReadFile(to)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := s.WriteFileAtomic(to, joinLines(old, current, extra)); err != nil {
		return err
	}
	if old == nil {
		return nil
	}
	root, _, err := s.handle()
	if err != nil {
		return err
	}
	if err := root.Remove(from); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// joinLines concatenates files of lines, ending each with a newline.
func joinLines(parts ...[]byte) []byte {
	var out []byte
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		out = append(out, part...)
		if part[len(part)-1] != '\n' {
			out = append(out, '\n')
		}
	}
	return out
}

// TakeInbox removes the lines of a session's inbox that take accepts (all
// when take is nil) and returns them; nil when there are none. The lines
// it leaves stay, and the file goes when none do. The common case, no
// inbox, is one stat and takes no lock (HOOK-1).
func (s *Store) TakeInbox(project, session string, take func(line []byte) bool) ([]byte, error) {
	name, err := inboxName(inboxDir, project, session)
	if err != nil {
		return nil, err
	}
	return s.takeLines(project, name, take)
}

// TakeLegacyInbox is TakeInbox on the session's inbox in the old answers/
// location, for the readers that deliver from it while it drains.
func (s *Store) TakeLegacyInbox(project, session string, take func(line []byte) bool) ([]byte, error) {
	name, err := inboxName(legacyInboxDir, project, session)
	if err != nil {
		return nil, err
	}
	return s.takeLines(project, name, take)
}

func (s *Store) takeLines(project, name string, take func(line []byte) bool) ([]byte, error) {
	root, _, err := s.handle()
	if err != nil {
		return nil, err
	}
	if _, err := root.Lstat(name); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	release, err := s.lock(project)
	if err != nil {
		return nil, err
	}
	defer release()
	data, err := s.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var taken, kept []byte
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		switch {
		case len(bytes.TrimSpace(line)) == 0:
		case take == nil || take(line):
			taken = append(append(taken, line...), '\n')
		default:
			kept = append(append(kept, line...), '\n')
		}
	}
	switch {
	case taken == nil:
		return nil, nil
	case kept != nil:
		return taken, s.WriteFileAtomic(name, kept)
	}
	if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return taken, nil
}

// DrainLegacyInboxes moves every inbox of a project left in the old
// answers/ location into its session's inbox, in front of what waits
// there, under the project lock, and reports how many it moved. serve
// runs it with retention, so the prompt hook, which checks only the
// inbox, delivers them too.
func (s *Store) DrainLegacyInboxes(project string) (int, error) {
	if err := s.checkProject(project); err != nil {
		return 0, err
	}
	folder := path.Join(project, ".flashheart", legacyInboxDir)
	entries, err := s.ReadDir(folder)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type().IsRegular() && strings.HasSuffix(name, inboxExt) && ValidName(name) && !strings.HasPrefix(name, ".") {
			names = append(names, name)
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
	moved := 0
	for _, name := range names {
		from := path.Join(folder, name)
		if !s.Exists(from) {
			continue
		}
		if err := s.moveInboxLocked(from, path.Join(project, ".flashheart", inboxDir, name), nil); err != nil {
			return moved, fmt.Errorf("move %s: %w", from, err)
		}
		moved++
	}
	return moved, nil
}

// PruneInboxes removes a project's inboxes, in either location, last
// written before before (FH-82), each checked again under the project
// lock, and reports how many it removed. Which inboxes may go is events'
// rule (events.Log.Prune).
func (s *Store) PruneInboxes(project string, before time.Time) (int, error) {
	removed, err := s.pruneSessionFiles(project, inboxDir, inboxExt, before)
	if err != nil {
		return removed, err
	}
	legacy, err := s.pruneSessionFiles(project, legacyInboxDir, inboxExt, before)
	return removed + legacy, err
}
