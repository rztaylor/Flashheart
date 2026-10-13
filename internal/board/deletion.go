package board

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Deleting a comment (FH-140): a person may delete a comment posted by
// mistake until a session has read it. Its entry leaves the activity file
// whole; the history of the write names it by id (CommentChanges), never
// its text.

// Refusals of a delete: once a session was handed the comment it has read
// it, and a change request waits on someone until it is resolved.
var (
	ErrDeleteDelivered = errors.New("the comment was delivered to its session, so it can no longer be deleted")
	ErrDeleteRequest   = errors.New("the comment is a change request that is not resolved, so it cannot be deleted; resolve it first")
)

// CanDeleteComment reports whether a person may delete a comment: not once
// it was delivered to the session it was sent to, nor while it is a change
// request open or addressed.
func CanDeleteComment(c Comment) error {
	switch {
	case !c.Delivered.IsZero():
		return fmt.Errorf("comment %s: %w", c.ID, ErrDeleteDelivered)
	case c.State == CommentChangeRequested || c.State == CommentAddressed:
		return fmt.Errorf("comment %s: %w", c.ID, ErrDeleteRequest)
	}
	return nil
}

// DeleteComment removes comment id's entry, heading to the next entry,
// and returns the file without it and the comment as it was. Every other
// byte stays, except that the blank lines that separated a last entry
// from the one before it go with it, so the file reads as if the comment
// had never been appended. A missing id, or one that is not a comment, is
// ErrNoComment; a comment that may not be deleted (CanDeleteComment) is
// refused and nothing changes.
func DeleteComment(data []byte, id string) ([]byte, Comment, error) {
	activity := ParseActivity(data)
	index := slices.IndexFunc(activity.entries, func(e entry) bool { return e.kind == EntryComment && e.id == id })
	at := slices.IndexFunc(activity.Comments, func(c Comment) bool { return c.ID == id })
	if index < 0 || at < 0 {
		return nil, Comment{}, fmt.Errorf("comment %s: %w", id, ErrNoComment)
	}
	comment := activity.Comments[at]
	if err := CanDeleteComment(comment); err != nil {
		return nil, Comment{}, err
	}
	activity.entries = slices.Delete(activity.entries, index, index+1)
	if index == len(activity.entries) {
		if index == 0 {
			activity.preamble = endOneLine(activity.preamble)
		} else {
			activity.entries[index-1].raw = endOneLine(activity.entries[index-1].raw)
		}
	}
	return activity.Bytes(), comment, nil
}

// endOneLine drops the blank lines at the end of text, after its last
// line ending.
func endOneLine(text string) string {
	for {
		body := strings.TrimSuffix(strings.TrimSuffix(text, "\n"), "\r")
		cut := strings.LastIndex(body, "\n")
		if body == text || cut < 0 || strings.TrimSpace(body[cut+1:]) != "" {
			return text
		}
		text = body[:cut+1]
	}
}
