package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

// whats lists a history entry's changes by what they record.
func whats(entry HistoryJSON) []string {
	var list []string
	for _, change := range entry.Changes {
		list = append(list, change.What)
	}
	return list
}

// Every board write that changes a ticket appends one history entry to its
// activity file, by the person at the board, naming what changed and never
// the ticket's text (D47, board-format §Activity).
func TestBoardWritesAppendOneHistoryEntry(t *testing.T) {
	t.Parallel()

	handler, log := activityAPI(t)
	expect := func(id string, want ...string) {
		t.Helper()
		history := ticketDetail(t, handler, id).History
		if len(history) == 0 {
			t.Fatalf("%s has no history", id)
		}
		last := history[len(history)-1]
		if last.By != "Robert" || !last.ByPerson || last.Time != "2026-10-09T18:30:00Z" || last.ID == "" || !slices.Equal(whats(last), want) {
			t.Errorf("%s history = %+v, want %v by Robert", id, last, want)
		}
	}
	count := func(id string) int { return len(ticketDetail(t, handler, id).History) }

	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", MoveRequest{Base: ticketDetail(t, handler, "AL-2").Hash, To: "done"}, http.StatusOK, nil)
	expect("AL-2", board.ChangeMoved)
	if history := ticketDetail(t, handler, "AL-2").History; history[0].Changes[0].From != "review" || history[0].Changes[0].To != "done" {
		t.Errorf("move = %+v", history[0].Changes)
	}

	// A reorder within its column is no history; a place in another
	// column is a move.
	top := ""
	send(t, handler, http.MethodPost, "/api/tickets/AL-6/move", MoveRequest{To: "backlog", After: &top}, http.StatusOK, nil)
	if n := count("AL-6"); n != 0 {
		t.Errorf("a reorder wrote %d history entries", n)
	}
	send(t, handler, http.MethodPost, "/api/tickets/AL-6/move", MoveRequest{To: "up-next", After: &top}, http.StatusOK, nil)
	expect("AL-6", board.ChangeMoved)

	send(t, handler, http.MethodPost, "/api/tickets/AL-5/move", MoveRequest{To: "in-progress", Reason: "Hotfix"}, http.StatusOK, nil)
	expect("AL-5", board.ChangeMoved)

	send(t, handler, http.MethodPatch, "/api/tickets/AL-4", map[string]any{"base": ticketDetail(t, handler, "AL-4").Hash,
		"fields": map[string]any{"workstream": "", "priority": "high", "title": "A secret title"}}, http.StatusOK, nil)
	expect("AL-4", board.ChangeEdited)
	if fields := ticketDetail(t, handler, "AL-4").History[0].Changes[0].Fields; !slices.Equal(fields, []string{"priority", "workstream", "title"}) {
		t.Errorf("edited fields = %v", fields)
	}

	send(t, handler, http.MethodPost, "/api/tickets/AL-3/criteria", CriterionRequest{Base: ticketDetail(t, handler, "AL-3").Hash, Index: 1, Checked: true}, http.StatusOK, nil)
	expect("AL-3", board.ChangeTicked)

	// A section edited in place (FH-66) names its section, not its text.
	send(t, handler, http.MethodPut, "/api/tickets/AL-3/section", SectionRequest{Base: ticketDetail(t, handler, "AL-3").Hash, Section: "Context", Content: "A secret context."}, http.StatusOK, nil)
	expect("AL-3", board.ChangeEdited)
	if history := ticketDetail(t, handler, "AL-3").History; !slices.Equal(history[len(history)-1].Changes[0].Fields, []string{"context"}) {
		t.Errorf("section edit fields = %v", history[len(history)-1].Changes[0].Fields)
	}

	raw := strings.Replace(ticketDetail(t, handler, "AL-3").Raw, "Fixture ticket.", "A secret body.", 1)
	send(t, handler, http.MethodPut, "/api/tickets/AL-3/raw", RawRequest{Base: ticketDetail(t, handler, "AL-3").Hash, Content: raw}, http.StatusOK, nil)
	expect("AL-3", board.ChangeEdited)

	review := ticketDetail(t, handler, "AL-2").Review
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/review/steps", ReviewStepRequest{Base: review.Hash, Index: 0, Checked: true}, http.StatusOK, nil)
	expect("AL-2", board.ChangeTicked)

	var created CreateResponse
	send(t, handler, http.MethodPost, "/api/projects/beta/tickets", CreateRequest{Title: "A secret plan", Description: "Secret."}, http.StatusCreated, &created)
	expect(created.ID, board.ChangeCreated)
	send(t, handler, http.MethodPost, "/api/tickets/"+created.ID+"/archive", "", http.StatusOK, nil)
	send(t, handler, http.MethodPost, "/api/tickets/"+created.ID+"/unarchive", "", http.StatusOK, nil)
	history := ticketDetail(t, handler, created.ID).History
	if got := [][]string{whats(history[0]), whats(history[1]), whats(history[2])}; len(history) != 3 ||
		!slices.EqualFunc(got, [][]string{{board.ChangeCreated}, {board.ChangeArchived}, {board.ChangeRestored}}, slices.Equal) {
		t.Errorf("history = %+v", history)
	}

	// An answer to an agent's question is a note, not history.
	before := count("AL-3")
	_ = log
	send(t, handler, http.MethodPost, "/api/tickets/AL-3/criteria", CriterionRequest{Base: "stale", Index: 0, Checked: true}, http.StatusConflict, nil)
	if after := count("AL-3"); after != before {
		t.Errorf("a refused write appended %d history entries", after-before)
	}

	// No ticket text reaches history.
	for _, id := range []string{"AL-2", "AL-3", "AL-4", "AL-5", "AL-6", created.ID} {
		body, _ := json.Marshal(ticketDetail(t, handler, id).History)
		if strings.Contains(strings.ToLower(string(body)), "secret") || strings.Contains(string(body), "Hotfix") {
			t.Errorf("%s history holds ticket text: %s", id, body)
		}
	}
}

func TestCommentsArePostedAndEditedAsTheHuman(t *testing.T) {
	t.Parallel()

	handler, log := activityAPI(t)
	detail := ticketDetail(t, handler, "AL-4")
	if len(detail.Comments) != 0 || len(detail.History) != 0 || detail.ActivityHash != "" {
		t.Fatalf("a ticket without activity = %+v %+v %q", detail.Comments, detail.History, detail.ActivityHash)
	}
	count := len(boardLog(t, log))

	var posted CommentResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-4/comments", CommentRequest{
		Body: "Keep the place <script>alert(1)</script>.", Quote: "Second criterion", QuoteFrom: "Acceptance Criteria 2",
	}, http.StatusCreated, &posted)
	comment := posted.Comment
	if comment.ID == "" || comment.By != "Robert" || !comment.ByPerson || comment.State != "open" || comment.Edited != "" ||
		comment.Quote != "Second criterion" || comment.QuoteFrom != "Acceptance Criteria 2" || posted.ActivityHash == "" || posted.Revision == 0 {
		t.Errorf("posted = %+v", posted)
	}
	count = expectComment(t, log, count, events.CommentAdded, comment.ID)

	detail = ticketDetail(t, handler, "AL-4")
	if len(detail.Comments) != 1 || detail.Comments[0] != comment || detail.ActivityHash != posted.ActivityHash {
		t.Fatalf("detail = %+v, hash %q", detail.Comments, detail.ActivityHash)
	}
	// Text is data: the response carries it as an escaped JSON string.
	if body := getJSON(t, handler, "/api/tickets/AL-4", http.StatusOK, nil).Body.String(); strings.Contains(body, "<script>") {
		t.Errorf("raw HTML in the response: %s", body)
	}

	var edited CommentResponse
	send(t, handler, http.MethodPatch, "/api/tickets/AL-4/comments/"+comment.ID, CommentEditRequest{Base: posted.ActivityHash, Body: "Keep the place everywhere."}, http.StatusOK, &edited)
	if edited.Comment.Body != "Keep the place everywhere." || edited.Comment.Edited == "" || edited.Comment.Quote != "Second criterion" || edited.ActivityHash == posted.ActivityHash {
		t.Errorf("edited = %+v", edited)
	}
	count = expectComment(t, log, count, events.CommentEdited, comment.ID)

	// A stale hash is a conflict with the current file; nothing changes.
	response := send2(t, handler, http.MethodPatch, "/api/tickets/AL-4/comments/"+comment.ID, CommentEditRequest{Base: posted.ActivityHash, Body: "Stale."}, http.StatusConflict)
	var conflict struct {
		Error   struct{ Code, Message string }
		Current ConflictJSON
	}
	if err := json.Unmarshal(response, &conflict); err != nil || conflict.Error.Code != "conflict" || conflict.Current.Hash != edited.ActivityHash ||
		!strings.Contains(conflict.Current.Content, "everywhere") || !strings.Contains(conflict.Error.Message, "comments") {
		t.Errorf("conflict = %+v (%v)", conflict, err)
	}
	if after := len(boardLog(t, log)); after != count {
		t.Errorf("a refused edit recorded %d events", after-count)
	}
	// The ticket file is not touched by comments.
	if ticketDetail(t, handler, "AL-4").Hash != detail.Hash {
		t.Error("a comment changed the ticket file")
	}
}

// send2 is send that returns the body of a response with the given status.
func send2(t *testing.T, handler http.Handler, method, path string, body any, status int) []byte {
	t.Helper()
	var raw json.RawMessage
	send(t, handler, method, path, body, status, &raw)
	return raw
}

// expectComment checks that the log grew by one comment event naming the
// comment, attributed to the human.
func expectComment(t *testing.T, log *events.Log, before int, kind events.Kind, id string) int {
	t.Helper()
	list := boardLog(t, log)
	if len(list) != before+1 {
		t.Fatalf("%s: %d new events, want 1", kind, len(list)-before)
	}
	var data events.TicketData
	got := list[len(list)-1]
	for _, e := range list {
		if e.Project == "alpha" {
			got = e
		}
	}
	if err := got.Decode(&data); err != nil {
		t.Fatal(err)
	}
	if got.Kind != kind || got.Run != events.Human || data.By != events.Human || data.Ticket != "AL-4" || data.Comment != id {
		t.Errorf("event = %+v %+v", got, data)
	}
	return len(list)
}

func TestCommentRequestsAreChecked(t *testing.T) {
	t.Parallel()

	handler, log := activityAPI(t)
	var posted CommentResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-4/comments", CommentRequest{Body: "First."}, http.StatusCreated, &posted)
	count := len(boardLog(t, log))
	for _, tc := range []struct {
		name, method, path string
		body               any
		status             int
	}{
		{"empty", http.MethodPost, "/api/tickets/AL-4/comments", CommentRequest{Body: "  \n"}, http.StatusBadRequest},
		{"too long", http.MethodPost, "/api/tickets/AL-4/comments", CommentRequest{Body: strings.Repeat("x", board.MaxSectionText+1)}, http.StatusRequestEntityTooLarge},
		{"long quote", http.MethodPost, "/api/tickets/AL-4/comments", CommentRequest{Body: "x", Quote: strings.Repeat("q", board.MaxQuote+1)}, http.StatusRequestEntityTooLarge},
		{"unknown field", http.MethodPost, "/api/tickets/AL-4/comments", `{"body":"x","by":"claude:1"}`, http.StatusBadRequest},
		{"missing ticket", http.MethodPost, "/api/tickets/AL-99/comments", CommentRequest{Body: "x"}, http.StatusNotFound},
		{"no base", http.MethodPatch, "/api/tickets/AL-4/comments/" + posted.Comment.ID, CommentEditRequest{Body: "x"}, http.StatusBadRequest},
		{"missing comment", http.MethodPatch, "/api/tickets/AL-4/comments/deadbeef", CommentEditRequest{Base: posted.ActivityHash, Body: "x"}, http.StatusNotFound},
		{"bad comment id", http.MethodPatch, "/api/tickets/AL-4/comments/..%2Fx", CommentEditRequest{Base: posted.ActivityHash, Body: "x"}, http.StatusNotFound},
		{"empty edit", http.MethodPatch, "/api/tickets/AL-4/comments/" + posted.Comment.ID, CommentEditRequest{Base: posted.ActivityHash, Body: ""}, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			send(t, handler, tc.method, tc.path, tc.body, tc.status, nil)
		})
	}
	if after := len(boardLog(t, log)); after != count {
		t.Errorf("refused comments recorded %d events", after-count)
	}
	if comments := ticketDetail(t, handler, "AL-4").Comments; len(comments) != 1 || comments[0].Body != "First." {
		t.Errorf("comments = %+v", comments)
	}
}

func TestAReadOnlyServerShowsActivityAndRefusesComments(t *testing.T) {
	t.Parallel()

	root := copyBoard(t, "sample")
	writer := store.New(root)
	t.Cleanup(func() { writer.Close() })
	if _, _, err := writer.As("Robert").AddComment("alpha", "AL-4", store.Merge, store.NewComment{Body: "Hello."}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.As("claude:5b0c7e2a").UpdateTicket("alpha", "AL-4", store.Merge, func(data []byte) ([]byte, error) {
		return []byte(strings.Replace(string(data), "status: up-next", "status: in-progress", 1)), nil
	}); err != nil {
		t.Fatal(err)
	}
	files := store.New(root)
	t.Cleanup(func() { files.Close() })
	handler := New(Options{Info: Info{Root: root}, Board: index.New(files, index.Options{}), Files: files})
	detail := ticketDetail(t, handler, "AL-4")
	if len(detail.Comments) != 1 || detail.Comments[0].Body != "Hello." || len(detail.History) != 1 ||
		detail.History[0].By != "claude:5b0c7e2a" || detail.History[0].ByPerson || detail.ActivityHash != "" {
		t.Errorf("detail = %+v %+v %q", detail.Comments, detail.History, detail.ActivityHash)
	}
	send(t, handler, http.MethodPost, "/api/tickets/AL-4/comments", CommentRequest{Body: "x"}, http.StatusNotImplemented, nil)
	if data, _ := os.ReadFile(filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", board.ActivityFile)); strings.Count(string(data), "· comment ·") != 1 {
		t.Errorf("activity = %s", data)
	}
}
