package board

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// Telling a session now (D47, FH-127): a person may send a comment to the
// live session holding the ticket as well as posting it. The comment
// names that run (`sent-to`) and, once the session is handed it, when
// (`delivered`); from then on it is locked, since the session has read
// what it says. Neither is history: no write to the ticket made them.

// ErrCommentDelivered refuses an edit of a comment its session has been
// handed.
var ErrCommentDelivered = errors.New("the comment was delivered to its session, so it can no longer be edited")

// IsRun reports whether text is a run id (agent:session, agent-protocol
// §2), what a comment is sent to.
func IsRun(text string) bool { return runAuthor.MatchString(text) }

// CanEditComment reports whether a person may still edit a comment: not
// once it has been delivered.
func CanEditComment(c Comment) error {
	if !c.Delivered.IsZero() {
		return fmt.Errorf("comment %s: %w", c.ID, ErrCommentDelivered)
	}
	return nil
}

// DeliverComment marks comment id delivered at at to run, rewriting its
// entry alone (EditComment), and returns the comment as it is then: the
// text the session is shown. ok is false, and data is returned as it was,
// when the comment was not sent to run or was delivered already: a
// comment is handed over once, to the run it was sent to. A missing id is
// ErrNoComment.
func DeliverComment(data []byte, id, run string, at time.Time) ([]byte, Comment, bool, error) {
	comments := ParseActivity(data).Comments
	index := slices.IndexFunc(comments, func(c Comment) bool { return c.ID == id })
	if index < 0 {
		return data, Comment{}, false, fmt.Errorf("comment %s: %w", id, ErrNoComment)
	}
	current := comments[index]
	if current.SentTo == "" || current.SentTo != run || !current.Delivered.IsZero() {
		return data, current, false, nil
	}
	next, comment, err := EditComment(data, id, func(c *Comment) { c.Delivered = at.UTC().Truncate(time.Second) })
	return next, comment, err == nil, err
}
