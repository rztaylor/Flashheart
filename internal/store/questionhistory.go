package store

import (
	"fmt"
	"path"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/scrub"
)

// Questions in a ticket's history (FH-190, board-format §Activity): what
// an agent asked about the ticket and what became of it are history
// entries in its activity file, each written once per question and
// outcome. The ticket file is not touched, except by a board answer's
// note, which is the same write (UpdateTicketNoting).

// RecordQuestion appends a history entry for a question change (asked,
// answered, answered in the session, session ended) to a live ticket's
// activity file, as the store's author, under the project lock, dated at
// (zero: now). The question's text and answer are scrubbed of secrets
// (SEC-3). It writes nothing, and reports false, when the history already
// records that change for that question. A store writing as no one, or a
// change that is not about a question, is ErrInvalidInput.
func (s *Store) RecordQuestion(project, id string, at time.Time, change board.Change) (bool, error) {
	if s.author == "" {
		return false, fmt.Errorf("%w: a question's history is written by someone", ErrInvalidInput)
	}
	change, err := questionChange(change)
	if err != nil {
		return false, err
	}
	wrote := false
	err = s.inTicket(project, id, func(t *tx, ticket string) error {
		noted, err := t.noteQuestion(path.Dir(ticket), change)
		if err != nil || !noted {
			return err
		}
		t.at, wrote = at, true
		return t.commit()
	})
	return wrote && err == nil, err
}

// UpdateTicketNoting is UpdateTicket that also records question changes
// in the same write (a board answer and its ## Notes bullet, FH-190),
// each unless the history already has it.
func (s *Store) UpdateTicketNoting(project, id string, pre Precondition, edit Edit, noted ...board.Change) (string, error) {
	for index, change := range noted {
		var err error
		if noted[index], err = questionChange(change); err != nil {
			return "", err
		}
	}
	return s.updateTicket(project, id, pre, edit, func(t *tx, ticket string) error {
		for _, change := range noted {
			if _, err := t.noteQuestion(path.Dir(ticket), change); err != nil {
				return err
			}
		}
		return nil
	})
}

// questionChange checks a question change and scrubs its text.
func questionChange(change board.Change) (board.Change, error) {
	if !board.IsQuestionChange(change.What) {
		return change, fmt.Errorf("%w: %q is not a question's change", ErrInvalidInput, change.What)
	}
	if !board.ValidEntryID(change.Question.ID) {
		return change, fmt.Errorf("%w: a question's change names its question", ErrInvalidInput)
	}
	q := change.Question
	q.Text, q.Answer = scrub.Secrets(q.Text), scrub.Secrets(q.Answer)
	options := make([]string, 0, len(q.Options))
	for _, option := range q.Options {
		options = append(options, scrub.Secrets(option))
	}
	q.Options = options
	change.Question = q
	return change, nil
}

// noteQuestion notes a question change for the ticket in folder unless its
// activity file, as the transaction has left it, already records it.
func (t *tx) noteQuestion(folder string, change board.Change) (bool, error) {
	data, err := t.readActivity(path.Join(folder, board.ActivityFile))
	if err != nil {
		return false, err
	}
	if board.ParseActivity(data).HasQuestionChange(change.Question.ID, change.What) {
		return false, nil
	}
	t.note(folder, change)
	return true, nil
}
