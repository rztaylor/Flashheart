package mcpserver

import (
	"path"
	"testing"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// tell posts a comment on ticket sent to run and queues it, as the board
// does (FH-127), and returns its id.
func (e *env) tell(ticket, run, body string) string {
	e.t.Helper()
	comment, _, err := e.store.As("Robert").AddComment("demo", ticket, store.Merge, store.NewComment{Body: body, SentTo: run})
	if err != nil {
		e.t.Fatal(err)
	}
	if err := events.New(e.store).Queue("demo", events.Entry{Type: events.EntryComment, ID: comment.ID, Run: run, Ticket: ticket, Project: "demo", Text: body, By: "Robert"}); err != nil {
		e.t.Fatal(err)
	}
	return comment.ID
}

// deliveredMark reports whether the comment is marked delivered on its
// ticket.
func (e *env) deliveredMark(ticket, comment string) bool {
	e.t.Helper()
	file, err := e.store.TicketFile("demo", ticket)
	if err != nil {
		e.t.Fatal(err)
	}
	activity, _, err := e.store.ReadTicketActivity("demo", path.Base(path.Dir(file)))
	if err != nil {
		e.t.Fatal(err)
	}
	for _, c := range activity.Comments {
		if c.ID == comment {
			return !c.Delivered.IsZero()
		}
	}
	e.t.Fatalf("no comment %s", comment)
	return false
}

// A comment sent from the board (FH-127) reaches the session with
// board_context, as it is on the ticket then, and is marked delivered
// there.
func TestBoardContextDeliversACommentSentFromTheBoard(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	e.startSession(session)
	comment := e.tell(id, session, "Use the blue one.")
	contains(t, e.ok("board_context", nil),
		"[Flashheart] Comments from the user on the board (their own words):", `- DM-1 (Robert): "Use the blue one."`)
	if !e.deliveredMark(id, comment) {
		t.Error("the comment is not marked delivered")
	}
}

// A run the MCP server started has no prompt hook: a comment sent to it
// arrives with its next tool result and is marked delivered.
func TestToolResultsDeliverACommentSentFromTheBoard(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	codex := e.dial(codexClient)
	e.okOn(codex, "claim", map[string]any{"ticket": id})
	run := e.started()[0].Run
	comment := e.tell(id, run, "Use the blue one.")
	contains(t, e.okOn(codex, "list_tickets", nil),
		"[Flashheart] Comments from the user on the board (their own words):", `- DM-1 (Robert): "Use the blue one."`)
	if !e.deliveredMark(id, comment) {
		t.Error("the comment is not marked delivered")
	}
}
