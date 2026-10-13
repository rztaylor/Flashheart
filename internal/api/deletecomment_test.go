package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
)

// A person deletes a comment posted by mistake (FH-140) on the hash of the
// activity they read: the comment leaves the ticket, history names it
// without its text, the human's comment.deleted is recorded, and a
// comment sent to a session and not yet delivered is withdrawn from that
// session's inbox.
func TestACommentIsDeletedAsTheHumanAndWithdrawnFromItsSession(t *testing.T) {
	t.Parallel()

	handler, log := tellAPI(t)
	holding(t, log, "claude:s1", "AL-4")
	var kept, posted CommentResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-4/comments", CommentRequest{Body: "Keep this."}, http.StatusCreated, &kept)
	send(t, handler, http.MethodPost, "/api/tickets/AL-4/comments", CommentRequest{Body: "The secret slip.", SendTo: "claude:s1"}, http.StatusCreated, &posted)
	if posted.Comment.Delivery != "waiting" {
		t.Fatalf("posted = %+v", posted)
	}
	count := len(boardLog(t, log))
	path := "/api/tickets/AL-4/comments/" + posted.Comment.ID

	// A stale hash is a conflict and deletes nothing.
	send(t, handler, http.MethodDelete, path, CommentDeleteRequest{Base: kept.ActivityHash}, http.StatusConflict, nil)
	if len(ticketDetail(t, handler, "AL-4").Comments) != 2 {
		t.Fatal("a stale delete removed the comment")
	}

	var deleted CommentDeletedResponse
	send(t, handler, http.MethodDelete, path, CommentDeleteRequest{Base: posted.ActivityHash}, http.StatusOK, &deleted)
	detail := ticketDetail(t, handler, "AL-4")
	if deleted.ActivityHash != detail.ActivityHash || deleted.Revision == 0 || deleted.Comment != posted.Comment.ID {
		t.Errorf("deleted = %+v, detail hash %q", deleted, detail.ActivityHash)
	}
	if len(detail.Comments) != 1 || detail.Comments[0].ID != kept.Comment.ID {
		t.Errorf("comments = %+v", detail.Comments)
	}
	last := detail.History[len(detail.History)-1]
	if last.By != "Robert" || !slices.Equal(whats(last), []string{board.ChangeDeleted}) || !slices.Equal(last.Changes[0].Comments, []string{posted.Comment.ID}) {
		t.Errorf("history = %+v", last)
	}
	if body, _ := json.Marshal(detail); strings.Contains(string(body), "secret slip") {
		t.Errorf("the ticket still holds the text: %s", body)
	}
	list := boardLog(t, log)
	recorded := slices.IndexFunc(list, func(e events.Event) bool {
		var data events.TicketData
		return e.Kind == events.CommentDeleted && e.Decode(&data) == nil && e.Run == events.Human &&
			data.By == events.Human && data.Ticket == "AL-4" && data.Comment == posted.Comment.ID
	})
	if len(list) != count+1 || recorded < 0 {
		t.Errorf("events = %+v, want one comment.deleted by the human", list[count:])
	}

	// Withdrawn: the session's inbox no longer holds it, so no path can
	// hand it over, and its text is not left on disk.
	if queued, err := log.Take("alpha", events.Mailbox{Session: "claude:s1"}); err != nil || len(queued) != 0 {
		t.Errorf("still queued: %+v, %v", queued, err)
	}
}

// A comment delivered to its session, or a change request not resolved,
// is refused with a reason in words, and nothing is recorded; requests
// are checked like an edit's.
func TestDeletingACommentIsRefusedWithAReason(t *testing.T) {
	t.Parallel()

	handler, log := tellAPI(t)
	holding(t, log, "claude:s1", "AL-2")
	var sent CommentResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/comments", CommentRequest{Body: "Read already.", SendTo: "claude:s1"}, http.StatusCreated, &sent)
	if _, err := events.Deliver(log, "alpha", []events.Mailbox{{Session: "claude:s1"}}, nil, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var requested RequestChangesResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/request-changes", RequestChangesRequest{Comment: &CommentRequest{Body: "Rename it."}}, http.StatusOK, &requested)
	request := requested.Comments[0].ID
	base := ticketDetail(t, handler, "AL-2").ActivityHash
	count := len(boardLog(t, log))

	for _, tc := range []struct {
		name, id, code, words string
		status                int
	}{
		{"delivered", sent.Comment.ID, "delivered", "can no longer be deleted", http.StatusConflict},
		{"an open request", request, "open_request", "Resolve it", http.StatusConflict},
	} {
		var refused errorBody
		send(t, handler, http.MethodDelete, "/api/tickets/AL-2/comments/"+tc.id, CommentDeleteRequest{Base: base}, tc.status, &refused)
		if refused.Error.Code != tc.code || !strings.Contains(refused.Error.Message, tc.words) {
			t.Errorf("%s: %+v", tc.name, refused)
		}
	}
	for _, tc := range []struct {
		name, path string
		body       any
		status     int
	}{
		{"no base", "/api/tickets/AL-2/comments/" + request, CommentDeleteRequest{}, http.StatusBadRequest},
		{"missing comment", "/api/tickets/AL-2/comments/deadbeef", CommentDeleteRequest{Base: base}, http.StatusNotFound},
		{"bad comment id", "/api/tickets/AL-2/comments/..%2Fx", CommentDeleteRequest{Base: base}, http.StatusNotFound},
		{"missing ticket", "/api/tickets/AL-99/comments/" + request, CommentDeleteRequest{Base: base}, http.StatusNotFound},
		{"unknown field", "/api/tickets/AL-2/comments/" + request, `{"base":"x","by":"claude:1"}`, http.StatusBadRequest},
	} {
		send(t, handler, http.MethodDelete, tc.path, tc.body, tc.status, nil)
	}
	if after := len(boardLog(t, log)); after != count {
		t.Errorf("refused deletes recorded %d events", after-count)
	}
	if comments := ticketDetail(t, handler, "AL-2").Comments; len(comments) != 2 {
		t.Errorf("comments = %+v", comments)
	}
}
