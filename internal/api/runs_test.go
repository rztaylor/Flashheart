package api

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// runsAPI serves the sample board, whose alpha event log has a session that
// claimed AL-3, edited files, ran a subagent and is waiting on permission.
func runsAPI(t *testing.T, now time.Time) http.Handler {
	t.Helper()
	root := copyBoard(t, "sample")
	files := store.New(root)
	t.Cleanup(func() { files.Close() })
	clock := func() time.Time { return now }
	return New(Options{
		Info:      Info{Root: root, Theme: "system"},
		Board:     index.New(files, index.Options{Now: clock, Events: events.New(files), Runs: runs.DefaultSettings()}),
		Files:     files,
		DoneLimit: 20,
	})
}

const sampleSession = "claude:3f2a9c1e-0000-0000-0000-000000000000"

func TestRunsEndpointListsRunsWithState(t *testing.T) {
	t.Parallel()

	handler := runsAPI(t, time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC))
	var response RunsResponse
	getJSON(t, handler, "/api/runs", http.StatusOK, &response)
	if len(response.Runs) != 2 || response.Counts.NeedsYou != 1 || response.Counts.Live != 1 {
		t.Fatalf("runs = %+v counts %+v", response.Runs, response.Counts)
	}
	session := response.Runs[0]
	if session.ID != sampleSession || session.Short != "claude:3f2a9c1e" || session.State != "needs-you" || session.Permission != "Bash" {
		t.Fatalf("session = %+v", session)
	}
	if session.Ticket != "AL-3" || session.TicketTitle != "Card panel" || session.LinkedBy != "claim" || !session.Dirty {
		t.Fatalf("session link = %+v", session)
	}
	if session.Progress.Done != 1 || session.Progress.Total != 3 || session.Progress.Current != "Review tab" {
		t.Fatalf("progress = %+v", session.Progress)
	}
	if len(session.Children) != 1 || session.Timeline != nil {
		t.Fatalf("children = %v, timeline sent in the list = %v", session.Children, session.Timeline)
	}
	child := response.Runs[1]
	if child.Kind != "subagent" || child.Parent != sampleSession || child.State != "ended" || child.AgentType != "Explore" || child.Ticket != "AL-3" {
		t.Fatalf("child = %+v", child)
	}

	// Project filter.
	getJSON(t, handler, "/api/runs?project=beta", http.StatusOK, &response)
	if len(response.Runs) != 0 {
		t.Fatalf("beta runs = %+v", response.Runs)
	}
	getJSON(t, handler, "/api/runs?project=missing", http.StatusNotFound, nil)

	// One run with its timeline.
	var one RunResponse
	getJSON(t, handler, "/api/run?id="+url.QueryEscape(sampleSession), http.StatusOK, &one)
	if len(one.Run.Timeline) == 0 || one.Run.Timeline[len(one.Run.Timeline)-1].Kind != "permission.requested" {
		t.Fatalf("timeline = %+v", one.Run.Timeline)
	}
	getJSON(t, handler, "/api/run?id=claude:nope", http.StatusNotFound, nil)
}

func TestEndedRunsOlderThanADayAreHiddenByDefault(t *testing.T) {
	t.Parallel()

	// A day and a half later both runs are Ended (stale) and over 24 hours old.
	handler := runsAPI(t, time.Date(2026, 10, 5, 13, 30, 0, 0, time.UTC))
	var response RunsResponse
	getJSON(t, handler, "/api/runs", http.StatusOK, &response)
	if len(response.Runs) != 0 || response.HiddenEnded != 2 {
		t.Fatalf("runs = %d, hidden %d", len(response.Runs), response.HiddenEnded)
	}
	getJSON(t, handler, "/api/runs?ended=all", http.StatusOK, &response)
	if len(response.Runs) != 2 || response.Runs[0].NoHandoff != true {
		t.Fatalf("all runs = %+v", response.Runs)
	}
}

func TestCardsAndProjectsCarryLiveRuns(t *testing.T) {
	t.Parallel()

	handler := runsAPI(t, time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC))
	var board BoardResponse
	getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
	card := cardByID(t, board.Cards, "AL-3")
	if card.Live == nil || card.Live.State != "needs-you" || card.Live.Agent != "claude" || card.Live.Step != "Review tab" || card.Live.Done != 1 || card.Live.Total != 3 || !card.NeedsYou || card.AgentWorking {
		t.Fatalf("AL-3 live = %+v needsYou %v working %v", card.Live, card.NeedsYou, card.AgentWorking)
	}
	if other := cardByID(t, board.Cards, "AL-4"); other.Live != nil || other.NeedsYou {
		t.Fatalf("AL-4 live = %+v", other.Live)
	}
	if board.Project.Runs.NeedsYou != 1 || board.Project.Runs.Live != 1 {
		t.Fatalf("project runs = %+v", board.Project.Runs)
	}

	var projects ProjectsResponse
	getJSON(t, handler, "/api/projects", http.StatusOK, &projects)
	if projects.Runs.NeedsYou != 1 {
		t.Fatalf("root runs = %+v", projects.Runs)
	}

	var ticket TicketResponse
	getJSON(t, handler, "/api/tickets/AL-3", http.StatusOK, &ticket)
	if len(ticket.Ticket.Runs) != 2 || len(ticket.Ticket.Runs[0].Timeline) == 0 {
		t.Fatalf("ticket runs = %+v", ticket.Ticket.Runs)
	}
}
