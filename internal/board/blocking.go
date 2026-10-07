package board

import (
	"fmt"
	"slices"
	"time"
)

// Board is every project under a root, and the projects archived under
// <root>/.archive/ (PRJ-5).
type Board struct {
	Projects         []Project
	ArchivedProjects []ArchivedProject
	// RetiredKeys are the keys of permanently deleted projects (KEY-5); their
	// ids count as done, like each project's Retired ids.
	RetiredKeys []string
}

// ArchivedProject is a project moved to <root>/.archive/. Its tickets count
// as done for other projects (like archived tickets) and its key stays
// taken (KEY-5).
type ArchivedProject struct {
	Name, DisplayName, Key string
	KeyDerived             bool
	Repos                  []string
	// IDs are its tickets' ids, live and archived within it.
	IDs []string
	// Archived is when it was archived (its directory's modification time).
	Archived time.Time
}

// Ref names a ticket (ID is its id) or a workstream (ID is its slug) in a
// project. A missing ticket has no project.
type Ref struct {
	Project string `json:"project"`
	ID      string `json:"id"`
}

// ReasonKind classifies why a ticket is blocked (CARD-4).
type ReasonKind string

const (
	// TicketDependency: a depends-on ticket is not in review or done.
	TicketDependency ReasonKind = "ticket"
	// WorkstreamDependency: a depended-on workstream is incomplete or missing.
	// Via names the ticket's own workstream when the dependency is declared
	// there; Via == Workstream with Missing means that workstream is missing.
	WorkstreamDependency ReasonKind = "workstream"
)

// Reason is one explanation of a blocked ticket.
type Reason struct {
	Kind       ReasonKind
	Ticket     Ref
	Workstream string
	Column     Column
	Pending    int
	Missing    bool
	Via        string
}

// Workstream statuses derived for display.
const (
	StatusActive    = "active"
	StatusBlocked   = "blocked"
	StatusCompleted = "completed"
)

// WorkstreamState is a workstream's derived progress.
type WorkstreamState struct {
	Status      string
	Done, Total int
	// Next is the first ticket in order not yet in review or done and not
	// blocked, else the first one not yet in review or done.
	Next    string
	Reasons []Reason
}

// Analysis is the derived blocking state of a board. Tickets and workstreams
// are keyed by Ref.
type Analysis struct {
	Blocked     map[Ref][]Reason
	Workstreams map[Ref]WorkstreamState
	Warnings    map[Ref][]string
}

type analyzer struct {
	project     map[string]string   // ticket id → project
	columns     map[string][]Column // ticket id → columns of every copy
	archived    map[string]bool
	retiredKeys map[string]bool
	workstreams map[Ref]Workstream
	members     map[string][]Ref // ticket id → workstreams listing it, in file order
}

// Analyze computes blocking reasons for every open ticket, derived workstream
// states, and membership warnings. Ticket ids are global across projects.
func Analyze(b Board) Analysis {
	a := analyzer{
		project: map[string]string{}, columns: map[string][]Column{}, archived: map[string]bool{}, retiredKeys: map[string]bool{},
		workstreams: map[Ref]Workstream{}, members: map[string][]Ref{},
	}
	for _, project := range b.Projects {
		for _, ticket := range project.Tickets {
			if ticket.ID == "" {
				continue
			}
			a.project[ticket.ID] = project.Name
			a.columns[ticket.ID] = append(a.columns[ticket.ID], ticket.Column)
		}
		// A permanently deleted ticket blocks nothing, even where a
		// reference to it survived (EDIT-8).
		for _, id := range project.Retired {
			a.archived[id] = true
		}
		for _, id := range project.Archived {
			a.archived[id] = true
			if _, known := a.project[id]; !known {
				a.project[id] = project.Name
			}
		}
		for _, workstream := range project.Workstreams {
			ref := Ref{project.Name, workstream.Slug}
			a.workstreams[ref] = workstream
			for _, id := range workstream.Tickets {
				if !slices.Contains(a.members[id], ref) {
					a.members[id] = append(a.members[id], ref)
				}
			}
		}
	}

	for _, key := range b.RetiredKeys {
		a.retiredKeys[key] = true
	}
	for _, project := range b.ArchivedProjects {
		for _, id := range project.IDs {
			a.archived[id] = true
			if _, known := a.project[id]; !known {
				a.project[id] = project.Name
			}
		}
	}

	result := Analysis{Blocked: map[Ref][]Reason{}, Workstreams: map[Ref]WorkstreamState{}, Warnings: map[Ref][]string{}}
	for _, project := range b.Projects {
		for _, ticket := range project.Tickets {
			if ticket.ID == "" {
				continue
			}
			ref := Ref{project.Name, ticket.ID}
			if warnings := a.membershipWarnings(project.Name, ticket); len(warnings) > 0 {
				result.Warnings[ref] = append(result.Warnings[ref], warnings...)
			}
			if !ticket.Column.Open() {
				continue
			}
			if reasons := a.reasons(project.Name, ticket); len(reasons) > 0 {
				result.Blocked[ref] = reasons
			}
		}
	}
	for _, project := range b.Projects {
		for _, workstream := range project.Workstreams {
			result.Workstreams[Ref{project.Name, workstream.Slug}] = a.workstreamState(project.Name, workstream, result.Blocked)
		}
	}
	return result
}

func (a analyzer) ref(id string) Ref { return Ref{a.project[id], id} }

// ticketColumn returns the column that keeps id from being satisfied (the
// first unsatisfied copy), whether it is satisfied, and whether it exists.
func (a analyzer) ticketColumn(id string) (Column, bool, bool) {
	if a.archived[id] {
		return "", true, true
	}
	columns := a.columns[id]
	if len(columns) == 0 {
		// Ids of a deleted project's key were removed on purpose (PRJ-5).
		if key, _, ok := ParseID(id); ok && a.retiredKeys[key] {
			return "", true, true
		}
		return "", false, false
	}
	for _, column := range columns {
		if !column.Satisfies() {
			return column, false, true
		}
	}
	return columns[0], true, true
}

func (a analyzer) ticketReason(kind ReasonKind, id, workstream string) (Reason, bool) {
	column, satisfied, exists := a.ticketColumn(id)
	if satisfied {
		return Reason{}, false
	}
	return Reason{Kind: kind, Ticket: a.ref(id), Workstream: workstream, Column: column, Missing: !exists}, true
}

// workstreamReason reports an incomplete or missing depended-on workstream.
func (a analyzer) workstreamReason(project, name, via string) (Reason, bool) {
	workstream, ok := a.workstreams[Ref{project, name}]
	if !ok {
		return Reason{Kind: WorkstreamDependency, Workstream: name, Missing: true, Via: via}, true
	}
	pending := 0
	for _, id := range workstream.Tickets {
		if _, satisfied, _ := a.ticketColumn(id); !satisfied {
			pending++
		}
	}
	if pending == 0 {
		return Reason{}, false
	}
	return Reason{Kind: WorkstreamDependency, Workstream: name, Pending: pending, Via: via}, true
}

func (a analyzer) reasons(project string, ticket Ticket) []Reason {
	var reasons []Reason
	for _, id := range ticket.DependsOn {
		if reason, blocked := a.ticketReason(TicketDependency, id, ""); blocked {
			reasons = append(reasons, reason)
		}
	}
	for _, name := range ticket.DependsOnWorkstreams {
		if reason, blocked := a.workstreamReason(project, name, ""); blocked {
			reasons = append(reasons, reason)
		}
	}
	if ticket.Workstream != "" {
		if _, ok := a.workstreams[Ref{project, ticket.Workstream}]; !ok {
			reasons = append(reasons, Reason{Kind: WorkstreamDependency, Workstream: ticket.Workstream, Missing: true, Via: ticket.Workstream})
		}
	}
	for _, ref := range a.members[ticket.ID] {
		workstream := a.workstreams[ref]
		for _, dependency := range workstream.DependsOnWorkstreams {
			if reason, blocked := a.workstreamReason(ref.Project, dependency, ref.ID); blocked {
				reasons = append(reasons, reason)
			}
		}
	}
	return reasons
}

func (a analyzer) membershipWarnings(project string, ticket Ticket) []string {
	var warnings []string
	var listedBy []string
	for _, ref := range a.members[ticket.ID] {
		if ref.Project == project {
			listedBy = append(listedBy, ref.ID)
		}
	}
	if ticket.Workstream != "" {
		if _, exists := a.workstreams[Ref{project, ticket.Workstream}]; exists && !slices.Contains(listedBy, ticket.Workstream) {
			warnings = append(warnings, fmt.Sprintf("not listed in workstream %s's tickets, so it does not count toward that workstream", ticket.Workstream))
		}
	}
	for _, name := range listedBy {
		if name != ticket.Workstream {
			field := "empty"
			if ticket.Workstream != "" {
				field = fmt.Sprintf("%q", ticket.Workstream)
			}
			warnings = append(warnings, fmt.Sprintf("workstream %s lists this ticket but its workstream field is %s", name, field))
		}
	}
	return warnings
}

func (a analyzer) workstreamState(project string, workstream Workstream, blocked map[Ref][]Reason) WorkstreamState {
	state := WorkstreamState{Status: StatusActive, Total: len(workstream.Tickets)}
	free := ""
	for _, id := range workstream.Tickets {
		_, satisfied, exists := a.ticketColumn(id)
		if satisfied {
			state.Done++
			continue
		}
		if free == "" && exists && len(blocked[a.ref(id)]) == 0 {
			free = id
		}
		if state.Next == "" {
			state.Next = id
			if !exists {
				state.Reasons = []Reason{{Kind: TicketDependency, Ticket: a.ref(id), Missing: true}}
			}
		}
	}
	// A workstream is an epic: it moves while any unfinished ticket can.
	if free != "" {
		state.Next, state.Reasons = free, nil
	}
	if state.Total > 0 && state.Done == state.Total {
		state.Status = StatusCompleted
		return state
	}
	var own []Reason
	for _, dependency := range workstream.DependsOnWorkstreams {
		if reason, isBlocked := a.workstreamReason(project, dependency, ""); isBlocked {
			own = append(own, reason)
		}
	}
	switch {
	case len(own) > 0:
		state.Reasons = own
	case state.Reasons == nil && state.Next != "":
		state.Reasons = blocked[a.ref(state.Next)]
	}
	if len(state.Reasons) > 0 {
		state.Status = StatusBlocked
	}
	return state
}

// Title returns the column's display name.
func (c Column) Title() string {
	switch c {
	case Backlog:
		return "Backlog"
	case UpNext:
		return "Up next"
	case InProgress:
		return "In progress"
	case Review:
		return "Ready to review"
	case Done:
		return "Done"
	}
	return string(c)
}

// Describe explains the reason in one sentence. Ticket ids are global, so
// they need no project prefix.
func (r Reason) Describe() string {
	state := func() string {
		switch {
		case r.Missing:
			return "does not exist"
		case r.Column == Backlog:
			return "is in Backlog"
		default:
			return "is " + r.Column.Title()
		}
	}
	switch r.Kind {
	case TicketDependency:
		return fmt.Sprintf("Depends on %s, which %s", r.Ticket.ID, state())
	case WorkstreamDependency:
		pending := "does not exist"
		if !r.Missing {
			noun := "tickets"
			if r.Pending == 1 {
				noun = "ticket"
			}
			pending = fmt.Sprintf("has %d %s not yet in review or done", r.Pending, noun)
		}
		switch {
		case r.Via == "":
			return fmt.Sprintf("Depends on workstream %s, which %s", r.Workstream, pending)
		case r.Via == r.Workstream && r.Missing:
			return fmt.Sprintf("Belongs to workstream %s, which does not exist", r.Workstream)
		default:
			return fmt.Sprintf("Its workstream %s depends on workstream %s, which %s", r.Via, r.Workstream, pending)
		}
	}
	return string(r.Kind)
}
