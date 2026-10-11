package events

import (
	"slices"
	"time"
)

// Kinds of Change, one per headline metric on the Overview (VIEW-3, FH-52).
const (
	ChangeDone     = "done"
	ChangeReview   = "review"
	ChangeCreated  = "created"
	ChangeCriteria = "criteria"
)

// Change is one event a headline metric counts: a ticket moved to Done or
// to Ready to review, a ticket created, or a ticket's criteria ticked.
type Change struct {
	Time    time.Time
	Project string
	Kind    string
	Ticket  string
}

// ChangeOf reports whether e is a Change, whoever made it. A criteria
// change is any ticket.updated naming the criteria field: the event says
// neither how many were ticked nor whether the board unticked one
// (agent-protocol §3).
func ChangeOf(e Event) (Change, bool) {
	change := Change{Time: e.Time, Project: e.Project}
	var data TicketData
	switch e.Kind {
	case TicketMoved, TicketCreated, TicketUpdated:
		if e.Decode(&data) != nil {
			return Change{}, false
		}
		change.Ticket = data.Ticket
	default:
		return Change{}, false
	}
	switch {
	case e.Kind == TicketCreated:
		change.Kind = ChangeCreated
	case e.Kind == TicketMoved && data.From != data.To && (data.To == ChangeDone || data.To == ChangeReview):
		change.Kind = data.To
	case e.Kind == TicketUpdated && slices.Contains(data.Fields, "criteria"):
		change.Kind = ChangeCriteria
	default:
		return Change{}, false
	}
	return change, true
}

// ChangeCounts are the headline metrics: distinct tickets moved to Done,
// moved to Ready to review and created, and criteria updates.
type ChangeCounts struct {
	Done, Review, Created, Criteria int
}

// CountChanges counts the changes strictly after since in a project, or in
// every project when project is "".
func CountChanges(changes []Change, project string, since time.Time) ChangeCounts {
	var counts ChangeCounts
	type key struct{ project, kind, ticket string }
	seen := map[key]bool{}
	for _, change := range changes {
		if (project != "" && change.Project != project) || !change.Time.After(since) {
			continue
		}
		if change.Kind == ChangeCriteria {
			counts.Criteria++
			continue
		}
		k := key{change.Project, change.Kind, change.Ticket}
		if seen[k] {
			continue
		}
		seen[k] = true
		switch change.Kind {
		case ChangeDone:
			counts.Done++
		case ChangeReview:
			counts.Review++
		case ChangeCreated:
			counts.Created++
		}
	}
	return counts
}
