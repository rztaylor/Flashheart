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
)

// Event kinds (agent-protocol §3).
const (
	RunStart            = "run.start"
	RunEnd              = "run.end"
	TurnStart           = "turn.start"
	TurnEnd             = "turn.end"
	ToolUsed            = "tool.used"
	PlanUpdated         = "plan.updated"
	PermissionRequested = "permission.requested"
	PermissionResolved  = "permission.resolved"
	Notification        = "notification"
	Compact             = "compact"
	Claim               = "claim"
	Release             = "release"
	Checkpoint          = "checkpoint"
	TicketMoved         = "ticket.moved"
	TicketUpdated       = "ticket.updated"
	TicketCreated       = "ticket.created"
	ReviewWritten       = "review.written"
	AttachmentAdded     = "attachment.added"
	QuestionAsked       = "question.asked"
	QuestionAnswered    = "question.answered"
	QuestionDelivered   = "question.delivered"
)

var known = map[string]bool{
	RunStart: true, RunEnd: true, TurnStart: true, TurnEnd: true, ToolUsed: true,
	PlanUpdated: true, PermissionRequested: true, PermissionResolved: true,
	Notification: true, Compact: true, Claim: true, Release: true, Checkpoint: true,
	TicketMoved: true, TicketUpdated: true, TicketCreated: true, ReviewWritten: true,
	AttachmentAdded: true, QuestionAsked: true, QuestionAnswered: true, QuestionDelivered: true,
}

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

// RunEndData is run.end's data.
type RunEndData struct {
	Reason string `json:"reason,omitempty"`
}

// TurnStartData is turn.start's data. The prompt is never stored; the
// working directory and branch are repeated so a run first seen mid-session
// still has them, and a branch switch is noticed.
type TurnStartData struct {
	Cwd      string `json:"cwd,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Worktree string `json:"worktree,omitempty"`
}

// TurnEndData is turn.end's data.
type TurnEndData struct {
	BlockedForHandoff bool `json:"blocked_for_handoff"`
}

// ToolData is tool.used's data. Path is repository-relative and set for
// edits only.
type ToolData struct {
	Tool    string `json:"tool"`
	OK      bool   `json:"ok"`
	Path    string `json:"path,omitempty"`
	Summary string `json:"summary,omitempty"`
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

// Event is one log record. Data is any of the *Data types when writing;
// events read from the log keep their raw data for Decode.
type Event struct {
	Time    time.Time
	Run     string
	Agent   string
	Kind    string
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
	Kind    string          `json:"kind"`
	Project string          `json:"project"`
	Data    json.RawMessage `json:"data"`
}

const timeLayout = "2006-01-02T15:04:05.000Z"

// MarshalLine encodes the event as one JSONL line with its newline.
func (e Event) MarshalLine() ([]byte, error) {
	data := e.raw
	if e.Data != nil {
		var err error
		if data, err = json.Marshal(e.Data); err != nil {
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
	if err := json.Unmarshal(line, &env); err != nil || env.V < 1 || env.Run == "" || !known[env.Kind] {
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
	type target struct{ project, file string }
	batches := map[target][]byte{}
	var order []target
	for _, e := range list {
		if e.Run == "" || e.Project == "" || !known[e.Kind] {
			return fmt.Errorf("event %q for run %q in project %q is incomplete", e.Kind, e.Run, e.Project)
		}
		line, err := e.MarshalLine()
		if err != nil {
			return err
		}
		key := target{e.Project, FileName(e.Time)}
		if _, seen := batches[key]; !seen {
			order = append(order, key)
		}
		batches[key] = append(batches[key], line...)
	}
	for _, key := range order {
		if err := l.store.AppendEventLines(key.project, key.file, batches[key]); err != nil {
			return err
		}
	}
	return nil
}

// Files lists a project's event files in date order.
func (l *Log) Files(project string) ([]string, error) { return l.store.EventFiles(project) }

// Read calls fn for every readable event in files dated on or after since's
// UTC date, oldest file first. Callers filter by exact time.
func (l *Log) Read(project string, since time.Time, fn func(Event)) error {
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
		if _, err := l.ReadFrom(project, file, 0, fn); err != nil {
			return err
		}
	}
	return nil
}

// ReadFrom calls fn for each complete line from offset on and returns the
// offset after the last complete line, so a line still being written is
// read next time.
func (l *Log) ReadFrom(project, file string, offset int64, fn func(Event)) (int64, error) {
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
		if e, ok := parseLine(line[:len(line)-1]); ok {
			fn(e)
		}
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

// QueueAnswer puts an answer in the inbox of the asking run's session.
func (l *Log) QueueAnswer(project, run string, d Delivery) error {
	session, _, _ := strings.Cut(run, "/")
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
		agent, _, _ := strings.Cut(d.Run, ":")
		out = append(out, Event{Time: at, Run: d.Run, Agent: agent, Kind: QuestionDelivered, Project: project, Data: DeliveredData{ID: d.ID}})
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
