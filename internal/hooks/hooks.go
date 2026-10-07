package hooks

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/gitinfo"
	"github.com/rztaylor/flashheart/internal/logfile"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/scrub"
	"github.com/rztaylor/flashheart/internal/store"
)

// MaxPayloadBytes bounds the payload read from stdin (agent-protocol §5.1).
const MaxPayloadBytes = 1 << 20

// Pending is an event an adapter wants recorded; hooks adds the time,
// agent and project.
type Pending struct {
	// Run is the full run id (<agent>:<session>[/<agent id>]).
	Run  string
	Kind string
	// Data is one of the events *Data types. Edit paths in ToolData may be
	// absolute; hooks makes them repository-relative.
	Data any
}

// Input is an adapter's reading of one payload.
type Input struct {
	Cwd    string
	Events []Pending
	// Recovery asks for the recovery note (session start, HOOK-3).
	Recovery bool
	// Answers names the session whose waiting answers go with this prompt
	// (HOOK-5).
	Answers string
	// Stop marks a turn end that handoff enforcement may block (HOOK-6);
	// StopActive is the agent saying a stop hook already kept it going.
	Stop, StopActive bool
	// Reply is output the adapter worked out alone (run stamping); it is
	// printed without opening the board.
	Reply *Output
}

// Output is what hooks wants said back to the agent.
type Output struct {
	// Context is additional context for the model, such as the recovery note.
	Context string
	// Block is the reason to refuse a stop (HOOK-6).
	Block string
	// UpdatedInput replaces a tool call's input (run stamping, §7.1).
	UpdatedInput []byte
}

// Adapter knows one agent's payloads and outputs (HOOK-7).
type Adapter interface {
	Agent() string
	Parse(event string, payload []byte) (Input, error)
	// Render returns the bytes to print, or nil to print nothing.
	Render(event string, out Output) []byte
}

// Options configure one invocation.
type Options struct {
	Root    string
	Event   string
	Stdin   io.Reader
	Stdout  io.Writer
	Now     func() time.Time
	Adapter Adapter
	// RefreshSkill keeps the agent's installed protocol skill current at
	// session start and reports whether it rewrote it (agent-protocol §5.4);
	// nil for agents without one.
	RefreshSkill func() (bool, error)
}

// Run handles one hook invocation. It never fails: errors and panics go to
// <root>/.flashheart/hook-errors.log (HOOK-1).
func Run(options Options) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logError(options, fmt.Errorf("panic: %v\n%s", recovered, debug.Stack()))
		}
	}()
	if err := handle(options); err != nil {
		logError(options, err)
	}
}

func logError(options Options, err error) {
	agent := "unknown"
	if options.Adapter != nil {
		agent = options.Adapter.Agent()
	}
	log := logfile.HookErrors(options.Root)
	defer log.Close()
	_, _ = fmt.Fprintf(log, "%s %s: %v", agent, options.Event, err)
}

func handle(options Options) error {
	if options.Adapter == nil {
		return errors.New("no adapter for this agent")
	}
	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
	payload, err := io.ReadAll(io.LimitReader(options.Stdin, MaxPayloadBytes+1))
	if err != nil {
		return fmt.Errorf("read payload: %w", err)
	}
	if len(payload) > MaxPayloadBytes {
		return fmt.Errorf("payload is larger than %d bytes", MaxPayloadBytes)
	}
	input, err := options.Adapter.Parse(options.Event, payload)
	if err != nil {
		return err
	}
	if input.Reply != nil {
		return write(options, *input.Reply)
	}
	if len(input.Events) == 0 && !input.Recovery {
		return nil
	}
	// A skill that cannot be refreshed is logged at once, whatever happens
	// to the rest of the hook; the session start goes on.
	skillNote, err := refreshSkill(options, input)
	if err != nil {
		logError(options, err)
	}

	settings, err := config.Load(options.Root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(options.Root, 0o755); err != nil {
		return fmt.Errorf("create board root: %w", err)
	}
	s, err := store.Open(options.Root)
	if err != nil {
		return err
	}
	defer s.Close()

	info := s.Where(input.Cwd, now())
	project, err := s.ProjectFor(info.Project, info.Repo, settings.AutoCreateProjects)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil // auto_create_projects is off and the project does not exist
	case errors.Is(err, store.ErrNeedsMigration):
		return nil // the board needs `flashheart migrate` first (MIG-1)
	case errors.Is(err, store.ErrProjectArchived):
		return nil // the project is archived; nothing is recorded until it is restored (PRJ-5)
	case err != nil:
		return err
	}

	at := now().UTC()
	list := make([]events.Event, 0, len(input.Events))
	for _, pending := range input.Events {
		list = append(list, events.Event{
			Time: at, Run: scrub.Limit(pending.Run, 300), Agent: options.Adapter.Agent(),
			Kind: pending.Kind, Project: project, Data: clean(pending.Data, info, input.Cwd),
		})
	}
	log := events.New(s)
	var out Output
	// A stop that cannot be checked is allowed and still recorded (HOOK-1).
	var problem error
	if input.Stop && !input.StopActive {
		out.Block, problem = enforce(s, log, project, list, at, settings)
	}
	if err := log.Append(list...); err != nil {
		return errors.Join(problem, err)
	}
	switch {
	case input.Recovery:
		if out.Context, err = recovery(s, log, project, info, list, at, settings); err != nil {
			return errors.Join(problem, err)
		}
		out.Context = joinNotes(skillNote, out.Context)
	case input.Answers != "":
		// Answers taken from the inbox are shown even if marking them
		// delivered fails, so they are never lost.
		out.Context, err = answers(log, project, input.Answers, at)
		problem = errors.Join(problem, err)
	}
	return errors.Join(problem, write(options, out))
}

func refreshSkill(options Options, input Input) (string, error) {
	if !input.Recovery || options.RefreshSkill == nil {
		return "", nil
	}
	updated, err := options.RefreshSkill()
	if err != nil {
		return "", fmt.Errorf("refresh skill: %w", err)
	}
	if updated {
		return protocol.SkillUpdatedNote, nil
	}
	return "", nil
}

func joinNotes(notes ...string) string {
	var kept []string
	for _, note := range notes {
		if note = strings.TrimRight(note, "\n"); note != "" {
			kept = append(kept, note)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, "\n") + "\n"
}

func write(options Options, out Output) error {
	if data := options.Adapter.Render(options.Event, out); len(data) > 0 && options.Stdout != nil {
		_, err := options.Stdout.Write(data)
		return err
	}
	return nil
}

// answers takes the answers waiting for a session, marks them delivered
// and renders them for its prompt (HOOK-5).
func answers(log *events.Log, project, session string, at time.Time) (string, error) {
	waiting, err := log.TakeAnswers(project, session)
	if err != nil || len(waiting) == 0 {
		return "", err
	}
	return protocol.AnswersNote(toAnswers(waiting)), log.MarkDelivered(project, waiting, at)
}

func toAnswers(list []events.Delivery) []protocol.Answer {
	out := make([]protocol.Answer, 0, len(list))
	for _, d := range list {
		out = append(out, protocol.Answer{Question: d.Question, Answer: d.Answer, By: d.By, Ticket: d.Ticket})
	}
	return out
}

// enforce decides whether to block a stop for a checkpoint (HOOK-6, §9)
// and marks the turn end when it does. The log is read only when the
// project enforces handoffs.
func enforce(s *store.Store, log *events.Log, project string, list []events.Event, now time.Time, settings config.Config) (string, error) {
	index := slices.IndexFunc(list, func(e events.Event) bool { return e.Kind == events.TurnEnd })
	if index < 0 {
		return "", nil
	}
	enabled := settings.EnforceHandoff
	override, err := s.EnforceHandoff(project)
	if err != nil {
		return "", err
	}
	if override != nil {
		enabled = *override
	}
	if !enabled {
		return "", nil
	}
	details, err := s.ReadProject(project)
	if err != nil {
		return "", err
	}
	set := runs.NewSet()
	if err := log.Read(project, now.Add(-recoveryWindow), set.Apply); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	run := list[index].Run
	r := set.Get(run)
	if r == nil || !r.Dirty() || r.BlockedForHandoff {
		return "", nil
	}
	link := set.Link(run, func(_, branch string) []string { return details.InProgressOnBranch(branch) })
	if link.Ticket == "" {
		return "", nil
	}
	list[index].Data = events.TurnEndData{BlockedForHandoff: true}
	return fmt.Sprintf("Flashheart: record a checkpoint on %s (done, next, files) before stopping.", link.Ticket), nil
}

// clean scrubs and bounds the agent-written strings in event data (SEC-3,
// HOOK-2) and fills in where the run is working.
func clean(data any, info gitinfo.Info, cwd string) any {
	text := scrub.Text
	switch d := data.(type) {
	case events.RunStartData:
		if d.Kind == events.KindSession {
			d.Cwd, d.Branch, d.Worktree = place(cwd, info)
		}
		d.Parent = scrub.Limit(d.Parent, 300)
		d.Source = text(d.Source, 40)
		d.AgentType = text(d.AgentType, events.MaxNameLength)
		return d
	case events.TurnStartData:
		d.Cwd, d.Branch, d.Worktree = place(cwd, info)
		return d
	case events.RunEndData:
		d.Reason = text(d.Reason, events.MaxReasonText)
		return d
	case events.ToolData:
		d.Tool = text(d.Tool, events.MaxNameLength)
		d.Path = text(info.Relative(d.Path), 500)
		d.Summary = text(d.Summary, events.MaxSummary)
		return d
	case events.PlanData:
		items := make([]events.PlanItem, 0, min(len(d.Items), events.MaxPlanItems))
		for _, item := range d.Items[:min(len(d.Items), events.MaxPlanItems)] {
			items = append(items, events.PlanItem{ID: text(item.ID, 64), Text: text(item.Text, events.MaxPlanText), Status: item.Status})
		}
		d.Items = items
		return d
	case events.PermissionData:
		d.Tool = text(d.Tool, events.MaxNameLength)
		d.Summary = text(d.Summary, events.MaxSummary)
		return d
	case events.ResolvedData:
		d.Tool = text(d.Tool, events.MaxNameLength)
		return d
	case events.NotificationData:
		d.Type = text(d.Type, 40)
		return d
	}
	return data
}

// place is where a run works, bounded so a pathological directory or branch
// name cannot overflow an event line or the recovery note.
func place(cwd string, info gitinfo.Info) (string, string, string) {
	return scrub.Limit(cwd, events.MaxNameLength*5), scrub.Text(info.Branch, events.MaxNameLength), scrub.Limit(info.Worktree, events.MaxNameLength*5)
}

// recoveryWindow is how far back session start looks for earlier runs.
const recoveryWindow = 24 * time.Hour

// recovery gathers the linked ticket and the previous run in this worktree
// and renders the note (HOOK-3).
func recovery(s *store.Store, log *events.Log, project string, info gitinfo.Info, appended []events.Event, now time.Time, settings config.Config) (string, error) {
	if len(appended) == 0 || appended[0].Kind != events.RunStart {
		return "", nil
	}
	current := appended[0].Run
	set := runs.NewSet()
	if err := log.Read(project, now.Add(-recoveryWindow), set.Apply); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if set.Get(current) == nil {
		for _, e := range appended {
			set.Apply(e)
		}
	}
	details, err := s.ReadProject(project)
	if err != nil {
		return "", err
	}
	byBranch := func(_, branch string) []string { return details.InProgressOnBranch(branch) }
	note := protocol.Recovery{Run: current, Project: project, Branch: info.Branch}
	if !details.KeyDerived {
		note.Key = details.Key
	}
	if link := set.Link(current, byBranch); link.Ticket != "" {
		if i := slices.IndexFunc(details.Tickets, func(t board.Ticket) bool { return t.ID == link.Ticket }); i >= 0 {
			ticket := details.Tickets[i]
			linked := &protocol.RecoveryTicket{ID: ticket.ID, Title: ticket.Title, Column: string(ticket.Column), LinkedBy: link.By}
			if ticket.Handoff != nil {
				linked.Next = ticket.Handoff.Next
			}
			note.Ticket = linked
		}
	}
	thresholds := runs.SettingsFor(settings.QuietMinutes, settings.LeaseMinutes)
	var previous *runs.Run
	for _, r := range set.Runs() {
		if r.ID == current || r.Kind != events.KindSession || r.Worktree == "" || r.Worktree != info.Worktree {
			continue
		}
		if previous == nil || r.LastActivity.After(previous.LastActivity) {
			previous = r
		}
	}
	if previous != nil {
		p := &protocol.PreviousRun{ID: previous.ID, Edits: previous.Edits}
		if set.State(previous.ID, now, thresholds) == runs.Ended {
			p.Ended = previous.EndedAt
			if p.Ended.IsZero() {
				p.Ended = previous.LastActivity
			}
		}
		note.Previous = p
	}
	// Answers waiting for this session or the previous one in this worktree
	// go in the note and count as delivered (HOOK-5).
	waiting, err := log.TakeAnswers(project, current)
	if err != nil {
		return "", err
	}
	if previous != nil {
		// The previous session's inbox goes too, so resuming it later does
		// not deliver the same answers again.
		theirs, err := log.TakeAnswers(project, previous.ID)
		if err != nil {
			return "", err
		}
		waiting = append(waiting, theirs...)
	}
	seen := map[string]bool{}
	for _, d := range waiting {
		seen[d.ID] = true
	}
	for _, id := range []string{current, previousID(previous)} {
		for _, q := range set.PendingAnswers(id) {
			if !seen[q.ID] {
				seen[q.ID] = true
				waiting = append(waiting, events.Delivery{ID: q.ID, Run: q.Run, Ticket: q.Ticket, Question: q.Text, Answer: q.Answer, By: q.AnsweredBy})
			}
		}
	}
	if len(waiting) > 0 {
		if err := log.MarkDelivered(project, waiting, now); err != nil {
			return "", err
		}
		note.Answered = toAnswers(waiting)
	}
	return protocol.RecoveryNote(note), nil
}

func previousID(r *runs.Run) string {
	if r == nil {
		return ""
	}
	return r.ID
}
