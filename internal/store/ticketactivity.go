package store

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/scrub"
)

// A ticket's activity file (board-format §Activity, D47) holds people's
// comments and the ticket's history beside the ticket file. History is
// appended by every ticket write a store writing as someone makes (As,
// tx.go); comments are added and edited here. Neither touches the ticket
// file, so a comment never contends with an agent rewriting its handoff.

// NewComment is a comment as a person writes it: its text and, optionally,
// the part of the ticket it quotes and where that is from, and the run it
// is sent to as well (FH-127; a run id, "" for none).
type NewComment struct {
	Body      string
	Quote     string
	QuoteFrom string
	SentTo    string
}

// ErrDelivered refuses a change to a comment its session has been handed
// (FH-127).
var ErrDelivered = errors.New("delivered")

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
	if input.SentTo != "" && !board.IsRun(input.SentTo) {
		return board.Comment{}, "", fmt.Errorf("%w: a comment is sent to a session's run, not %q", ErrInvalidInput, input.SentTo)
	}
	var comment board.Comment
	hash, err := s.updateActivity(project, id, pre, func(data []byte) ([]byte, error) {
		entryID, err := newEntryID(board.ParseActivity(data))
		if err != nil {
			return nil, err
		}
		comment = board.Comment{ID: entryID, Time: s.now(), By: s.author, State: board.CommentOpen, SentTo: input.SentTo, Quote: quote, QuoteFrom: from, Body: body}
		return board.AppendComment(data, id, comment), nil
	})
	return comment, hash, err
}

// EditComment replaces the text of comment in a live ticket's activity
// file and marks it edited, under the project lock once pre holds (the
// hash of the file the person read). A comment the file does not have is
// ErrNotFound, and one delivered to its session (FH-127) ErrDelivered. It
// returns the comment and the file's new hash.
func (s *Store) EditComment(project, id, comment string, pre Precondition, body string) (board.Comment, string, error) {
	if err := s.commenter(); err != nil {
		return board.Comment{}, "", err
	}
	body, _, _, err := board.CheckComment(scrub.Secrets(body), "", "")
	if err != nil {
		return board.Comment{}, "", fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	var edited board.Comment
	hash, err := s.updateActivity(project, id, pre, editUndelivered(comment, &edited, func(c *board.Comment) {
		c.Body, c.Edited = body, s.now()
	}))
	return edited, hash, err
}

// UnsendComment clears the run a person sent a comment to, under the
// project lock, as the file is then (FH-127): the board unsends a comment
// it could not queue, so it does not say the comment waits for a session.
// A delivered comment is ErrDelivered. It returns the comment and the
// file's new hash.
func (s *Store) UnsendComment(project, id, comment string) (board.Comment, string, error) {
	if err := s.commenter(); err != nil {
		return board.Comment{}, "", err
	}
	var unsent board.Comment
	hash, err := s.updateActivity(project, id, Merge, editUndelivered(comment, &unsent, func(c *board.Comment) { c.SentTo = "" }))
	return unsent, hash, err
}

// editUndelivered is an activity edit that applies edit to comment unless
// it was delivered to its session, keeping the result in out.
func editUndelivered(comment string, out *board.Comment, edit func(*board.Comment)) Edit {
	return func(data []byte) ([]byte, error) {
		locked := false
		next, changed, err := board.EditComment(data, comment, func(c *board.Comment) {
			if locked = board.CanEditComment(*c) != nil; !locked {
				edit(c)
			}
		})
		switch {
		case errors.Is(err, board.ErrNoComment):
			return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
		case err != nil:
			return nil, err
		case locked:
			return nil, fmt.Errorf("%w: %v", ErrDelivered, board.CanEditComment(changed))
		}
		*out = changed
		return next, nil
	}
}

// DeliverComment records that comment of a live ticket reached run, the
// run it was sent to, at at (FH-127), under the project lock as the file
// is then, and returns the comment as it is: the text its session is
// shown. ok is false, and nothing is written, when the comment is not
// there, was not sent to run or was delivered already: it is handed over
// once. Whoever delivers a session's inbox calls it (events.Deliver); it
// needs no author and records no history, since the ticket did not
// change.
func (s *Store) DeliverComment(project, id, comment, run string, at time.Time) (board.Comment, bool, error) {
	var delivered board.Comment
	ok := false
	err := s.inTicket(project, id, func(t *tx, ticket string) error {
		name := path.Join(path.Dir(ticket), board.ActivityFile)
		if err := t.editActivity(name, func(data []byte) ([]byte, error) {
			next, current, handed, err := board.DeliverComment(data, comment, run, at)
			if errors.Is(err, board.ErrNoComment) {
				return data, nil
			}
			delivered, ok = current, handed
			return next, err
		}); err != nil {
			return err
		}
		return t.commit()
	})
	if err != nil {
		return board.Comment{}, false, err
	}
	return delivered, ok, nil
}

// ErrOpenRequest refuses deleting a change request that is not resolved
// (FH-140).
var ErrOpenRequest = errors.New("open_request")

// DeleteComment removes comment from a live ticket's activity file, as a
// person deleting what they posted by mistake (FH-140), under the project
// lock once pre holds (the hash of the file the person read); the write's
// history names the comment, never its text. A comment the file does not
// have is ErrNotFound; one delivered to its session ErrDelivered, and a
// change request open or addressed ErrOpenRequest. It returns the comment
// as it was (whom it was sent to, to withdraw it from that session's
// inbox) and the file's new hash.
func (s *Store) DeleteComment(project, id, comment string, pre Precondition) (board.Comment, string, error) {
	if err := s.commenter(); err != nil {
		return board.Comment{}, "", err
	}
	var deleted board.Comment
	var hash string
	err := s.inTicket(project, id, func(t *tx, ticket string) error {
		name := path.Join(path.Dir(ticket), board.ActivityFile)
		current, err := t.readActivity(name)
		if err != nil {
			return err
		}
		if err := pre.check(name, current); err != nil {
			return err
		}
		if err := t.editActivity(name, func(data []byte) ([]byte, error) {
			next, removed, err := board.DeleteComment(data, comment)
			switch {
			case errors.Is(err, board.ErrNoComment):
				return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
			case errors.Is(err, board.ErrDeleteDelivered):
				return nil, fmt.Errorf("%w: %v", ErrDelivered, err)
			case errors.Is(err, board.ErrDeleteRequest):
				return nil, fmt.Errorf("%w: %v", ErrOpenRequest, err)
			case err != nil:
				return nil, err
			}
			deleted = removed
			return next, nil
		}); err != nil {
			return err
		}
		if err := t.commit(); err != nil {
			return err
		}
		final, err := t.readActivity(name)
		hash = Hash(final)
		return err
	})
	if err != nil {
		return board.Comment{}, "", err
	}
	return deleted, hash, nil
}

// TicketComment reads comment of a live ticket as its activity file has
// it now; ok is false when the file does not have it (deleted, FH-140).
// It takes no lock: a write replaces the file atomically.
func (s *Store) TicketComment(project, id, comment string) (board.Comment, bool, error) {
	if err := s.checkTicket(project, id); err != nil {
		return board.Comment{}, false, err
	}
	ticket, err := s.TicketFile(project, id)
	if err != nil {
		return board.Comment{}, false, err
	}
	data, err := s.begin().readActivity(path.Join(path.Dir(ticket), board.ActivityFile))
	if err != nil {
		return board.Comment{}, false, err
	}
	comments := board.ParseActivity(data).Comments
	if at := slices.IndexFunc(comments, func(c board.Comment) bool { return c.ID == comment }); at >= 0 {
		return comments[at], true, nil
	}
	return board.Comment{}, false, nil
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

// ErrNotInReview refuses a request for changes on a ticket that is no
// longer in review (FH-126).
var ErrNotInReview = errors.New("the ticket is not in review")

// RequestedChanges reports a request for changes: the comments it marked
// (those named, then the one it added), the ticket's new hash and the
// activity file's.
type RequestedChanges struct {
	Comments     []board.Comment
	Hash         string
	ActivityHash string
}

// RequestChanges is a person's request for changes on a ticket in review
// (D47, FH-126): in one write under the project lock it moves the ticket
// back to In progress, keeping its branch and pull request, marks the
// named comments as change requested and appends added, when given, as a
// change request; the history entry names the move and the comments.
// It needs a comment to name or add; a ticket not in review (as the
// write finds it) is ErrNotInReview and a missing comment ErrNotFound,
// and either writes nothing.
func (s *Store) RequestChanges(project, id string, comments []string, added *NewComment) (RequestedChanges, error) {
	if err := s.commenter(); err != nil {
		return RequestedChanges{}, err
	}
	if len(comments) == 0 && added == nil {
		return RequestedChanges{}, fmt.Errorf("%w: name a comment or write one to request changes", ErrInvalidInput)
	}
	var body, quote, from string
	if added != nil {
		var err error
		if body, quote, from, err = board.CheckComment(scrub.Secrets(added.Body), scrub.Secrets(added.Quote), added.QuoteFrom); err != nil {
			return RequestedChanges{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	}
	var result RequestedChanges
	err := s.inTicket(project, id, func(t *tx, ticket string) error {
		current, err := t.read(ticket)
		if err != nil {
			return err
		}
		if status, _ := mdfile.Parse(current).String("status"); status != string(board.Review) {
			return fmt.Errorf("%s is %s: %w", id, status, ErrNotInReview)
		}
		if result.Hash, err = t.edit(ticket, Merge, func(data []byte) ([]byte, error) {
			return mdfile.SetScalar(data, "status", string(board.InProgress))
		}, true); err != nil {
			return err
		}
		name := path.Join(path.Dir(ticket), board.ActivityFile)
		ids := slices.Clone(comments)
		err = t.editActivity(name, func(data []byte) ([]byte, error) {
			for _, comment := range comments {
				if data, err = setCommentState(data, comment, board.CommentChangeRequested, true); err != nil {
					return nil, err
				}
			}
			if added == nil {
				return data, nil
			}
			entryID, err := newEntryID(board.ParseActivity(data))
			if err != nil {
				return nil, err
			}
			ids = append(ids, entryID)
			return board.AppendComment(data, id, board.Comment{
				ID: entryID, Time: s.now(), By: s.author, State: board.CommentChangeRequested, Quote: quote, QuoteFrom: from, Body: body,
			}), nil
		})
		if err != nil {
			return err
		}
		if err := t.commit(); err != nil {
			return err
		}
		final, err := t.readActivity(name)
		if err != nil {
			return err
		}
		result.ActivityHash = Hash(final)
		for _, comment := range board.ParseActivity(final).Comments {
			if slices.Contains(ids, comment.ID) {
				result.Comments = append(result.Comments, comment)
			}
		}
		return nil
	})
	return result, err
}

// SetCommentState moves a comment of a live ticket to state to, as the
// store's writer: a person asks for, resolves or reopens a change, an
// agent marks one addressed (board.CommentTransition). The change merges
// with the file as it is under the project lock, and its history entry
// names it. It returns the comment and the activity file's new hash. A
// missing comment is ErrNotFound; a change the writer cannot make, or a
// store writing as no one, ErrInvalidInput.
func (s *Store) SetCommentState(project, id, comment string, to board.CommentState) (board.Comment, string, error) {
	if s.author == "" {
		return board.Comment{}, "", fmt.Errorf("%w: a comment's state is changed by someone", ErrInvalidInput)
	}
	var changed board.Comment
	var hash string
	err := s.inTicket(project, id, func(t *tx, ticket string) error {
		name := path.Join(path.Dir(ticket), board.ActivityFile)
		if err := t.editActivity(name, func(data []byte) ([]byte, error) {
			return setCommentState(data, comment, to, board.ByPerson(s.author))
		}); err != nil {
			return err
		}
		if err := t.commit(); err != nil {
			return err
		}
		final, err := t.readActivity(name)
		if err != nil {
			return err
		}
		hash = Hash(final)
		at := slices.IndexFunc(board.ParseActivity(final).Comments, func(c board.Comment) bool { return c.ID == comment })
		if at >= 0 {
			changed = board.ParseActivity(final).Comments[at]
		}
		return nil
	})
	return changed, hash, err
}

// setCommentState is board.SetCommentState with the store's errors.
func setCommentState(data []byte, comment string, to board.CommentState, person bool) ([]byte, error) {
	next, _, err := board.SetCommentState(data, comment, to, person)
	switch {
	case errors.Is(err, board.ErrNoComment):
		return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
	case errors.Is(err, board.ErrCommentState):
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return next, err
}

// inTicket runs write in a transaction on a live ticket's files under the
// project lock; write commits it.
func (s *Store) inTicket(project, id string, write func(t *tx, ticket string) error) error {
	if err := s.checkTicket(project, id); err != nil {
		return err
	}
	release, err := s.lock(project)
	if err != nil {
		return err
	}
	defer release()
	ticket, err := s.TicketFile(project, id)
	if err != nil {
		return err
	}
	return write(s.begin(), ticket)
}

// addressing marks comments addressed in the activity file beside ticket,
// in the transaction t, as an agent (FH-126).
func (s *Store) addressing(addressed []string) func(t *tx, ticket string) error {
	if len(addressed) == 0 {
		return nil
	}
	return func(t *tx, ticket string) error {
		if s.author == "" {
			return fmt.Errorf("%w: a comment's state is changed by someone", ErrInvalidInput)
		}
		name := path.Join(path.Dir(ticket), board.ActivityFile)
		return t.editActivity(name, func(data []byte) ([]byte, error) {
			var err error
			for _, comment := range addressed {
				if data, err = setCommentState(data, comment, board.CommentAddressed, board.ByPerson(s.author)); err != nil {
					return nil, err
				}
			}
			return data, nil
		})
	}
}
