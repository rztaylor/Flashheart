package index

import (
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/runs"
)

// TicketSessions summarises the runs working for a ticket (RUN-9, FH-50):
// the run that speaks for it, its state, last activity and handoff flags,
// and the ticket's subagents by state.
type TicketSessions struct {
	// Run is the live linked run that most needs attention, else the most
	// recently active linked run; empty when no run is linked. A subagent
	// speaks for a ticket only when it claimed the ticket itself and its
	// session is not linked to it.
	Run   string
	Agent string
	// State is Run's state: Ended when no linked run is live, empty when
	// there is no run.
	State runs.State
	// LastActivity is the latest event of any run counted here.
	LastActivity time.Time
	// NoLiveSession marks an in-progress ticket that no live run works on.
	NoLiveSession bool
	// Dirty and NoHandoff are Run's flags: edits since its last checkpoint,
	// and that while Ended (RUN-3).
	Dirty, NoHandoff bool
	Subagents        SubagentCounts
}

// SubagentCounts counts a ticket's subagents by state: Done is Ended,
// NeedsYou is Needs you and Running is any other live state (subagents
// do not wait for the user).
type SubagentCounts struct {
	Done, Running, NeedsYou int
}

// Sessions summarises the runs linked to a ticket (RUN-5), each counted
// once: the sessions linked to it, their subagents that inherit the link
// (no claim of their own, or a claim on the session's ticket), and
// subagents that claimed the ticket themselves. A subagent that claimed
// another ticket counts only there. ok is false when no run is linked and
// the ticket is not in progress.
func (s *Snapshot) Sessions(ticket board.Ticket) (TicketSessions, bool) {
	var sessions map[string]bool
	for _, run := range s.Runs {
		if run.Parent == "" && run.Link.Ticket == ticket.ID {
			if sessions == nil {
				sessions = map[string]bool{}
			}
			sessions[run.ID] = true
		}
	}
	var summary TicketSessions
	lead, found := -1, false
	for i, run := range s.Runs {
		if run.Link.Ticket != ticket.ID {
			continue
		}
		subagent := run.Parent != ""
		found = true
		if run.LastActivity.After(summary.LastActivity) {
			summary.LastActivity = run.LastActivity
		}
		if subagent {
			switch run.State {
			case runs.Ended:
				summary.Subagents.Done++
			case runs.NeedsYou:
				summary.Subagents.NeedsYou++
			default:
				summary.Subagents.Running++
			}
		}
		if (!subagent || !sessions[run.Parent]) && (lead < 0 || leads(run, s.Runs[lead])) {
			lead = i
		}
	}
	if lead >= 0 {
		run := s.Runs[lead]
		summary.Run, summary.Agent, summary.State = run.ID, run.Agent, run.State
		summary.Dirty, summary.NoHandoff = run.Dirty, run.NoHandoff
	}
	summary.NoLiveSession = ticket.Column == board.InProgress && (lead < 0 || summary.State == runs.Ended)
	return summary, found || ticket.Column == board.InProgress
}

// leads reports whether run a speaks for a ticket before run b: a live run
// before an ended one, then the state that most needs attention, then the
// most recently active.
func leads(a, b runs.View) bool {
	if aLive, bLive := a.State != runs.Ended, b.State != runs.Ended; aLive != bLive {
		return aLive
	}
	if a.State != b.State {
		return statePriority[a.State] < statePriority[b.State]
	}
	return a.LastActivity.After(b.LastActivity)
}
