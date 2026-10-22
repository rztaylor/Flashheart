package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
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

// A run the MCP server started for a session without hooks is MCP-only:
// its question's answer goes with its next Flashheart tool call. Once its
// session's hooks report it (a Codex run named by its session), it is not
// (agent-protocol §7.1, FH-74).
func TestRunsSayWhichReportOnlyThroughTheMCPServer(t *testing.T) {
	t.Parallel()

	const run = "codex:019a7c00-0000-7000-8000-000000000001"
	line := func(ts, kind, data string) string {
		return `{"v":1,"ts":"2026-10-04T` + ts + `.000Z","run":"` + run + `","agent":"codex","kind":"` + kind + `","project":"alpha","data":` + data + `}`
	}
	hookless := []string{
		line("13:10:00", "run.start", `{"kind":"session","cwd":"/Users/example/src/alpha","branch":"main","worktree":"/Users/example/src/alpha","source":"mcp"}`),
		line("13:11:00", "question.asked", `{"id":"q-9","kind":"question","text":"Which schema?"}`),
		line("13:12:00", "question.answered", `{"id":"q-9","answer":"v2","by":"Robert"}`),
	}
	for _, tc := range []struct {
		name    string
		lines   []string
		mcpOnly bool
	}{
		{"without hooks", hookless, true},
		{"once its hooks report it", append(slices.Clone(hookless), line("13:13:00", "turn.end", `{}`)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handler := runsAPI(t, time.Date(2026, 10, 4, 13, 20, 0, 0, time.UTC), tc.lines...)
			var one RunResponse
			getJSON(t, handler, "/api/run?id="+url.QueryEscape(run), http.StatusOK, &one)
			got := one.Run
			if got.MCPOnly != tc.mcpOnly || got.Reason.MCPOnly != tc.mcpOnly || !got.Reason.Answered ||
				len(got.Questions) != 1 || got.Questions[0].MCPOnly != tc.mcpOnly {
				t.Fatalf("run = mcpOnly %v, reason %+v, questions %+v", got.MCPOnly, got.Reason, got.Questions)
			}
		})
	}
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
	if card.Live == nil || card.Live.State != "needs-you" || card.Live.Agent != "claude" || !card.NeedsYou || card.AgentWorking {
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
// on the card, as the Runs tab does.
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
	want := reasonJSON(runs.Reason{Kind: runs.ReasonPermission, Permission: "Bash", WaitingOn: child, AgentType: "Explore"})
	if live == nil || live.State != "needs-you" || live.Reason != want {
		t.Fatalf("live = %+v", live)
	}
	// The Runs tab gets the same reason for the session, and the subagent
	// its own prompt; a subagent's short id is its session's (§2).
	ticket := ticketDetail(t, handler, "AL-3")
	for _, run := range ticket.Runs {
		switch run.ID {
		case sampleSession:
			if run.Reason != want || run.Short != "claude:3f2a9c1e" {
				t.Fatalf("session = %+v", run)
			}
		case child:
			if run.Reason != reasonJSON(runs.Reason{Kind: runs.ReasonPermission, Permission: "Bash"}) || run.Short != "claude:3f2a9c1e" {
				t.Fatalf("subagent = %+v", run)
			}
		}
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

// The sample log, like any from before D51, holds a plan.updated (FH-133):
// it still folds, with the session's state, link and timeline as before,
// and no run, card or timeline carries a plan.
func TestOldPlanEventsFoldWithoutAPlan(t *testing.T) {
	t.Parallel()

	handler := runsAPI(t, time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC))
	var raw struct {
		Runs []map[string]json.RawMessage `json:"runs"`
	}
	getJSON(t, handler, "/api/runs", http.StatusOK, &raw)
	if len(raw.Runs) != 2 {
		t.Fatalf("runs = %d, want the session and its subagent", len(raw.Runs))
	}
	for _, run := range raw.Runs {
		for _, field := range []string{"plan", "progress"} {
			if _, found := run[field]; found {
				t.Errorf("run %s carries %q: %s", run["id"], field, run[field])
			}
		}
	}
	var panel struct {
		Ticket struct {
			Runs []struct {
				State    string `json:"state"`
				Timeline []struct {
					Kind string `json:"kind"`
				} `json:"timeline"`
			} `json:"runs"`
		} `json:"ticket"`
	}
	getJSON(t, handler, "/api/tickets/AL-3", http.StatusOK, &panel)
	if len(panel.Ticket.Runs) == 0 || panel.Ticket.Runs[0].State != "needs-you" {
		t.Fatalf("AL-3 runs = %+v", panel.Ticket.Runs)
	}
	for _, entry := range panel.Ticket.Runs[0].Timeline {
		if entry.Kind == "plan.updated" {
			t.Fatalf("timeline shows a plan: %+v", panel.Ticket.Runs[0].Timeline)
		}
	}
	var board struct {
		Cards []struct {
			ID   string                     `json:"id"`
			Live map[string]json.RawMessage `json:"live"`
		} `json:"cards"`
	}
	getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
	live := false
	for _, card := range board.Cards {
		if card.ID != "AL-3" {
			continue
		}
		live = card.Live != nil
		for _, field := range []string{"done", "total", "step"} {
			if _, found := card.Live[field]; found {
				t.Errorf("AL-3's live run carries %q", field)
			}
		}
	}
	if !live {
		t.Fatal("AL-3 has no live run")
	}
}

// An orchestrated run with three subagents shows as a tree on its ticket's
// Runs tab (agent-protocol §10): every subagent under its session with its
// own state and checkpoints, including one that claimed another
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
	if child.Ticket != "AL-2" || child.TicketTitle == "" {
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

// Each card carries a summary of the sessions working for its ticket
// (RUN-9, FH-50): the run that speaks for it, last activity, handoff flags
// and subagent counts; an in-progress ticket nobody works on any more is
// marked as having no live session.
func TestCardsSummariseTheirSessions(t *testing.T) {
	t.Parallel()

	running := `{"v":1,"ts":"2026-10-04T13:28:00.000Z","run":"` + sampleSession + `/e1","agent":"claude","kind":"run.start","project":"alpha","data":{"kind":"subagent","parent":"` + sampleSession + `","agent_type":"Explore"}}`
	live := runsAPI(t, time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC), running)
	var raw struct {
		Cards []map[string]json.RawMessage `json:"cards"`
	}
	getJSON(t, live, "/api/projects/alpha/board", http.StatusOK, &raw)
	var summary map[string]any
	for _, card := range raw.Cards {
		var id string
		_ = json.Unmarshal(card["id"], &id)
		switch id {
		case "AL-3":
			if err := json.Unmarshal(card["sessions"], &summary); err != nil {
				t.Fatalf("AL-3 sessions = %s: %v", card["sessions"], err)
			}
		case "AL-4":
			if _, ok := card["sessions"]; ok {
				t.Fatalf("AL-4, with no runs and not in progress, has sessions %s", card["sessions"])
			}
		}
	}
	want := map[string]any{
		"run": sampleSession, "short": "claude:3f2a9c1e", "agent": "claude", "state": "needs-you",
		"lastActivity": "2026-10-04T13:28:00Z", "noLiveSession": false, "dirty": true, "noHandoff": false,
		"subagents": map[string]any{"done": 1, "running": 1, "needsYou": 0},
	}
	gotJSON, _ := json.Marshal(summary)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("AL-3 sessions = %s\nwant %s", gotJSON, wantJSON)
	}

	// A day later the session has gone stale: AL-3, still in progress, has
	// no live session and no handoff for its last edits.
	stale := runsAPI(t, time.Date(2026, 10, 5, 13, 30, 0, 0, time.UTC), running)
	var board AllBoardResponse
	getJSON(t, stale, "/api/all/board", http.StatusOK, &board)
	got := cardByID(t, board.Cards, "AL-3").Sessions
	if got == nil || got.State != "ended" || !got.NoLiveSession || !got.NoHandoff || got.LastActivity != "2026-10-04T13:28:00Z" ||
		got.Subagents != (SubagentCountsJSON{Done: 2}) {
		t.Fatalf("stale AL-3 sessions = %+v", got)
	}
}

// A session that holds one ticket and asks about another needs you on the
// card of the ticket it asks about, which words the question's reason; its
// own card shows it as live, waiting for that answer (FH-185). A
// question about its own ticket, or about no ticket, keeps it on its own
// card as before.
func TestAQuestionAboutAnotherTicketNeedsYouOnThatCard(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC)
	asked := func(ticket string) string {
		return `{"v":1,"ts":"2026-10-04T13:28:00.000Z","run":"` + sampleSession + `","agent":"claude","kind":"question.asked","project":"alpha","data":{"id":"q1","ticket":"` + ticket + `","kind":"review","text":"Is the card panel right?"}}`
	}
	// The sample session's permission prompt is answered by a tool call.
	resolved := `{"v":1,"ts":"2026-10-04T13:27:00.000Z","run":"` + sampleSession + `","agent":"claude","kind":"tool.used","project":"alpha","data":{"tool":"Bash","ok":true}}`
	review := reasonJSON(runs.Reason{Kind: runs.ReasonQuestion, Question: "review"})

	t.Run("about another ticket", func(t *testing.T) {
		t.Parallel()
		handler := runsAPI(t, now, resolved, asked("AL-4"))
		var board BoardResponse
		getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
		if board.Project.Runs.NeedsYou != 1 || board.Project.Runs.NeedsYouUnticketed != 0 {
			t.Fatalf("runs = %+v", board.Project.Runs)
		}
		about := cardByID(t, board.Cards, "AL-4")
		if !about.NeedsYou || about.OpenQuestions != 1 || about.Reason != review || about.Live != nil {
			t.Fatalf("AL-4 = needsYou %v questions %d reason %+v live %+v", about.NeedsYou, about.OpenQuestions, about.Reason, about.Live)
		}
		held := cardByID(t, board.Cards, "AL-3")
		if held.NeedsYou || held.OpenQuestions != 0 || held.Reason.Kind != "" || held.Live == nil || held.Live.State != "waiting" || held.Live.Reason.Kind != "" || held.AgentWorking {
			t.Fatalf("AL-3 = needsYou %v questions %d reason %+v live %+v", held.NeedsYou, held.OpenQuestions, held.Reason, held.Live)
		}
		// The run itself is in Needs you with the question as its reason,
		// as the Runs tab and the Overview show it.
		var response RunsResponse
		getJSON(t, handler, "/api/runs", http.StatusOK, &response)
		for _, run := range response.Runs {
			if run.ID == sampleSession && (run.State != "needs-you" || run.Reason != review || run.Ticket != "AL-3") {
				t.Fatalf("run = %+v", run)
			}
		}
	})

	t.Run("about its own ticket", func(t *testing.T) {
		t.Parallel()
		handler := runsAPI(t, now, resolved, asked("AL-3"))
		var board BoardResponse
		getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
		held := cardByID(t, board.Cards, "AL-3")
		if !held.NeedsYou || held.OpenQuestions != 1 || held.Reason != review || held.Live == nil || held.Live.State != "needs-you" || held.Live.Reason != review {
			t.Fatalf("AL-3 = needsYou %v questions %d reason %+v live %+v", held.NeedsYou, held.OpenQuestions, held.Reason, held.Live)
		}
		if other := cardByID(t, board.Cards, "AL-4"); other.NeedsYou || other.Reason.Kind != "" {
			t.Fatalf("AL-4 = needsYou %v reason %+v", other.NeedsYou, other.Reason)
		}
	})

	t.Run("about no ticket", func(t *testing.T) {
		t.Parallel()
		handler := runsAPI(t, now, resolved, asked(""))
		var board BoardResponse
		getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
		held := cardByID(t, board.Cards, "AL-3")
		if !held.NeedsYou || held.OpenQuestions != 0 || held.Reason != review || held.Live == nil || held.Live.State != "needs-you" {
			t.Fatalf("AL-3 = needsYou %v questions %d reason %+v live %+v", held.NeedsYou, held.OpenQuestions, held.Reason, held.Live)
		}
		if board.Project.Runs.NeedsYouUnticketed != 0 {
			t.Fatalf("runs = %+v", board.Project.Runs)
		}
	})

	t.Run("about a ticket not on the board", func(t *testing.T) {
		t.Parallel()
		handler := runsAPI(t, now, resolved, asked("AL-99"))
		var board BoardResponse
		getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
		// No card can show it: the Needs you notice names it (FH-157).
		if held := cardByID(t, board.Cards, "AL-3"); held.NeedsYou || held.Live == nil || held.Live.State != "waiting" {
			t.Fatalf("AL-3 = needsYou %v live %+v", held.NeedsYou, held.Live)
		}
		if board.Project.Runs.NeedsYou != 1 || board.Project.Runs.NeedsYouUnticketed != 1 {
			t.Fatalf("runs = %+v", board.Project.Runs)
		}
	})
}

// The band's pill and each project's rail badge count what the Needs you
// filter shows (FH-186): every ticket on the board that needs you, once
// however many questions it has, plus each run in Needs you that no card
// shows. One session that asks about three tickets needs you three times,
// as the Overview's Needs your decision lists three questions; seen on
// Froodbase, where one session's four questions made the pill say 1.
func TestTheNeedsYouCountIsWhatTheFilterShows(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC)
	asked := func(id, ticket string) string {
		return `{"v":1,"ts":"2026-10-04T13:28:00.000Z","run":"` + sampleSession + `","agent":"claude","kind":"question.asked","project":"alpha","data":{"id":"` + id + `","ticket":"` + ticket + `","kind":"question","text":"Which?"}}`
	}
	// The sample session's permission prompt is answered by a tool call.
	resolved := `{"v":1,"ts":"2026-10-04T13:27:00.000Z","run":"` + sampleSession + `","agent":"claude","kind":"tool.used","project":"alpha","data":{"tool":"Bash","ok":true}}`
	questions := []string{asked("q1", "AL-4"), asked("q2", "AL-5"), asked("q3", "AL-6"), asked("q4", "AL-4")}
	// claude:s2, on main, links to no ticket and waits on a permission
	// prompt: no card shows it, so the filter's notice names it.
	unticketed := []string{
		`{"v":1,"ts":"2026-10-04T13:20:00.000Z","run":"claude:s2","agent":"claude","kind":"run.start","project":"alpha","data":{"kind":"session","branch":"main","worktree":"/Users/example/src/alpha"}}`,
		`{"v":1,"ts":"2026-10-04T13:21:00.000Z","run":"claude:s2","agent":"claude","kind":"permission.requested","project":"alpha","data":{"tool":"Bash"}}`,
	}
	needing := func(cards []Card) []string {
		var ids []string
		for _, card := range cards {
			if card.NeedsYou {
				ids = append(ids, card.ID)
			}
		}
		slices.Sort(ids)
		return ids
	}
	check := func(t *testing.T, handler http.Handler, wantCards []string, want int) {
		t.Helper()
		var board BoardResponse
		getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
		if got := needing(board.Cards); !slices.Equal(got, wantCards) {
			t.Fatalf("cards needing you = %v, want %v", got, wantCards)
		}
		if board.Project.Runs.NeedsYouShown != want {
			t.Errorf("board runs = %+v, want %d shown", board.Project.Runs, want)
		}
		var projects ProjectsResponse
		getJSON(t, handler, "/api/projects", http.StatusOK, &projects)
		if projects.Runs.NeedsYouShown != want {
			t.Errorf("pill: runs = %+v, want %d shown", projects.Runs, want)
		}
		for _, project := range projects.Projects {
			if project.Name == "alpha" && project.Runs.NeedsYouShown != want {
				t.Errorf("rail badge: alpha runs = %+v, want %d shown", project.Runs, want)
			}
		}
	}

	t.Run("questions about three tickets", func(t *testing.T) {
		t.Parallel()
		// AL-4's two questions are one card; AL-3, the session's own, only
		// waits for the answers.
		check(t, runsAPI(t, now, append([]string{resolved}, questions...)...), []string{"AL-4", "AL-5", "AL-6"}, 3)
	})

	t.Run("and a permission prompt on its own card", func(t *testing.T) {
		t.Parallel()
		// The prompt needs you on AL-3, the session's card: once, not once
		// for the run and again for the card.
		check(t, runsAPI(t, now, questions...), []string{"AL-3", "AL-4", "AL-5", "AL-6"}, 4)
	})

	t.Run("and a run no card shows", func(t *testing.T) {
		t.Parallel()
		check(t, runsAPI(t, now, append(append([]string{resolved}, questions...), unticketed...)...), []string{"AL-4", "AL-5", "AL-6"}, 4)
	})
}
