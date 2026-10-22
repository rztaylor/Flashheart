package hooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/gitchange"
	"github.com/rztaylor/flashheart/internal/gitinfo"
	"github.com/rztaylor/flashheart/internal/logfile"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// MaxPayloadBytes bounds the payload read from stdin (agent-protocol §5.1).
const MaxPayloadBytes = 1 << 20

// Pending is an event an adapter wants recorded; hooks adds the time,
// agent and project.
type Pending struct {
	// Run is the full run id (<agent>:<session>[/<agent id>],
	// events.RunID); an event whose run does not parse or names another
	// agent is logged and dropped.
	Run  string
	Kind events.Kind
	// Data is one of the events *Data types. A ToolData is classified by
	// the adapter (Shell, Paths); its edit paths may be absolute and hooks
	// makes them repository-relative. A tool.used is an
	// observation: hooks records it as throttled activity (activity.go).
	Data any
}

// Input is an adapter's reading of one payload.
type Input struct {
	Cwd    string
	Events []Pending
	// Recovery asks for the recovery note (session start, HOOK-3).
	Recovery bool
	// Inbox names the session whose waiting inbox entries (answers,
	// comments, change requests) go with this prompt (HOOK-5, D54).
	Inbox string
	// Stop marks a turn end that handoff enforcement may block (HOOK-6);
	// StopActive is the agent saying a stop hook already kept it going.
	Stop, StopActive bool
	// Reply is output the adapter worked out alone (run stamping); it is
	// printed without opening the board.
	Reply *Output
	// Settled is the adapter seeing a run's checkpoint tool return: the
	// session's runs may record the paths they edited again (FH-55).
	Settled bool
	// Notes is the agent showing the model the context this event's
	// output carries, so a reminder owed to the session goes with it
	// (D50).
	Notes bool
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
	// RefreshProtocol keeps the agent's installed protocol text (its skill
	// and the section of its global CLAUDE.md or AGENTS.md) current at
	// session start and returns the notes that open the context when it
	// rewrote them (agent-protocol §5.4); nil for agents without one.
	RefreshProtocol func() (string, error)
	// ChangeTimeout is how long after the invocation began the worktree
	// check at a session's turn end and end may run; zero means
	// DefaultChangeTimeout.
	ChangeTimeout time.Duration
}

// DefaultChangeTimeout keeps a turn end or end within HOOK-1's 100 ms
// however long the log or git takes: the worktree check ends at most 75 ms
// after the hook began, with what reading the payload, the board and the
// log left of them (about 50 ms on a busy log; git status and log take
// 10–30 ms on a mid-sized repository), which leaves the process's start
// and exit and the append after the check the rest. A check cut short is
// logged and the run left as it was (HOOK-1). It runs only for a session
// that ran shell commands since its last checkpoint and is not already
// due, so it is rarely in the p95.
const DefaultChangeTimeout = 75 * time.Millisecond

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

// ownRuns keeps the pending events whose run id follows the grammar and
// names the adapter's agent (agent-protocol §2). The rest are an adapter's
// bug: each is logged and dropped, and the others are still recorded
// (HOOK-1).
func ownRuns(options Options, pending []Pending) []Pending {
	kept := pending[:0:0]
	for _, p := range pending {
		run, err := events.ParseRunID(p.Run)
		if err == nil && run.Agent() != options.Adapter.Agent() {
			err = fmt.Errorf("run %q is not a %s run", p.Run, options.Adapter.Agent())
		}
		if err != nil {
			logError(options, fmt.Errorf("dropped a %s event: %w", p.Kind, err))
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

func handle(options Options) error {
	if options.Adapter == nil {
		return errors.New("no adapter for this agent")
	}
	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
	// The worktree check's deadline runs on the real clock, from here.
	began := time.Now()
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
	input.Events = ownRuns(options, input.Events)
	if input.Reply != nil {
		return write(options, *input.Reply)
	}
	if len(input.Events) == 0 && !input.Recovery {
		return nil
	}
	// Activity outside git is not recorded (PRJ-4): the store opens the root
	// lazily, so nothing is written, not even a missing root.
	s := store.New(options.Root)
	defer s.Close()
	settings, err := s.ReadConfig()
	if err != nil {
		return err
	}
	info := s.Where(input.Cwd, now())
	if info.Repo == "" {
		return nil
	}
	// Protocol text that cannot be refreshed is logged at once, whatever
	// happens to the rest of the hook; the session start goes on.
	refreshNote, err := refreshProtocol(options, input)
	if err != nil {
		logError(options, err)
	}
	// A root that cannot be created fails the hook open, logged (HOOK-1).
	if err := s.EnsureRoot(); err != nil {
		return err
	}

	project, err := s.ProjectFor(info.Project, info.Repo, settings.AutoCreateProjects)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil // auto_create_projects is off and the project does not exist
	case errors.Is(err, store.ErrProjectArchived):
		return nil // the project is archived; nothing is recorded until it is restored (PRJ-5)
	case err != nil:
		return err
	}

	at := now().UTC()
	list := make([]events.Event, 0, len(input.Events))
	for _, pending := range input.Events {
		list = append(list, events.Event{
			Time: at, Run: pending.Run, Agent: options.Adapter.Agent(),
			Kind: pending.Kind, Project: project, Data: clean(pending.Data, info, input.Cwd),
		})
	}
	log := events.New(s)
	interval := time.Duration(settings.ActivitySeconds) * time.Second
	var out Output
	// A stop or end that cannot be checked is allowed and still recorded
	// (HOOK-1).
	var problem error
	var decided verdict
	if index := sessionEnd(list); index >= 0 {
		// The session's pending activity is recorded first, so the worktree
		// check and enforcement see every shell command and edit.
		end := list[index]
		claim := ""
		_, problem = record(log, project, list, func(state *sessionState) []events.Event {
			claim = state.Claim
			return flushRuns(state, end, at)
		})
		timeout := options.ChangeTimeout
		if timeout <= 0 {
			timeout = DefaultChangeTimeout
		}
		var err error
		decided, err = settle(s, log, project, info, &list[index], input.Stop && !input.StopActive, at, settings, began.Add(timeout), claim == "")
		out.Block = decided.block
		problem = errors.Join(problem, err)
	}
	remind := false
	appended, err := record(log, project, list, func(state *sessionState) []events.Event {
		recorded := throttle(state, list, at, interval, input.Settled)
		remind = remindToClaim(state, decided, input.Notes, list[0].Run)
		return recorded
	})
	if err != nil {
		return errors.Join(problem, err)
	}
	switch {
	case input.Recovery:
		// A note with answers is written even when recording their
		// delivery failed, so they are never lost; the failure is logged.
		out.Context, err = recovery(s, log, inboxOf(log), project, info, appended, at, settings)
		problem = errors.Join(problem, err)
		out.Context = joinNotes(refreshNote, out.Context)
	case input.Inbox != "":
		// Every prompt gets what waits in the session's inbox, the user's
		// prompt or not: a board answer or comment reaches the agent
		// whenever it gets a turn. Only who answers a question in the
		// session depends on the prompt's origin (D41).
		out.Context, err = prompted(inboxOf(log), project, input.Inbox, at)
		problem = errors.Join(problem, err)
	}
	if remind {
		out.Context = joinNotes(out.Context, protocol.ClaimReminder)
	}
	return errors.Join(problem, write(options, out))
}

func refreshProtocol(options Options, input Input) (string, error) {
	if !input.Recovery || options.RefreshProtocol == nil {
		return "", nil
	}
	note, err := options.RefreshProtocol()
	if err != nil {
		return "", fmt.Errorf("refresh protocol text: %w", err)
	}
	return note, nil
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

// inboxOf is the session inbox delivery uses: the log itself, except in a
// test that watches or fails it.
var inboxOf = func(log *events.Log) events.Inbox { return log }

// prompted delivers what waits in a session's inbox and renders it for its
// prompt (HOOK-5), even when recording its delivery failed. The prompt hook
// reads no log and checks one file, the session's inbox (HOOK-1): an inbox
// left in the old location drains into it (D54) or goes with the session's
// next recovery note, board_context or await.
func prompted(inbox events.Inbox, project, session string, at time.Time) (string, error) {
	waiting, err := events.Deliver(inbox, project, []events.Mailbox{{Session: session}}, nil, at)
	return protocol.InboxNote(waiting), err
}

// sessionEnd is the index of a session's turn end or end in list, or -1.
func sessionEnd(list []events.Event) int {
	return slices.IndexFunc(list, func(e events.Event) bool {
		return (e.Kind == events.TurnEnd || e.Kind == events.RunEnd) && !events.RunOf(e.Run).IsSubagent()
	})
}

// settle handles a session's turn end or end before it is recorded: it
// marks the event when the worktree changed since the run's last checkpoint
// other than through the edit tools (agent-protocol §4, FH-53), and at a
// stop the agent may be blocked at, decides whether to block it for a
// checkpoint (HOOK-6, §9), marking the turn end when it does. At a turn
// end of a session not yet reminded (remind), it decides whether the
// session owes the claim reminder (D50, claim.go), held as the stop's
// reason where it would be blocked for a checkpoint were it linked. The
// log is read only when there is a worktree to check, the project
// enforces handoffs or a reminder may be owed, and the project's tickets
// only when a dirty run's link turns on a branch match. A failed worktree
// check is reported, and enforcement still runs.
func settle(s *store.Store, log *events.Log, project string, info gitinfo.Info, end *events.Event, stop bool, now time.Time, settings config.Config, deadline time.Time, remind bool) (verdict, error) {
	enforcing := false
	if stop && end.Kind == events.TurnEnd {
		enforcing = settings.EnforceHandoff
		override, err := s.EnforceHandoff(project)
		if err != nil {
			return verdict{}, err
		}
		if override != nil {
			enforcing = *override
		}
	}
	remind = remind && end.Kind == events.TurnEnd
	if info.Worktree == "" && !enforcing && !remind {
		return verdict{}, nil
	}
	// Only the session's own events and the claims decide its edits,
	// windows and link.
	set := runs.NewSet()
	if err := readRuns(log, project, now, runs.Keep(end.Run), set.Apply); err != nil {
		return verdict{}, err
	}
	changed, problem := worktreeChanged(set, end.Run, info, deadline)
	markEnd(end, changed, false)
	r := set.Get(end.Run)
	if r == nil || !(changed || set.Dirty(end.Run)) {
		return verdict{}, problem
	}
	handoff := enforcing && !r.BlockedForHandoff
	if !handoff && !remind {
		return verdict{}, problem
	}
	var details board.Project
	var unread error
	read := false
	link := set.Link(end.Run, func(_, branch string) []string {
		if !read {
			details, unread = s.ReadProject(project)
			read = true
		}
		return details.InProgressOnBranch(branch)
	})
	switch {
	case unread != nil:
		return verdict{}, errors.Join(problem, unread)
	case link.Ticket == "" && remind:
		decided := verdict{remind: true}
		if enforcing {
			decided.block = protocol.ClaimReminder
		}
		return decided, problem
	case link.Ticket == "" || !handoff:
		return verdict{}, problem
	}
	markEnd(end, false, true)
	return verdict{block: fmt.Sprintf("Flashheart: record a checkpoint on %s (done, next, files) before stopping.", link.Ticket)}, problem
}

// worktreeChanged asks git whether a file in the run's worktree changed
// during one of its (or its subagents') shell commands since its last
// checkpoint, only when the run is not already due and ran one: the edit
// tools record their own paths, and a file changed while the session waited
// is not its doing.
func worktreeChanged(set *runs.Set, run string, info gitinfo.Info, deadline time.Time) (bool, error) {
	if info.Worktree == "" || set.Get(run) == nil || set.Dirty(run) {
		return false, nil
	}
	windows := set.ShellWindows(run)
	if len(windows) == 0 {
		return false, nil
	}
	// Start just before the first window, so a change at its very start
	// counts (ChangedSince counts only times after since).
	since := windows[0].From.Add(-time.Nanosecond)
	within := func(modified time.Time) bool {
		return slices.ContainsFunc(windows, func(w runs.Window) bool { return w.Contains(modified) })
	}
	gitDir := ""
	if info.Head != "" {
		gitDir = filepath.Dir(info.Head)
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	changed, err := gitchange.ChangedSince(ctx, info.Worktree, gitDir, since, within)
	if err != nil {
		return false, fmt.Errorf("check the worktree for changes: %w", err)
	}
	return changed, nil
}

// markEnd sets a turn end's or end's flags.
func markEnd(end *events.Event, changed, blocked bool) {
	switch data := end.Data.(type) {
	case events.TurnEndData:
		data.WorktreeChanged = data.WorktreeChanged || changed
		data.BlockedForHandoff = data.BlockedForHandoff || blocked
		end.Data = data
	case events.RunEndData:
		data.WorktreeChanged = data.WorktreeChanged || changed
		end.Data = data
	}
}

// clean fills in where the run is working and makes edit paths
// repository-relative; events bounds and scrubs every string on append
// (SEC-3, HOOK-2), and tool data at once, since hooks keeps its paths.
func clean(data any, info gitinfo.Info, cwd string) any {
	switch d := data.(type) {
	case events.RunStartData:
		if d.Kind == events.KindSession {
			d.Cwd, d.Branch, d.Worktree = cwd, info.Branch, info.Worktree
		}
		return d
	case events.TurnStartData:
		d.Cwd, d.Branch, d.Worktree = cwd, info.Branch, info.Worktree
		return d
	case events.ToolData:
		paths := make([]string, 0, len(d.Paths))
		for _, path := range d.Paths {
			paths = append(paths, info.Relative(path))
		}
		d.Paths, d.Path = paths, info.Relative(d.Path)
		// The paths go into the session's activity state before any event,
		// so they are bounded by the events rule now.
		return events.Bound(d)
	}
	return data
}

// readRuns reads the project's events of the run window (runs.Lookback) that
// keep accepts, without decoding the others; a project without events has
// none.
func readRuns(log *events.Log, project string, now time.Time, keep func(events.Header) bool, fn func(events.Event)) error {
	if err := log.ReadSelected(project, now.Add(-runs.Lookback), keep, fn); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// recovery gathers the linked ticket and the previous run in this worktree
// and renders the note (HOOK-3). It reads the log's lines and headers once,
// finds the previous session from the headers, then decodes the two
// sessions' events and the claims, which decide all the note says.
func recovery(s *store.Store, log *events.Log, inbox events.Inbox, project string, info gitinfo.Info, appended []events.Event, now time.Time, settings config.Config) (string, error) {
	if len(appended) == 0 || appended[0].Kind != events.RunStart {
		return "", nil
	}
	current := appended[0].Run
	headers, err := log.ReadHeaders(project, now.Add(-runs.Lookback))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	latest, set := runs.NewLatest(), runs.NewSet()
	previousID := ""
	if headers != nil {
		headers.Each(latest.Note)
		headers.SelectBackward(latest.Wants, latest.Apply)
		previousID = latest.Session(info.Worktree, current)
		headers.Select(runs.Keep(current, previousID), set.Apply)
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
			linked := &protocol.RecoveryTicket{ID: ticket.ID, Title: ticket.Title, Column: ticket.Status(), LinkedBy: link.By}
			if ticket.Handoff != nil {
				linked.Next = ticket.Handoff.Next
			}
			note.Ticket = linked
			// Its open change requests reach whichever session picks it
			// up, though none was queued for this one (FH-126). An
			// activity file that cannot be read leaves them out.
			if activity, _, err := s.ReadTicketActivity(project, ticket.Folder); err == nil {
				note.Requests = protocol.ChangeRequests(ticket.ID, board.OpenRequests(activity.Comments))
			}
		}
	}
	thresholds := runs.SettingsFor(settings.QuietMinutes, settings.LeaseMinutes)
	if previous := set.Get(previousID); previous != nil {
		p := &protocol.PreviousRun{ID: previous.ID, Edits: previous.Edits}
		if set.State(previous.ID, now, thresholds) == runs.Ended {
			p.Ended = previous.EndedAt
			if p.Ended.IsZero() {
				p.Ended = previous.LastActivity
			}
		}
		note.Previous = p
	}
	// What waits in this session's inbox, in either location (D54), and
	// the answers waiting for the previous session in this worktree go in
	// the note and count as delivered (HOOK-5). The previous session's
	// answers go too, so resuming it later does not deliver them again;
	// its comments and change requests were sent to it and stay for it.
	// What was taken is shown even if recording its delivery failed, so it
	// is never lost.
	boxes := []events.Mailbox{{Session: current, Legacy: true}}
	if set.Get(previousID) != nil {
		boxes = append(boxes, events.Mailbox{Session: previousID, AnswersOnly: true, Legacy: true})
	}
	waiting, err := events.Deliver(inbox, project, boxes, set.PendingDeliveries(current, previousID), now)
	note.Inbox = waiting
	return protocol.RecoveryNote(note), err
}
