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
		{"its own question", with(ask(2, session, "q1", "review")), 3, session, Reason{Kind: ReasonQuestion, Question: "review", Ticket: "AL-1"}},
		{"its own question before a subagent's prompt", with(ask(2, session, "q1", "decision"), prompt(3, explore, "Bash")), 4, session, Reason{Kind: ReasonQuestion, Question: "decision", Ticket: "AL-1"}},
		{"a subagent's prompt", with(prompt(2, explore, "Bash")), 3, session, Reason{Kind: ReasonPermission, Permission: "Bash", WaitingOn: explore, AgentType: "Explore"}},
		{"a subagent's prompt before an earlier subagent's question", with(ask(2, explore, "q1", "question"), prompt(3, plan, "Bash")), 4, session, Reason{Kind: ReasonPermission, Permission: "Bash", WaitingOn: plan, AgentType: "Plan"}},
		{"a subagent's question", with(ask(2, plan, "q1", "blocked")), 3, session, Reason{Kind: ReasonQuestion, Question: "blocked", Ticket: "AL-1", WaitingOn: plan, AgentType: "Plan"}},
		{"an untyped subagent's question", []events.Event{start(0), turn(1), subagent(1.2, explore, ""), ask(2, explore, "q1", "question")}, 3, session, Reason{Kind: ReasonQuestion, Question: "question", Ticket: "AL-1", WaitingOn: explore}},
		{"the first of two subagents' questions", with(ask(2, plan, "q1", "blocked"), ask(3, explore, "q2", "question")), 4, session, Reason{Kind: ReasonQuestion, Question: "question", Ticket: "AL-1", WaitingOn: explore, AgentType: "Explore"}},
		{"an unanswered question before an earlier answered one", with(ask(2, session, "q1", "decision"), answer(3, session, "q1"), ask(4, session, "q2", "review")), 5, session, Reason{Kind: ReasonQuestion, Question: "review", Ticket: "AL-1"}},
		{"a subagent's prompt before its own answered question", with(ask(2, session, "q1", "decision"), answer(3, session, "q1"), prompt(4, explore, "Bash")), 5, session, Reason{Kind: ReasonPermission, Permission: "Bash", WaitingOn: explore, AgentType: "Explore"}},
		{"its own answer waiting for delivery", with(ask(2, session, "q1", "decision"), answer(3, session, "q1")), 4, session, Reason{Kind: ReasonQuestion, Question: "decision", Ticket: "AL-1", Answered: true}},
		{"its own answer before a subagent's", with(ask(2, explore, "q1", "question"), answer(3, explore, "q1"), ask(4, session, "q2", "blocked"), answer(5, session, "q2")), 6, session, Reason{Kind: ReasonQuestion, Question: "blocked", Ticket: "AL-1", Answered: true}},
		{"a subagent's answer waiting for delivery", with(ask(2, explore, "q1", "question"), answer(3, explore, "q1")), 4, session, Reason{Kind: ReasonQuestion, Question: "question", Ticket: "AL-1", Answered: true, WaitingOn: explore, AgentType: "Explore"}},
		{"a delivered answer is no reason", with(ask(2, session, "q1", "question"), answer(3, session, "q1"), delivered(4, session, "q1")), 5, session, Reason{}},
		{"an ended subagent's prompt is no reason", with(prompt(2, explore, "Bash"), ev(3, explore, events.RunEnd, events.RunEndData{})), 4, session, Reason{}},
		{"an ended subagent's open question still holds its session", with(ask(2, explore, "q1", "question"), ev(3, explore, events.RunEnd, events.RunEndData{})), 4, session, Reason{Kind: ReasonQuestion, Question: "question", Ticket: "AL-1", WaitingOn: explore, AgentType: "Explore"}},
		{"a subagent's own prompt is its own", with(prompt(2, explore, "Bash")), 3, explore, Reason{Kind: ReasonPermission, Permission: "Bash"}},
		{"a stale run's prompt is no reason", with(prompt(2, session, "Bash")), 13 * 60, session, Reason{}},
		{"a question about no ticket", with(ev(2, session, events.QuestionAsked, events.QuestionData{ID: "q1", Kind: "decision", Text: "Which project?"})), 3, session, Reason{Kind: ReasonQuestion, Question: "decision"}},
		{"a subagent's question about another ticket", with(ev(2, plan, events.QuestionAsked, events.QuestionData{ID: "q1", Ticket: "AL-2", Kind: "review", Text: "Review AL-2?"})), 3, session, Reason{Kind: ReasonQuestion, Question: "review", Ticket: "AL-2", WaitingOn: plan, AgentType: "Plan"}},
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

// A reason is about a ticket's card when the human acts on it there
// (FH-185): a permission prompt is the session's own card's, a question is
// the card's of the ticket it is about, and a question about no ticket is
// the session's card's. No reason is about any card.
func TestReasonAboutTicket(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		reason Reason
		ticket string
		want   bool
	}{
		{"no reason", Reason{}, "AL-1", false},
		{"a permission prompt on the session's card", Reason{Kind: ReasonPermission, Permission: "Bash"}, "AL-1", true},
		{"a question about the ticket", Reason{Kind: ReasonQuestion, Question: "review", Ticket: "AL-1"}, "AL-1", true},
		{"a question about another ticket", Reason{Kind: ReasonQuestion, Question: "review", Ticket: "AL-2"}, "AL-1", false},
		{"an answer waiting for another ticket's question", Reason{Kind: ReasonQuestion, Question: "review", Ticket: "AL-2", Answered: true}, "AL-1", false},
		{"a question about no ticket", Reason{Kind: ReasonQuestion, Question: "decision"}, "AL-1", true},
		{"a subagent's question about the ticket", Reason{Kind: ReasonQuestion, Question: "blocked", Ticket: "AL-1", WaitingOn: "claude:s/a1"}, "AL-1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.reason.About(tc.ticket); got != tc.want {
				t.Fatalf("About(%q) = %v, want %v", tc.ticket, got, tc.want)
			}
		})
	}
}

// QuestionReason gives a card the reason a question would give the run it
// waits on (D39): a subagent's names the subagent, a session's own carries
// MCPOnly with Answered.
func TestQuestionReasonMatchesTheRunsReason(t *testing.T) {
	t.Parallel()

	plan := session + "/b2"
	set := NewSet()
	for _, e := range []events.Event{
		start(0), turn(1),
		ev(1.4, plan, events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: session, AgentType: "Plan"}),
		ev(2, plan, events.QuestionAsked, events.QuestionData{ID: "q1", Ticket: "AL-2", Kind: "review", Text: "Review AL-2?"}),
		ask(3, session, "q2", "decision"), answer(4, session, "q2"),
	} {
		set.Apply(e)
	}
	views := map[string]View{}
	for _, v := range set.Views(at(5), DefaultSettings(), nil) {
		views[v.ID] = v
	}
	if got, want := QuestionReason(views[plan].Questions[0], views[plan]), (Reason{Kind: ReasonQuestion, Question: "review", Ticket: "AL-2", WaitingOn: plan, AgentType: "Plan"}); got != want {
		t.Fatalf("subagent's question = %+v, want %+v", got, want)
	}
	// The session's own answered question, with its ticket.
	if got, want := QuestionReason(views[session].Questions[0], views[session]), (Reason{Kind: ReasonQuestion, Question: "decision", Ticket: "AL-1", Answered: true}); got != want {
		t.Fatalf("own question = %+v, want %+v", got, want)
	}
}
