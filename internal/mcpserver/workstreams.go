package mcpserver

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/scrub"
	"github.com/rztaylor/flashheart/internal/store"
)

// CreateWorkstreamInput is create_workstream's input.
type CreateWorkstreamInput struct {
	Attribution
	Title                string   `json:"title" jsonschema:"what the line of work is called; its slug is made from it"`
	Goal                 string   `json:"goal" jsonschema:"the shared outcome the tickets deliver"`
	Priority             string   `json:"priority,omitempty" jsonschema:"high, medium (default) or low"`
	Tickets              []string `json:"tickets,omitempty" jsonschema:"ids of your project's tickets; order is for display only, depends_on orders the work"`
	DependsOnWorkstreams []string `json:"depends_on_workstreams,omitempty" jsonschema:"slugs of workstreams that must finish first"`
	Tags                 []string `json:"tags,omitempty"`
}

func (srv *server) createWorkstream(c *call, input CreateWorkstreamInput) (string, error) {
	project, err := c.projectArg("")
	if err != nil {
		return "", err
	}
	if len(input.Goal) > board.MaxSectionText {
		return "", fail("too_large", fmt.Sprintf("keep the goal under %d KiB; link to longer documents", board.MaxSectionText>>10), "the goal is too long")
	}
	if len(input.Tickets) > maxItems {
		return "", fail("invalid_input", fmt.Sprintf("list at most %d tickets; add more with update_ticket", maxItems), "too many tickets")
	}
	tickets, err := c.ticketsOf(project, input.Tickets)
	if err != nil {
		return "", err
	}
	var dependencies []string
	for _, slug := range items(input.DependsOnWorkstreams) {
		dependencies = append(dependencies, strings.TrimSpace(slug))
	}
	title := scrub.Text(input.Title, board.MaxTitle)
	created, err := c.files().CreateWorkstream(project.Name, store.NewWorkstream{
		Title: title, Goal: scrub.Secrets(input.Goal), Priority: strings.TrimSpace(input.Priority),
		Tickets: tickets, DependsOnWorkstreams: dependencies, Tags: items(input.Tags),
	})
	if err != nil {
		return "", err
	}
	var list []events.Event
	for _, id := range tickets {
		list = append(list, events.Event{Kind: events.TicketUpdated, Data: events.TicketData{Ticket: id, Fields: []string{"workstream"}}})
	}
	if err := c.record(project.Name, list...); err != nil {
		return "", err
	}
	text := fmt.Sprintf("Created workstream %s %q", created.Slug, scrub.Limit(title, 200))
	if len(tickets) > 0 {
		text += " with " + strings.Join(tickets, ", ")
	}
	text += fmt.Sprintf(". Add tickets with create_ticket or update_ticket workstream=%s; they can be worked on in any order, so give a ticket depends_on when it needs others first.", created.Slug)
	return text + fmt.Sprintf("\nok workstream=%s\n", created.Slug), nil
}

// ticketsOf finds the tickets a workstream tool moves, each a live ticket
// of project that no other session holds (target), and returns their ids.
func (c *call) ticketsOf(project *board.Project, ids []string) ([]string, error) {
	var out []string
	for _, id := range ids {
		owner, ticket, err := c.target(id)
		if err != nil {
			return nil, err
		}
		if owner.Name != project.Name {
			return nil, fail("invalid_input", "list tickets of project "+project.Name, "%s belongs to project %s", ticket.ID, owner.Name)
		}
		out = append(out, ticket.ID)
	}
	return out, nil
}

// UpdateWorkstreamInput is update_workstream's input.
type UpdateWorkstreamInput struct {
	Attribution
	Workstream    string         `json:"workstream" jsonschema:"the workstream's slug, as board_context lists it"`
	Project       string         `json:"project,omitempty" jsonschema:"a project name or key; default: your project"`
	Set           map[string]any `json:"set,omitempty" jsonschema:"fields to set: title, goal, priority (strings); tags, depends-on-workstreams (lists of slugs). The slug stays when the title changes; rename it with rename_to."`
	AddTickets    []string       `json:"add_tickets,omitempty" jsonschema:"ids of the project's tickets to join it, appended in order; each leaves any other workstream"`
	RemoveTickets []string       `json:"remove_tickets,omitempty" jsonschema:"ids of its tickets to leave it; they then belong to no workstream"`
	RenameTo      string         `json:"rename_to,omitempty" jsonschema:"a new slug (lower-case letters, digits, '.', '_', '-'): renames its file and rewrites every ticket and workstream that names it"`
}

func (srv *server) updateWorkstream(c *call, input UpdateWorkstreamInput) (string, error) {
	project, err := c.projectArg(input.Project)
	if err != nil {
		return "", err
	}
	slug := strings.TrimSpace(input.Workstream)
	rename := strings.TrimSpace(input.RenameTo)
	if rename == slug {
		rename = ""
	}
	// Decoding and scrubbing agent text is the tool's; which fields change
	// and how they are checked and written is board's and store's
	// (board.NewWorkstreamChanges, store.EditWorkstream; FH-46, FH-158).
	var requested []board.FieldChange
	for key, raw := range input.Set {
		change := board.FieldChange{Key: key}
		list, known := board.WorkstreamField(key)
		switch {
		case !known:
			// board refuses it, naming the fields it sets.
		case list:
			values, ok := stringList(raw)
			if !ok {
				return "", fail("invalid_input", "pass a list of strings", "%s must be a list of strings", key)
			}
			change.List = items(values)
		case key == "goal":
			value, ok := raw.(string)
			if !ok {
				return "", fail("invalid_input", "pass a string", "goal must be a string")
			}
			change.Value = scrub.Secrets(value)
		default:
			value, ok := raw.(string)
			if !ok {
				return "", fail("invalid_input", "pass a string", "%s must be a string", key)
			}
			change.Value = scrub.Text(value, board.MaxTitle)
		}
		requested = append(requested, change)
	}
	changes, err := board.NewWorkstreamChanges(requested)
	if errors.Is(err, board.ErrNotEditable) {
		return "", fail("invalid_input", "set only "+strings.Join(board.WorkstreamFields, ", ")+"; rename the slug with rename_to, and change members with add_tickets and remove_tickets", "%v", err)
	}
	if err != nil {
		return "", fail("invalid_input", "correct the value", "%v", err)
	}
	if len(input.AddTickets)+len(input.RemoveTickets) > maxItems {
		return "", fail("invalid_input", fmt.Sprintf("add and remove at most %d tickets in one call", maxItems), "too many tickets")
	}
	add, err := c.ticketsOf(project, input.AddTickets)
	if err != nil {
		return "", err
	}
	remove, err := c.ticketsOf(project, input.RemoveTickets)
	if err != nil {
		return "", err
	}
	if changes.Len() == 0 && len(add) == 0 && len(remove) == 0 && rename == "" {
		return "", fail("invalid_input", "pass set, add_tickets, remove_tickets or rename_to", "nothing to change")
	}
	_, base, err := srv.store.ReadWorkstream(project.Name, slug)
	if errors.Is(err, store.ErrNotFound) {
		var available []string
		for _, workstream := range project.Workstreams {
			available = append(available, workstream.Slug)
		}
		return "", &store.WorkstreamNotFoundError{Project: project.Name, Slug: slug, Available: available}
	}
	if err != nil {
		return "", err
	}
	// The fields are set over the workstream as it was read; a change made
	// in between is a conflict, not lost (STO-3). A rename rewrites the
	// references in tickets other sessions may hold: it changes only which
	// slug they name, so it is not refused as claimed (D71).
	edited, err := c.files().EditWorkstream(project.Name, slug, store.Unchanged(base), store.WorkstreamEdit{Changes: changes, Add: add, Remove: remove, Rename: rename})
	var cycle *board.CycleError
	switch {
	case errors.As(err, &cycle):
		return "", fail("invalid_input", "leave out the dependency that leads back, or remove it from the workstream that has it", "%v", cycle)
	case errors.Is(err, store.ErrConflict):
		return "", fail("conflict", "call update_workstream again", "workstream %s changed while it was being edited", slug)
	case errors.Is(err, store.ErrExists):
		return "", fail("invalid_input", "choose another slug for rename_to", "workstream %s already exists in %s", rename, project.Name)
	case err != nil:
		return "", err
	}
	var list []events.Event
	for _, id := range slices.Concat(add, remove) {
		list = append(list, events.Event{Kind: events.TicketUpdated, Data: events.TicketData{Ticket: id, Fields: []string{"workstream"}}})
	}
	var rewrote []string
	for _, ticket := range edited.Rewritten {
		rewrote = append(rewrote, ticket.ID)
		if slices.Contains(add, ticket.ID) {
			continue
		}
		list = append(list, events.Event{Kind: events.TicketUpdated, Data: events.TicketData{Ticket: ticket.ID, Fields: ticket.Fields}})
	}
	if err := c.record(project.Name, list...); err != nil {
		return "", err
	}
	fields := changes.Keys()
	if rename != "" {
		fields = append([]string{"slug"}, fields...)
	}
	if len(add)+len(remove) > 0 {
		fields = append(fields, "tickets")
	}
	title := edited.Slug
	if data, _, err := srv.store.ReadWorkstream(project.Name, edited.Slug); err == nil {
		title = board.ParseWorkstream(edited.Slug, data).Title
	}
	text := fmt.Sprintf("Updated workstream %s %q", edited.Slug, scrub.Limit(title, 200))
	var parts []string
	if rename != "" {
		part := "renamed from " + slug
		if len(rewrote) > 0 {
			part += ", rewrote " + strings.Join(rewrote, ", ")
		}
		parts = append(parts, part)
	}
	if len(changes.Keys()) > 0 {
		parts = append(parts, strings.Join(changes.Keys(), ", "))
	}
	if len(parts) > 0 {
		text += ": " + strings.Join(parts, "; ")
	}
	if len(add) > 0 {
		text += "; added " + strings.Join(add, ", ")
	}
	if len(remove) > 0 {
		text += "; removed " + strings.Join(remove, ", ")
	}
	return text + fmt.Sprintf(".\nok workstream=%s changed=%s\n", edited.Slug, strings.Join(fields, ",")), nil
}
