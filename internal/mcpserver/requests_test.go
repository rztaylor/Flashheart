package mcpserver

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// requestChange adds a person's comment to a ticket and marks it change
// requested, as Request changes on the board does (FH-126).
func (e *env) requestChange(id, body string) string {
	e.t.Helper()
	comment := e.comment("Robert", id, store.NewComment{Body: body})
	if _, _, err := e.store.As("Robert").SetCommentState("demo", id, comment, board.CommentChangeRequested); err != nil {
		e.t.Fatal(err)
	}
	return comment
}

// ticketActivity reads a ticket's activity file.
func (e *env) ticketActivity(id string) board.Activity {
	e.t.Helper()
	folders, err := filepath.Glob(filepath.Join(e.root, "demo", "tickets", id+"-*"))
	if err != nil || len(folders) != 1 {
		e.t.Fatalf("folders = %v, %v", folders, err)
	}
	activity, _, err := e.store.ReadTicketActivity("demo", filepath.Base(folders[0]))
	if err != nil {
		e.t.Fatal(err)
	}
	return activity
}

// An agent marks a change request addressed with update_ticket's
// addressed, with a note in the same call: one history entry, by its run
// (FH-126, agent-protocol §7.2).
func TestUpdateTicketMarksChangeRequestsAddressed(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	request := e.requestChange(id, "Rename the button to Save.")
	remark := e.comment("Robert", id, store.NewComment{Body: "Nice work."})
	e.startSession(session)
	e.ok("claim", map[string]any{"ticket": id})
	before := len(e.ticketActivity(id).History)

	out := e.ok("update_ticket", map[string]any{"ticket": id, "addressed": []string{request}, "append_notes": "Renamed the button."})
	contains(t, out, "ok ticket=DM-1 changed=notes,addressed")
	activity := e.ticketActivity(id)
	if activity.Comments[0].State != board.CommentAddressed || activity.Comments[1].State != board.CommentOpen {
		t.Errorf("states = %s, %s", activity.Comments[0].State, activity.Comments[1].State)
	}
	if len(activity.History) != before+1 {
		t.Fatalf("history = %+v, want one more entry", activity.History)
	}
	entry := activity.History[len(activity.History)-1]
	if entry.By != session || len(entry.Changes) != 1 || entry.Changes[0].What != board.ChangeAddressed || entry.Changes[0].Comments[0] != request {
		t.Errorf("entry = %+v", entry)
	}
	if !strings.Contains(e.file(id), "Renamed the button.") {
		t.Error("the note was not written")
	}

	// Addressed alone is a change; a remark is not a change request, and
	// an unknown id names nothing. Refused calls write nothing.
	e.ok("update_ticket", map[string]any{"ticket": id, "addressed": []string{request}})
	text := e.fails("update_ticket", map[string]any{"ticket": id, "addressed": []string{remark}, "append_notes": "No."}, "invalid_input")
	contains(t, text, remark, "not a change request")
	e.fails("update_ticket", map[string]any{"ticket": id, "addressed": []string{"deadbeef"}}, "not_found")
	if strings.Contains(e.file(id), "No.") {
		t.Error("a refused call wrote its note")
	}
}

// Moving a ticket to review warns while a change request is open, naming
// it; it never refuses (FH-126, EDIT-3).
func TestMoveToReviewWarnsOfOpenChangeRequests(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	request := e.requestChange(id, "Rename the button to Save.")
	addressed := e.requestChange(id, "Fix the header.")
	if _, err := e.store.As(session).EditTicketAddressing("demo", id, store.Merge, board.FieldChanges{}, nil, []string{addressed}); err != nil {
		t.Fatal(err)
	}
	out := e.ok("move", map[string]any{"ticket": id, "to": "review"})
	contains(t, out, `warning: 1 change request is still open: `+request+` "Rename the button to Save." (Robert).`, "update_ticket", "ok ticket=DM-1 column=review")
	if strings.Contains(out, addressed) {
		t.Errorf("an addressed request is named as open:\n%s", out)
	}
}

// The session working on a ticket finds its open change requests in
// board_context, from the ticket itself, so a session that claims later
// gets them though nothing was queued for it; one already in its inbox
// shows once (FH-126, agent-protocol §7.2).
func TestBoardContextNamesOpenChangeRequests(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	first := e.requestChange(id, "Rename the button to Save.")
	second := e.requestChange(id, "Fix the header.")
	e.startSession(session)
	e.ok("claim", map[string]any{"ticket": id})
	if err := events.New(e.store).Queue("demo", events.Entry{Type: events.EntryChangeRequest, ID: second, Run: session, Ticket: id, Text: "Fix the header.", By: "Robert", Time: e.now}); err != nil {
		t.Fatal(err)
	}
	out := e.ok("board_context", nil)
	contains(t, out,
		"[Flashheart] Changes requested by the user on the board (their own words):",
		`- DM-1 (Robert, id `+second+`): "Fix the header."`,
		"Open change requests on DM-1 (the user's own words):",
		`- id `+first+` (Robert): "Rename the button to Save."`,
	)
	if strings.Count(out, "Fix the header.") != 1 {
		t.Errorf("a request in the inbox shows twice:\n%s", out)
	}

	// get_ticket names each request's id and state with the comments.
	contains(t, e.ok("get_ticket", map[string]any{"ticket": id}), "- Robert, 2026-10-06 10:00 UTC, change requested, id "+first+":")
}
