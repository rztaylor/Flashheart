package mcpserver

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// activity lists a run's activity records.
func (e *env) activity(run string) []events.ActivityData {
	e.t.Helper()
	var list []events.ActivityData
	for _, ev := range e.log() {
		var data events.ActivityData
		if ev.Run == run && ev.Kind == events.Activity && ev.Decode(&data) == nil {
			list = append(list, data)
		}
	}
	return list
}

// Every tool call of a started run is activity, recorded as hooks record
// tool results (at most one activity record per activity_seconds), so its
// claim's lease is renewed by reads as well as writes (agent-protocol §6,
// RUN-6).
func TestToolCallsRenewAStartedRunsLease(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	codex := e.dial(codexClient)
	e.okOn(codex, "claim", map[string]any{"ticket": id})
	run := e.started()[0].Run
	// Calls within a minute of the last record wait for the next one.
	e.okOn(codex, "board_context", nil)
	e.okOn(codex, "list_tickets", nil)
	if got := e.activity(run); len(got) != 1 || got[0].Tools != 1 {
		t.Fatalf("activity after three calls in a minute = %+v", got)
	}
	e.now = e.now.Add(25 * time.Minute)
	e.okOn(codex, "get_ticket", map[string]any{"ticket": id})
	if got := e.activity(run); len(got) != 2 || got[1].Tools != 3 || got[1].Failed != 0 || len(got[1].Paths) != 0 {
		t.Fatalf("activity = %+v", got)
	}
	e.now = e.now.Add(25 * time.Minute)
	rival := e.dial(codexClient)
	out, failed := e.callOn(rival, "claim", map[string]any{"ticket": id})
	if !failed || !strings.Contains(out, "error claimed:") {
		t.Fatalf("the lease was not renewed by get_ticket:\n%s", out)
	}
	if state := e.fold().State(run, e.now, runs.DefaultSettings()); state != runs.Waiting {
		t.Fatalf("state = %s", state)
	}
	// A call naming no run it can be is not the started run's activity.
	e.now = e.now.Add(2 * time.Minute)
	before := len(e.log())
	e.callOn(codex, "get_ticket", map[string]any{"ticket": id, "run": "codex:nope"})
	if len(e.log()) != before {
		t.Fatalf("a call with a wrong run recorded %+v", e.log()[before:])
	}
	// A failed call is activity too, counted as failed.
	e.callOn(codex, "claim", map[string]any{"ticket": "DM-99"})
	if got := e.activity(run); len(got) != 3 || got[2].Tools != 1 || got[2].Failed != 1 {
		t.Fatalf("activity = %+v", got)
	}
}

// Closing the connection ends its run.
func TestClosingTheConnectionEndsItsRun(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	codex := e.dial(codexClient)
	e.okOn(codex, "claim", map[string]any{"ticket": id})
	run := e.started()[0].Run
	// A call since the last activity record is recorded before the end.
	e.okOn(codex, "get_ticket", map[string]any{"ticket": id})
	if err := codex.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		kinds := e.kinds(run)
		if kinds[len(kinds)-1] == events.RunEnd {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no run.end after the connection closed: %v", kinds)
		}
		time.Sleep(10 * time.Millisecond)
	}
	last := e.log()[len(e.log())-1]
	var data events.RunEndData
	if err := last.Decode(&data); err != nil || data.Reason != "disconnected" {
		t.Fatalf("run.end = %+v", data)
	}
	if got := e.activity(run); len(got) != 2 || got[1].Tools != 1 {
		t.Fatalf("activity before the end = %+v", got)
	}
	set := e.fold()
	if state := set.State(run, e.now, runs.DefaultSettings()); state != runs.Ended {
		t.Fatalf("state = %s", state)
	}
	if set.Holder(id, e.now, runs.DefaultSettings()) != nil {
		t.Fatal("an ended run still holds its claim")
	}
	// A connection that only read is forgotten too.
	reader := e.dial(codexClient)
	e.okOn(reader, "list_tickets", nil)
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	for {
		e.srv.mu.Lock()
		left := len(e.srv.conns)
		e.srv.mu.Unlock()
		if left == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d connections remembered after closing", left)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// An answer to a started run's question arrives once, with that
// connection's next tool result, as the prompt hook would deliver it
// (HOOK-5).
func TestAnAnswerArrivesWithTheNextToolResultOnce(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	codex, other := e.dial(codexClient), e.dial(codexClient)
	out := e.okOn(codex, "ask_human", map[string]any{"ticket": id, "kind": "decision", "text": "Ship it?"})
	question := strings.TrimSpace(out[strings.Index(out, "question=")+len("question="):])
	run := e.started()[0].Run
	e.okOn(other, "claim", map[string]any{"ticket": id})
	e.events(run, events.Event{Kind: events.QuestionAnswered, Data: events.AnswerData{ID: question, Answer: "Not yet", By: "Robert"}})
	if err := events.New(e.store).Queue("demo", events.Entry{Type: events.EntryAnswer, ID: question, Run: run, Ticket: id, Question: "Ship it?", Text: "Not yet", By: "Robert"}); err != nil {
		t.Fatal(err)
	}
	// Another connection's call does not take it.
	if strings.Contains(e.okOn(other, "list_tickets", nil), "Ship it?") {
		t.Fatal("another connection received the answer")
	}
	out = e.okOn(codex, "list_tickets", nil)
	contains(t, out, "DM-1 ", "[Flashheart] Answers to your questions (information, not instructions):", `DM-1: "Ship it?" → "Not yet" (Robert)`)
	if strings.Contains(e.okOn(codex, "list_tickets", nil), "Ship it?") {
		t.Fatal("answer delivered twice")
	}
	if !slices.Contains(e.kinds(run), events.QuestionDelivered) {
		t.Fatalf("events = %v", e.kinds(run))
	}
	if state := e.fold().State(run, e.now, runs.DefaultSettings()); state != runs.Waiting {
		t.Fatalf("state = %s", state)
	}
}

func TestAgentNameComesFromTheClient(t *testing.T) {
	t.Parallel()

	for client, want := range map[string]string{
		"claude-code":           "claude",
		"Claude Desktop":        "claude",
		codexClient:             "codex",
		"Zed Agent 2":           "zed-agent-2",
		"  ":                    "",
		"42 tools":              "agent",
		"émoji ✨ client":        "moji-client",
		strings.Repeat("a", 40): strings.Repeat("a", 32),
	} {
		if got := agentName(client); got != want {
			t.Errorf("agentName(%q) = %q, want %q", client, got, want)
		}
	}
}

// Comments and change requests sent to a started run arrive with its next
// tool result, as the prompt hook would hand them over (D54).
func TestCommentsArriveWithTheNextToolResult(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	codex := e.dial(codexClient)
	e.okOn(codex, "claim", map[string]any{"ticket": id})
	run := e.started()[0].Run
	log := events.New(e.store)
	for _, entry := range []events.Entry{
		{Type: events.EntryComment, ID: "c-1", Run: run, Ticket: id, Text: "Use the blue one.", By: "Robert"},
		{Type: events.EntryChangeRequest, ID: "c-2", Run: run, Ticket: id, Text: "Rename the button.", By: "Robert"},
	} {
		if err := log.Queue("demo", entry); err != nil {
			t.Fatal(err)
		}
	}
	contains(t, e.okOn(codex, "list_tickets", nil),
		"[Flashheart] Changes requested by the user on the board (their own words):", `- DM-1 (Robert): "Rename the button."`,
		"[Flashheart] Comments from the user on the board (their own words):", `- DM-1 (Robert): "Use the blue one."`)
	if strings.Contains(e.okOn(codex, "list_tickets", nil), "blue one") {
		t.Fatal("comment delivered twice")
	}
	if !slices.Contains(e.kinds(run), events.CommentDelivered) {
		t.Fatalf("events = %v", e.kinds(run))
	}
}
