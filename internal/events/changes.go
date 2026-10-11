package events

import (
	"slices"
	"time"
)

// Kinds of Change, one per headline metric on the Overview (VIEW-3, FH-52).
const (
	ChangeCheckpoint = "checkpoint"
	ChangeReview     = "review"
	ChangeCreated    = "created"
	ChangeCriteria   = "criteria"
)

// Change is one event a headline metric counts: a checkpoint, a ticket
// moved to Ready to review, a ticket created, or a ticket's criteria
// ticked. Project is the log it was read from; Run made it.
type Change struct {
	Time    time.Time
	Project string
	Kind    string
	Ticket  string
	Run     string
}

// ChangeOf reports whether e is a Change, whoever made it. Only agents
// record checkpoints. A criteria change is any ticket.updated naming the
// criteria field: the event says neither how many were ticked nor whether
// the board unticked one (agent-protocol §3).
func ChangeOf(e Event) (Change, bool) {
	change := Change{Time: e.Time, Project: e.Project, Run: e.Run}
	switch e.Kind {
	case Checkpoint:
		var data CheckpointData
		if e.Decode(&data) != nil {
			return Change{}, false
		}
		change.Kind, change.Ticket = ChangeCheckpoint, data.Ticket
		return change, true
	case TicketMoved, TicketCreated, TicketUpdated:
	default:
		return Change{}, false
	}
	var data TicketData
	if e.Decode(&data) != nil {
		return Change{}, false
	}
	change.Ticket = data.Ticket
	switch {
	case e.Kind == TicketCreated:
		change.Kind = ChangeCreated
	case e.Kind == TicketMoved && data.From != data.To && data.To == ChangeReview:
		change.Kind = ChangeReview
	case e.Kind == TicketUpdated && slices.Contains(data.Fields, "criteria"):
		change.Kind = ChangeCriteria
	default:
		return Change{}, false
	}
	return change, true
}

// ChangeCounts are the headline metrics: checkpoints, distinct tickets
// moved to Ready to review and created, and criteria updates.
type ChangeCounts struct {
	Checkpoints, Review, Created, Criteria int
}

// CountChanges counts the changes strictly after since in a project, or in
// every project when project is "". A checkpoint is recorded in both the
// run's and the ticket's project when they differ; it counts once.
func CountChanges(changes []Change, project string, since time.Time) ChangeCounts {
	var counts ChangeCounts
	type ticketKey struct{ project, kind, ticket string }
	type checkpointKey struct {
		at          int64
		run, ticket string
	}
	tickets := map[ticketKey]bool{}
	checkpoints := map[checkpointKey]bool{}
	for _, change := range changes {
		if (project != "" && change.Project != project) || !change.Time.After(since) {
			continue
		}
		switch change.Kind {
		case ChangeCriteria:
			counts.Criteria++
		case ChangeCheckpoint:
			key := checkpointKey{change.Time.UnixNano(), change.Run, change.Ticket}
			if !checkpoints[key] {
				checkpoints[key] = true
				counts.Checkpoints++
			}
		case ChangeReview, ChangeCreated:
			key := ticketKey{change.Project, change.Kind, change.Ticket}
			if tickets[key] {
				continue
			}
			tickets[key] = true
			if change.Kind == ChangeReview {
				counts.Review++
			} else {
				counts.Created++
			}
		}
	}
	return counts
}
