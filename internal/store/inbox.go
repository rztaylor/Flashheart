package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
)

// Answers inbox: answers to a session's questions waiting for its next
// prompt (HOOK-5), one JSONL file per session in
// <project>/.flashheart/answers/. It is a delivery queue derived from the
// event log, so the prompt hook needs one stat and no log read; the answer
// itself lives in the log and the ticket (STO-6). The line format belongs
// to events.

func inboxName(project, run string) (string, error) {
	name := strings.ReplaceAll(run, ":", "--")
	if !ValidProject(project) || !ValidName(name+".jsonl") || strings.Contains(run, "/") {
		return "", fmt.Errorf("inbox %s/%s: %w", project, run, ErrInvalidName)
	}
	return path.Join(project, ".flashheart", "answers", name+".jsonl"), nil
}

// AppendInbox adds lines to a session's inbox under the project lock.
func (s *Store) AppendInbox(project, run string, lines []byte) error {
	name, err := inboxName(project, run)
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
	current, err := s.ReadFile(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return s.WriteFileAtomic(name, append(current, lines...))
}

// TakeInbox returns and removes a session's inbox; nil when it is empty.
// The common case, no inbox, takes no lock.
func (s *Store) TakeInbox(project, run string) ([]byte, error) {
	name, err := inboxName(project, run)
	if err != nil {
		return nil, err
	}
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
	if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return data, nil
}
