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

var boardWriteTime = time.Date(2026, 10, 9, 18, 30, 0, 0, time.UTC)

// activityAPI is writableAPI with an event log and a fixed clock.
func activityAPI(t *testing.T) (http.Handler, *events.Log) {
	t.Helper()
	root := copyBoard(t, "sample")
	files := store.New(root)
	t.Cleanup(func() { files.Close() })
	log := events.New(files)
	return New(Options{
		Info:   Info{Root: root, Theme: "system"},
		Board:  index.New(files, index.Options{}),
		Files:  files,
		Writer: files,
		Events: log,
		Now:    func() time.Time { return boardWriteTime },
	}), log
}

// boardLog reads every event of the sample's projects.
func boardLog(t *testing.T, log *events.Log) []events.Event {
	t.Helper()
	var list []events.Event
	for _, project := range []string{"alpha", "beta"} {
		if err := log.Read(project, time.Time{}, func(e events.Event) { list = append(list, e) }); err != nil {
			t.Fatal(err)
		}
	}
	return list
}

// recorded is what one board write should leave in the log.
type recorded struct {
	kind, project, ticket string
	from, to              string
	fields                []string
}

// expectRecorded checks that the log grew by exactly the one event want,
// attributed to the human, and returns the new length.
func expectRecorded(t *testing.T, log *events.Log, before int, want recorded) int {
	t.Helper()
	list := boardLog(t, log)
	if len(list) != before+1 {
		t.Fatalf("%s %s: %d new events, want 1", want.kind, want.ticket, len(list)-before)
	}
	// Each project's log is read in order, so the new event is the last of
	// its project's.
	var got events.Event
	for _, e := range list {
		if e.Project == want.project {
			got = e
		}
	}
	var data events.TicketData
	if err := got.Decode(&data); err != nil {
		t.Fatal(err)
	}
	if got.Run != events.Human || got.Agent != events.Human || data.By != events.Human || !got.Time.Equal(boardWriteTime) ||
		got.Kind != want.kind || got.Project != want.project || data.Ticket != want.ticket ||
		data.From != want.from || data.To != want.to || !slices.Equal(data.Fields, want.fields) || data.Reason != "" {
		t.Errorf("event = %+v data %+v, want %+v", got, data, want)
	}
	if activity, ok := events.HumanActivityOf(got); !ok || activity.Ticket != want.ticket {
		t.Errorf("HumanActivityOf = %+v, %v", activity, ok)
	}
	return len(list)
}

// Every board write that changes a ticket records one event naming it,
// attributed to the human, with no ticket text (FH-49, agent-protocol §3).
func TestBoardWritesRecordTheHuman(t *testing.T) {
	t.Parallel()

	handler, log := activityAPI(t)
	count := len(boardLog(t, log))

	var saved WriteResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", MoveRequest{Base: ticketDetail(t, handler, "AL-2").Hash, To: "done"}, http.StatusOK, &saved)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketMoved, project: "alpha", ticket: "AL-2", from: "review", to: "done"})

	// A reorder within a column changes the ticket's rank: it is a move too.
	top := ""
	send(t, handler, http.MethodPost, "/api/tickets/AL-6/move", MoveRequest{To: "backlog", After: &top}, http.StatusOK, nil)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketMoved, project: "alpha", ticket: "AL-6", from: "backlog", to: "backlog"})

	// A blocked start keeps its reason in the ticket, not the event.
	send(t, handler, http.MethodPost, "/api/tickets/AL-5/move", MoveRequest{To: "in-progress", Reason: "Hotfix"}, http.StatusOK, nil)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketMoved, project: "alpha", ticket: "AL-5", from: "backlog", to: "in-progress"})

	send(t, handler, http.MethodPatch, "/api/tickets/AL-4", map[string]any{"base": ticketDetail(t, handler, "AL-4").Hash,
		"fields": map[string]any{"workstream": "board-ui", "priority": "high", "title": "A secret title"}}, http.StatusOK, nil)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketUpdated, project: "alpha", ticket: "AL-4", fields: []string{"priority", "title", "workstream"}})

	send(t, handler, http.MethodPost, "/api/tickets/AL-3/criteria", CriterionRequest{Base: ticketDetail(t, handler, "AL-3").Hash, Index: 1, Checked: true}, http.StatusOK, &saved)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketUpdated, project: "alpha", ticket: "AL-3", fields: []string{"criteria"}})

	// A raw edit names no fields: what changed is not worked out.
	raw := strings.Replace(ticketDetail(t, handler, "AL-3").Raw, "Fixture ticket.", "A secret body.", 1)
	send(t, handler, http.MethodPut, "/api/tickets/AL-3/raw", RawRequest{Base: saved.Hash, Content: raw}, http.StatusOK, nil)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketUpdated, project: "alpha", ticket: "AL-3"})

	review := ticketDetail(t, handler, "AL-2").Review
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/review/steps", ReviewStepRequest{Base: review.Hash, Index: 0, Checked: true}, http.StatusOK, nil)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketUpdated, project: "alpha", ticket: "AL-2", fields: []string{"review"}})

	var created CreateResponse
	send(t, handler, http.MethodPost, "/api/projects/beta/tickets", CreateRequest{Title: "A secret plan", Description: "Secret."}, http.StatusCreated, &created)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketCreated, project: "beta", ticket: created.ID})

	send(t, handler, http.MethodPost, "/api/tickets/"+created.ID+"/archive", "", http.StatusOK, nil)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketUpdated, project: "beta", ticket: created.ID, fields: []string{"archived"}})
	send(t, handler, http.MethodPost, "/api/tickets/"+created.ID+"/unarchive", "", http.StatusOK, nil)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketUpdated, project: "beta", ticket: created.ID, fields: []string{"archived"}})

	send(t, handler, http.MethodPost, "/api/tickets/AL-3/archive", "", http.StatusOK, nil)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketUpdated, project: "alpha", ticket: "AL-3", fields: []string{"archived"}})
	var plan DeletePlanJSON
	getJSON(t, handler, "/api/projects/alpha/archive/AL-3", http.StatusOK, &plan)
	send(t, handler, http.MethodPost, "/api/projects/alpha/archive/AL-3/delete", DeleteRequest{Token: plan.Token, Confirm: "AL-3"}, http.StatusOK, nil)
	count = expectRecorded(t, log, count, recorded{kind: events.TicketUpdated, project: "alpha", ticket: "AL-3", fields: []string{"deleted"}})

	// No ticket text reaches the log.
	for _, e := range boardLog(t, log) {
		line, _ := e.MarshalLine()
		if strings.Contains(string(line), "secret") || strings.Contains(string(line), "Secret") || strings.Contains(string(line), "Hotfix") {
			t.Errorf("ticket text in the log: %s", line)
		}
	}

	// Refused writes, and writes that are not to a ticket, record nothing.
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", MoveRequest{Base: "stale", To: "backlog"}, http.StatusConflict, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-4/criteria", CriterionRequest{Base: ticketDetail(t, handler, "AL-4").Hash, Index: 9, Checked: true}, http.StatusNotFound, nil)
	send(t, handler, http.MethodPut, "/api/projects/alpha/workstreams/board-ui/order", OrderRequest{Tickets: []string{"AL-4", "AL-2"}}, http.StatusOK, nil)
	if after := len(boardLog(t, log)); after != count {
		t.Errorf("refused or non-ticket writes appended %d events", after-count)
	}
}
