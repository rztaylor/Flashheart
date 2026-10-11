package hooks

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
)

// Throttled tool activity (FH-55, agent-protocol §3). Adapters report every
// tool result as a tool.used; hooks records them as activity records, at
// most one per run per interval, counting the tool uses and failures in
// between. A record is made at once for an edit of a path the run has not
// recorded since its last checkpoint (so dirty is never late) and for the
// first tool result after a permission request (which it resolves), and a
// run's pending activity is recorded before its turn end or end (a
// session's with its subagents'), so the worktree check and handoff
// enforcement see all of it. The session's state between hooks lives in
// one small file under the project lock (store.UpdateActivity).

// shellTools are the agents' shell command tools, whose edits name no path.
var shellTools = []string{"Bash", "PowerShell"}

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
	// Paths are the edit paths recorded since the run's last checkpoint,
	// oldest first.
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
	var ids []string
	for id := range s.Runs {
		if id == run || !strings.Contains(run, "/") && strings.HasPrefix(id, run+"/") {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// flush returns the run's pending activity as a record at, if it has any,
// with the newly edited path, and starts counting afresh.
func (r *runState) flush(e events.Event, at time.Time, path string) (events.Event, bool) {
	if r.Tools == 0 {
		return events.Event{}, false
	}
	data := events.ActivityData{Tools: r.Tools, Failed: r.Failed, Shell: r.Shell}
	if path != "" {
		data.Paths = []string{path}
	}
	r.Tools, r.Failed, r.Shell, r.Logged = 0, 0, nil, at
	return events.Event{Time: at, Run: e.Run, Agent: e.Agent, Kind: events.Activity, Project: e.Project, Data: data}, true
}

// addShell adds a command's window, merging it into the last one when they
// meet and the oldest two beyond the record's bound.
func (r *runState) addShell(span events.Span) {
	if last := len(r.Shell) - 1; last >= 0 && !span.From.After(r.Shell[last].To) {
		if span.To.After(r.Shell[last].To) {
			r.Shell[last].To = span.To
		}
		return
	}
	r.Shell = append(r.Shell, span)
	if len(r.Shell) > events.MaxActivitySpans {
		r.Shell[1].From = r.Shell[0].From
		r.Shell = slices.Delete(r.Shell, 0, 1)
	}
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
			if e.Kind == events.TurnEnd || e.Kind == events.RunEnd {
				out = append(out, flushRuns(state, e, at)...)
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
			if e.Kind == events.PermissionRequested {
				r.Logged = time.Time{}
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
		if data.OK && slices.Contains(shellTools, data.Tool) {
			from := r.Last
			if from.IsZero() || from.After(at) {
				from = at
			}
			r.addShell(events.Span{From: from, To: at})
		}
		r.Last = at
		path := ""
		if data.OK && data.Path != "" && !slices.Contains(r.Paths, data.Path) {
			path = data.Path
			r.remember(path)
		}
		if path != "" || r.Logged.IsZero() || at.Sub(r.Logged) >= interval {
			if record, ok := r.flush(e, at, path); ok {
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

// flushRuns records the pending activity of the runs an end settles.
func flushRuns(state *sessionState, end events.Event, at time.Time) []events.Event {
	var out []events.Event
	for _, id := range state.ending(end.Run) {
		source := end
		source.Run = id
		if record, ok := state.Runs[id].flush(source, at, ""); ok {
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
	session, _, _ := strings.Cut(list[0].Run, "/")
	var appended []events.Event
	err := log.AppendWithState(project, session, func(raw []byte) ([]byte, []events.Event, error) {
		state := decodeState(raw)
		appended = apply(state)
		return state.encode(), appended, nil
	})
	return appended, err
}
