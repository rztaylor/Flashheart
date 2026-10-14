package hooks

import (
	"github.com/rztaylor/flashheart/internal/events"
)

// The claim reminder (D50, agent-protocol §9). A session whose turn end
// finds it changed files since its last checkpoint (its subagents' edits
// and the worktree check's shell changes included, as for handoff
// enforcement) while it holds no ticket, by claim or by branch, is asked
// once to claim one (protocol.ClaimReminder). Where the project enforces
// handoffs and the stop can be held, the reminder is its reason;
// otherwise it is due, and goes as context with the session's next event
// whose output the agent shows its model (Input.Notes), never a
// subagent's. The session's state file remembers it (sessionState.Claim),
// so no event records it and the session is never asked twice.

// The session's claim reminder states.
const (
	claimDue   = "due"
	claimAsked = "asked"
)

// verdict is what settle decided at a session's turn end or end.
type verdict struct {
	// block is the reason to hold the stop: a checkpoint (HOOK-6) or the
	// claim reminder.
	block string
	// remind is the session owing the claim reminder: given as block when
	// the stop is held, due otherwise.
	remind bool
}

// remindToClaim updates the session's claim reminder for one hook and
// reports whether this hook's output carries it as context: a turn end
// that owes it marks it asked when it held the stop with it and due
// otherwise; a due reminder goes with the next event of the session whose
// output reaches its model (notes) and is then asked.
func remindToClaim(state *sessionState, v verdict, notes bool, run string) bool {
	switch {
	case v.remind && v.block != "":
		state.Claim = claimAsked
	case v.remind:
		state.Claim = claimDue
	case notes && state.Claim == claimDue && !events.RunOf(run).IsSubagent():
		state.Claim = claimAsked
		return true
	}
	return false
}
