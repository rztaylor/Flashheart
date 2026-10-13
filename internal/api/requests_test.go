package api

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

// requestsAPI is activityAPI with runs folded from its log, so the
// snapshot knows who holds a ticket.
func requestsAPI(t *testing.T) (http.Handler, *events.Log) {
	t.Helper()
	root := copyBoard(t, "sample")
	files := store.New(root)
	files.SetClock(func() time.Time { return boardWriteTime })
	t.Cleanup(func() { files.Close() })
	log := events.New(files)
	return New(Options{
		Info:   Info{Root: root, Theme: "system"},
		Board:  index.New(files, index.Options{Events: log}),
		Files:  files,
		Writer: files.As("Robert"),
		Events: log,
		Now:    func() time.Time { return boardWriteTime },
	}), log
}

// holding records a live session holding a ticket's claim, as its hooks
// and claim would (agent-protocol §6).
func holding(t *testing.T, log *events.Log, run, ticket string) {
	t.Helper()
	now := time.Now().UTC()
	for _, e := range []events.Event{
		{Time: now.Add(-2 * time.Minute), Run: run, Agent: "claude", Kind: events.RunStart, Project: "alpha", Data: events.RunStartData{Kind: events.KindSession}},
		{Time: now.Add(-time.Minute), Run: run, Agent: "claude", Kind: events.Claim, Project: "alpha", Data: events.TicketData{Ticket: ticket, Home: "alpha"}},
	} {
		if err := log.Append(e); err != nil {
			t.Fatal(err)
		}
	}
}

func commentStates(detail TicketDetail) []string {
	var states []string
	for _, comment := range detail.Comments {
		states = append(states, comment.State)
	}
	return states
}

// Request changes on a ticket in review (FH-126): the named and new
// comments become change requests, the ticket goes back to In progress on
// its branch, the session holding it gets them in its inbox, and the card
// counts them.
func TestRequestChangesSendsTheRequestsToTheHolder(t *testing.T) {
	t.Parallel()

	handler, log := requestsAPI(t)
	holding(t, log, "claude:s1", "AL-2")
	var posted CommentResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/comments", CommentRequest{Body: "The header wraps."}, http.StatusCreated, &posted)

	var result RequestChangesResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/request-changes", RequestChangesRequest{
		Comments: []string{posted.Comment.ID}, Comment: &CommentRequest{Body: "Rename the button to Save."},
	}, http.StatusOK, &result)
	if result.SentTo != "claude:s1" || len(result.Comments) != 2 || result.ActivityHash == "" || result.Hash == "" {
		t.Fatalf("result = %+v", result)
	}
	detail := ticketDetail(t, handler, "AL-2")
	if detail.Column != "in-progress" || detail.Branch != "feature/board-columns" {
		t.Errorf("AL-2 is %s on %q", detail.Column, detail.Branch)
	}
	if states := commentStates(detail); !slices.Equal(states, []string{"change-requested", "change-requested"}) {
		t.Errorf("states = %v", states)
	}
	if detail.ChangeRequests != (RequestsJSON{Open: 2}) {
		t.Errorf("detail counts = %+v", detail.ChangeRequests)
	}
	last := detail.History[len(detail.History)-1]
	if !slices.Equal(whats(last), []string{board.ChangeMoved, board.ChangeRequested}) || !slices.Equal(last.Changes[1].Comments, []string{posted.Comment.ID, result.Comments[1].ID}) {
		t.Errorf("history = %+v", last)
	}

	waiting, err := log.Take("alpha", events.Mailbox{Session: "claude:s1"})
	if err != nil || len(waiting) != 2 {
		t.Fatalf("inbox = %+v, %v", waiting, err)
	}
	for index, entry := range waiting {
		if entry.Type != events.EntryChangeRequest || entry.Run != "claude:s1" || entry.Ticket != "AL-2" || entry.By != "Robert" || entry.ID != result.Comments[index].ID {
			t.Errorf("entry %d = %+v", index, entry)
		}
	}
	if waiting[1].Text != "Rename the button to Save." {
		t.Errorf("text = %q", waiting[1].Text)
	}

	var page BoardResponse
	getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &page)
	at := slices.IndexFunc(page.Cards, func(c Card) bool { return c.ID == "AL-2" })
	if at < 0 || page.Cards[at].ChangeRequests != (RequestsJSON{Open: 2}) {
		t.Errorf("card = %+v", page.Cards[at])
	}

	// The move back to review warns, naming both, and still moves.
	var moved WriteResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", MoveRequest{To: "review"}, http.StatusOK, &moved)
	warning := strings.Join(moved.Warnings, "\n")
	for _, want := range []string{"2 change requests are still open", posted.Comment.ID, `"Rename the button to Save."`} {
		if !strings.Contains(warning, want) {
			t.Errorf("warnings %q lack %q", moved.Warnings, want)
		}
	}
	if ticketDetail(t, handler, "AL-2").Column != "review" {
		t.Error("the warning refused the move")
	}
}

// With no live session holding the ticket nothing is queued: the next
// session finds the requests in the ticket (recovery note, board_context).
func TestRequestChangesQueuesNothingWithoutAHolder(t *testing.T) {
	t.Parallel()

	handler, log := requestsAPI(t)
	var result RequestChangesResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/request-changes", RequestChangesRequest{Comment: &CommentRequest{Body: "Again."}}, http.StatusOK, &result)
	if result.SentTo != "" || len(result.Comments) != 1 {
		t.Errorf("result = %+v", result)
	}
	if waiting, _ := log.Take("alpha", events.Mailbox{Session: "claude:s1"}); len(waiting) != 0 {
		t.Errorf("queued %+v", waiting)
	}
}

func TestRequestChangesIsChecked(t *testing.T) {
	t.Parallel()

	handler, _ := requestsAPI(t)
	send(t, handler, http.MethodPost, "/api/tickets/AL-3/request-changes", RequestChangesRequest{Comment: &CommentRequest{Body: "x"}}, http.StatusConflict, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/request-changes", RequestChangesRequest{}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/request-changes", RequestChangesRequest{Comments: []string{"deadbeef"}}, http.StatusNotFound, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/request-changes", RequestChangesRequest{Comments: []string{"../x"}}, http.StatusNotFound, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/request-changes", RequestChangesRequest{Comment: &CommentRequest{Body: strings.Repeat("x", board.MaxSectionText+1)}}, http.StatusRequestEntityTooLarge, nil)
	if detail := ticketDetail(t, handler, "AL-2"); detail.Column != "review" || len(detail.Comments) != 0 {
		t.Errorf("a refused request changed AL-2: %s, %d comments", detail.Column, len(detail.Comments))
	}
}

// At the next review the person resolves or reopens each request; each
// change is history, and a reopened request reaches the holder.
func TestRequestsAreResolvedAndReopened(t *testing.T) {
	t.Parallel()

	handler, log := requestsAPI(t)
	var result RequestChangesResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/request-changes", RequestChangesRequest{Comment: &CommentRequest{Body: "One."}}, http.StatusOK, &result)
	id := result.Comments[0].ID
	path := "/api/tickets/AL-2/comments/" + id

	var resolved CommentResponse
	send(t, handler, http.MethodPatch, path, CommentEditRequest{State: "resolved"}, http.StatusOK, &resolved)
	if resolved.Comment.State != "resolved" || resolved.SentTo != "" {
		t.Errorf("resolved = %+v", resolved)
	}
	holding(t, log, "claude:s2", "AL-2")
	var reopened CommentResponse
	send(t, handler, http.MethodPatch, path, CommentEditRequest{State: "change-requested"}, http.StatusOK, &reopened)
	if reopened.Comment.State != "change-requested" || reopened.SentTo != "claude:s2" {
		t.Errorf("reopened = %+v", reopened)
	}
	if waiting, _ := log.Take("alpha", events.Mailbox{Session: "claude:s2"}); len(waiting) != 1 || waiting[0].ID != id || waiting[0].Type != events.EntryChangeRequest {
		t.Errorf("inbox = %+v", waiting)
	}
	history := ticketDetail(t, handler, "AL-2").History
	var got []string
	for _, entry := range history {
		got = append(got, whats(entry)...)
	}
	if want := []string{board.ChangeMoved, board.ChangeRequested, board.ChangeResolved, board.ChangeReopened}; !slices.Equal(got, want) {
		t.Errorf("history = %v, want %v", got, want)
	}

	send(t, handler, http.MethodPatch, path, CommentEditRequest{State: "addressed"}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPatch, path, CommentEditRequest{State: "open"}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPatch, path, CommentEditRequest{State: "resolved", Body: "x", Base: "y"}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPatch, "/api/tickets/AL-2/comments/deadbeef", CommentEditRequest{State: "resolved"}, http.StatusNotFound, nil)
}
