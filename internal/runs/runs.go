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
	// Lease is how long a claim outlives its run's last activity (§6).
	Lease time.Duration
}

// DefaultSettings returns quiet_minutes 10, stale_hours 12 and
// lease_minutes 30.
func DefaultSettings() Settings {
	return Settings{Quiet: 10 * time.Minute, Stale: 12 * time.Hour, Lease: 30 * time.Minute}
}

// SettingsFor returns the default settings with config.yaml's
// quiet_minutes and lease_minutes (values below 1 keep the default).
func SettingsFor(quietMinutes, leaseMinutes int) Settings {
	settings := DefaultSettings()
	if quietMinutes > 0 {
		settings.Quiet = time.Duration(quietMinutes) * time.Minute
	}
	if leaseMinutes > 0 {
		settings.Lease = time.Duration(leaseMinutes) * time.Minute
	}
	return settings
}

// QuestionAnsweredInSession is the timeline kind marking a question the
// user answered in the session's own chat (derived from turn.start, not an
// event of its own).
const QuestionAnsweredInSession = "question.answered-in-session"

// Bounds on what a run keeps in memory.
const (
	MaxTimeline  = 200
	MaxFiles     = 20
	MaxQuestions = 20
)

// Entry is one timeline event of a run, with the fields worth showing.
type Entry struct {
	Time   time.Time `json:"time"`
	Kind   string    `json:"kind"`
	Tool   string    `json:"tool,omitempty"`
	Path   string    `json:"path,omitempty"`
	Failed bool      `json:"failed,omitempty"`
	Detail string    `json:"detail,omitempty"`
	Ticket string    `json:"ticket,omitempty"`
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
	// checkpoint (dirty), where a turn end or end that found the worktree
	// changed by other means, such as shell commands, counts as one;
	// Files lists edited paths, most recent first.
	Tools          int
	Edits          int
	Files          []string
	LastCheckpoint time.Time
	Claim          string
	// Home is the project the run works in, as its last claim recorded it.
	Home string
	// Permission is the tool awaiting permission, or "?" when the agent did
	// not say which; empty when nothing is pending.
	Permission string
	// Questions are the run's own questions, oldest first.
	Questions []Question
	// BlockedForHandoff is set when handoff enforcement blocked a stop in
	// the current turn (HOOK-6).
	BlockedForHandoff bool
	Timeline          []Entry

	turnOpen bool
	ended    bool
	// windows are the run's successful shell commands since the last
	// checkpoint, oldest first, merged where they overlap.
	windows []Window
}

// shellTools are the agents' shell command tools, whose edits name no path.
var shellTools = []string{"Bash", "PowerShell"}

// Shell command windows (agent-protocol §4).
const (
	// ShellSlack widens each window for clock and flush granularity.
	ShellSlack = 2 * time.Second
	// MaxWindows bounds a run's windows; the oldest two merge beyond it.
	MaxWindows = 100
)

// Window is when a shell command may have changed files: from the run's
// previous event to the command's tool.used, widened by ShellSlack.
type Window struct {
	From, To time.Time
}

// Contains reports whether t falls inside the window, ends included.
func (w Window) Contains(t time.Time) bool { return !t.Before(w.From) && !t.After(w.To) }

// addWindow appends w, merging it into the last window when they overlap
// and the oldest two when there are too many.
func (r *Run) addWindow(w Window) {
	if last := len(r.windows) - 1; last >= 0 && !w.From.After(r.windows[last].To) {
		if w.To.After(r.windows[last].To) {
			r.windows[last].To = w.To
		}
		return
	}
	r.windows = append(r.windows, w)
	if len(r.windows) > MaxWindows {
		r.windows[1].From = r.windows[0].From
		r.windows = slices.Delete(r.windows, 0, 1)
	}
}

// Question is one ask_human question and what became of it (RUN-8).
type Question struct {
	ID      string    `json:"id"`
	Run     string    `json:"run"`
	Ticket  string    `json:"ticket,omitempty"`
	Kind    string    `json:"kind"`
	Text    string    `json:"text"`
	Options []string  `json:"options,omitempty"`
	Asked   time.Time `json:"asked"`
	// Answer is set once a human answered on the board; Delivered once the
	// run was told.
	Answer     string    `json:"answer,omitempty"`
	AnsweredBy string    `json:"answeredBy,omitempty"`
	AnsweredAt time.Time `json:"answeredAt,omitzero"`
	Delivered  bool      `json:"delivered,omitempty"`
	// AnsweredInSession is set when the user prompted the asking session
	// before answering on the board: the answer went into the session's own
	// chat, so the question is delivered and no longer waits (§4).
	AnsweredInSession bool `json:"answeredInSession,omitempty"`
	// SessionEnded is set by readers when the asking run has ended: the
	// question can still be answered, and the answer waits for the session
	// to resume (RUN-8), but it does not need you (VIEW-2).
	SessionEnded bool `json:"sessionEnded,omitempty"`
}

// Answered reports whether a human answered the question.
func (q Question) Answered() bool { return !q.AnsweredAt.IsZero() }

// Open reports whether the question still needs the human: unanswered, or
// answered but not yet delivered to the run (agent-protocol §4).
func (q Question) Open() bool { return !q.Delivered }

// Dirty reports the run's own edits since its last checkpoint; Set.Dirty
// counts a session's subagents too.
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
	// A subagent first seen ending did nothing visible (Claude Code's
	// desktop app stops internal helper agents it never reported starting),
	// so it is not a run.
	if _, known := s.runs[e.Run]; !known && e.Kind == events.RunEnd && strings.Contains(e.Run, "/") {
		return
	}
	r := s.run(e)
	// previous is the run's last event before this one, where a shell
	// command's window starts.
	previous := r.LastActivity
	if previous.IsZero() || previous.After(e.Time) {
		previous = e.Time
	}
	if e.Time.After(r.LastActivity) {
		r.LastActivity = e.Time
	}
	entry := Entry{Time: e.Time, Kind: e.Kind}
	// answers is set by a prompt from the user, which answers the session's
	// open questions after the prompt's own timeline entry.
	answers := false
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
		// A session that ends mid-turn (interrupted, so no Stop) is not still
		// working when it resumes.
		r.ended, r.EndedAt, r.EndReason, r.Permission, r.turnOpen = true, e.Time, data.Reason, "", false
		if data.WorktreeChanged {
			r.Edits++
		}
		s.settleChildren(r)
		entry.Detail = data.Reason
	case events.TurnStart:
		var data events.TurnStartData
		_ = e.Decode(&data)
		r.turnOpen, r.Permission, r.BlockedForHandoff = true, "", false
		s.settleChildren(r)
		r.Cwd = first(data.Cwd, r.Cwd)
		r.Branch = first(data.Branch, r.Branch)
		r.Worktree = first(data.Worktree, r.Worktree)
		if data.Background {
			entry.Detail = "background"
		}
		answers = !data.Background
	case events.TurnEnd:
		var data events.TurnEndData
		_ = e.Decode(&data)
		r.turnOpen, r.Permission = false, ""
		r.BlockedForHandoff = r.BlockedForHandoff || data.BlockedForHandoff
		if data.WorktreeChanged {
			r.Edits++
		}
		s.settleChildren(r)
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
		if data.OK && slices.Contains(shellTools, data.Tool) {
			r.addWindow(Window{From: previous.Add(-ShellSlack), To: e.Time.Add(ShellSlack)})
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
		// A session owns its ticket's handoff: a checkpoint by it or any of
		// its subagents settles the edits of all of them (HOOK-6, §10). A
		// subagent's checkpoint on a ticket of its own settles only its own.
		members := s.family(r)
		if root := members[0]; r != root && root.Claim != "" && data.Ticket != root.Claim {
			members = []*Run{r}
		}
		for _, member := range members {
			member.Edits, member.windows, member.LastCheckpoint = 0, nil, e.Time
		}
		entry.Ticket = data.Ticket
	case events.Claim, events.Release, events.TicketMoved, events.TicketUpdated, events.TicketCreated, events.ReviewWritten, events.AttachmentAdded:
		var data events.TicketData
		_ = e.Decode(&data)
		entry.Ticket = data.Ticket
		switch {
		case e.Kind == events.Claim && r.Parent != "" && s.runs[r.Parent] != nil && s.runs[r.Parent].Claim == data.Ticket:
			// A subagent works under its session's claim; the session keeps
			// holding the ticket.
		case e.Kind == events.Claim:
			r.Claim = data.Ticket
			if data.Home != "" {
				r.Home = data.Home
			}
			// A ticket has one holder: a claim takes it from any other run.
			for _, id := range s.order {
				if other := s.runs[id]; other != r && other.Claim == data.Ticket {
					other.Claim = ""
				}
			}
		case e.Kind == events.Release && r.Claim == data.Ticket:
			r.Claim = ""
		}
	case events.QuestionAsked:
		var data events.QuestionData
		_ = e.Decode(&data)
		entry.Ticket, entry.Detail = data.Ticket, data.Kind
		if data.ID != "" && slices.Contains(events.QuestionKinds, data.Kind) && r.question(data.ID) == nil {
			r.Questions = append(r.Questions, Question{ID: data.ID, Run: r.ID, Ticket: data.Ticket, Kind: data.Kind, Text: data.Text, Options: data.Options, Asked: e.Time})
			r.boundQuestions()
		}
	case events.QuestionAnswered:
		var data events.AnswerData
		_ = e.Decode(&data)
		if q := r.question(data.ID); q != nil {
			q.Answer, q.AnsweredBy, q.AnsweredAt = data.Answer, data.By, e.Time
			entry.Ticket = q.Ticket
		}
	case events.QuestionDelivered:
		var data events.DeliveredData
		_ = e.Decode(&data)
		if q := r.question(data.ID); q != nil {
			q.Delivered = true
			entry.Ticket = q.Ticket
		}
	}
	r.Timeline = append(r.Timeline, entry)
	if answers {
		s.answerInSession(r, e.Time)
	}
	if len(r.Timeline) > MaxTimeline {
		r.Timeline = slices.Delete(r.Timeline, 0, len(r.Timeline)-MaxTimeline)
	}
}

func (r *Run) question(id string) *Question {
	for index := range r.Questions {
		if r.Questions[index].ID == id {
			return &r.Questions[index]
		}
	}
	return nil
}

// boundQuestions keeps MaxQuestions, dropping delivered ones first.
func (r *Run) boundQuestions() {
	for len(r.Questions) > MaxQuestions {
		index := slices.IndexFunc(r.Questions, func(q Question) bool { return q.Delivered })
		if index < 0 {
			index = 0
		}
		r.Questions = slices.Delete(r.Questions, index, index+1)
	}
}

// answerInSession settles the unanswered questions of a session and its
// subagents when the user prompts the session: whatever the prompt says,
// the user is talking to the session, so the question no longer waits on
// the board (agent-protocol §4). A question already answered on the board
// keeps waiting for its delivery, which the same prompt's hook makes.
func (s *Set) answerInSession(r *Run, at time.Time) {
	for _, run := range append([]*Run{r}, s.children(r)...) {
		for index := range run.Questions {
			q := &run.Questions[index]
			if q.Delivered || q.Answered() {
				continue
			}
			q.Delivered, q.AnsweredInSession = true, true
			run.Timeline = append(run.Timeline, Entry{Time: at, Kind: QuestionAnsweredInSession, Ticket: q.Ticket, Detail: q.Kind})
		}
	}
}

func (r *Run) openQuestion() bool {
	return slices.ContainsFunc(r.Questions, Question.Open)
}

// settleChildren clears the pending permission prompts of a session's
// subagents when the session itself moves on: a denial sends no event, and
// a subagent may never report again, so the prompt would otherwise hold the
// session in Needs you until it went stale.
func (s *Set) settleChildren(r *Run) {
	for _, id := range r.Children {
		if child := s.runs[id]; child != nil {
			child.Permission = ""
		}
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
	if r.Permission != "" || r.openQuestion() {
		return NeedsYou
	}
	for _, childID := range r.Children {
		if child := s.runs[childID]; child != nil && (!child.ended && child.Permission != "" || child.openQuestion()) {
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

// Holder returns the run whose claim on ticket is live at now: the run is
// not Ended and it, or one of its subagents, was active within the lease
// (agent-protocol §6). It returns nil when the ticket is free.
func (s *Set) Holder(ticket string, now time.Time, settings Settings) *Run {
	for _, id := range s.order {
		r := s.runs[id]
		if r.Claim != ticket {
			continue
		}
		if s.State(id, now, settings) == Ended || now.Sub(s.lastActivity(r)) >= settings.Lease {
			return nil
		}
		return r
	}
	return nil
}

// Claimant is the run whose claim names a ticket, live or lapsed.
func (s *Set) Claimant(ticket string) *Run {
	for _, id := range s.order {
		if r := s.runs[id]; r.Claim == ticket {
			return r
		}
	}
	return nil
}

// PendingAnswers lists the answered questions of a session and its
// subagents that have not been delivered yet (HOOK-5), oldest first.
func (s *Set) PendingAnswers(session string) []Question {
	r := s.runs[session]
	if r == nil {
		return nil
	}
	var pending []Question
	for _, run := range append([]*Run{r}, s.children(r)...) {
		for _, q := range run.Questions {
			if q.Answered() && !q.Delivered {
				pending = append(pending, q)
			}
		}
	}
	slices.SortStableFunc(pending, func(a, b Question) int { return a.AnsweredAt.Compare(b.AnsweredAt) })
	return pending
}

// family is a run's session and the session's subagents.
func (s *Set) family(r *Run) []*Run {
	root := r
	if r.Parent != "" && s.runs[r.Parent] != nil {
		root = s.runs[r.Parent]
	}
	return append([]*Run{root}, s.children(root)...)
}

// Edits counts a run's edits since the last checkpoint; a session's
// include its subagents', whose work rolls up to it (§10).
func (s *Set) Edits(id string) int {
	r := s.runs[id]
	if r == nil {
		return 0
	}
	if r.Parent != "" {
		return r.Edits
	}
	edits := 0
	for _, member := range s.family(r) {
		edits += member.Edits
	}
	return edits
}

// Dirty reports edits since the last checkpoint, a session's subagents'
// included: its handoff is due (HOOK-6).
func (s *Set) Dirty(id string) bool { return s.Edits(id) > 0 }

// ShellWindows returns the windows of the run's successful shell commands
// since its last checkpoint, with its subagents' for a session, sorted,
// merged where they overlap and cut at the checkpoint; nil when there are
// none. A worktree change counts as the run's only inside one of them: the
// edit tools record their own paths, and a file changed while the session
// waited is not its doing.
func (s *Set) ShellWindows(id string) []Window {
	r := s.runs[id]
	if r == nil {
		return nil
	}
	members := []*Run{r}
	if r.Parent == "" {
		members = s.family(r)
	}
	var list []Window
	for _, member := range members {
		for _, w := range member.windows {
			if w.From.Before(r.LastCheckpoint) {
				w.From = r.LastCheckpoint
			}
			if !w.To.Before(w.From) {
				list = append(list, w)
			}
		}
	}
	slices.SortFunc(list, func(a, b Window) int { return a.From.Compare(b.From) })
	var merged []Window
	for _, w := range list {
		if last := len(merged) - 1; last >= 0 && !w.From.After(merged[last].To) {
			if w.To.After(merged[last].To) {
				merged[last].To = w.To
			}
			continue
		}
		merged = append(merged, w)
	}
	return merged
}

func (s *Set) children(r *Run) []*Run {
	var list []*Run
	for _, id := range r.Children {
		if child := s.runs[id]; child != nil {
			list = append(list, child)
		}
	}
	return list
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

// View is a run with its derived state, link and flags at one moment. It
// shares nothing with the Set, so it can be read while the Set changes.
type View struct {
	ID, Agent, Kind, Parent, Project string
	Cwd, Branch, Worktree            string
	Source, AgentType, EndReason     string
	Children                         []string

	State     State
	Link      Link
	Dirty     bool
	NoHandoff bool
	// Permission is the tool awaiting permission ("?" when unknown).
	Permission string

	Started time.Time
	// LastActivity includes the run's subagents.
	LastActivity time.Time
	EndedAt      time.Time

	Tools, Edits int
	Files        []string
	Plan         []events.PlanItem
	Progress     Progress
	Questions    []Question
	Timeline     []Entry
}

// Views derives every run's view at now, in first-seen order.
func (s *Set) Views(now time.Time, settings Settings, byBranch InProgress) []View {
	views := make([]View, 0, len(s.order))
	for _, id := range s.order {
		r := s.runs[id]
		state := s.State(id, now, settings)
		link := s.Link(id, byBranch)
		views = append(views, View{
			ID: r.ID, Agent: r.Agent, Kind: r.Kind, Parent: r.Parent, Project: r.Project,
			Cwd: r.Cwd, Branch: r.Branch, Worktree: r.Worktree,
			Source: r.Source, AgentType: r.AgentType, EndReason: r.EndReason,
			Children:   slices.Clone(r.Children),
			State:      state,
			Link:       link,
			Dirty:      s.Dirty(id),
			NoHandoff:  s.Dirty(id) && state == Ended && link.Ticket != "",
			Permission: r.Permission,
			Started:    r.Started, LastActivity: s.lastActivity(r), EndedAt: r.EndedAt,
			Tools: r.Tools, Edits: s.Edits(id),
			Files:     slices.Clone(r.Files),
			Plan:      slices.Clone(r.Plan),
			Progress:  r.Progress(),
			Questions: cloneQuestions(r.Questions),
			Timeline:  slices.Clone(r.Timeline),
		})
	}
	return views
}

func cloneQuestions(list []Question) []Question {
	out := slices.Clone(list)
	for index := range out {
		out[index].Options = slices.Clone(out[index].Options)
	}
	return out
}
