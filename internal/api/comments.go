package api

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// A ticket's comments and history (D47, board-format §Activity): read with
// the ticket, and comments posted and edited by the person at the board.
// The store writes them as that person (store.As); this file decodes,
// checks sizes and records the human's events.

// CommentJSON is a person's comment on a ticket. Edited is "" until it is
// edited; Quote is the part of the ticket it quotes, from QuoteFrom.
type CommentJSON struct {
	ID        string `json:"id"`
	Time      string `json:"time"`
	By        string `json:"by"`
	ByPerson  bool   `json:"byPerson"`
	State     string `json:"state"`
	Edited    string `json:"edited"`
	QuoteFrom string `json:"quoteFrom"`
	Quote     string `json:"quote"`
	Body      string `json:"body"`
}

// HistoryJSON is one write that changed the ticket: who, when and what.
type HistoryJSON struct {
	ID       string       `json:"id"`
	Time     string       `json:"time"`
	By       string       `json:"by"`
	ByPerson bool         `json:"byPerson"`
	Changes  []ChangeJSON `json:"changes"`
}

// ChangeJSON is one thing a write changed (board.Change): what is created,
// moved, edited, ticked, unticked, archived or restored.
type ChangeJSON struct {
	What     string   `json:"what"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Fields   []string `json:"fields"`
	Criteria []int    `json:"criteria"`
	Steps    []int    `json:"steps"`
}

// CommentRequest is POST /api/tickets/{id}/comments.
type CommentRequest struct {
	Body      string `json:"body"`
	Quote     string `json:"quote"`
	QuoteFrom string `json:"quoteFrom"`
}

// CommentEditRequest is PATCH /api/tickets/{id}/comments/{comment}: the
// new text, and the hash of the activity the person read (STO-3).
type CommentEditRequest struct {
	Base string `json:"base"`
	Body string `json:"body"`
}

// CommentResponse reports a saved comment, the activity's new hash and the
// board revision.
type CommentResponse struct {
	Comment      CommentJSON `json:"comment"`
	ActivityHash string      `json:"activityHash"`
	Revision     uint64      `json:"revision"`
}

var commentID = regexp.MustCompile(`^[0-9A-Za-z_-]{1,64}$`)

func (b boardAPI) registerComments(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tickets/{id}/comments", b.addComment)
	mux.HandleFunc("PATCH /api/tickets/{id}/comments/{comment}", b.editComment)
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
	if !decode(w, r, &request) || !requireBase(w, request.Base) || !commentFits(w, request.Body, "") {
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
	if snapshot, _ := b.board.Rebuild(); snapshot != nil {
		revision = snapshot.Revision
	}
	writeJSON(w, status, CommentResponse{Comment: commentJSON(comment), ActivityHash: hash, Revision: revision})
}

func commentJSON(c board.Comment) CommentJSON {
	edited := ""
	if !c.Edited.IsZero() {
		edited = c.Edited.UTC().Format(time.RFC3339)
	}
	return CommentJSON{
		ID: c.ID, Time: c.Time.UTC().Format(time.RFC3339), By: c.By, ByPerson: board.ByPerson(c.By),
		State: string(c.State), Edited: edited, QuoteFrom: c.QuoteFrom, Quote: c.Quote, Body: c.Body,
	}
}

// activityJSON is a ticket's comments and history, oldest first.
func activityJSON(activity board.Activity) ([]CommentJSON, []HistoryJSON) {
	comments := make([]CommentJSON, 0, len(activity.Comments))
	for _, comment := range activity.Comments {
		comments = append(comments, commentJSON(comment))
	}
	history := make([]HistoryJSON, 0, len(activity.History))
	for _, entry := range activity.History {
		changes := make([]ChangeJSON, 0, len(entry.Changes))
		for _, change := range entry.Changes {
			changes = append(changes, ChangeJSON{
				What: change.What, From: change.From, To: change.To,
				Fields: nonNil(change.Fields), Criteria: nonNil(change.Criteria), Steps: nonNil(change.Steps),
			})
		}
		history = append(history, HistoryJSON{
			ID: entry.ID, Time: entry.Time.UTC().Format(time.RFC3339), By: entry.By, ByPerson: board.ByPerson(entry.By), Changes: changes,
		})
	}
	return comments, history
}
