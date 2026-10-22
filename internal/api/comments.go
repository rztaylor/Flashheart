package api

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/store"
)

// A ticket's comments and history (D47, board-format §Activity): read with
// the ticket, and comments posted and edited by the person at the board.
// The store writes them as that person (store.As); this file decodes,
// checks sizes and records the human's events.

// CommentJSON is a person's comment on a ticket. ByShort names its author
// as the board shows it (a run's short id, agent-protocol §2); Edited is
// "" until it is edited; Quote is the part of the ticket it quotes, from
// QuoteFrom. A comment sent to a session (FH-127) names its run (SentTo,
// SentToShort) and where its delivery stands: waiting, delivered (at
// Delivered; it can no longer be edited) or missed (the session ended
// first); Delivery is "" for a comment not sent.
type CommentJSON struct {
	ID          string `json:"id"`
	Time        string `json:"time"`
	By          string `json:"by"`
	ByShort     string `json:"byShort"`
	ByPerson    bool   `json:"byPerson"`
	State       string `json:"state"`
	Edited      string `json:"edited"`
	SentTo      string `json:"sentTo"`
	SentToShort string `json:"sentToShort"`
	Delivery    string `json:"delivery"`
	Delivered   string `json:"delivered"`
	QuoteFrom   string `json:"quoteFrom"`
	Quote       string `json:"quote"`
	Body        string `json:"body"`
}

// HistoryJSON is one write that changed the ticket: who (ByShort as the
// board names them), when and what.
type HistoryJSON struct {
	ID       string       `json:"id"`
	Time     string       `json:"time"`
	By       string       `json:"by"`
	ByShort  string       `json:"byShort"`
	ByPerson bool         `json:"byPerson"`
	Changes  []ChangeJSON `json:"changes"`
}

// ChangeJSON is one thing a write changed (board.Change): what is created,
// moved, edited, ticked, unticked, archived, restored, a comment's
// state: change-requested, addressed, resolved or reopened, deleted
// comments, or a question asked, answered, answered-in-session or
// session-ended.
type ChangeJSON struct {
	What     string   `json:"what"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Fields   []string `json:"fields"`
	Criteria []int    `json:"criteria"`
	Steps    []int    `json:"steps"`
	// Comments are the ids of the comments whose state changed (FH-126),
	// or that were deleted (FH-140).
	Comments []string `json:"comments"`
	// Question is the question an asked, answered, answered-in-session
	// or session-ended change is about, with its text (FH-190).
	Question *QuestionChangeJSON `json:"question,omitempty"`
}

// QuestionChangeJSON is a question in the ticket's history (FH-190): its
// id, and as far as the change records them its kind, text, options and
// the answer given on the board.
type QuestionChangeJSON struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Text    string   `json:"text"`
	Options []string `json:"options"`
	Answer  string   `json:"answer"`
}

// CommentRequest is POST /api/tickets/{id}/comments. SendTo, when set,
// also sends the comment to that run, a live session holding the ticket
// or one of its subagents (TicketDetail.TellSessions, FH-127).
type CommentRequest struct {
	Body      string `json:"body"`
	Quote     string `json:"quote"`
	QuoteFrom string `json:"quoteFrom"`
	SendTo    string `json:"sendTo,omitempty"`
}

// CommentEditRequest is PATCH /api/tickets/{id}/comments/{comment}: the
// new text, and the hash of the activity the person read (STO-3); or,
// alone, the state the person moves a change request to: resolved, or
// change-requested to reopen it (FH-126).
type CommentEditRequest struct {
	Base  string `json:"base,omitempty"`
	Body  string `json:"body,omitempty"`
	State string `json:"state,omitempty"`
}

// CommentResponse reports a saved comment, the activity's new hash and the
// board revision; for a reopened change request (FH-126) or a comment
// sent to a session (FH-127), the run it was sent to, in full and as its
// short id, and any warning.
type CommentResponse struct {
	Comment      CommentJSON `json:"comment"`
	ActivityHash string      `json:"activityHash"`
	Revision     uint64      `json:"revision"`
	SentTo       string      `json:"sentTo,omitempty"`
	SentToShort  string      `json:"sentToShort,omitempty"`
	Warnings     []string    `json:"warnings,omitempty"`
}

var commentID = regexp.MustCompile(`^[0-9A-Za-z_-]{1,64}$`)

func (b boardAPI) registerComments(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tickets/{id}/comments", b.addComment)
	mux.HandleFunc("PATCH /api/tickets/{id}/comments/{comment}", b.editComment)
	mux.HandleFunc("DELETE /api/tickets/{id}/comments/{comment}", b.deleteComment)
	mux.HandleFunc("POST /api/tickets/{id}/request-changes", b.requestChanges)
}

func (b boardAPI) addComment(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request CommentRequest
	if !decode(w, r, &request) || !commentFits(w, request.Body, request.Quote) {
		return
	}
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	if request.SendTo != "" {
		b.tellComment(w, writer, project.Name, ticket, request)
		return
	}
	// A comment is appended to the activity as it is (a merge, STO-3).
	comment, hash, err := writer.AddComment(project.Name, ticket.ID, store.Merge, store.NewComment{Body: request.Body, Quote: request.Quote, QuoteFrom: request.QuoteFrom})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	b.recordHuman(project.Name, events.CommentAdded, events.TicketData{Ticket: ticket.ID, Comment: comment.ID})
	b.savedComment(w, http.StatusCreated, comment, hash)
}

func (b boardAPI) editComment(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request CommentEditRequest
	if !decode(w, r, &request) {
		return
	}
	if request.State != "" {
		b.setCommentState(w, r, writer, request)
		return
	}
	if !requireBase(w, request.Base) || !commentFits(w, request.Body, "") {
		return
	}
	id := r.PathValue("comment")
	if !commentID.MatchString(id) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("No comment %q", id))
		return
	}
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	comment, hash, err := writer.EditComment(project.Name, ticket.ID, id, store.Unchanged(request.Base), request.Body)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	b.recordHuman(project.Name, events.CommentEdited, events.TicketData{Ticket: ticket.ID, Comment: comment.ID})
	b.savedComment(w, http.StatusOK, comment, hash)
}

// CommentDeleteRequest is DELETE /api/tickets/{id}/comments/{comment}:
// the hash of the activity the person read (STO-3, FH-140).
type CommentDeleteRequest struct {
	Base string `json:"base"`
}

// CommentDeletedResponse reports a deleted comment by id, the activity's
// new hash and the board revision.
type CommentDeletedResponse struct {
	Comment      string `json:"comment"`
	ActivityHash string `json:"activityHash"`
	Revision     uint64 `json:"revision"`
}

// deleteComment deletes a comment the person posted by mistake (FH-140),
// unless its session has read it or it is a change request not resolved,
// and withdraws it from the inbox of any session it may wait for.
func (b boardAPI) deleteComment(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request CommentDeleteRequest
	if !decode(w, r, &request) || !requireBase(w, request.Base) {
		return
	}
	id := r.PathValue("comment")
	if !commentID.MatchString(id) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("No comment %q", id))
		return
	}
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	comment, hash, err := writer.DeleteComment(project.Name, ticket.ID, id, store.Unchanged(request.Base))
	switch {
	case errors.Is(err, store.ErrDelivered):
		writeError(w, http.StatusConflict, "delivered", "This comment was delivered to its session, which has read it, so it can no longer be deleted.")
		return
	case errors.Is(err, store.ErrOpenRequest):
		writeError(w, http.StatusConflict, "open_request", "This comment is a change request that is not resolved, so it cannot be deleted. Resolve it first.")
		return
	case err != nil:
		writeStoreError(w, err)
		return
	}
	b.recordHuman(project.Name, events.CommentDeleted, events.TicketData{Ticket: ticket.ID, Comment: comment.ID})
	b.withdraw(project.Name, ticket.ID, comment)
	response := CommentDeletedResponse{Comment: comment.ID, ActivityHash: hash}
	if snapshot, _ := b.board.Rebuild(); snapshot != nil {
		response.Revision = snapshot.Revision
	}
	writeJSON(w, http.StatusOK, response)
}

// withdraw takes a deleted comment out of the inboxes it may wait in: the
// session it was sent to and the ticket's live claimants, to whom a change
// request goes. Delivery leaves out a comment that is gone in any case
// (events.Deliver), so a failure here loses nothing and is not reported.
func (b boardAPI) withdraw(project, ticket string, comment board.Comment) {
	if b.events == nil {
		return
	}
	snapshot, err := b.board.RefreshRuns()
	if err != nil || snapshot == nil {
		return
	}
	targets := snapshot.Claimants(ticket)
	if sent, ok := snapshot.Run(comment.SentTo); ok {
		targets = append(targets, sent)
	}
	done := map[string]bool{}
	for _, run := range targets {
		key := run.Project + "\x00" + events.SessionOf(run.ID)
		if done[key] {
			continue
		}
		done[key] = true
		_ = b.events.Withdraw(run.Project, run.ID, project, ticket, comment.ID)
	}
}

// commentFits answers 413 for a comment or quote over its limit.
func commentFits(w http.ResponseWriter, body, quote string) bool {
	switch {
	case len(body) > board.MaxSectionText:
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("A comment is limited to %d KiB", board.MaxSectionText>>10))
	case len(quote) > board.MaxQuote:
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("A quote is limited to %d KiB", board.MaxQuote>>10))
	default:
		return true
	}
	return false
}

// savedComment rebuilds the snapshot so the response carries the new
// revision.
func (b boardAPI) savedComment(w http.ResponseWriter, status int, comment board.Comment, hash string) {
	revision := uint64(0)
	snapshot, _ := b.board.Rebuild()
	if snapshot != nil {
		revision = snapshot.Revision
	}
	writeJSON(w, status, CommentResponse{Comment: commentJSON(comment, snapshot), ActivityHash: hash, Revision: revision})
}

// shortAuthor is how the board names an entry's author: a run by its short
// id (agent-protocol §2), a person as written.
func shortAuthor(by string) string {
	if board.ByPerson(by) {
		return by
	}
	return protocol.ShortRun(by)
}

// commentJSON is a comment as the API sends it, its delivery judged by the
// snapshot's runs (nil: waiting until delivered).
func commentJSON(c board.Comment, snapshot *index.Snapshot) CommentJSON {
	edited := ""
	if !c.Edited.IsZero() {
		edited = c.Edited.UTC().Format(time.RFC3339)
	}
	return CommentJSON{
		ID: c.ID, Time: c.Time.UTC().Format(time.RFC3339), By: c.By, ByShort: shortAuthor(c.By), ByPerson: board.ByPerson(c.By),
		State: string(c.State), Edited: edited, QuoteFrom: c.QuoteFrom, Quote: c.Quote, Body: c.Body,
		SentTo: c.SentTo, SentToShort: protocol.ShortRun(c.SentTo), Delivery: delivery(snapshot, c), Delivered: deliveredAt(c),
	}
}

// activityJSON is a ticket's comments and history, oldest first.
func activityJSON(activity board.Activity, snapshot *index.Snapshot) ([]CommentJSON, []HistoryJSON) {
	comments := make([]CommentJSON, 0, len(activity.Comments))
	for _, comment := range activity.Comments {
		comments = append(comments, commentJSON(comment, snapshot))
	}
	history := make([]HistoryJSON, 0, len(activity.History))
	for _, entry := range activity.History {
		changes := make([]ChangeJSON, 0, len(entry.Changes))
		for _, change := range entry.Changes {
			item := ChangeJSON{
				What: change.What, From: change.From, To: change.To,
				Fields: nonNil(change.Fields), Criteria: nonNil(change.Criteria), Steps: nonNil(change.Steps), Comments: nonNil(change.Comments),
			}
			if board.IsQuestionChange(change.What) {
				q := change.Question
				item.Question = &QuestionChangeJSON{ID: q.ID, Kind: q.Kind, Text: q.Text, Options: nonNil(q.Options), Answer: q.Answer}
			}
			changes = append(changes, item)
		}
		history = append(history, HistoryJSON{
			ID: entry.ID, Time: entry.Time.UTC().Format(time.RFC3339), By: entry.By, ByShort: shortAuthor(entry.By), ByPerson: board.ByPerson(entry.By), Changes: changes,
		})
	}
	return comments, history
}
