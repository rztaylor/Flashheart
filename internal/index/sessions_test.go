package index

import (
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
)

// Fixtures for the session summary table: runs in project alpha, where
// in-progress AL-3 is on branch feature/x.
var sessionsNow = time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)

func sessionEvent(minutesAgo float64, run, kind string, data any) events.Event {
	at := sessionsNow.Add(-time.Duration(minutesAgo * float64(time.Minute)))
	return events.Event{Time: at, Run: run, Agent: "claude", Kind: kind, Project: "alpha", Data: data}
}

func sessionStart(minutesAgo float64, run, branch string) []events.Event {
	return []events.Event{
		sessionEvent(minutesAgo, run, events.RunStart, events.RunStartData{Kind: events.KindSession, Branch: branch}),
		sessionEvent(minutesAgo, run, events.TurnStart, events.TurnStartData{}),
	}
}

func subagentStart(minutesAgo float64, run string) events.Event {
	parent, _, _ := strings.Cut(run, "/")
	return sessionEvent(minutesAgo, run, events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: parent, AgentType: "general-purpose"})
}

func claimEvent(minutesAgo float64, run, ticket string) events.Event {
	return sessionEvent(minutesAgo, run, events.Claim, events.TicketData{Ticket: ticket})
}

func endEvent(minutesAgo float64, run string) events.Event {
	return sessionEvent(minutesAgo, run, events.RunEnd, events.RunEndData{Reason: "other"})
}

// foldSnapshot folds events into runs as the index does and returns a
// snapshot holding them, linking runs on feature/x to AL-3 by branch.
func foldSnapshot(list []events.Event) *Snapshot {
	set := runs.NewSet()
	for _, e := range list {
		set.Apply(e)
	}
	byBranch := func(project, branch string) []string {
		if project == "alpha" && branch == "feature/x" {
			return []string{"AL-3"}
		}
		return nil
	}
	return &Snapshot{Runs: set.Views(sessionsNow, runs.DefaultSettings(), byBranch)}
}

func concat(lists ...[]events.Event) []events.Event {
	var all []events.Event
	for _, list := range lists {
		all = append(all, list...)
	}
	return all
}

// A ticket's session summary speaks for its linked runs: the live one that
// most needs attention, its last activity and handoff flags, and every
// subagent working for the ticket counted once (FH-50).
func TestTicketSessions(t *testing.T) {
	t.Parallel()

	inProgress := board.Ticket{ID: "AL-3", Column: board.InProgress}
	review := board.Ticket{ID: "AL-3", Column: board.Review}
	cases := []struct {
		name   string
		ticket board.Ticket
		events []events.Event
		want   TicketSessions
		ok     bool
	}{
		{
			name:   "3 ended and 2 running subagents",
			ticket: inProgress,
			events: concat(sessionStart(30, "claude:s1", "main"), []events.Event{
				claimEvent(29, "claude:s1", "AL-3"),
				subagentStart(20, "claude:s1/a1"), subagentStart(20, "claude:s1/a2"), subagentStart(20, "claude:s1/a3"),
				subagentStart(10, "claude:s1/a4"), subagentStart(10, "claude:s1/a5"),
				endEvent(15, "claude:s1/a1"), endEvent(14, "claude:s1/a2"), endEvent(2, "claude:s1/a3"),
			}),
			want: TicketSessions{
				Run: "claude:s1", Agent: "claude", State: runs.Working, LastActivity: sessionsNow.Add(-2 * time.Minute),
				Subagents: SubagentCounts{Done: 3, Running: 2},
			},
			ok: true,
		},
		{
			name:   "a subagent awaiting permission needs you",
			ticket: inProgress,
			events: concat(sessionStart(30, "claude:s1", "main"), []events.Event{
				claimEvent(29, "claude:s1", "AL-3"),
				subagentStart(10, "claude:s1/a1"),
				sessionEvent(5, "claude:s1/a1", events.PermissionRequested, events.PermissionData{Tool: "Bash"}),
			}),
			want: TicketSessions{
				Run: "claude:s1", Agent: "claude", State: runs.NeedsYou, LastActivity: sessionsNow.Add(-5 * time.Minute),
				Subagents: SubagentCounts{NeedsYou: 1},
			},
			ok: true,
		},
		{
			name:   "an in-progress ticket whose run ended has no live session",
			ticket: inProgress,
			events: concat(sessionStart(60, "claude:s1", "main"), []events.Event{
				claimEvent(59, "claude:s1", "AL-3"),
				subagentStart(50, "claude:s1/a1"),
				sessionEvent(45, "claude:s1", events.ToolUsed, events.ToolData{Tool: "Edit", OK: true, Path: "a.go"}),
				endEvent(40, "claude:s1"),
			}),
			want: TicketSessions{
				Run: "claude:s1", Agent: "claude", State: runs.Ended, LastActivity: sessionsNow.Add(-40 * time.Minute),
				NoLiveSession: true, Dirty: true, NoHandoff: true,
				Subagents: SubagentCounts{Done: 1},
			},
			ok: true,
		},
		{
			name:   "a ticket in review is not missing a session",
			ticket: review,
			events: concat(sessionStart(60, "claude:s1", "main"), []events.Event{
				claimEvent(59, "claude:s1", "AL-3"),
				endEvent(40, "claude:s1"),
			}),
			want: TicketSessions{Run: "claude:s1", Agent: "claude", State: runs.Ended, LastActivity: sessionsNow.Add(-40 * time.Minute)},
			ok:   true,
		},
		{
			name:   "a branch-linked run counts like a claim",
			ticket: inProgress,
			events: concat(sessionStart(30, "claude:s1", "feature/x"), []events.Event{
				subagentStart(10, "claude:s1/a1"),
				subagentStart(8, "claude:s1/a2"), endEvent(3, "claude:s1/a2"),
			}),
			want: TicketSessions{
				Run: "claude:s1", Agent: "claude", State: runs.Working, LastActivity: sessionsNow.Add(-3 * time.Minute),
				Subagents: SubagentCounts{Done: 1, Running: 1},
			},
			ok: true,
		},
		{
			name:   "a subagent claiming its session's ticket is counted once",
			ticket: inProgress,
			events: concat(sessionStart(30, "claude:s1", "main"), []events.Event{
				claimEvent(29, "claude:s1", "AL-3"),
				subagentStart(10, "claude:s1/a1"), claimEvent(9, "claude:s1/a1", "AL-3"),
			}),
			want: TicketSessions{
				Run: "claude:s1", Agent: "claude", State: runs.Working, LastActivity: sessionsNow.Add(-9 * time.Minute),
				Subagents: SubagentCounts{Running: 1},
			},
			ok: true,
		},
		{
			name:   "a subagent claiming a branch-linked session's ticket is counted once",
			ticket: inProgress,
			events: concat(sessionStart(30, "claude:s1", "feature/x"), []events.Event{
				subagentStart(10, "claude:s1/a1"), claimEvent(9, "claude:s1/a1", "AL-3"),
			}),
			want: TicketSessions{
				Run: "claude:s1", Agent: "claude", State: runs.Working, LastActivity: sessionsNow.Add(-9 * time.Minute),
				Subagents: SubagentCounts{Running: 1},
			},
			ok: true,
		},
		{
			name:   "a subagent that claimed the ticket under another ticket's session speaks for it",
			ticket: inProgress,
			events: concat(sessionStart(30, "claude:s1", "main"), []events.Event{
				claimEvent(29, "claude:s1", "AL-9"),
				subagentStart(10, "claude:s1/a1"), claimEvent(9, "claude:s1/a1", "AL-3"),
				subagentStart(8, "claude:s1/a2"), claimEvent(7, "claude:s1/a2", "AL-4"),
			}),
			want: TicketSessions{
				Run: "claude:s1/a1", Agent: "claude", State: runs.Working, LastActivity: sessionsNow.Add(-9 * time.Minute),
				Subagents: SubagentCounts{Running: 1},
			},
			ok: true,
		},
		{
			// Subagents that claimed other tickets count only there; one with
			// no claim, or one that claimed the session's ticket, rolls up.
			name:   "the orchestrating session's ticket leaves out subagents that claimed other tickets",
			ticket: board.Ticket{ID: "AL-9", Column: board.InProgress},
			events: concat(sessionStart(30, "claude:s1", "main"), []events.Event{
				claimEvent(29, "claude:s1", "AL-9"),
				subagentStart(10, "claude:s1/a1"), claimEvent(9, "claude:s1/a1", "AL-3"),
				subagentStart(8, "claude:s1/a2"), claimEvent(7, "claude:s1/a2", "AL-4"),
				subagentStart(6, "claude:s1/a3"),
				subagentStart(5, "claude:s1/a4"), claimEvent(4, "claude:s1/a4", "AL-9"),
			}),
			want: TicketSessions{
				Run: "claude:s1", Agent: "claude", State: runs.Working, LastActivity: sessionsNow.Add(-4 * time.Minute),
				Subagents: SubagentCounts{Running: 2},
			},
			ok: true,
		},
		{
			name:   "a live session speaks for the ticket over an ended one",
			ticket: inProgress,
			events: concat(sessionStart(90, "claude:s1", "main"), []events.Event{
				claimEvent(89, "claude:s1", "AL-3"),
				subagentStart(80, "claude:s1/a1"),
				endEvent(70, "claude:s1"),
			}, sessionStart(60, "claude:s2", "feature/x"), []events.Event{
				sessionEvent(20, "claude:s2", events.TurnEnd, events.TurnEndData{}),
			}),
			want: TicketSessions{
				Run: "claude:s2", Agent: "claude", State: runs.Waiting, LastActivity: sessionsNow.Add(-20 * time.Minute),
				Subagents: SubagentCounts{Done: 1},
			},
			ok: true,
		},
		{
			name:   "an in-progress ticket with no runs has no live session",
			ticket: inProgress,
			events: sessionStart(10, "claude:s1", "main"),
			want:   TicketSessions{NoLiveSession: true},
			ok:     true,
		},
		{
			name:   "a ticket with no runs that is not in progress has no summary",
			ticket: review,
			events: sessionStart(10, "claude:s1", "main"),
			ok:     false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := foldSnapshot(tc.events).Sessions(tc.ticket)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("Sessions = %+v, %v\nwant       %+v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}
