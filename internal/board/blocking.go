package board

import (
	"fmt"
	"slices"
	"strings"
)

// Board is every project under a root.
type Board struct {
	Projects []Project
}

// Ref names a ticket or workstream in a project.
type Ref struct {
	Project string `json:"project"`
	Slug    string `json:"slug"`
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
	// WorkstreamOrder: an earlier ticket in the workstream is not finished.
	WorkstreamOrder ReasonKind = "order"
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
	// Next is the first ticket in order that is not yet in review or done.
	Next    string
	Reasons []Reason
}

// Analysis is the derived blocking state of a board.
type Analysis struct {
	Blocked     map[Ref][]Reason
	Workstreams map[Ref]WorkstreamState
	Warnings    map[Ref][]string
}

type analyzer struct {
	columns     map[Ref][]Column
	archived    map[Ref]bool
	workstreams map[Ref]Workstream
	members     map[Ref][]string // ticket → workstreams listing it, in file order
}

// Analyze computes blocking reasons for every todo and in-progress ticket,
// derived workstream states, and membership warnings.
func Analyze(b Board) Analysis {
	a := analyzer{
		columns: map[Ref][]Column{}, archived: map[Ref]bool{},
		workstreams: map[Ref]Workstream{}, members: map[Ref][]string{},
	}
	for _, project := range b.Projects {
		for _, ticket := range project.Tickets {
			ref := Ref{project.Name, ticket.Slug}
			a.columns[ref] = append(a.columns[ref], ticket.Column)
		}
		for _, slug := range project.Archived {
			a.archived[Ref{project.Name, slug}] = true
		}
		for _, workstream := range project.Workstreams {
			a.workstreams[Ref{project.Name, workstream.Slug}] = workstream
			for _, slug := range workstream.Tickets {
				ref := a.resolve(project.Name, slug)
				if !slices.Contains(a.members[ref], workstream.Slug) {
					a.members[ref] = append(a.members[ref], workstream.Slug)
				}
			}
		}
	}

	result := Analysis{Blocked: map[Ref][]Reason{}, Workstreams: map[Ref]WorkstreamState{}, Warnings: map[Ref][]string{}}
	for _, project := range b.Projects {
		for _, ticket := range project.Tickets {
			ref := Ref{project.Name, ticket.Slug}
			if warnings := a.membershipWarnings(project.Name, ticket); len(warnings) > 0 {
				result.Warnings[ref] = append(result.Warnings[ref], warnings...)
			}
			if ticket.Column != Todo && ticket.Column != InProgress {
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

// resolve turns "slug" or "project/slug" into a Ref.
func (a analyzer) resolve(project, name string) Ref {
	if other, slug, found := strings.Cut(name, "/"); found {
		return Ref{other, slug}
	}
	return Ref{project, name}
}

// ticketColumn returns the column that keeps ref from being satisfied (the
// first unsatisfied copy), whether ref is satisfied, and whether it exists.
func (a analyzer) ticketColumn(ref Ref) (Column, bool, bool) {
	if a.archived[ref] {
		return "", true, true
	}
	columns := a.columns[ref]
	if len(columns) == 0 {
		return "", false, false
	}
	for _, column := range columns {
		if !column.Satisfies() {
			return column, false, true
		}
	}
	return columns[0], true, true
}

func (a analyzer) ticketReason(kind ReasonKind, ref Ref, workstream string) (Reason, bool) {
	column, satisfied, exists := a.ticketColumn(ref)
	if satisfied {
		return Reason{}, false
	}
	return Reason{Kind: kind, Ticket: ref, Workstream: workstream, Column: column, Missing: !exists}, true
}

// workstreamReason reports an incomplete or missing depended-on workstream.
func (a analyzer) workstreamReason(project, name, via string) (Reason, bool) {
	ref := a.resolve(project, name)
	workstream, ok := a.workstreams[ref]
	if !ok {
		return Reason{Kind: WorkstreamDependency, Workstream: name, Missing: true, Via: via}, true
	}
	pending := 0
	for _, slug := range workstream.Tickets {
		if _, satisfied, _ := a.ticketColumn(a.resolve(ref.Project, slug)); !satisfied {
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
	self := Ref{project, ticket.Slug}
	for _, name := range ticket.DependsOn {
		if reason, blocked := a.ticketReason(TicketDependency, a.resolve(project, name), ""); blocked {
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
	for _, name := range a.members[self] {
		workstream := a.workstreams[Ref{project, name}]
		for _, dependency := range workstream.DependsOnWorkstreams {
			if reason, blocked := a.workstreamReason(project, dependency, name); blocked {
				reasons = append(reasons, reason)
			}
		}
		for _, earlier := range workstream.Tickets {
			ref := a.resolve(project, earlier)
			if ref == self {
				break
			}
			if reason, blocked := a.ticketReason(WorkstreamOrder, ref, name); blocked {
				reasons = append(reasons, reason)
			}
		}
	}
	return reasons
}

func (a analyzer) membershipWarnings(project string, ticket Ticket) []string {
	var warnings []string
	listedBy := a.members[Ref{project, ticket.Slug}]
	if ticket.Workstream != "" {
		if _, exists := a.workstreams[Ref{project, ticket.Workstream}]; exists && !slices.Contains(listedBy, ticket.Workstream) {
			warnings = append(warnings, fmt.Sprintf("not listed in workstream %s's tickets, so its order does not apply", ticket.Workstream))
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
	for _, slug := range workstream.Tickets {
		ref := a.resolve(project, slug)
		_, satisfied, exists := a.ticketColumn(ref)
		if satisfied {
			state.Done++
			continue
		}
		if state.Next == "" {
			state.Next = slug
			if !exists {
				state.Reasons = []Reason{{Kind: TicketDependency, Ticket: ref, Missing: true}}
			}
		}
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
		state.Reasons = blocked[a.resolve(project, state.Next)]
	}
	if len(state.Reasons) > 0 {
		state.Status = StatusBlocked
	}
	return state
}

// Title returns the column's display name.
func (c Column) Title() string {
	switch c {
	case Todo:
		return "To do"
	case InProgress:
		return "In progress"
	case ReadyToReview:
		return "Ready to review"
	case Done:
		return "Done"
	}
	return string(c)
}

// Describe explains the reason in one sentence, naming tickets in other
// projects as project/slug relative to the project being viewed.
func (r Reason) Describe(viewing string) string {
	ticket := r.Ticket.Slug
	if r.Ticket.Project != viewing {
		ticket = r.Ticket.Project + "/" + r.Ticket.Slug
	}
	state := func() string {
		switch {
		case r.Missing:
			return "does not exist"
		case r.Column == Todo:
			return "is in To do"
		default:
			return "is " + r.Column.Title()
		}
	}
	switch r.Kind {
	case TicketDependency:
		return fmt.Sprintf("Depends on %s, which %s", ticket, state())
	case WorkstreamOrder:
		return fmt.Sprintf("Comes after %s in workstream %s, which %s", ticket, r.Workstream, state())
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
