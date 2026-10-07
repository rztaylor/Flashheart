package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// runsAPI serves the sample board, whose alpha event log has a session that
// claimed AL-3, edited files, ran a subagent and is waiting on permission.
func runsAPI(t *testing.T, now time.Time, extra ...string) http.Handler {
	t.Helper()
	root := copyBoard(t, "sample")
	if len(extra) > 0 {
		log := filepath.Join(root, "alpha", ".flashheart", "events", "2026-10-04.jsonl")
		file, err := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range extra {
			if _, err := file.WriteString(line + "\n"); err != nil {
				t.Fatal(err)
			}
		}
		file.Close()
	}
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

// A session held up by its subagent's prompt says which subagent and tool
// on the card, as the Agents view does.
func TestLiveBadgeNamesTheSubagentItWaitsOn(t *testing.T) {
	t.Parallel()

	child := sampleSession + "/e1"
	handler := runsAPI(t, time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC),
		`{"v":1,"ts":"2026-10-04T13:27:00.000Z","run":"`+sampleSession+`","agent":"claude","kind":"tool.used","project":"alpha","data":{"tool":"Bash","ok":true}}`,
		`{"v":1,"ts":"2026-10-04T13:28:00.000Z","run":"`+child+`","agent":"claude","kind":"run.start","project":"alpha","data":{"kind":"subagent","parent":"`+sampleSession+`","agent_type":"Explore"}}`,
		`{"v":1,"ts":"2026-10-04T13:29:00.000Z","run":"`+child+`","agent":"claude","kind":"permission.requested","project":"alpha","data":{"tool":"Bash"}}`,
	)
	var board BoardResponse
	getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
	live := cardByID(t, board.Cards, "AL-3").Live
	if live == nil || live.State != "needs-you" || live.Permission != "Bash" || live.WaitingOn != "Explore" {
		t.Fatalf("live = %+v", live)
	}
}

// golden returns the events a recorded Claude Code hook payload yields
// (testdata/hooks/claude), as event-log lines for run at ts.
func golden(t *testing.T, payload, run, ts string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "hooks", "claude", payload+".events.json"))
	if err != nil {
		t.Fatal(err)
	}
	var list []struct {
		Kind string          `json:"kind"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(bytes.ReplaceAll(data, []byte("claude:340b083f-5d70-41b2-8cff-ec700908097a"), []byte(sampleSession)), &list); err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0, len(list))
	for _, event := range list {
		var compact bytes.Buffer
		if err := json.Compact(&compact, event.Data); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, `{"v":1,"ts":"`+ts+`","run":"`+run+`","agent":"claude","kind":"`+event.Kind+`","project":"alpha","data":`+compact.String()+`}`)
	}
	return lines
}

// An orchestrated run with three subagents shows as a tree on its ticket's
// Runs tab (agent-protocol §10): every subagent under its session with its
// own state, plan and checkpoints, including one that claimed another
// ticket.
func TestTicketRunsCarryTheSubagentTree(t *testing.T) {
	t.Parallel()

	done, working, waiting := sampleSession+"/b1", sampleSession+"/b2", sampleSession+"/b3"
	var lines []string
	for _, child := range []string{done, working, waiting} {
		lines = append(lines, golden(t, "SubagentStart/explore", child, "2026-10-04T13:27:00.000Z")...)
	}
	lines = append(lines, golden(t, "SubagentStop/explore", done, "2026-10-04T13:28:00.000Z")...)
	lines = append(lines,
		`{"v":1,"ts":"2026-10-04T13:28:10.000Z","run":"`+working+`","agent":"claude","kind":"claim","project":"alpha","data":{"ticket":"AL-2","force":false}}`,
		`{"v":1,"ts":"2026-10-04T13:28:20.000Z","run":"`+working+`","agent":"claude","kind":"plan.updated","project":"alpha","data":{"items":[{"text":"Read the board","status":"completed"},{"text":"Write the tests","status":"in_progress"}]}}`,
		`{"v":1,"ts":"2026-10-04T13:28:30.000Z","run":"`+working+`","agent":"claude","kind":"checkpoint","project":"alpha","data":{"ticket":"AL-2","done":1,"next":1,"files":0,"questions":0}}`,
	)
	lines = append(lines, golden(t, "PostToolUse/subagent-bash", working, "2026-10-04T13:29:00.000Z")...)
	lines = append(lines, golden(t, "PermissionRequest/bash", waiting, "2026-10-04T13:29:30.000Z")...)
	handler := runsAPI(t, time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC), lines...)

	var panel TicketResponse
	getJSON(t, handler, "/api/tickets/AL-3", http.StatusOK, &panel)
	byID := map[string]RunJSON{}
	for _, run := range panel.Ticket.Runs {
		byID[run.ID] = run
	}
	session, ok := byID[sampleSession]
	if !ok || len(session.Children) != 4 {
		t.Fatalf("session = %+v", session)
	}
	want := map[string]string{done: "ended", working: "working", waiting: "needs-you"}
	for id, state := range want {
		child, ok := byID[id]
		if !ok || child.Parent != sampleSession || child.State != state || child.AgentType != "Explore" || child.Timeline == nil {
			t.Fatalf("%s = %+v, want %s under the session", id, child, state)
		}
	}
	child := byID[working]
	if child.Ticket != "AL-2" || child.TicketTitle == "" || child.Progress.Current != "Write the tests" {
		t.Fatalf("working child = %+v", child)
	}
	checkpointed := false
	for _, entry := range child.Timeline {
		checkpointed = checkpointed || entry.Kind == "checkpoint" && entry.Ticket == "AL-2"
	}
	if !checkpointed {
		t.Fatalf("working child's timeline lacks its checkpoint: %+v", child.Timeline)
	}

	// On its own ticket the subagent is listed, still naming its session.
	getJSON(t, handler, "/api/tickets/AL-2", http.StatusOK, &panel)
	if len(panel.Ticket.Runs) != 1 || panel.Ticket.Runs[0].ID != working || panel.Ticket.Runs[0].Parent != sampleSession {
		t.Fatalf("AL-2 runs = %+v", panel.Ticket.Runs)
	}
}

// A session's subagents on the Runs tab are bounded, most recent first.
func TestTicketRunsBoundSubagentsPerSession(t *testing.T) {
	t.Parallel()

	var lines []string
	for n := range maxSubagents + 5 {
		ts := time.Date(2026, 10, 4, 13, 27, n, 0, time.UTC).Format("2006-01-02T15:04:05.000Z")
		lines = append(lines, `{"v":1,"ts":"`+ts+`","run":"`+sampleSession+`/c`+strconv.Itoa(n)+`","agent":"claude","kind":"run.start","project":"alpha","data":{"kind":"subagent","parent":"`+sampleSession+`","agent_type":"Explore"}}`)
	}
	handler := runsAPI(t, time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC), lines...)
	var panel TicketResponse
	getJSON(t, handler, "/api/tickets/AL-3", http.StatusOK, &panel)
	children := 0
	for _, run := range panel.Ticket.Runs {
		if run.Parent == sampleSession {
			children++
			if run.ID == sampleSession+"/c0" || run.ID == sampleSession+"/a1" {
				t.Fatalf("an older subagent was kept: %s", run.ID)
			}
		}
	}
	if children != maxSubagents {
		t.Fatalf("subagents = %d, want %d", children, maxSubagents)
	}
}
