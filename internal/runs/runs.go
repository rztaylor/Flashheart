package runs

import (
	"slices"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
)

// State is a run's derived state (RUN-3).
type State string

// Run states, in the Agents view's lane order.
const (
	Working  State = "working"
	NeedsYou State = "needs-you"
	Waiting  State = "waiting"
	Quiet    State = "quiet"
	Ended    State = "ended"
)

// Settings are the clock thresholds of the state table.
type Settings struct {
	// Quiet is how long a working run may be silent before it is Quiet.
	Quiet time.Duration
	// Stale is how long any run may be silent before it is Ended.
	Stale time.Duration
}

// DefaultSettings returns quiet_minutes 10 and stale_hours 12.
func DefaultSettings() Settings {
	return Settings{Quiet: 10 * time.Minute, Stale: 12 * time.Hour}
}

// Bounds on what a run keeps in memory.
const (
	MaxTimeline = 200
	MaxFiles    = 20
)

// Entry is one timeline event of a run, with the fields worth showing.
type Entry struct {
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"`
	Tool    string    `json:"tool,omitempty"`
	Path    string    `json:"path,omitempty"`
	Failed  bool      `json:"failed,omitempty"`
	Detail  string    `json:"detail,omitempty"`
	Ticket  string    `json:"ticket,omitempty"`
	Subject string    `json:"subject,omitempty"`
}

// Run is everything the events say about one session or subagent (RUN-2).
type Run struct {
	ID        string
	Agent     string
	Kind      string
	Parent    string
	Children  []string
	Project   string
	Cwd       string
	Branch    string
	Worktree  string
	Source    string
	AgentType string

	Started      time.Time
	LastActivity time.Time
	EndedAt      time.Time
	EndReason    string

	Plan []events.PlanItem
	// Tools counts tool uses; Edits counts successful edits since the last
	// checkpoint (dirty); Files lists edited paths, most recent first.
	Tools          int
	Edits          int
	Files          []string
	LastCheckpoint time.Time
	Claim          string
	// Permission is the tool awaiting permission, or "?" when the agent did
	// not say which; empty when nothing is pending.
	Permission string
	Timeline   []Entry

	turnOpen bool
	ended    bool
}

// Dirty reports edits since the run's last checkpoint.
func (r *Run) Dirty() bool { return r.Edits > 0 }

// Progress summarises a plan: done and total items, and the current step
// (the first in progress, else the first pending).
type Progress struct {
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Current string `json:"current,omitempty"`
}

// Progress returns the plan's progress.
func (r *Run) Progress() Progress {
	p := Progress{Total: len(r.Plan)}
	pending := ""
	for _, item := range r.Plan {
		switch item.Status {
		case events.PlanCompleted:
			p.Done++
		case events.PlanInProgress:
			if p.Current == "" {
				p.Current = item.Text
			}
		default:
			if pending == "" {
				pending = item.Text
			}
		}
	}
	if p.Current == "" {
		p.Current = pending
	}
	return p
}

// Set is the fold of one or more projects' events into runs.
type Set struct {
	runs  map[string]*Run
	order []string
}

// NewSet returns an empty set.
func NewSet() *Set { return &Set{runs: map[string]*Run{}} }

// Get returns a run by id, or nil.
func (s *Set) Get(id string) *Run { return s.runs[id] }

// Runs returns every run in first-seen order.
func (s *Set) Runs() []*Run {
	list := make([]*Run, 0, len(s.order))
	for _, id := range s.order {
		list = append(list, s.runs[id])
	}
	return list
}

func (s *Set) run(e events.Event) *Run {
	r, ok := s.runs[e.Run]
	if !ok {
		r = &Run{ID: e.Run, Agent: e.Agent, Project: e.Project, Kind: events.KindSession, Started: e.Time}
		if parent, _, isChild := strings.Cut(e.Run, "/"); isChild {
			r.Kind = events.KindSubagent
			r.Parent = parent
			r.turnOpen = true
		}
		s.runs[e.Run] = r
		s.order = append(s.order, e.Run)
		if r.Parent != "" {
			if parent, ok := s.runs[r.Parent]; ok && !slices.Contains(parent.Children, r.ID) {
				parent.Children = append(parent.Children, r.ID)
			}
		} else {
			// Children seen before their parent.
			for _, id := range s.order {
				if child := s.runs[id]; child.Parent == r.ID && !slices.Contains(r.Children, id) {
					r.Children = append(r.Children, id)
				}
			}
		}
	}
	return r
}

// Apply folds one event into the set. Unknown or malformed data is ignored.
func (s *Set) Apply(e events.Event) {
	if e.Run == "" {
		return
	}
	r := s.run(e)
	if e.Time.After(r.LastActivity) {
		r.LastActivity = e.Time
	}
	entry := Entry{Time: e.Time, Kind: e.Kind}
	switch e.Kind {
	case events.RunStart:
		var data events.RunStartData
		_ = e.Decode(&data)
		if r.ended || r.Started.After(e.Time) {
			r.Started = e.Time
		}
		r.ended, r.EndedAt, r.EndReason, r.Permission = false, time.Time{}, "", ""
		if data.Kind == events.KindSubagent {
			r.Kind, r.turnOpen = events.KindSubagent, true
		}
		if data.Parent != "" {
			r.Parent = data.Parent
		}
		r.Cwd = first(data.Cwd, r.Cwd)
		r.Branch = first(data.Branch, r.Branch)
		r.Worktree = first(data.Worktree, r.Worktree)
		r.Source = first(data.Source, r.Source)
		r.AgentType = first(data.AgentType, r.AgentType)
		entry.Detail = first(data.Source, data.AgentType)
	case events.RunEnd:
		var data events.RunEndData
		_ = e.Decode(&data)
		r.ended, r.EndedAt, r.EndReason, r.Permission = true, e.Time, data.Reason, ""
		if r.Kind == events.KindSubagent {
			r.turnOpen = false
		}
		entry.Detail = data.Reason
	case events.TurnStart:
		var data events.TurnStartData
		_ = e.Decode(&data)
		r.turnOpen, r.Permission = true, ""
		r.Cwd = first(data.Cwd, r.Cwd)
		r.Branch = first(data.Branch, r.Branch)
		r.Worktree = first(data.Worktree, r.Worktree)
	case events.TurnEnd:
		r.turnOpen, r.Permission = false, ""
	case events.ToolUsed:
		var data events.ToolData
		_ = e.Decode(&data)
		r.Tools++
		r.Permission = ""
		if data.OK && data.Path != "" {
			r.Edits++
			r.Files = slices.DeleteFunc(r.Files, func(f string) bool { return f == data.Path })
			r.Files = append([]string{data.Path}, r.Files...)
			if len(r.Files) > MaxFiles {
				r.Files = r.Files[:MaxFiles]
			}
		}
		entry.Tool, entry.Path, entry.Failed = data.Tool, data.Path, !data.OK
	case events.PlanUpdated:
		var data events.PlanData
		_ = e.Decode(&data)
		r.Plan = applyPlan(r.Plan, data)
		entry.Detail = r.Progress().Current
	case events.PermissionRequested:
		var data events.PermissionData
		_ = e.Decode(&data)
		r.Permission = first(data.Tool, r.Permission, "?")
		entry.Tool = data.Tool
	case events.PermissionResolved:
		var data events.ResolvedData
		_ = e.Decode(&data)
		r.Permission = ""
		entry.Tool, entry.Detail = data.Tool, data.Outcome
	case events.Notification:
		var data events.NotificationData
		_ = e.Decode(&data)
		entry.Detail = data.Type
	case events.Compact:
		var data events.CompactData
		_ = e.Decode(&data)
		entry.Detail = data.Phase
	case events.Checkpoint:
		var data events.CheckpointData
		_ = e.Decode(&data)
		r.Edits, r.LastCheckpoint = 0, e.Time
		entry.Ticket = data.Ticket
	case events.Claim, events.Release, events.TicketMoved, events.TicketUpdated, events.TicketCreated, events.ReviewWritten, events.AttachmentAdded:
		var data events.TicketData
		_ = e.Decode(&data)
		entry.Ticket = data.Ticket
		switch {
		case e.Kind == events.Claim:
			r.Claim = data.Ticket
		case e.Kind == events.Release && r.Claim == data.Ticket:
			r.Claim = ""
		}
	}
	r.Timeline = append(r.Timeline, entry)
	if len(r.Timeline) > MaxTimeline {
		r.Timeline = slices.Delete(r.Timeline, 0, len(r.Timeline)-MaxTimeline)
	}
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func applyPlan(plan []events.PlanItem, data events.PlanData) []events.PlanItem {
	if !data.Merge {
		plan = slices.Clone(data.Items)
	} else {
		plan = slices.Clone(plan)
		for _, item := range data.Items {
			index := slices.IndexFunc(plan, func(p events.PlanItem) bool { return item.ID != "" && p.ID == item.ID })
			switch {
			case item.Status == events.PlanDeleted:
				if index >= 0 {
					plan = slices.Delete(plan, index, index+1)
				}
			case index >= 0:
				if item.Text != "" {
					plan[index].Text = item.Text
				}
				plan[index].Status = item.Status
			default:
				plan = append(plan, item)
			}
		}
	}
	if len(plan) > events.MaxPlanItems {
		plan = plan[:events.MaxPlanItems]
	}
	return plan
}

// lastActivity is the latest event of the run or any of its subagents.
func (s *Set) lastActivity(r *Run) time.Time {
	last := r.LastActivity
	for _, id := range r.Children {
		if child := s.runs[id]; child != nil && child.LastActivity.After(last) {
			last = child.LastActivity
		}
	}
	return last
}

// State derives a run's state at now (agent-protocol §4; first match wins).
// A subagent ends with its session, and a session needs you while one of
// its live subagents does.
func (s *Set) State(id string, now time.Time, settings Settings) State {
	r := s.runs[id]
	if r == nil {
		return Ended
	}
	last := s.lastActivity(r)
	if r.ended || now.Sub(last) >= settings.Stale {
		return Ended
	}
	if parent := s.runs[r.Parent]; parent != nil && s.State(parent.ID, now, settings) == Ended {
		return Ended
	}
	if r.Permission != "" {
		return NeedsYou
	}
	for _, childID := range r.Children {
		if child := s.runs[childID]; child != nil && !child.ended && child.Permission != "" {
			return NeedsYou
		}
	}
	if !r.turnOpen {
		return Waiting
	}
	if now.Sub(last) >= settings.Quiet {
		return Quiet
	}
	return Working
}

// How a run is linked to a ticket (RUN-5).
const (
	LinkClaim  = "claim"
	LinkBranch = "branch"
)

// Link is a run's ticket and how it was linked.
type Link struct {
	Ticket string `json:"ticket"`
	By     string `json:"by"`
}

// InProgress lists a project's in-progress ticket ids whose branch matches.
type InProgress func(project, branch string) []string

// Link returns a run's ticket: its claim, else the single in-progress ticket
// on its branch; subagents inherit their session's link.
func (s *Set) Link(id string, byBranch InProgress) Link {
	r := s.runs[id]
	if r == nil {
		return Link{}
	}
	if r.Claim != "" {
		return Link{Ticket: r.Claim, By: LinkClaim}
	}
	if r.Parent != "" {
		if s.runs[r.Parent] != nil {
			return s.Link(r.Parent, byBranch)
		}
		return Link{}
	}
	if r.Branch == "" || byBranch == nil {
		return Link{}
	}
	if tickets := byBranch(r.Project, r.Branch); len(tickets) == 1 {
		return Link{Ticket: tickets[0], By: LinkBranch}
	}
	return Link{}
}

// NoHandoff reports an Ended run that is linked to a ticket and edited files
// since its last checkpoint (RUN-3).
func (s *Set) NoHandoff(id string, now time.Time, settings Settings, byBranch InProgress) bool {
	r := s.runs[id]
	return r != nil && r.Dirty() && s.State(id, now, settings) == Ended && s.Link(id, byBranch).Ticket != ""
}
