package hooks

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
)

// Throttled tool activity (FH-55, agent-protocol §3). Adapters report every
// tool result as a tool.used; hooks records them as activity records, at
// most one per run per interval, counting the tool uses and failures in
// between. A record is made at once for an edit of a path the run has not
// recorded in this turn or since its last checkpoint (so dirty is never
// late) and for the
// first tool result after a permission request (which it resolves), and a
// run's pending activity is recorded before its turn end or end (a
// session's with its subagents'), so the worktree check and handoff
// enforcement see all of it. The session's state between hooks lives in
// one small file under the project lock (store.UpdateActivity).

// Bounds on the state, which is read and written on every tool result.
const (
	maxStateRuns  = 64
	maxStatePaths = 200
)

// sessionState is a session's activity state, keyed by run id (the
// session's own and its subagents').
type sessionState struct {
	Runs map[string]*runState `json:"runs"`
}

// runState is one run's activity not yet recorded.
type runState struct {
	// Last is the run's latest hook event, where its next shell command's
	// window starts.
	Last time.Time `json:"last,omitzero"`
	// Logged is when an event of the run was last recorded; zero records its
	// next tool result at once.
	Logged time.Time     `json:"logged,omitzero"`
	Tools  int           `json:"tools,omitempty"`
	Failed int           `json:"failed,omitempty"`
	Shell  []events.Span `json:"shell,omitempty"`
	// Paths are the edit paths recorded in the run's current turn and since
	// its last checkpoint, oldest first.
	Paths []string `json:"paths,omitempty"`
}

// decodeState reads a session's state; a missing or unreadable one starts
// afresh, since it only saves events and a lost count is no harm.
func decodeState(data []byte) *sessionState {
	state := &sessionState{}
	if len(data) > 0 && json.Unmarshal(data, state) != nil {
		state = &sessionState{}
	}
	if state.Runs == nil {
		state.Runs = map[string]*runState{}
	}
	for id, r := range state.Runs {
		if r == nil {
			delete(state.Runs, id)
		}
	}
	return state
}

// encode returns the state's file content, or nil when no run is left.
func (s *sessionState) encode() []byte {
	if len(s.Runs) == 0 {
		return nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	return data
}

func (s *sessionState) run(id string) *runState {
	r := s.Runs[id]
	if r == nil {
		r = &runState{}
		s.Runs[id] = r
	}
	return r
}

// expire forgets the runs whose latest hook event is stale_hours before at
// (FH-82): the stale rule has ended them (agent-protocol §4), so what they
// left pending is dropped rather than recorded now, which would date it
// wrongly and reopen them. A session that never reports its end keeps
// nothing for longer, and retention removes its file (store.PruneActivity).
func (s *sessionState) expire(at time.Time) {
	stale := runs.DefaultSettings().Stale
	for id, r := range s.Runs {
		if at.Sub(r.Last) >= stale {
			delete(s.Runs, id)
		}
	}
}

// bound keeps the most recently active runs.
func (s *sessionState) bound() {
	for len(s.Runs) > maxStateRuns {
		oldest := ""
		for id, r := range s.Runs {
			if oldest == "" || r.Last.Before(s.Runs[oldest].Last) {
				oldest = id
			}
		}
		delete(s.Runs, oldest)
	}
}

// ending lists the runs an end of run settles, in id order: a session's
// turn end or end covers the session and its subagents, a subagent's end
// only itself.
func (s *sessionState) ending(run string) []string {
	ended := events.RunOf(run)
	var ids []string
	for id := range s.Runs {
		if id == run || !ended.IsSubagent() && events.RunOf(id).Session() == ended {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// flush returns the run's pending activity as a record at, if it has any,
// with the newly edited paths, and starts counting afresh.
func (r *runState) flush(e events.Event, at time.Time, paths []string) (events.Event, bool) {
	if r.Tools == 0 {
		return events.Event{}, false
	}
	data := events.ActivityData{Tools: r.Tools, Failed: r.Failed, Shell: r.Shell, Paths: paths}
	r.Tools, r.Failed, r.Shell, r.Logged = 0, 0, nil, at
	return events.Event{Time: at, Run: e.Run, Agent: e.Agent, Kind: events.Activity, Project: e.Project, Data: data}, true
}

// remember records an edit path as recorded since the last checkpoint.
func (r *runState) remember(path string) {
	r.Paths = append(r.Paths, path)
	if len(r.Paths) > maxStatePaths {
		r.Paths = slices.Delete(r.Paths, 0, len(r.Paths)-maxStatePaths)
	}
}

// throttle turns a hook's events into those to record, at, updating the
// session's state: tool.used becomes activity records as described above;
// every other event is recorded, after the pending activity of the runs it
// ends. settled forgets the recorded paths of every run of the session,
// because one of them recorded a checkpoint, which settles them (§10;
// recording a path again is harmless).
func throttle(state *sessionState, list []events.Event, at time.Time, interval time.Duration, settled bool) []events.Event {
	var out []events.Event
	for _, e := range list {
		if e.Kind != events.ToolUsed {
			switch {
			case e.Kind == events.TurnEnd || e.Kind == events.RunEnd:
				out = append(out, flushRuns(state, e, at)...)
			case e.Kind == events.RunStart && state.Runs[e.Run] != nil:
				// A run starting again after a crash (it reported no end)
				// records what its last life left pending first (FH-82).
				if record, ok := state.Runs[e.Run].flush(e, at, nil); ok {
					out = append(out, record)
				}
			}
			out = append(out, e)
			if e.Kind == events.RunEnd {
				for _, id := range state.ending(e.Run) {
					delete(state.Runs, id)
				}
				continue
			}
			r := state.run(e.Run)
			r.Last, r.Logged = at, at
			switch {
			case e.Kind == events.PermissionRequested:
				r.Logged = time.Time{}
			case e.Kind == events.RunStart, e.Kind == events.TurnStart && fromUser(e):
				// The user's prompt (a subagent: its start) begins a turn
				// whose paths are recorded afresh, so a lost checkpoint
				// signal costs at most one turn.
				r.Paths = nil
			}
			continue
		}
		data, ok := e.Data.(events.ToolData)
		if !ok {
			_ = e.Decode(&data)
		}
		r := state.run(e.Run)
		r.Tools++
		if !data.OK {
			r.Failed++
		}
		// The adapter says which results are shell commands; a command's
		// window runs from the run's previous event to its result.
		if data.OK && data.Shell {
			from := r.Last
			if from.IsZero() || from.After(at) {
				from = at
			}
			r.Shell = events.AddSpan(r.Shell, events.Span{From: from, To: at}, events.MaxActivitySpans)
		}
		r.Last = at
		var paths []string
		if data.OK {
			for _, path := range data.EditPaths() {
				if path != "" && !slices.Contains(r.Paths, path) {
					paths = append(paths, path)
					r.remember(path)
				}
			}
		}
		if len(paths) > 0 || r.Logged.IsZero() || at.Sub(r.Logged) >= interval {
			if record, ok := r.flush(e, at, paths); ok {
				out = append(out, record)
			}
		}
	}
	if settled {
		for _, r := range state.Runs {
			r.Paths = nil
		}
	}
	state.bound()
	return out
}

// fromUser reports a turn the user's prompt started (FH-57).
func fromUser(e events.Event) bool {
	data, ok := e.Data.(events.TurnStartData)
	if !ok {
		_ = e.Decode(&data)
	}
	return data.FromUser()
}

// flushRuns records the pending activity of the runs an end settles.
func flushRuns(state *sessionState, end events.Event, at time.Time) []events.Event {
	var out []events.Event
	for _, id := range state.ending(end.Run) {
		source := end
		source.Run = id
		if record, ok := state.Runs[id].flush(source, at, nil); ok {
			out = append(out, record)
		}
	}
	return out
}

// record appends what apply returns together with the session's activity
// state, which apply updates, under the project lock, and returns what it
// appended. An adapter reports one session per payload; list names it.
func record(log *events.Log, project string, list []events.Event, apply func(*sessionState) []events.Event) ([]events.Event, error) {
	if len(list) == 0 {
		return nil, nil
	}
	session := events.RunOf(list[0].Run).Session().String()
	var appended []events.Event
	err := log.AppendWithState(project, session, func(raw []byte) ([]byte, []events.Event, error) {
		state := decodeState(raw)
		state.expire(list[0].Time)
		appended = apply(state)
		return state.encode(), appended, nil
	})
	return appended, err
}
