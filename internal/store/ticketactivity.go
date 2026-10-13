package store

import (
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/scrub"
)

// A ticket's activity file (board-format §Activity, D47) holds people's
// comments and the ticket's history beside the ticket file. History is
// appended by every ticket write a store writing as someone makes (As,
// tx.go); comments are added and edited here. Neither touches the ticket
// file, so a comment never contends with an agent rewriting its handoff.

// NewComment is a comment as a person writes it: its text and, optionally,
// the part of the ticket it quotes and where that is from.
type NewComment struct {
	Body      string
	Quote     string
	QuoteFrom string
}

// ReadTicketActivity returns a live ticket's activity file, parsed, with
// its content hash, the precondition of an edit; a ticket without one has
// no activity and hash "".
func (s *Store) ReadTicketActivity(project, folder string) (board.Activity, string, error) {
	if !ValidProject(project) || !ValidName(folder) {
		return board.Activity{}, "", fmt.Errorf("activity %s/%s: %w", project, folder, ErrInvalidName)
	}
	root, fsys, err := s.handle()
	if err != nil {
		return board.Activity{}, "", err
	}
	name := path.Join(project, "tickets", folder, board.ActivityFile)
	if _, err := root.Lstat(name); errors.Is(err, fs.ErrNotExist) {
		return board.Activity{}, "", nil
	}
	r := &reader{root: root, fsys: fsys}
	data, _, problem := r.read(name)
	if problem != "" {
		return board.Activity{}, "", fmt.Errorf("activity %s/%s: %s", project, folder, problem)
	}
	return board.ParseActivity(data), Hash(data), nil
}

// AddComment appends a comment by the person this store writes as (As) to
// a live ticket's activity file under the project lock, once pre holds for
// that file (Merge: it is appended to the file as it is then). Agents
// never write comments: a store writing as a run, or as no one, is
// refused. Its text and quote are scrubbed of secrets (SEC-3) and checked
// (board.CheckComment). It returns the comment and the file's new hash.
func (s *Store) AddComment(project, id string, pre Precondition, input NewComment) (board.Comment, string, error) {
	if err := s.commenter(); err != nil {
		return board.Comment{}, "", err
	}
	body, quote, from, err := board.CheckComment(scrub.Secrets(input.Body), scrub.Secrets(input.Quote), input.QuoteFrom)
	if err != nil {
		return board.Comment{}, "", fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	var comment board.Comment
	hash, err := s.updateActivity(project, id, pre, func(data []byte) ([]byte, error) {
		entryID, err := newEntryID(board.ParseActivity(data))
		if err != nil {
			return nil, err
		}
		comment = board.Comment{ID: entryID, Time: s.now(), By: s.author, State: board.CommentOpen, Quote: quote, QuoteFrom: from, Body: body}
		return board.AppendComment(data, id, comment), nil
	})
	return comment, hash, err
}

// EditComment replaces the text of comment in a live ticket's activity
// file and marks it edited, under the project lock once pre holds (the
// hash of the file the person read). A comment the file does not have is
// ErrNotFound. It returns the comment and the file's new hash.
func (s *Store) EditComment(project, id, comment string, pre Precondition, body string) (board.Comment, string, error) {
	if err := s.commenter(); err != nil {
		return board.Comment{}, "", err
	}
	body, _, _, err := board.CheckComment(scrub.Secrets(body), "", "")
	if err != nil {
		return board.Comment{}, "", fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	var edited board.Comment
	hash, err := s.updateActivity(project, id, pre, func(data []byte) ([]byte, error) {
		next, changed, err := board.EditComment(data, comment, func(c *board.Comment) {
			c.Body, c.Edited = body, s.now()
		})
		if errors.Is(err, board.ErrNoComment) {
			return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
		}
		edited = changed
		return next, err
	})
	return edited, hash, err
}

// commenter refuses a store that does not write as a person.
func (s *Store) commenter() error {
	if s.author == "" || !board.ByPerson(s.author) {
		return fmt.Errorf("%w: comments are written by people on the board", ErrInvalidInput)
	}
	return nil
}

// updateActivity applies edit to a live ticket's activity file (empty when
// missing) under the project lock, once pre holds, and writes it
// atomically; a result over MaxFileBytes is ErrTooLarge and writes
// nothing. It returns the file's new hash.
func (s *Store) updateActivity(project, id string, pre Precondition, edit Edit) (string, error) {
	if err := s.checkTicket(project, id); err != nil {
		return "", err
	}
	release, err := s.lock(project)
	if err != nil {
		return "", err
	}
	defer release()
	ticket, err := s.TicketFile(project, id)
	if err != nil {
		return "", err
	}
	name := path.Join(path.Dir(ticket), board.ActivityFile)
	t := s.begin()
	current, err := t.readActivity(name)
	if err != nil {
		return "", err
	}
	if err := pre.check(name, current); err != nil {
		return "", err
	}
	next, err := edit(current)
	if err != nil {
		return "", err
	}
	if len(next) > MaxFileBytes {
		return "", fmt.Errorf("the ticket's activity is limited to %d KiB: %w", MaxFileBytes>>10, ErrTooLarge)
	}
	t.put(name, next)
	return Hash(next), t.commit()
}
