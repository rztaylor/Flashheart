package runs

import (
	"testing"

	"github.com/rztaylor/flashheart/internal/events"
)

// A run the MCP server started for an agent without hooks has no turn
// events: its tool calls are its only activity (agent-protocol §4, FH-15).
const mcpRun = "codex:mcp-0a1b2c3d4e5f"

func mcpEvent(minutes float64, kind string, data any) events.Event {
	e := ev(minutes, mcpRun, kind, data)
	e.Agent = "codex"
	return e
}

func mcpStart(minutes float64) events.Event {
	return mcpEvent(minutes, events.RunStart, events.RunStartData{Kind: events.KindSession, Cwd: "/src/alpha", Branch: "feature/x", Worktree: "/src/alpha", Source: events.SourceMCP})
}

func mcpCall(minutes float64, name string) events.Event {
	return mcpEvent(minutes, events.ToolUsed, events.ToolData{Tool: name, OK: true})
}

func TestMCPStartedRuns(t *testing.T) {
	t.Parallel()

	settings := DefaultSettings()
	question := mcpEvent(3, events.QuestionAsked, events.QuestionData{ID: "q-1", Kind: "question", Text: "Which schema?"})
	answered := mcpEvent(5, events.QuestionAnswered, events.AnswerData{ID: "q-1", Answer: "v2", By: "Robert"})
	delivered := mcpEvent(6, events.QuestionDelivered, events.DeliveredData{ID: "q-1"})
	claim := mcpEvent(1, events.Claim, events.TicketData{Ticket: "AL-1"})
	cases := []struct {
		name    string
		events  []events.Event
		now     float64
		want    State
		holding bool
	}{
		{"started by a write: waiting, never working", []events.Event{mcpStart(0), claim}, 2, Waiting, true},
		{"tool calls are activity but not a turn", []events.Event{mcpStart(0), claim, mcpCall(1, "claim"), mcpCall(20, "get_ticket")}, 21, Waiting, true},
		{"a question needs you", []events.Event{mcpStart(0), claim, question}, 4, NeedsYou, true},
		{"answered, not yet delivered: still needs you", []events.Event{mcpStart(0), claim, question, answered}, 5.5, NeedsYou, true},
		{"delivered in a tool result: waiting", []events.Event{mcpStart(0), claim, question, answered, delivered}, 7, Waiting, true},
		{"no tool call for the lease: the claim lapses", []events.Event{mcpStart(0), claim, mcpCall(1, "claim")}, 32, Waiting, false},
		{"a tool call renews the lease", []events.Event{mcpStart(0), claim, mcpCall(1, "claim"), mcpCall(25, "list_tickets")}, 50, Waiting, true},
		{"disconnected: ended", []events.Event{mcpStart(0), claim, mcpEvent(4, events.RunEnd, events.RunEndData{Reason: "disconnected"})}, 5, Ended, false},
		{"ended beats an open question", []events.Event{mcpStart(0), question, mcpEvent(4, events.RunEnd, events.RunEndData{Reason: "disconnected"})}, 5, Ended, false},
		{"a server that died: ended by the stale rule", []events.Event{mcpStart(0), claim, mcpCall(1, "claim")}, 12*60 + 2, Ended, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			set := NewSet()
			for _, e := range tc.events {
				set.Apply(e)
			}
			if got := set.State(mcpRun, at(tc.now), settings); got != tc.want {
				t.Errorf("state = %s, want %s", got, tc.want)
			}
			if holder := set.Holder("AL-1", at(tc.now), settings); (holder != nil) != tc.holding {
				t.Errorf("holder = %v, want holding %v", holder, tc.holding)
			}
			r := set.Get(mcpRun)
			if r.Source != events.SourceMCP || r.Kind != events.KindSession || len(r.Plan) != 0 || r.Edits != 0 || r.Permission != "" {
				t.Errorf("run = %+v", r)
			}
		})
	}
}
