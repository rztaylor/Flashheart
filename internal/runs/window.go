package runs

import (
	"time"

	"github.com/rztaylor/flashheart/internal/events"
)

// Lookback is the run window: how far back every reader folds a project's
// event log into runs, the board (index), the MCP server, `flashheart
// await` and the hooks alike. Each reads the whole daily files from the
// one holding now minus Lookback on (events.Log.Read), so they all see the
// same runs and claims. Two days: any live run (stale after 12 hours), a
// day of ended runs (VIEW-3) and a claim made the day before.
const Lookback = 48 * time.Hour

// Keep selects the events a fold needs to derive the given sessions
// exactly as a fold of every event would: their own and their subagents'
// events, and every claim and release, since a claim takes its ticket from
// whichever run held it. Folded with only these, the sessions and their
// subagents come out the same (state, link, edits, shell windows,
// questions, timeline) while every other run is partial; a hook that asks
// about one or two sessions reads the log without decoding the rest
// (events.Log.ReadSelected). It relies on a subagent's parent being the
// session its id names (agent-protocol §2), as every adapter records it.
func Keep(sessions ...string) func(events.Header) bool {
	wanted := map[string]bool{}
	for _, session := range sessions {
		if session != "" {
			wanted[session] = true
		}
	}
	return func(h events.Header) bool {
		return h.Kind == events.Claim || h.Kind == events.Release || wanted[events.SessionOf(h.Run)]
	}
}

// Latest finds the session that was active last in a worktree, as a fold
// of every event finds it, from little more than the events' headers. A
// hook reads the log once with it to learn which earlier session to fold
// in full (Keep). Give it every header in fold order (Note), then, newest
// first, the events it wants (Wants, Apply): each session's run.start
// events (a session can say it is a subagent) and its latest run.start or
// turn.start that names a worktree, the events that say where a run works,
// which are all it decodes.
type Latest struct {
	runs  map[string]*placed
	order []string
}

// placed is what Latest keeps of a run: what the fold's Kind, Worktree and
// LastActivity would be.
type placed struct {
	subagent bool
	worktree string
	last     time.Time
}

// NewLatest returns an empty Latest.
func NewLatest() *Latest { return &Latest{runs: map[string]*placed{}} }

// Note takes an event's time as its run's activity, skipping what the
// fold skips.
func (l *Latest) Note(h events.Header) {
	if h.Run == "" || h.Run == events.Human {
		return
	}
	r, known := l.runs[h.Run]
	if !known {
		subagent := events.RunOf(h.Run).IsSubagent()
		// The fold makes no run of a subagent first seen ending.
		if h.Kind == events.RunEnd && subagent {
			return
		}
		r = &placed{subagent: subagent, last: h.Time}
		l.runs[h.Run] = r
		l.order = append(l.order, h.Run)
	}
	if h.Time.After(r.last) {
		r.last = h.Time
	}
}

// Wants reports, newest first, whether Apply needs an event: a session's
// run.start, or its turn.start while no later event named its worktree.
func (l *Latest) Wants(h events.Header) bool {
	r := l.runs[h.Run]
	if r == nil || r.subagent {
		return false
	}
	return h.Kind == events.RunStart || h.Kind == events.TurnStart && r.worktree == ""
}

// Apply reads, newest first, where a session works from the events Wants
// asked for: the latest worktree named, as the fold keeps it, and whether
// it started as a subagent.
func (l *Latest) Apply(e events.Event) {
	r := l.runs[e.Run]
	if r == nil {
		return
	}
	worktree := ""
	switch e.Kind {
	case events.RunStart:
		var data events.RunStartData
		_ = e.Decode(&data)
		if data.Kind == events.KindSubagent {
			r.subagent = true
		}
		worktree = data.Worktree
	case events.TurnStart:
		var data events.TurnStartData
		_ = e.Decode(&data)
		worktree = data.Worktree
	}
	r.worktree = first(r.worktree, worktree)
}

// Session returns the session other than except that works in worktree
// and was active last (the first seen of a tie), or "" when there is none:
// the previous run of the recovery note (HOOK-3).
func (l *Latest) Session(worktree, except string) string {
	found, last := "", time.Time{}
	for _, id := range l.order {
		r := l.runs[id]
		if id == except || r.subagent || r.worktree == "" || r.worktree != worktree {
			continue
		}
		if found == "" || r.last.After(last) {
			found, last = id, r.last
		}
	}
	return found
}
