package api

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

// tellAPI is requestsAPI whose snapshot reads the board and the log
// afresh on every request, as serve's watcher keeps it current.
func tellAPI(t *testing.T) (http.Handler, *events.Log) {
	t.Helper()
	root := copyBoard(t, "sample")
	files := store.New(root)
	files.SetClock(func() time.Time { return boardWriteTime })
	t.Cleanup(func() { files.Close() })
	log := events.New(files)
	return New(Options{
		Info:   Info{Root: root, Theme: "system"},
		Board:  index.New(files, index.Options{Events: log, MaxAge: time.Nanosecond}),
		Files:  files,
		Writer: files.As("Robert"),
		Events: log,
		Now:    func() time.Time { return boardWriteTime },
	}), log
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func tellRuns(detail TicketDetail) []string {
	var ids []string
	for _, run := range detail.TellSessions {
		ids = append(ids, run.Run)
	}
	return ids
}

func commentByID(t *testing.T, detail TicketDetail, id string) CommentJSON {
	t.Helper()
	at := slices.IndexFunc(detail.Comments, func(c CommentJSON) bool { return c.ID == id })
	if at < 0 {
		t.Fatalf("no comment %s in %+v", id, detail.Comments)
	}
	return detail.Comments[at]
}

// Tell a session now (D47, FH-127): the ticket offers the live session
// holding it; a comment sent to it is posted naming the run and queued in
// its inbox with the ticket's project, waits until the session takes it,
// then shows delivered and can no longer be edited.
func TestACommentSentToTheHoldingSessionIsDeliveredAndLocked(t *testing.T) {
	t.Parallel()

	handler, log := tellAPI(t)
	if runs := tellRuns(ticketDetail(t, handler, "AL-2")); len(runs) != 0 {
		t.Fatalf("an unclaimed ticket offers %v", runs)
	}
	holding(t, log, "claude:s1", "AL-2")
	detail := ticketDetail(t, handler, "AL-2")
	if !slices.Equal(tellRuns(detail), []string{"claude:s1"}) || detail.TellSessions[0].Short == "" || detail.TellSessions[0].State == "" {
		t.Fatalf("tell sessions = %+v", detail.TellSessions)
	}

	var posted CommentResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/comments", CommentRequest{Body: "Use the blue one.", SendTo: "claude:s1"}, http.StatusCreated, &posted)
	if posted.SentTo != "claude:s1" || posted.SentToShort != "claude:s1" || posted.Comment.SentTo != "claude:s1" || posted.Comment.Delivery != "waiting" || len(posted.Warnings) != 0 {
		t.Fatalf("posted = %+v", posted)
	}
	if got := commentByID(t, ticketDetail(t, handler, "AL-2"), posted.Comment.ID); got.Delivery != "waiting" || got.SentToShort == "" {
		t.Errorf("comment = %+v", got)
	}

	// Edited before delivery: allowed, and the session gets the new text.
	var edited CommentResponse
	send(t, handler, http.MethodPatch, "/api/tickets/AL-2/comments/"+posted.Comment.ID, CommentEditRequest{Base: posted.ActivityHash, Body: "Use the green one."}, http.StatusOK, &edited)

	waiting, err := events.Deliver(log, "alpha", []events.Mailbox{{Session: "claude:s1"}}, nil, time.Now().UTC())
	if err != nil || len(waiting) != 1 {
		t.Fatalf("delivered %+v, %v", waiting, err)
	}
	if e := waiting[0]; e.Type != events.EntryComment || e.Ticket != "AL-2" || e.Project != "alpha" || e.Text != "Use the green one." || e.By != "Robert" {
		t.Errorf("entry = %+v", e)
	}
	detail = ticketDetail(t, handler, "AL-2")
	got := commentByID(t, detail, posted.Comment.ID)
	if got.Delivery != "delivered" || got.Delivered == "" {
		t.Errorf("comment = %+v", got)
	}
	var refused errorBody
	send(t, handler, http.MethodPatch, "/api/tickets/AL-2/comments/"+posted.Comment.ID, CommentEditRequest{Base: detail.ActivityHash, Body: "Too late."}, http.StatusConflict, &refused)
	if refused.Error.Code != "delivered" {
		t.Errorf("edit after delivery = %+v", refused)
	}
}

// The server checks the chosen run when the comment is sent: one that is
// not a live claimant of the ticket is refused, and nothing is posted or
// queued.
func TestTellRefusesARunThatIsNotALiveClaimant(t *testing.T) {
	t.Parallel()

	handler, log := tellAPI(t)
	var refused errorBody
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/comments", CommentRequest{Body: "Hi", SendTo: "claude:s1"}, http.StatusConflict, &refused)
	if refused.Error.Code != "not_live" || !strings.Contains(refused.Error.Message, "AL-2") {
		t.Errorf("no holder: %+v", refused)
	}
	holding(t, log, "claude:s1", "AL-2")
	holding(t, log, "claude:s2", "AL-3")
	for name, run := range map[string]string{"another ticket's holder": "claude:s2", "unknown": "claude:nobody", "not a run": "Robert"} {
		var body errorBody
		send(t, handler, http.MethodPost, "/api/tickets/AL-2/comments", CommentRequest{Body: "Hi", SendTo: run}, http.StatusConflict, &body)
		if body.Error.Code != "not_live" {
			t.Errorf("%s: %+v", name, body)
		}
	}
	if err := log.Append(events.Event{Time: time.Now().UTC(), Run: "claude:s1", Agent: "claude", Kind: events.RunEnd, Project: "alpha", Data: events.RunEndData{Reason: "other"}}); err != nil {
		t.Fatal(err)
	}
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/comments", CommentRequest{Body: "Hi", SendTo: "claude:s1"}, http.StatusConflict, nil)
	if comments := ticketDetail(t, handler, "AL-2").Comments; len(comments) != 0 {
		t.Errorf("a refused comment was posted: %+v", comments)
	}
	for _, session := range []string{"claude:s1", "claude:s2"} {
		if queued, _ := log.Take("alpha", events.Mailbox{Session: session}); len(queued) != 0 {
			t.Errorf("queued for %s: %+v", session, queued)
		}
	}
}

// A comment whose session ends before taking it stays on the ticket,
// undelivered, and says so; it stays editable, and nothing is queued for
// the session that claims the ticket next.
func TestACommentItsSessionNeverTookSaysSo(t *testing.T) {
	t.Parallel()

	handler, log := tellAPI(t)
	holding(t, log, "claude:s1", "AL-2")
	var posted CommentResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/comments", CommentRequest{Body: "Use the blue one.", SendTo: "claude:s1"}, http.StatusCreated, &posted)
	if err := log.Append(events.Event{Time: time.Now().UTC(), Run: "claude:s1", Agent: "claude", Kind: events.RunEnd, Project: "alpha", Data: events.RunEndData{Reason: "other"}}); err != nil {
		t.Fatal(err)
	}
	holding(t, log, "claude:s2", "AL-2")
	detail := ticketDetail(t, handler, "AL-2")
	if got := commentByID(t, detail, posted.Comment.ID); got.Delivery != "missed" || got.SentTo != "claude:s1" || got.Delivered != "" {
		t.Errorf("comment = %+v", got)
	}
	if !slices.Equal(tellRuns(detail), []string{"claude:s2"}) {
		t.Errorf("tell sessions = %v", tellRuns(detail))
	}
	if queued, _ := log.Take("alpha", events.Mailbox{Session: "claude:s2"}); len(queued) != 0 {
		t.Errorf("queued for the next session: %+v", queued)
	}
	send(t, handler, http.MethodPatch, "/api/tickets/AL-2/comments/"+posted.Comment.ID, CommentEditRequest{Base: detail.ActivityHash, Body: "Still mine to edit."}, http.StatusOK, nil)
}
