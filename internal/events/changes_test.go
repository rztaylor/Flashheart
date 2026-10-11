package events

import (
	"testing"
	"time"
)

func TestChangeOf(t *testing.T) {
	t.Parallel()

	agent := "claude:s1"
	by := func(kind string, data any) Event {
		return Event{Time: t0, Run: agent, Agent: "claude", Kind: kind, Project: "alpha", Data: data}
	}
	for _, test := range []struct {
		name  string
		event Event
		want  Change
		ok    bool
	}{
		{"a move to review", by(TicketMoved, TicketData{Ticket: "AL-2", From: "in-progress", To: "review", By: agent}),
			Change{Time: t0, Project: "alpha", Kind: ChangeReview, Ticket: "AL-2"}, true},
		{"the human's move to done", ByHuman(t0, "alpha", TicketMoved, TicketData{Ticket: "AL-2", From: "review", To: "done"}),
			Change{Time: t0, Project: "alpha", Kind: ChangeDone, Ticket: "AL-2"}, true},
		{"a new ticket", by(TicketCreated, TicketData{Ticket: "AL-9", By: agent}),
			Change{Time: t0, Project: "alpha", Kind: ChangeCreated, Ticket: "AL-9"}, true},
		{"criteria ticked with other fields", by(TicketUpdated, TicketData{Ticket: "AL-2", Fields: []string{"tags", "criteria", "notes"}}),
			Change{Time: t0, Project: "alpha", Kind: ChangeCriteria, Ticket: "AL-2"}, true},
		{"a claim's move to in progress", by(TicketMoved, TicketData{Ticket: "AL-2", From: "up-next", To: "in-progress", By: agent}), Change{}, false},
		{"a reorder within review", ByHuman(t0, "alpha", TicketMoved, TicketData{Ticket: "AL-2", From: "review", To: "review"}), Change{}, false},
		{"a move back from review", by(TicketMoved, TicketData{Ticket: "AL-2", From: "review", To: "in-progress", By: agent}), Change{}, false},
		{"an edit of other fields", by(TicketUpdated, TicketData{Ticket: "AL-2", Fields: []string{"notes"}}), Change{}, false},
		{"a review written", by(ReviewWritten, TicketData{Ticket: "AL-2"}), Change{}, false},
		{"a checkpoint", by(Checkpoint, TicketData{Ticket: "AL-2"}), Change{}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ChangeOf(test.event)
			if ok != test.ok || got != test.want {
				t.Errorf("ChangeOf = %+v, %v; want %+v, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

// The Overview's headline metrics count what happened strictly after the
// anchor, in one project or all of them; a ticket counts once per metric
// however often it moved, and every criteria update counts (FH-52).
func TestCountChanges(t *testing.T) {
	t.Parallel()

	at := func(minutes int) time.Time { return t0.Add(time.Duration(minutes) * time.Minute) }
	change := func(minutes int, project, kind, ticket string) Change {
		return Change{Time: at(minutes), Project: project, Kind: kind, Ticket: ticket}
	}
	log := []Change{
		change(-10, "alpha", ChangeReview, "AL-1"), // before the anchor
		change(0, "alpha", ChangeCreated, "AL-0"),  // at the anchor
		change(1, "alpha", ChangeReview, "AL-2"),
		change(2, "alpha", ChangeReview, "AL-2"), // moved back and to review again
		change(3, "alpha", ChangeDone, "AL-2"),
		change(4, "alpha", ChangeCreated, "AL-7"),
		change(5, "alpha", ChangeCriteria, "AL-2"),
		change(6, "alpha", ChangeCriteria, "AL-2"),
		change(7, "beta", ChangeReview, "BE-1"),
		change(8, "beta", ChangeCreated, "BE-4"),
		change(9, "beta", ChangeCriteria, "BE-1"),
	}
	for _, test := range []struct {
		name    string
		project string
		since   time.Time
		want    ChangeCounts
	}{
		{"every project", "", t0, ChangeCounts{Done: 1, Review: 2, Created: 2, Criteria: 3}},
		{"one project", "alpha", t0, ChangeCounts{Done: 1, Review: 1, Created: 1, Criteria: 2}},
		{"the other project", "beta", t0, ChangeCounts{Review: 1, Created: 1, Criteria: 1}},
		{"a later anchor", "", at(5), ChangeCounts{Review: 1, Created: 1, Criteria: 2}},
		{"an earlier anchor", "alpha", at(-20), ChangeCounts{Done: 1, Review: 2, Created: 2, Criteria: 2}},
		{"nothing since", "", at(9), ChangeCounts{}},
		{"a project with none", "gamma", t0, ChangeCounts{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := CountChanges(log, test.project, test.since); got != test.want {
				t.Errorf("CountChanges(%q, %v) = %+v, want %+v", test.project, test.since, got, test.want)
			}
		})
	}
}
