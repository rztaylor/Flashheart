package hooks

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"slices"
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

// cacheFile is the cwd cache, relative to the root (agent-protocol §2).
const cacheFile = ".flashheart/cache/cwd.json"

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
}

// Output is what hooks wants said back to the agent.
type Output struct {
	// Context is additional context for the model, such as the recovery note.
	Context string
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
	if len(input.Events) == 0 && !input.Recovery {
		return nil
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

	info := resolve(s, input.Cwd, now())
	project, err := s.ProjectFor(info.Project, info.Repo, settings.AutoCreateProjects)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil // auto_create_projects is off and the project does not exist
	case errors.Is(err, store.ErrNeedsMigration):
		return nil // the board needs `flashheart migrate` first (MIG-1)
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
	if err := log.Append(list...); err != nil {
		return err
	}
	if !input.Recovery {
		return nil
	}
	note, err := recovery(s, log, project, info, list, at, settings)
	if err != nil {
		return err
	}
	if out := options.Adapter.Render(options.Event, Output{Context: note}); len(out) > 0 && options.Stdout != nil {
		_, err = options.Stdout.Write(out)
		return err
	}
	return nil
}

// resolve finds the working directory's repository through the cwd cache,
// writing the cache back only when it changed.
func resolve(s *store.Store, cwd string, now time.Time) gitinfo.Info {
	data, _ := s.ReadFile(cacheFile)
	cache := gitinfo.ParseCache(data)
	info, _ := cache.Resolve(cwd, now)
	if cache.Changed() {
		_ = s.WriteFileAtomic(cacheFile, cache.Marshal())
	}
	return info
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
			if file, err := s.TicketFile(project, ticket.ID); err == nil {
				linked.File = filepath.Join(s.Path(), filepath.FromSlash(path.Clean(file)))
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
	return protocol.RecoveryNote(note), nil
}
