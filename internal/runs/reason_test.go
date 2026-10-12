package runs

import (
	"testing"

	"github.com/rztaylor/flashheart/internal/events"
)

// Why a run needs you is one derived value (FH-98, D39): the run's own
// permission prompt, its own unanswered question, a subagent's prompt, a
// subagent's unanswered question, then an answer waiting for delivery, the
// run's own before a subagent's. First match wins.
func TestReasonPrecedence(t *testing.T) {
	t.Parallel()

	explore, plan := session+"/a1", session+"/b2"
	subagent := func(minutes float64, id, agentType string) events.Event {
		return ev(minutes, id, events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: session, AgentType: agentType})
	}
	prompt := func(minutes float64, run, tool string) events.Event {
		return ev(minutes, run, events.PermissionRequested, events.PermissionData{Tool: tool})
	}
	open := []events.Event{start(0), turn(1), subagent(1.2, explore, "Explore"), subagent(1.4, plan, "Plan")}
	with := func(list ...events.Event) []events.Event { return append(append([]events.Event{}, open...), list...) }
	cases := []struct {
		name   string
		events []events.Event
		now    float64
		run    string
		want   Reason
	}{
		{"working: no reason", with(), 2, session, Reason{}},
		{"its own prompt", with(prompt(2, session, "Bash")), 3, session, Reason{Kind: ReasonPermission, Permission: "Bash"}},
		{"a prompt that names no tool", with(ev(2, session, events.PermissionRequested, events.PermissionData{})), 3, session, Reason{Kind: ReasonPermission, Permission: "?"}},
		{"its own prompt before its own question", with(ask(2, session, "q1", "decision"), prompt(3, session, "Edit")), 4, session, Reason{Kind: ReasonPermission, Permission: "Edit"}},
		{"its own question", with(ask(2, session, "q1", "review")), 3, session, Reason{Kind: ReasonQuestion, Question: "review"}},
		{"its own question before a subagent's prompt", with(ask(2, session, "q1", "decision"), prompt(3, explore, "Bash")), 4, session, Reason{Kind: ReasonQuestion, Question: "decision"}},
		{"a subagent's prompt", with(prompt(2, explore, "Bash")), 3, session, Reason{Kind: ReasonPermission, Permission: "Bash", WaitingOn: explore, AgentType: "Explore"}},
		{"a subagent's prompt before an earlier subagent's question", with(ask(2, explore, "q1", "question"), prompt(3, plan, "Bash")), 4, session, Reason{Kind: ReasonPermission, Permission: "Bash", WaitingOn: plan, AgentType: "Plan"}},
		{"a subagent's question", with(ask(2, plan, "q1", "blocked")), 3, session, Reason{Kind: ReasonQuestion, Question: "blocked", WaitingOn: plan, AgentType: "Plan"}},
		{"an untyped subagent's question", []events.Event{start(0), turn(1), subagent(1.2, explore, ""), ask(2, explore, "q1", "question")}, 3, session, Reason{Kind: ReasonQuestion, Question: "question", WaitingOn: explore}},
		{"the first of two subagents' questions", with(ask(2, plan, "q1", "blocked"), ask(3, explore, "q2", "question")), 4, session, Reason{Kind: ReasonQuestion, Question: "question", WaitingOn: explore, AgentType: "Explore"}},
		{"an unanswered question before an earlier answered one", with(ask(2, session, "q1", "decision"), answer(3, session, "q1"), ask(4, session, "q2", "review")), 5, session, Reason{Kind: ReasonQuestion, Question: "review"}},
		{"a subagent's prompt before its own answered question", with(ask(2, session, "q1", "decision"), answer(3, session, "q1"), prompt(4, explore, "Bash")), 5, session, Reason{Kind: ReasonPermission, Permission: "Bash", WaitingOn: explore, AgentType: "Explore"}},
		{"its own answer waiting for delivery", with(ask(2, session, "q1", "decision"), answer(3, session, "q1")), 4, session, Reason{Kind: ReasonQuestion, Question: "decision", Answered: true}},
		{"its own answer before a subagent's", with(ask(2, explore, "q1", "question"), answer(3, explore, "q1"), ask(4, session, "q2", "blocked"), answer(5, session, "q2")), 6, session, Reason{Kind: ReasonQuestion, Question: "blocked", Answered: true}},
		{"a subagent's answer waiting for delivery", with(ask(2, explore, "q1", "question"), answer(3, explore, "q1")), 4, session, Reason{Kind: ReasonQuestion, Question: "question", Answered: true, WaitingOn: explore, AgentType: "Explore"}},
		{"a delivered answer is no reason", with(ask(2, session, "q1", "question"), answer(3, session, "q1"), delivered(4, session, "q1")), 5, session, Reason{}},
		{"an ended subagent's prompt is no reason", with(prompt(2, explore, "Bash"), ev(3, explore, events.RunEnd, events.RunEndData{})), 4, session, Reason{}},
		{"an ended subagent's open question still holds its session", with(ask(2, explore, "q1", "question"), ev(3, explore, events.RunEnd, events.RunEndData{})), 4, session, Reason{Kind: ReasonQuestion, Question: "question", WaitingOn: explore, AgentType: "Explore"}},
		{"a subagent's own prompt is its own", with(prompt(2, explore, "Bash")), 3, explore, Reason{Kind: ReasonPermission, Permission: "Bash"}},
		{"a stale run's prompt is no reason", with(prompt(2, session, "Bash")), 13 * 60, session, Reason{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			set := NewSet()
			for _, e := range tc.events {
				set.Apply(e)
			}
			for _, v := range set.Views(at(tc.now), DefaultSettings(), nil) {
				if v.ID != tc.run {
					continue
				}
				if v.Reason != tc.want {
					t.Fatalf("reason = %+v, want %+v", v.Reason, tc.want)
				}
				// A run needs you exactly when it has a reason.
				if (v.State == NeedsYou) != (v.Reason.Kind != "") {
					t.Fatalf("state %s with reason %+v", v.State, v.Reason)
				}
				return
			}
			t.Fatalf("no view of %s", tc.run)
		})
	}
}
