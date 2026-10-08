package runs

import (
	"testing"

	"github.com/rztaylor/flashheart/internal/events"
)

const other = "claude:s2"

func claim(minutes float64, run, ticket string) events.Event {
	return ev(minutes, run, events.Claim, events.TicketData{Ticket: ticket})
}

func TestClaimIsALeaseRenewedByActivity(t *testing.T) {
	t.Parallel()

	settings := DefaultSettings()
	cases := []struct {
		name   string
		events []events.Event
		now    float64
		holder string
	}{
		{"claimed: held", []events.Event{start(0), claim(1, session, "AL-1")}, 10, session},
		{"silent past the lease: free", []events.Event{start(0), claim(1, session, "AL-1")}, 32, ""},
		{"the run's own activity renews it", []events.Event{start(0), claim(1, session, "AL-1"), tool(20, "Edit", "a.go")}, 45, session},
		{"a subagent's activity renews it", []events.Event{start(0), claim(1, session, "AL-1"),
			ev(2, session+"/a1", events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: session}),
			ev(25, session+"/a1", events.ToolUsed, events.ToolData{Tool: "Read", OK: true})}, 50, session},
		{"an ended run holds nothing", []events.Event{start(0), claim(1, session, "AL-1"), ev(2, session, events.RunEnd, events.RunEndData{})}, 3, ""},
		{"released: free", []events.Event{start(0), claim(1, session, "AL-1"), ev(2, session, events.Release, events.TicketData{Ticket: "AL-1"})}, 3, ""},
		{"claiming another ticket gives up the first", []events.Event{start(0), claim(1, session, "AL-1"), claim(2, session, "AL-2")}, 3, ""},
		{"a forced claim takes it over", []events.Event{start(0), claim(1, session, "AL-1"),
			ev(2, other, events.RunStart, events.RunStartData{Kind: events.KindSession}),
			ev(3, other, events.Claim, events.TicketData{Ticket: "AL-1", Force: true, Reason: "previous session died"})}, 4, other},
		{"unclaimed: free", []events.Event{start(0)}, 1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			set := NewSet()
			for _, e := range tc.events {
				set.Apply(e)
			}
			holder := ""
			if r := set.Holder("AL-1", at(tc.now), settings); r != nil {
				holder = r.ID
			}
			if holder != tc.holder {
				t.Fatalf("holder at %v = %q, want %q", tc.now, holder, tc.holder)
			}
		})
	}
}

func TestTakenOverClaimLeavesTheFormerHolderUnlinked(t *testing.T) {
	t.Parallel()

	set := NewSet()
	set.Apply(start(0))
	set.Apply(claim(1, session, "AL-1"))
	set.Apply(ev(2, other, events.Claim, events.TicketData{Ticket: "AL-1", Force: true}))
	if link := set.Link(session, nil); link.Ticket != "" {
		t.Fatalf("former holder still linked: %+v", link)
	}
	if link := set.Link(other, nil); link != (Link{Ticket: "AL-1", By: LinkClaim}) {
		t.Fatalf("new holder link = %+v", link)
	}
}

func ask(minutes float64, run, id, kind string) events.Event {
	return ev(minutes, run, events.QuestionAsked, events.QuestionData{ID: id, Ticket: "AL-1", Kind: kind, Text: "Use the old schema?", Options: []string{"Yes", "No"}})
}

func answer(minutes float64, run, id string) events.Event {
	return ev(minutes, run, events.QuestionAnswered, events.AnswerData{ID: id, Answer: "Yes", By: "Robert"})
}

func delivered(minutes float64, run, id string) events.Event {
	return ev(minutes, run, events.QuestionDelivered, events.DeliveredData{ID: id})
}

func TestQuestionsNeedYouUntilTheAnswerIsDelivered(t *testing.T) {
	t.Parallel()

	settings := DefaultSettings()
	sub := session + "/a1"
	subStart := ev(1.5, sub, events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: session})
	cases := []struct {
		name   string
		events []events.Event
		now    float64
		run    string
		want   State
	}{
		{"asked: needs you", []events.Event{start(0), turn(1), ask(2, session, "q1", "decision"), stop(3)}, 4, session, NeedsYou},
		{"a prompt in the asking session answers it there", []events.Event{start(0), turn(1), ask(2, session, "q1", "question"), stop(3), turn(5)}, 6, session, Working},
		{"a prompt in the session answers its subagent's question", []events.Event{start(0), turn(1), subStart, ask(2, sub, "q2", "question"), stop(3), turn(5)}, 6, session, Working},
		{"a background task's notice leaves the question open", []events.Event{start(0), turn(1), ask(2, session, "q1", "question"), stop(3), ev(5, session, events.TurnStart, events.TurnStartData{Background: true})}, 6, session, NeedsYou},
		{"another session's prompt leaves the question open", []events.Event{start(0), turn(1), ask(2, session, "q1", "decision"), stop(3), ev(5, "claude:s2", events.TurnStart, events.TurnStartData{})}, 6, session, NeedsYou},
		{"a prompt before the answer is delivered leaves it waiting for delivery", []events.Event{start(0), turn(1), ask(2, session, "q1", "decision"), stop(3), answer(4, session, "q1"), turn(5)}, 6, session, NeedsYou},
		{"answered but not delivered: still needs you", []events.Event{start(0), turn(1), ask(2, session, "q1", "blocked"), stop(3), answer(4, session, "q1")}, 5, session, NeedsYou},
		{"delivered: waiting again", []events.Event{start(0), turn(1), ask(2, session, "q1", "review"), stop(3), answer(4, session, "q1"), delivered(5, session, "q1")}, 6, session, Waiting},
		{"a subagent's question holds its session", []events.Event{start(0), turn(1), subStart, ask(2, sub, "q2", "question")}, 3, session, NeedsYou},
		{"delivered to the session clears the subagent's question", []events.Event{start(0), turn(1), subStart, ask(2, sub, "q2", "question"), answer(3, sub, "q2"), delivered(4, sub, "q2")}, 5, session, Working},
		{"ended beats an open question", []events.Event{start(0), turn(1), ask(2, session, "q1", "question"), ev(3, session, events.RunEnd, events.RunEndData{})}, 4, session, Ended},
		{"an unknown kind is not a question", []events.Event{start(0), turn(1), ask(2, session, "q1", "chat")}, 3, session, Working},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			set := NewSet()
			for _, e := range tc.events {
				set.Apply(e)
			}
			if got := set.State(tc.run, at(tc.now), settings); got != tc.want {
				t.Fatalf("state = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestPendingAnswersCoverTheSessionAndItsSubagents(t *testing.T) {
	t.Parallel()

	sub := session + "/a1"
	set := NewSet()
	for _, e := range []events.Event{
		start(0), turn(1),
		ev(1.5, sub, events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: session}),
		ask(2, session, "q1", "decision"), ask(3, sub, "q2", "question"), ask(4, session, "q3", "question"),
		answer(5, session, "q1"), answer(6, sub, "q2"), delivered(7, session, "q1"),
	} {
		set.Apply(e)
	}
	pending := set.PendingAnswers(session)
	if len(pending) != 1 || pending[0].ID != "q2" || pending[0].Run != sub || pending[0].Answer != "Yes" || pending[0].AnsweredBy != "Robert" {
		t.Fatalf("pending = %+v", pending)
	}
	views := set.Views(at(8), DefaultSettings(), nil)
	// The subagent's question is on the subagent's own run.
	if len(views[0].Questions) != 2 || len(views[1].Questions) != 1 {
		t.Fatalf("session view questions = %+v", views[0].Questions)
	}
	q3 := views[0].Questions[1]
	if q3.ID != "q3" || q3.Answered() || q3.Text != "Use the old schema?" || len(q3.Options) != 2 {
		t.Fatalf("open question = %+v", q3)
	}
}

func TestQuestionsAreBounded(t *testing.T) {
	t.Parallel()

	set := NewSet()
	set.Apply(start(0))
	for i := range MaxQuestions + 5 {
		set.Apply(ask(float64(i+1), session, "q"+string(rune('a'+i%26))+string(rune('a'+i/26)), "question"))
	}
	if got := len(set.Get(session).Questions); got != MaxQuestions {
		t.Fatalf("kept %d questions, want %d", got, MaxQuestions)
	}
}

func TestAPromptAnswersTheSessionsOpenQuestionsInTheSession(t *testing.T) {
	t.Parallel()

	sub := session + "/a1"
	other := "claude:s2"
	set := NewSet()
	for _, e := range []events.Event{
		start(0), turn(1),
		ev(1.5, sub, events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: session}),
		ask(2, session, "q1", "decision"), ask(2.5, sub, "q2", "question"), ask(3, other, "q3", "question"),
		ask(3.5, session, "q4", "review"), answer(3.8, session, "q4"),
		stop(4), turn(5),
	} {
		set.Apply(e)
	}
	q1, q2, q3, q4 := *set.Get(session).question("q1"), *set.Get(sub).question("q2"), *set.Get(other).question("q3"), *set.Get(session).question("q4")
	for _, q := range []Question{q1, q2} {
		if !q.AnsweredInSession || q.Open() || q.Answered() {
			t.Fatalf("question %s = %+v, want answered in the session and closed", q.ID, q)
		}
	}
	if q3.AnsweredInSession || !q3.Open() {
		t.Fatalf("another session's question = %+v, want open", q3)
	}
	// An answer given on the board waits for delivery; the prompt does not
	// replace it.
	if q4.AnsweredInSession || !q4.Open() || q4.Answer != "Yes" {
		t.Fatalf("answered question = %+v, want waiting for delivery", q4)
	}
	if pending := set.PendingAnswers(session); len(pending) != 1 || pending[0].ID != "q4" {
		t.Fatalf("pending = %+v, want only q4", pending)
	}
	// The question stays in the run's history, marked in its timeline.
	var marked []string
	for _, entry := range set.Get(session).Timeline {
		if entry.Kind == QuestionAnsweredInSession {
			marked = append(marked, entry.Ticket)
		}
	}
	if len(marked) != 1 || marked[0] != "AL-1" {
		t.Fatalf("timeline marks = %v, want one for AL-1", marked)
	}
	// The mark follows the prompt that answered it.
	if timeline := set.Get(session).Timeline; timeline[len(timeline)-2].Kind != events.TurnStart || timeline[len(timeline)-1].Kind != QuestionAnsweredInSession {
		t.Fatalf("timeline ends %+v", timeline[len(timeline)-2:])
	}
	if got := set.State(session, at(6), DefaultSettings()); got != NeedsYou {
		t.Fatalf("state with q4 undelivered = %s, want needs-you", got)
	}
	set.Apply(delivered(5, session, "q4"))
	if got := set.State(session, at(6), DefaultSettings()); got != Working {
		t.Fatalf("state = %s, want working", got)
	}
}
