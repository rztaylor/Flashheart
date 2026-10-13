package board

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Change requests (D47, FH-126): in review a person marks comments as a
// change requested and the ticket goes back to In progress; an agent marks
// each one addressed; at the next review the person resolves or reopens
// it. A comment's state is its `state:` attribute in the activity file,
// and each change of state is history, worked out from the file before
// and after the write (CommentChanges, D58).

// ErrCommentState reports a state change a comment cannot make, or not by
// that writer.
var ErrCommentState = errors.New("the comment's state cannot change that way")

// CommentTransition reports whether a comment in state from may move to
// state to, by a person (on the board) or by an agent. A person asks for
// a change on any comment (reopening one addressed or resolved) and
// resolves a request; an agent only marks a change request addressed.
// Staying in the same state is always allowed, so a repeated call changes
// nothing.
func CommentTransition(from, to CommentState, person bool) error {
	if from == to && (person || to == CommentAddressed) {
		return nil
	}
	refuse := func(reason string) error { return fmt.Errorf("%w: %s", ErrCommentState, reason) }
	if !person {
		switch {
		case to != CommentAddressed:
			return refuse("agents only mark a change request addressed")
		case from == CommentOpen:
			return refuse("it is a comment, not a change request")
		case from == CommentResolved:
			return refuse("the user already resolved it")
		}
		return nil
	}
	switch to {
	case CommentChangeRequested:
		return nil
	case CommentResolved:
		if from == CommentChangeRequested || from == CommentAddressed {
			return nil
		}
		return refuse("only a change request is resolved")
	case CommentAddressed:
		return refuse("the agent working on the ticket marks a change request addressed")
	}
	return refuse("a comment does not go back to open")
}

// SetCommentState moves comment id to state to, by a person or an agent
// (CommentTransition), rewriting that comment's entry alone (EditComment).
// A missing id is ErrNoComment; a change it cannot make, ErrCommentState.
func SetCommentState(data []byte, id string, to CommentState, person bool) ([]byte, Comment, error) {
	activity := ParseActivity(data)
	at := slices.IndexFunc(activity.Comments, func(c Comment) bool { return c.ID == id })
	if at < 0 {
		return nil, Comment{}, fmt.Errorf("comment %s: %w", id, ErrNoComment)
	}
	current := activity.Comments[at]
	if err := CommentTransition(current.State, to, person); err != nil {
		return nil, Comment{}, fmt.Errorf("comment %s (%s): %w", id, strings.ReplaceAll(string(current.State), "-", " "), err)
	}
	if current.State == to {
		return data, current, nil
	}
	return EditComment(data, id, func(c *Comment) { c.State = to })
}

// changeOrder is the order CommentChanges lists the kinds of change in.
var changeOrder = []string{ChangeRequested, ChangeReopened, ChangeAddressed, ChangeResolved, ChangeDeleted}

// CommentChanges is what a write changed in a ticket's comments, as
// history records it: the comments asked for as changes (a new comment
// written as one too), reopened, addressed and resolved, and those deleted
// (FH-140), by id, in the file's order; never their text. Nil when no
// comment's state changed and none was deleted.
func CommentChanges(before, after []byte) []Change {
	was := map[string]CommentState{}
	previous := ParseActivity(before).Comments
	for _, comment := range previous {
		was[comment.ID] = comment.State
	}
	byKind := map[string][]string{}
	current := ParseActivity(after).Comments
	for _, comment := range previous {
		if !slices.ContainsFunc(current, func(c Comment) bool { return c.ID == comment.ID }) {
			byKind[ChangeDeleted] = append(byKind[ChangeDeleted], comment.ID)
		}
	}
	for _, comment := range current {
		from, known := was[comment.ID]
		if !known {
			from = CommentOpen
		}
		if from == comment.State {
			continue
		}
		kind := ""
		switch comment.State {
		case CommentChangeRequested:
			kind = ChangeRequested
			if from == CommentAddressed || from == CommentResolved {
				kind = ChangeReopened
			}
		case CommentAddressed:
			kind = ChangeAddressed
		case CommentResolved:
			kind = ChangeResolved
		default:
			continue
		}
		byKind[kind] = append(byKind[kind], comment.ID)
	}
	var changes []Change
	for _, kind := range changeOrder {
		if ids := byKind[kind]; len(ids) > 0 {
			changes = append(changes, Change{What: kind, Comments: ids})
		}
	}
	return changes
}

// RequestCount counts a ticket's change requests: Open waits for an
// agent, Addressed waits for the person to resolve or reopen it.
type RequestCount struct {
	Open, Addressed int
}

// Any reports whether a change request waits on anyone.
func (c RequestCount) Any() bool { return c.Open+c.Addressed > 0 }

// CountRequests counts people's comments that are change requests still
// open or addressed. A comment whose author is not a person was not
// written on the board, so it asks for nothing.
func CountRequests(comments []Comment) RequestCount {
	var count RequestCount
	for _, comment := range comments {
		if !ByPerson(comment.By) {
			continue
		}
		switch comment.State {
		case CommentChangeRequested:
			count.Open++
		case CommentAddressed:
			count.Addressed++
		}
	}
	return count
}

// RequestsIn counts an activity file's change requests. A file without
// those states, the usual case, is not parsed.
func RequestsIn(data []byte) RequestCount {
	if !bytes.Contains(data, []byte("state: "+CommentChangeRequested)) && !bytes.Contains(data, []byte("state: "+CommentAddressed)) {
		return RequestCount{}
	}
	return CountRequests(ParseActivity(data).Comments)
}

// OpenRequests lists people's change requests no agent has addressed yet,
// oldest first.
func OpenRequests(comments []Comment) []Comment {
	var open []Comment
	for _, comment := range comments {
		if ByPerson(comment.By) && comment.State == CommentChangeRequested {
			open = append(open, comment)
		}
	}
	return open
}

// RequestExcerpt is a change request's text on one line, cut to limit
// characters with an ellipsis, as warnings and lists name it.
func RequestExcerpt(body string, limit int) string {
	line := strings.Join(strings.Fields(body), " ")
	if runes := []rune(line); len(runes) > limit {
		return strings.TrimRight(string(runes[:limit-1]), " ") + "…"
	}
	return line
}

// openRequestsWarning names the change requests still open, for a move to
// review (EDIT-3, FH-126); "" when there are none.
func openRequestsWarning(open []Comment) string {
	if len(open) == 0 {
		return ""
	}
	items := make([]string, 0, len(open))
	for _, comment := range open {
		items = append(items, fmt.Sprintf("%s %q (%s)", comment.ID, RequestExcerpt(comment.Body, 60), OneLine(comment.By, 60)))
	}
	if len(open) == 1 {
		return "1 change request is still open: " + items[0] + "."
	}
	return fmt.Sprintf("%d change requests are still open: %s.", len(open), strings.Join(items, "; "))
}
