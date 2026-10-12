package mcpserver

import (
	"fmt"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/scrub"
	"github.com/rztaylor/flashheart/internal/store"
)

// ClaimInput is claim's input.
type ClaimInput struct {
	Attribution
	Ticket string `json:"ticket" jsonschema:"the ticket id, like FH-42"`
	Force  bool   `json:"force,omitempty" jsonschema:"take a ticket another live session holds, or start a blocked one; needs reason"`
	Reason string `json:"reason,omitempty" jsonschema:"why force is right; recorded in the ticket's notes"`
}

func (srv *server) claim(c *call, input ClaimInput) (string, error) {
	run := c.run // never "": the tool belongs to a run (belongsToRun)
	project, ticket, err := c.target(input.Ticket)
	if err != nil {
		return "", err
	}
	if ticket.NeedsRepair() {
		return "", fail("needs_repair", "ask the human to repair it on the board", "%s needs repair: %s", ticket.ID, strings.Join(ticket.Repair, "; "))
	}
	if ticket.Column == board.Done {
		return "", fail("invalid_input", "create a ticket for new work on it, or ask the human to reopen it", "%s is %s", ticket.ID, ticket.Status())
	}
	reason := scrub.Text(input.Reason, events.MaxReasonText)
	if input.Force && reason == "" {
		return "", fail("invalid_input", "say why in reason", "force needs a reason")
	}
	var notes []string
	holder := c.holder(ticket.ID)
	if holder != nil && !related(holder.ID, run) {
		if !input.Force {
			return "", fail("claimed", "choose another ticket, or pass force with a reason if that session is gone",
				"%s is held by %s (%s)", ticket.ID, protocol.ShortRun(holder.ID), c.state(holder))
		}
		notes = append(notes, c.note(fmt.Sprintf("Claimed from %s (%s): %s", protocol.ShortRun(holder.ID), c.state(holder), reason)))
	}
	reasons := c.blocked(project, ticket)
	startsBlocked := len(reasons) > 0 && ticket.Column != board.InProgress
	if startsBlocked {
		if !input.Force {
			return "", fail("blocked", "work on what blocks it first, or pass force with a reason to start it anyway",
				"%s is blocked: %s", ticket.ID, strings.Join(reasons, "; "))
		}
		notes = append(notes, board.BlockedStartNote(c.now, reason))
	}

	var list []events.Event
	// One explicit claim per run: claiming another releases the first, in
	// the log of the released ticket's project, where its holder is read.
	if current := c.set.Get(run); current != nil && current.Claim != "" && current.Claim != ticket.ID {
		released := events.Event{Kind: events.Release, Data: events.TicketData{Ticket: current.Claim, Reason: "claimed " + ticket.ID}}
		if err := c.record(c.projectOf(current.Claim, project.Name), released); err != nil {
			return "", err
		}
	}
	column := ticket.Column
	moves := column == board.Backlog || column == board.UpNext
	if moves {
		column = board.InProgress
	}
	setBranch := ticket.Branch == "" && c.where.Branch != ""
	if moves || setBranch || len(notes) > 0 {
		_, err := srv.store.UpdateTicket(project.Name, ticket.ID, store.Merge, func(data []byte) ([]byte, error) {
			var err error
			if moves {
				if data, err = mdfile.SetScalar(data, "status", string(column)); err != nil {
					return nil, err
				}
			}
			if setBranch {
				if data, err = mdfile.SetScalar(data, "branch", c.where.Branch); err != nil {
					return nil, err
				}
			}
			for _, note := range notes {
				if data, err = mdfile.AppendToSection(data, board.SectionNotes, note); err != nil {
					return nil, err
				}
			}
			return data, nil
		})
		if err != nil {
			return "", err
		}
	}
	list = append(list, events.Event{Kind: events.Claim, Data: events.TicketData{Ticket: ticket.ID, Force: input.Force, Reason: reason, Home: c.project}})
	if moves {
		list = append(list, events.Event{Kind: events.TicketMoved, Data: events.TicketData{Ticket: ticket.ID, From: string(ticket.Column), To: string(column), By: run}})
	}
	if err := c.record(project.Name, list...); err != nil {
		return "", err
	}

	lines := []string{fmt.Sprintf("Claimed %s %q (%s).", ticket.ID, scrub.Limit(ticket.Title, 200), column)}
	if ticket.Handoff != nil {
		lines = append(lines, handoffLines(ticket.Handoff.HandoffNote, 10, func(s string) string { return scrub.Limit(s, 300) })...)
	}
	var open []string
	for _, criterion := range ticket.Criteria {
		if !criterion.Done {
			open = append(open, criterion.Text)
		}
	}
	if len(open) > 0 {
		lines = append(lines, fmt.Sprintf("Unticked criteria (%d of %d):", len(open), len(ticket.Criteria)))
		lines = append(lines, bullets(open, 15, func(s string) string { return scrub.Limit(s, 300) })...)
	}
	lines = append(lines, "Read it in full with get_ticket. "+dataNote, fmt.Sprintf("ok ticket=%s column=%s", ticket.ID, column))
	return strings.Join(lines, "\n") + "\n", nil
}

// ReleaseInput is release's input.
type ReleaseInput struct {
	Attribution
	Ticket string `json:"ticket" jsonschema:"the ticket id"`
	Reason string `json:"reason,omitempty"`
}

func (srv *server) release(c *call, input ReleaseInput) (string, error) {
	run := c.run // never "": the tool belongs to a run (belongsToRun)
	project, ticket, err := c.target(input.Ticket)
	if err != nil {
		return "", err
	}
	if r := c.set.Get(run); r == nil || r.Claim != ticket.ID {
		return fmt.Sprintf("You do not hold %s; nothing to release.\nok ticket=%s\n", ticket.ID, ticket.ID), nil
	}
	if err := c.record(project.Name, events.Event{Kind: events.Release, Data: events.TicketData{Ticket: ticket.ID, Reason: input.Reason}}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Released %s; it stays in %s.\nok ticket=%s\n", ticket.ID, ticket.Column, ticket.ID), nil
}
