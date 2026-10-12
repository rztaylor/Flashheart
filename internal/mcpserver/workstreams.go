package mcpserver

import (
	"fmt"
	"strings"

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
	if len(input.Goal) > maxDescription {
		return "", fail("too_large", "keep the goal under 16 KiB; link to longer documents", "the goal is too long")
	}
	if len(input.Tickets) > maxItems {
		return "", fail("invalid_input", fmt.Sprintf("list at most %d tickets; add more with update_ticket", maxItems), "too many tickets")
	}
	var tickets []string
	for _, id := range input.Tickets {
		owner, ticket, err := c.target(id)
		if err != nil {
			return "", err
		}
		if owner.Name != project.Name {
			return "", fail("invalid_input", "list tickets of project "+project.Name, "%s belongs to project %s", ticket.ID, owner.Name)
		}
		tickets = append(tickets, ticket.ID)
	}
	var dependencies []string
	for _, slug := range items(input.DependsOnWorkstreams) {
		dependencies = append(dependencies, strings.TrimSpace(slug))
	}
	title := scrub.Text(input.Title, 300)
	created, err := srv.store.CreateWorkstream(project.Name, store.NewWorkstream{
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
