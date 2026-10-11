package runs

import (
	"testing"
	"time"

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

// mcpCall is the activity record of the run's Flashheart tool calls.
func mcpCall(minutes float64, tools int) events.Event {
	return mcpEvent(minutes, events.Activity, events.ActivityData{Tools: tools})
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
		{"tool calls are activity but not a turn", []events.Event{mcpStart(0), claim, mcpCall(1, 1), mcpCall(20, 1)}, 21, Waiting, true},
		{"a question needs you", []events.Event{mcpStart(0), claim, question}, 4, NeedsYou, true},
		{"answered, not yet delivered: still needs you", []events.Event{mcpStart(0), claim, question, answered}, 5.5, NeedsYou, true},
		{"delivered in a tool result: waiting", []events.Event{mcpStart(0), claim, question, answered, delivered}, 7, Waiting, true},
		{"no tool call for the lease: the claim lapses", []events.Event{mcpStart(0), claim, mcpCall(1, 1)}, 32, Waiting, false},
		{"a tool call renews the lease", []events.Event{mcpStart(0), claim, mcpCall(1, 1), mcpCall(25, 1)}, 50, Waiting, true},
		{"disconnected: ended", []events.Event{mcpStart(0), claim, mcpEvent(4, events.RunEnd, events.RunEndData{Reason: "disconnected"})}, 5, Ended, false},
		{"ended beats an open question", []events.Event{mcpStart(0), question, mcpEvent(4, events.RunEnd, events.RunEndData{Reason: "disconnected"})}, 5, Ended, false},
		{"a server that died: ended by the stale rule", []events.Event{mcpStart(0), claim, mcpCall(1, 1)}, 12*60 + 2, Ended, false},
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

// A started run's agent that switches branches has its run.start restated
// with the new branch (agent-protocol §7.1): the timeline shows that as a
// branch switch, not a second connection (FH-71). A hook run's own
// restarts keep their source.
func TestRestatedStartsInTheTimeline(t *testing.T) {
	t.Parallel()

	restated := mcpEvent(2, events.RunStart, events.RunStartData{Kind: events.KindSession, Cwd: "/src/alpha", Branch: "feature/next", Worktree: "/src/alpha", Source: events.SourceMCP})
	hook := func(minutes float64, source string) events.Event {
		return ev(minutes, session, events.RunStart, events.RunStartData{Kind: events.KindSession, Branch: "feature/y", Source: source})
	}
	ended := mcpEvent(1, events.RunEnd, events.RunEndData{Reason: events.RunEndDisconnected})
	cases := []struct {
		name   string
		run    string
		events []events.Event
		want   []Entry
	}{
		{"a branch switch", mcpRun, []events.Event{mcpStart(0), mcpCall(1, 1), restated},
			[]Entry{{Kind: events.RunStart, Detail: events.SourceMCP}, {Kind: events.Activity, Tools: 1}, {Kind: events.RunStart, Detail: events.SourceMCP, Branch: "feature/next"}}},
		{"the first start", mcpRun, []events.Event{mcpStart(0)},
			[]Entry{{Kind: events.RunStart, Detail: events.SourceMCP}}},
		{"a start after the run ended", mcpRun, []events.Event{mcpStart(0), ended, restated},
			[]Entry{{Kind: events.RunStart, Detail: events.SourceMCP}, {Kind: events.RunEnd, Detail: events.RunEndDisconnected}, {Kind: events.RunStart, Detail: events.SourceMCP}}},
		{"a hook run resumed", session, []events.Event{start(0), ev(1, session, events.RunEnd, events.RunEndData{Reason: "other"}), hook(2, "resume")},
			[]Entry{{Kind: events.RunStart, Detail: "startup"}, {Kind: events.RunEnd, Detail: "other"}, {Kind: events.RunStart, Detail: "resume"}}},
		{"a hook run resumed without an end", session, []events.Event{start(0), hook(2, "resume")},
			[]Entry{{Kind: events.RunStart, Detail: "startup"}, {Kind: events.RunStart, Detail: "resume"}}},
		{"a hook run compacted", session, []events.Event{start(0), hook(2, "compact")},
			[]Entry{{Kind: events.RunStart, Detail: "startup"}, {Kind: events.RunStart, Detail: "compact"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			set := NewSet()
			for _, e := range tc.events {
				set.Apply(e)
			}
			timeline := set.Get(tc.run).Timeline
			if len(timeline) != len(tc.want) {
				t.Fatalf("timeline = %+v", timeline)
			}
			for index, entry := range timeline {
				entry.Time = time.Time{}
				if entry != tc.want[index] {
					t.Errorf("entry %d = %+v, want %+v", index, entry, tc.want[index])
				}
			}
		})
	}
}
