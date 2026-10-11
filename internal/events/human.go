package events

import "time"

// Human is the run, agent and `by` of the events a board write records: the
// person at the board, who has no run (agent-protocol §3). Run ids are
// `agent:session`, so it never names one.
const Human = "human"

// ByHuman returns the event recording that the human changed a ticket on the
// board: kind is ticket.moved, ticket.updated or ticket.created, and data
// names the ticket with at most the columns or field names it changed.
func ByHuman(at time.Time, project, kind string, data TicketData) Event {
	data.By = Human
	return Event{Time: at, Run: Human, Agent: Human, Kind: kind, Project: project, Data: data}
}

// HumanActivity is one thing the human did on the board: a ticket write, or
// an answer to an agent's question (whose ticket the event does not name).
type HumanActivity struct {
	Time    time.Time `json:"time"`
	Project string    `json:"project"`
	Kind    string    `json:"kind"`
	Ticket  string    `json:"ticket,omitempty"`
}

// HumanActivityOf reports whether e records the human acting on the board.
// Edits made to ticket files outside the board are not events, so they never
// count.
func HumanActivityOf(e Event) (HumanActivity, bool) {
	activity := HumanActivity{Time: e.Time, Project: e.Project, Kind: e.Kind}
	switch e.Kind {
	case QuestionAnswered:
		return activity, true
	case TicketMoved, TicketUpdated, TicketCreated:
		if e.Run != Human {
			return HumanActivity{}, false
		}
		var data TicketData
		_ = e.Decode(&data)
		activity.Ticket = data.Ticket
		return activity, true
	}
	return HumanActivity{}, false
}
