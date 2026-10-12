package events

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/store"
)

// Version is the envelope version written in "v".
const Version = 1

// MaxLineBytes bounds one encoded event; longer lines are skipped on read.
const MaxLineBytes = 64 << 10

// Limits on agent-written strings in event data (HOOK-2, agent-protocol §3).
const (
	MaxPlanItems  = 50
	MaxPlanText   = 200
	MaxSummary    = 120
	MaxNameLength = 200
	MaxReasonText = 200
	// MaxQuestionText bounds question.asked text; MaxAnswerText an answer.
	MaxQuestionText = 1000
	MaxAnswerText   = 1000
	MaxOptions      = 10
	MaxOptionText   = 200
	// MaxActivitySpans bounds an activity record's shell windows.
	MaxActivitySpans = 20
	// MaxToolPaths bounds the paths of one tool result.
	MaxToolPaths = 50
)

// Plan item statuses; "deleted" appears only in merges and removes the item.
const (
	PlanPending    = "pending"
	PlanInProgress = "in_progress"
	PlanCompleted  = "completed"
	PlanDeleted    = "deleted"
)

// Run kinds in RunStartData.Kind.
const (
	KindSession  = "session"
	KindSubagent = "subagent"
)

// SourceMCP is run.start's source for a run the MCP server started for a
// connection that reached it without hooks (agent-protocol §7.1); hook
// sources are the agent's own (startup, resume, clear, compact).
const SourceMCP = "mcp"

// RunEndDisconnected is run.end's reason when an MCP-started run's
// connection closes.
const RunEndDisconnected = "disconnected"

// RunStartData is run.start's data.
type RunStartData struct {
	Kind      string `json:"kind"`
	Parent    string `json:"parent,omitempty"`
	Cwd       string `json:"cwd,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Worktree  string `json:"worktree,omitempty"`
	Source    string `json:"source,omitempty"`
	AgentType string `json:"agent_type,omitempty"`
}

// RunEndData is run.end's data. WorktreeChanged is the hook's finding that
// the session's worktree changed after its last checkpoint other than
// through the edit tools, such as by shell commands (agent-protocol §4);
// only the fact is stored, never what changed.
type RunEndData struct {
	Reason          string `json:"reason,omitempty"`
	WorktreeChanged bool   `json:"worktree_changed,omitempty"`
}

// Turn origins: who started a turn, classified by the agent's adapter from
// the prompt, which is never stored (agent-protocol §3).
const (
	// OriginUser is a prompt the user typed.
	OriginUser = "user"
	// OriginTask is a notice the agent's harness submitted: one of its
	// background commands or agents finished, or the app reports an event.
	OriginTask = "task"
	// OriginAgent is a message from another agent: a subagent's hand-back
	// or a message between agents.
	OriginAgent = "agent"
)

// TurnStartData is turn.start's data. The prompt is never stored; the
// working directory and branch are repeated so a run first seen mid-session
// still has them, and a branch switch is noticed. Origin says who started
// the turn; Background marks every turn the user did not start, as it did
// before Origin existed, so older readers keep such turns from answering
// the session's questions (agent-protocol §4).
type TurnStartData struct {
	Origin     string `json:"origin,omitempty"`
	Background bool   `json:"background,omitempty"`
	Cwd        string `json:"cwd,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Worktree   string `json:"worktree,omitempty"`
}

// FromUser reports whether the user started the turn, so it answers the
// session's open questions (D28). Only OriginUser is the user; a turn
// without an origin, from a log older than the field, is the user's
// unless it is marked Background.
func (d TurnStartData) FromUser() bool {
	if d.Origin != "" {
		return d.Origin == OriginUser
	}
	return !d.Background
}

// TurnEndData is turn.end's data. WorktreeChanged is as in RunEndData.
type TurnEndData struct {
	BlockedForHandoff bool `json:"blocked_for_handoff"`
	WorktreeChanged   bool `json:"worktree_changed,omitempty"`
}

// ToolData is tool.used's data: one tool result, classified by the agent's
// adapter, so nothing past it reads tool names. Shell marks a shell
// command, whose edits name no path; Paths are the files an edit changed,
// repository-relative once hooks has cleaned them (one result may edit
// several). Path is the single edit path of tool.used records in logs from
// before protocol 2, which carry no Shell either.
type ToolData struct {
	Tool    string   `json:"tool"`
	OK      bool     `json:"ok"`
	Shell   bool     `json:"shell,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	Path    string   `json:"path,omitempty"`
	Summary string   `json:"summary,omitempty"`
}

// EditPaths returns the paths the result edited, a legacy Path included.
func (d ToolData) EditPaths() []string {
	if d.Path == "" {
		return d.Paths
	}
	return append([]string{d.Path}, d.Paths...)
}

// ActivityData is activity's data: a run's tool use since its previous
// activity record, recorded at most once per activity interval unless an
// edit names a path the run has not recorded in this turn or since its
// last checkpoint, or
// a permission request waits on the next tool result (agent-protocol §3).
// Paths are those newly edited paths, repository-relative; Shell holds the
// windows of the run's successful shell commands, each from the run's
// previous event to the command's result, merged where they meet.
type ActivityData struct {
	Tools  int      `json:"tools"`
	Failed int      `json:"failed,omitempty"`
	Paths  []string `json:"paths,omitempty"`
	Shell  []Span   `json:"shell,omitempty"`
}

// PlanItem is one entry of a run's plan. ID is set by agents whose plan
// tools name items, so merges can update them.
type PlanItem struct {
	ID     string `json:"id,omitempty"`
	Text   string `json:"text,omitempty"`
	Status string `json:"status"`
}

// PlanData is plan.updated's data: the whole plan, or with Merge, items to
// add, update or (status deleted) remove by ID.
type PlanData struct {
	Merge bool       `json:"merge,omitempty"`
	Items []PlanItem `json:"items"`
}

// PermissionData is permission.requested's data.
type PermissionData struct {
	Tool    string `json:"tool,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// ResolvedData is permission.resolved's data.
type ResolvedData struct {
	Outcome string `json:"outcome"`
	Tool    string `json:"tool,omitempty"`
}

// NotificationData is notification's data: the type only, never the message.
type NotificationData struct {
	Type string `json:"type"`
}

// CompactData is compact's data.
type CompactData struct {
	Phase string `json:"phase"`
}

// AttachmentData is the data of attachment.added: the ticket and the
// stored file (under files/) with its kind.
type AttachmentData struct {
	Ticket string `json:"ticket"`
	File   string `json:"file"`
	Kind   string `json:"kind"`
}

// TicketData carries the ticket of claim, release, review.written and the
// ticket.* kinds. By names who acted when it was not the event's run
// (`human` for the UI); From and To are a move's columns, Fields an
// update's changed fields.
type TicketData struct {
	Ticket string   `json:"ticket"`
	Force  bool     `json:"force,omitempty"`
	Reason string   `json:"reason,omitempty"`
	By     string   `json:"by,omitempty"`
	From   string   `json:"from,omitempty"`
	To     string   `json:"to,omitempty"`
	Fields []string `json:"fields,omitempty"`
	// Home is the claimant's own project on a claim, so a reader of the
	// ticket's log knows where the holder's activity is recorded.
	Home string `json:"home,omitempty"`
}

// Question kinds (RUN-8).
const (
	QuestionKindQuestion = "question"
	QuestionKindDecision = "decision"
	QuestionKindReview   = "review"
	QuestionKindBlocked  = "blocked"
)

// QuestionKinds lists the kinds ask_human accepts.
var QuestionKinds = []string{QuestionKindQuestion, QuestionKindDecision, QuestionKindReview, QuestionKindBlocked}

// QuestionData is question.asked's data. Ticket is empty for a question
// about no ticket.
type QuestionData struct {
	ID      string   `json:"id"`
	Ticket  string   `json:"ticket,omitempty"`
	Kind    string   `json:"kind"`
	Text    string   `json:"text"`
	Options []string `json:"options,omitempty"`
}

// AnswerData is question.answered's data, recorded on the asking run.
type AnswerData struct {
	ID     string `json:"id"`
	Answer string `json:"answer"`
	By     string `json:"by"`
}

// DeliveredData is question.delivered's data: the answer reached the run.
type DeliveredData struct {
	ID string `json:"id"`
}

// CheckpointData is checkpoint's data.
type CheckpointData struct {
	Ticket    string `json:"ticket"`
	Done      int    `json:"done"`
	Next      int    `json:"next"`
	Files     int    `json:"files"`
	Questions int    `json:"questions"`
}

// Event is one log record. Data is its kind's Data type when writing (or
// nil for none); events read from the log keep their raw data for Decode.
type Event struct {
	Time    time.Time
	Run     string
	Agent   string
	Kind    Kind
	Project string
	Data    any

	raw json.RawMessage
}

// envelope is the on-disk field order.
type envelope struct {
	V       int             `json:"v"`
	TS      string          `json:"ts"`
	Run     string          `json:"run"`
	Agent   string          `json:"agent"`
	Kind    Kind            `json:"kind"`
	Project string          `json:"project"`
	Data    json.RawMessage `json:"data"`
}

const timeLayout = "2006-01-02T15:04:05.000Z"

// MarshalLine encodes the event as one JSONL line with its newline. The
// data must be the kind's type (kinds); its strings are bounded and
// scrubbed here, whoever built the event (SEC-3, HOOK-2).
func (e Event) MarshalLine() ([]byte, error) {
	data := e.raw
	if e.Data != nil {
		bounded, err := checkData(e.Kind, e.Data)
		if err != nil {
			return nil, err
		}
		if data, err = json.Marshal(bounded); err != nil {
			return nil, err
		}
	}
	if len(data) == 0 || string(data) == "null" {
		data = json.RawMessage("{}")
	}
	line, err := json.Marshal(envelope{V: Version, TS: e.Time.UTC().Format(timeLayout), Run: e.Run, Agent: e.Agent, Kind: e.Kind, Project: e.Project, Data: data})
	if err != nil {
		return nil, err
	}
	if len(line) >= MaxLineBytes {
		return nil, fmt.Errorf("event %s is %d bytes; the limit is %d", e.Kind, len(line), MaxLineBytes)
	}
	return append(line, '\n'), nil
}

// Decode unmarshals the event's data into v.
func (e Event) Decode(v any) error {
	if e.Data != nil {
		raw, err := json.Marshal(e.Data)
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, v)
	}
	if len(e.raw) == 0 {
		return nil
	}
	return json.Unmarshal(e.raw, v)
}

// parseLine decodes one line; ok is false for lines readers skip.
func parseLine(line []byte) (Event, bool) {
	var env envelope
	if err := json.Unmarshal(line, &env); err != nil || env.V < 1 || env.Run == "" || !env.Kind.Known() {
		return Event{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, env.TS)
	if err != nil {
		return Event{}, false
	}
	return Event{Time: ts.UTC(), Run: env.Run, Agent: env.Agent, Kind: env.Kind, Project: env.Project, raw: env.Data}, true
}

// FileName is the event file for t's UTC date.
func FileName(t time.Time) string { return t.UTC().Format("2006-01-02") + ".jsonl" }

// Log reads and writes the event logs of a root.
type Log struct {
	store *store.Store
}

// New returns the log of the root s.
func New(s *store.Store) *Log { return &Log{store: s} }

// Append writes events to their projects' daily files. Each file gets one
// locked append, so a batch lands together.
func (l *Log) Append(list ...Event) error {
	batches, order, err := encode(list)
	if err != nil {
		return err
	}
	for _, key := range order {
		if err := l.store.AppendEventLines(key.project, key.file, batches[key]); err != nil {
			return err
		}
	}
	return nil
}

type target struct{ project, file string }

// encode groups the events' lines by project and file, in first-seen order.
func encode(list []Event) (map[target][]byte, []target, error) {
	batches := map[target][]byte{}
	var order []target
	for _, e := range list {
		if e.Run == "" || e.Project == "" || !e.Kind.Known() {
			return nil, nil, fmt.Errorf("event %q for run %q in project %q is incomplete", e.Kind, e.Run, e.Project)
		}
		line, err := e.MarshalLine()
		if err != nil {
			return nil, nil, err
		}
		key := target{e.Project, FileName(e.Time)}
		if _, seen := batches[key]; !seen {
			order = append(order, key)
		}
		batches[key] = append(batches[key], line...)
	}
	return batches, order, nil
}

// AppendWithState appends events together with replacing a session's hook
// activity state, under the project lock that guards both, so concurrent
// hooks of one session see each other's state (FH-55). update gets the
// state (nil when there is none) and returns the new state (nil removes
// it) and the events to append, all in project. The state's format belongs
// to hooks.
func (l *Log) AppendWithState(project, session string, update func(state []byte) ([]byte, []Event, error)) error {
	return l.store.UpdateActivity(project, session, func(state []byte) ([]byte, map[string][]byte, error) {
		next, list, err := update(state)
		if err != nil {
			return nil, nil, err
		}
		batches, _, err := encode(list)
		if err != nil {
			return nil, nil, err
		}
		lines := map[string][]byte{}
		for key, batch := range batches {
			if key.project != project {
				return nil, nil, fmt.Errorf("event for project %q appended with %q's state", key.project, project)
			}
			lines[key.file] = batch
		}
		return next, lines, nil
	})
}

// Files lists a project's event files in date order.
func (l *Log) Files(project string) ([]string, error) { return l.store.EventFiles(project) }

// Read calls fn for every readable event in files dated on or after since's
// UTC date, oldest file first. Callers filter by exact time.
func (l *Log) Read(project string, since time.Time, fn func(Event)) error {
	return l.read(project, since, func(file string) error {
		_, err := l.ReadFrom(project, file, 0, fn)
		return err
	})
}

// read calls each for the project's files dated on or after since's UTC
// date, oldest first.
func (l *Log) read(project string, since time.Time, each func(file string) error) error {
	files, err := l.store.EventFiles(project)
	if err != nil {
		return err
	}
	first := ""
	if !since.IsZero() {
		first = FileName(since)
	}
	for _, file := range files {
		if file < first {
			continue
		}
		if err := each(file); err != nil {
			return err
		}
	}
	return nil
}

// ReadFrom calls fn for each complete line from offset on and returns the
// offset after the last complete line, so a line still being written is
// read next time.
func (l *Log) ReadFrom(project, file string, offset int64, fn func(Event)) (int64, error) {
	return l.lines(project, file, offset, func(line []byte) {
		if e, ok := parseLine(line); ok {
			fn(e)
		}
	})
}

// lines calls fn for each complete line from offset on, without its
// newline, skipping lines too long to be an event; the slice is valid only
// during the call. It returns the offset after the last complete line.
func (l *Log) lines(project, file string, offset int64, fn func(line []byte)) (int64, error) {
	handle, err := l.store.OpenEventFile(project, file)
	if err != nil {
		return offset, err
	}
	defer handle.Close()
	if _, err := handle.Seek(offset, io.SeekStart); err != nil {
		return offset, err
	}
	reader := bufio.NewReaderSize(handle, MaxLineBytes)
	skipping := false
	for {
		line, err := reader.ReadSlice('\n')
		switch {
		case errors.Is(err, bufio.ErrBufferFull):
			// Too long to be an event: skip to the end of the line.
			offset += int64(len(line))
			skipping = true
			continue
		case errors.Is(err, io.EOF):
			return offset, nil
		case err != nil:
			return offset, err
		}
		offset += int64(len(line))
		if skipping {
			skipping = false
			continue
		}
		fn(line[:len(line)-1])
	}
}

// Prune deletes a project's event files dated more than days before now's
// UTC date and reports how many it removed.
func (l *Log) Prune(project string, now time.Time, days int) (int, error) {
	files, err := l.store.EventFiles(project)
	if err != nil {
		return 0, err
	}
	cutoff := FileName(now.UTC().AddDate(0, 0, -days))
	removed := 0
	for _, file := range files {
		if file >= cutoff {
			break
		}
		if err := l.store.RemoveEventFile(project, file); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// Delivery is one answer waiting in a session's inbox for its next prompt
// (HOOK-5). Run is the asking run (the session or one of its subagents).
type Delivery struct {
	ID       string `json:"id"`
	Run      string `json:"run"`
	Ticket   string `json:"ticket,omitempty"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	By       string `json:"by"`
}

// Inbox is the answers inbox as delivery uses it: taking a session's
// queued answers and recording that they reached their runs. *Log is one.
type Inbox interface {
	TakeAnswers(project, session string) ([]Delivery, error)
	MarkDelivered(project string, list []Delivery, at time.Time) error
}

// Deliver hands over the answers waiting for sessions (HOOK-5): it takes
// each session's inbox, adds pending (answered questions the fold shows
// undelivered, whose inbox may be gone), keeps each question once, and
// records question.delivered for them all at at. It returns everything it
// took even when an inbox or the record fails, so the caller shows them:
// an answer must never be lost, and one whose record failed is still
// undelivered in the log, so a later delivery shows it again.
func Deliver(in Inbox, project string, sessions []string, pending []Delivery, at time.Time) ([]Delivery, error) {
	var list []Delivery
	var problems error
	seen := map[string]bool{}
	add := func(d Delivery) {
		if d.ID != "" && !seen[d.ID] {
			seen[d.ID] = true
			list = append(list, d)
		}
	}
	for _, session := range sessions {
		if session == "" {
			continue
		}
		taken, err := in.TakeAnswers(project, session)
		problems = errors.Join(problems, err)
		for _, d := range taken {
			add(d)
		}
	}
	for _, d := range pending {
		add(d)
	}
	if len(list) > 0 {
		problems = errors.Join(problems, in.MarkDelivered(project, list, at))
	}
	return list, problems
}

// QueueAnswer puts an answer in the inbox of the asking run's session.
func (l *Log) QueueAnswer(project, run string, d Delivery) error {
	session := RunOf(run).Session().String()
	line, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return l.store.AppendInbox(project, session, append(line, '\n'))
}

// MarkDelivered records that answers taken from an inbox reached their
// runs (question.delivered on each asking run).
func (l *Log) MarkDelivered(project string, list []Delivery, at time.Time) error {
	out := make([]Event, 0, len(list))
	for _, d := range list {
		out = append(out, Event{Time: at, Run: d.Run, Agent: RunOf(d.Run).Agent(), Kind: QuestionDelivered, Project: project, Data: DeliveredData{ID: d.ID}})
	}
	return l.Append(out...)
}

// TakeAnswers empties a session's inbox and returns its answers, skipping
// malformed lines.
func (l *Log) TakeAnswers(project, session string) ([]Delivery, error) {
	data, err := l.store.TakeInbox(project, session)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	var list []Delivery
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		var d Delivery
		// An answer queued twice (a retried answer) is delivered once.
		if json.Unmarshal([]byte(line), &d) == nil && d.ID != "" && !seen[d.ID] {
			seen[d.ID] = true
			list = append(list, d)
		}
	}
	return list, nil
}
