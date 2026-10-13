package api

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// Telling a session now (D47, FH-127): a person may send a comment to a
// live session that has claimed the ticket as well as posting it. The
// ticket offers those runs (index.Snapshot.Claimants); the server checks
// the chosen one again when the comment is sent, posts the comment naming
// it and queues the comment in its session's inbox, from which every
// delivery path hands it over and marks it delivered on the comment
// (events.Deliver). Nothing is ever queued for a later session.

// TellSessionJSON is a run a comment may be sent to now: a live session
// holding the ticket, or one of its live subagents (Parent its session,
// AgentType what kind of subagent it is).
type TellSessionJSON struct {
	Run       string `json:"run"`
	Short     string `json:"short"`
	Agent     string `json:"agent"`
	State     string `json:"state"`
	Parent    string `json:"parent"`
	AgentType string `json:"agentType"`
}

// Comment delivery states (CommentJSON.Delivery) for a comment sent to a
// session: waiting for it to be taken, delivered, or missed because the
// session ended first. A comment not sent has none.
const (
	deliveryWaiting   = "waiting"
	deliveryDelivered = "delivered"
	deliveryMissed    = "missed"
)

// tellSessions lists the runs a comment on ticket may be sent to now.
func tellSessions(snapshot *index.Snapshot, ticket string) []TellSessionJSON {
	list := []TellSessionJSON{}
	for _, run := range snapshot.Claimants(ticket) {
		list = append(list, TellSessionJSON{
			Run: run.ID, Short: protocol.ShortRun(run.ID), Agent: run.Agent, State: string(run.State), Parent: run.Parent, AgentType: run.AgentType,
		})
	}
	return list
}

// delivery is where a comment sent to a session stands, as the snapshot's
// runs say: a run the window no longer has has ended.
func delivery(snapshot *index.Snapshot, c board.Comment) string {
	switch {
	case c.SentTo == "":
		return ""
	case !c.Delivered.IsZero():
		return deliveryDelivered
	case snapshot == nil:
		return deliveryWaiting
	}
	if run, ok := snapshot.Run(c.SentTo); ok && run.State != runs.Ended {
		return deliveryWaiting
	}
	return deliveryMissed
}

// claimant finds run among the live claimants of ticket in a snapshot
// with the latest events, so a session that ended or let go since the page
// was read is not sent to.
func (b boardAPI) claimant(ticket, run string) (runs.View, bool) {
	snapshot, err := b.board.RefreshRuns()
	if err != nil || snapshot == nil {
		return runs.View{}, false
	}
	list := snapshot.Claimants(ticket)
	at := slices.IndexFunc(list, func(v runs.View) bool { return v.ID == run })
	if at < 0 {
		return runs.View{}, false
	}
	return list[at], true
}

// tellComment is POST /api/tickets/{id}/comments with sendTo: the comment
// is posted naming the run and queued for it, once the run is found to be
// a live claimant of the ticket; otherwise nothing is posted. A comment
// that cannot be queued stays posted, unsent, with a warning.
func (b boardAPI) tellComment(w http.ResponseWriter, writer Writer, project string, ticket board.Ticket, request CommentRequest) {
	run, ok := b.claimant(ticket.ID, request.SendTo)
	if !ok {
		writeError(w, http.StatusConflict, "not_live", fmt.Sprintf("%s is not a live session holding %s now, so the comment was not posted. Post it without telling a session, or choose another.", protocol.ShortRun(request.SendTo), ticket.ID))
		return
	}
	comment, hash, err := writer.AddComment(project, ticket.ID, store.Merge, store.NewComment{Body: request.Body, Quote: request.Quote, QuoteFrom: request.QuoteFrom, SentTo: run.ID})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	b.recordHuman(project, events.CommentAdded, events.TicketData{Ticket: ticket.ID, Comment: comment.ID})
	response := CommentResponse{ActivityHash: hash, SentTo: run.ID, SentToShort: protocol.ShortRun(run.ID)}
	if err := b.queue(run, project, ticket.ID, events.EntryComment, []board.Comment{comment}); err != nil {
		response.SentTo, response.SentToShort = "", ""
		response.Warnings = []string{fmt.Sprintf("The comment is on %s, but it could not be sent to %s now (%v); the session sees it when it reads the ticket.", ticket.ID, protocol.ShortRun(run.ID), err)}
		if unsent, unsentHash, err := writer.UnsendComment(project, ticket.ID, comment.ID); err == nil {
			comment, response.ActivityHash = unsent, unsentHash
		}
	}
	snapshot, _ := b.board.Rebuild()
	if snapshot != nil {
		response.Revision = snapshot.Revision
	}
	response.Comment = commentJSON(comment, snapshot)
	writeJSON(w, http.StatusCreated, response)
}

// queue puts comments of type typ on ticket in the inbox of run's session,
// in its home project, naming the ticket's project, so the delivery that
// takes them marks them delivered on the ticket (agent-protocol §4).
func (b boardAPI) queue(run runs.View, project, ticket string, typ events.EntryType, comments []board.Comment) error {
	if b.events == nil {
		return errors.New("this server keeps no event log")
	}
	now := b.now().UTC()
	for _, comment := range comments {
		entry := events.Entry{Type: typ, ID: comment.ID, Run: run.ID, Ticket: ticket, Project: project, Text: comment.Body, By: comment.By, Time: now}
		if err := b.events.Queue(run.Project, entry); err != nil {
			return err
		}
	}
	return nil
}

// deliveredAt is a comment's delivery time as the API sends it.
func deliveredAt(c board.Comment) string {
	if c.Delivered.IsZero() {
		return ""
	}
	return c.Delivered.UTC().Format(time.RFC3339)
}
