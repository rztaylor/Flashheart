package api

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/store"
)

// Change requests in review (D47, FH-126): the person at the board asks
// for changes on a ticket in review, which goes back to In progress, and
// later resolves or reopens each request an agent addressed. The store
// writes the states and their history as that person; this file decodes,
// records the human's events and sends the requests to the live session
// holding the ticket, if any.

// RequestsJSON counts a ticket's change requests (board.RequestCount):
// Open waits for an agent, Addressed for the person.
type RequestsJSON struct {
	Open      int `json:"open"`
	Addressed int `json:"addressed"`
}

func requestsJSON(count board.RequestCount) RequestsJSON {
	return RequestsJSON{Open: count.Open, Addressed: count.Addressed}
}

// RequestChangesRequest is POST /api/tickets/{id}/request-changes: the
// existing comments to mark as change requested, and a new one to add as
// one; at least one of them.
type RequestChangesRequest struct {
	Comments []string        `json:"comments"`
	Comment  *CommentRequest `json:"comment,omitempty"`
}

// RequestChangesResponse reports a request for changes: the comments now
// change requested, the ticket's and the activity's new hashes, the board
// revision, and the run they were sent to ("" when no live session holds
// the ticket: the next one finds them on it).
type RequestChangesResponse struct {
	Comments     []CommentJSON `json:"comments"`
	Hash         string        `json:"hash"`
	ActivityHash string        `json:"activityHash"`
	Revision     uint64        `json:"revision"`
	SentTo       string        `json:"sentTo"`
	Warnings     []string      `json:"warnings"`
}

func (b boardAPI) requestChanges(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request RequestChangesRequest
	if !decode(w, r, &request) {
		return
	}
	var added *store.NewComment
	if request.Comment != nil {
		if !commentFits(w, request.Comment.Body, request.Comment.Quote) {
			return
		}
		added = &store.NewComment{Body: request.Comment.Body, Quote: request.Comment.Quote, QuoteFrom: request.Comment.QuoteFrom}
	}
	for _, id := range request.Comments {
		if !commentID.MatchString(id) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("No comment %q", id))
			return
		}
	}
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	result, err := writer.RequestChanges(project.Name, ticket.ID, request.Comments, added)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	b.recordHuman(project.Name, events.TicketMoved, events.TicketData{Ticket: ticket.ID, From: string(board.Review), To: string(board.InProgress)})
	for _, comment := range result.Comments {
		if added != nil && !slices.Contains(request.Comments, comment.ID) {
			b.recordHuman(project.Name, events.CommentAdded, events.TicketData{Ticket: ticket.ID, Comment: comment.ID})
		} else {
			b.recordHuman(project.Name, events.CommentEdited, events.TicketData{Ticket: ticket.ID, Comment: comment.ID, Fields: []string{"state"}})
		}
	}
	sentTo, warnings := b.sendRequests(project.Name, ticket.ID, result.Comments)
	response := RequestChangesResponse{
		Comments: make([]CommentJSON, 0, len(result.Comments)), Hash: result.Hash, ActivityHash: result.ActivityHash,
		SentTo: sentTo, Warnings: nonNil(warnings),
	}
	snapshot, _ := b.board.Rebuild()
	if snapshot != nil {
		response.Revision = snapshot.Revision
	}
	for _, comment := range result.Comments {
		response.Comments = append(response.Comments, commentJSON(comment, snapshot))
	}
	writeJSON(w, http.StatusOK, response)
}

// setCommentState is PATCH /api/tickets/{id}/comments/{comment} with a
// state: the person resolves a request or reopens it (asks for the
// change again). A reopened request is sent to the live holder.
func (b boardAPI) setCommentState(w http.ResponseWriter, r *http.Request, writer Writer, request CommentEditRequest) {
	to := board.CommentState(request.State)
	if request.Body != "" || request.Base != "" || (to != board.CommentResolved && to != board.CommentChangeRequested) {
		writeError(w, http.StatusBadRequest, "invalid_input", "Send either a comment's new text with its base, or its state: resolved or change-requested")
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
	// A state change merges with the activity as it is, as a tick does.
	comment, hash, err := writer.SetCommentState(project.Name, ticket.ID, id, to)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	b.recordHuman(project.Name, events.CommentEdited, events.TicketData{Ticket: ticket.ID, Comment: comment.ID, Fields: []string{"state"}})
	response := CommentResponse{ActivityHash: hash}
	if to == board.CommentChangeRequested {
		response.SentTo, response.Warnings = b.sendRequests(project.Name, ticket.ID, []board.Comment{comment})
	}
	snapshot, _ := b.board.Rebuild()
	if snapshot != nil {
		response.Revision = snapshot.Revision
	}
	response.Comment = commentJSON(comment, snapshot)
	writeJSON(w, http.StatusOK, response)
}

// sendRequests queues change requests for the live session holding the
// ticket (index.Snapshot.Holder) in its home project's inbox, which
// delivers them with its next prompt or tool result (agent-protocol §4).
// It returns that run, or "" when no live session holds the ticket: the
// next session reads them from the ticket (recovery note, board_context).
// The requests are saved already, so a failure to queue is a warning.
func (b boardAPI) sendRequests(project, ticket string, comments []board.Comment) (string, []string) {
	if b.events == nil || len(comments) == 0 {
		return "", nil
	}
	snapshot, err := b.board.RefreshRuns()
	if err != nil || snapshot == nil {
		return "", nil
	}
	holder, ok := snapshot.Holder(ticket)
	if !ok {
		return "", nil
	}
	if err := b.queue(holder, project, ticket, events.EntryChangeRequest, comments); err != nil {
		return "", []string{fmt.Sprintf("The changes are on %s, but they could not be sent to %s now (%v); the session sees them when it reads the ticket.", ticket, protocol.ShortRun(holder.ID), err)}
	}
	return holder.ID, nil
}
