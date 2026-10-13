package events

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/rztaylor/flashheart/internal/scrub"
)

// Kind is an event kind (agent-protocol §3).
type Kind string

// Event kinds. ToolUsed is one tool call: since protocol 2 (FH-55) hooks
// write throttled Activity records instead, and readers still fold
// tool.used from older logs.
const (
	RunStart            Kind = "run.start"
	RunEnd              Kind = "run.end"
	TurnStart           Kind = "turn.start"
	TurnEnd             Kind = "turn.end"
	ToolUsed            Kind = "tool.used"
	Activity            Kind = "activity"
	PlanUpdated         Kind = "plan.updated"
	PermissionRequested Kind = "permission.requested"
	PermissionResolved  Kind = "permission.resolved"
	Notification        Kind = "notification"
	Compact             Kind = "compact"
	Claim               Kind = "claim"
	Release             Kind = "release"
	Checkpoint          Kind = "checkpoint"
	TicketMoved         Kind = "ticket.moved"
	TicketUpdated       Kind = "ticket.updated"
	TicketCreated       Kind = "ticket.created"
	ReviewWritten       Kind = "review.written"
	AttachmentAdded     Kind = "attachment.added"
	QuestionAsked       Kind = "question.asked"
	QuestionAnswered    Kind = "question.answered"
	QuestionDelivered   Kind = "question.delivered"
	// CommentAdded, CommentEdited and CommentDeleted record the human
	// commenting on a ticket on the board (D47, FH-140): the ticket and the
	// comment's id, never its text.
	CommentAdded   Kind = "comment.added"
	CommentEdited  Kind = "comment.edited"
	CommentDeleted Kind = "comment.deleted"
	// CommentDelivered records a comment or change request taken from a
	// session's inbox and shown to it (D54), on the run it was sent to.
	CommentDelivered Kind = "comment.delivered"
)

// kinds is the one list of the kinds the log records, each with its data
// type. Readers skip any other kind; writers are refused it.
var kinds = map[Kind]reflect.Type{
	RunStart:            reflect.TypeFor[RunStartData](),
	RunEnd:              reflect.TypeFor[RunEndData](),
	TurnStart:           reflect.TypeFor[TurnStartData](),
	TurnEnd:             reflect.TypeFor[TurnEndData](),
	ToolUsed:            reflect.TypeFor[ToolData](),
	Activity:            reflect.TypeFor[ActivityData](),
	PlanUpdated:         reflect.TypeFor[PlanData](),
	PermissionRequested: reflect.TypeFor[PermissionData](),
	PermissionResolved:  reflect.TypeFor[ResolvedData](),
	Notification:        reflect.TypeFor[NotificationData](),
	Compact:             reflect.TypeFor[CompactData](),
	Claim:               reflect.TypeFor[TicketData](),
	Release:             reflect.TypeFor[TicketData](),
	Checkpoint:          reflect.TypeFor[CheckpointData](),
	TicketMoved:         reflect.TypeFor[TicketData](),
	TicketUpdated:       reflect.TypeFor[TicketData](),
	TicketCreated:       reflect.TypeFor[TicketData](),
	ReviewWritten:       reflect.TypeFor[TicketData](),
	AttachmentAdded:     reflect.TypeFor[AttachmentData](),
	QuestionAsked:       reflect.TypeFor[QuestionData](),
	QuestionAnswered:    reflect.TypeFor[AnswerData](),
	QuestionDelivered:   reflect.TypeFor[DeliveredData](),
	CommentAdded:        reflect.TypeFor[TicketData](),
	CommentEdited:       reflect.TypeFor[TicketData](),
	CommentDeleted:      reflect.TypeFor[TicketData](),
	CommentDelivered:    reflect.TypeFor[CommentDeliveredData](),
}

// Known reports a kind the log records.
func (k Kind) Known() bool { _, ok := kinds[k]; return ok }

// Data is an event's data. Each type bounds and scrubs the strings agents
// (and people) write into it; the encoder applies it to every event, so no
// writer has to remember.
type Data interface {
	bound() Data
}

// Bound is d with its strings bounded and scrubbed as Append writes them,
// for a caller that keeps event data elsewhere first (hooks' activity
// state).
func Bound(d Data) Data { return d.bound() }

// checkData returns data bounded, or why it does not fit kind.
func checkData(kind Kind, data any) (Data, error) {
	want, ok := kinds[kind]
	if !ok {
		return nil, fmt.Errorf("event kind %q is unknown", kind)
	}
	if got := reflect.TypeOf(data); got != want {
		return nil, fmt.Errorf("event %s takes %s data, not %s", kind, want.Name(), got)
	}
	return data.(Data).bound(), nil
}

// Bounds of strings without a limit of their own.
const (
	maxID    = 64   // question and plan item ids
	maxRef   = 300  // run ids, `by`, tickets, files
	maxWord  = 40   // enums written by adapters: kinds, sources, statuses
	maxPath  = 1000 // working directories and worktrees
	maxPaths = 500  // one edited path
	maxField = 20   // fields of a ticket update
)

// line is agent text as one scrubbed line.
func line(s string, limit int) string { return scrub.Text(s, limit) }

// block is text kept as written, line breaks and all, but scrubbed and cut.
func block(s string, limit int) string {
	return scrub.Limit(scrub.Secrets(strings.ToValidUTF8(s, "�")), limit)
}

// lines bounds a list: at most most items, each one line.
func lines(list []string, most, limit int) []string {
	if list == nil {
		return nil
	}
	out := make([]string, 0, min(len(list), most))
	for _, item := range list[:min(len(list), most)] {
		out = append(out, line(item, limit))
	}
	return out
}

func (d RunStartData) bound() Data {
	d.Kind, d.Parent, d.Source = line(d.Kind, maxWord), line(d.Parent, maxRef), line(d.Source, maxWord)
	d.Cwd, d.Branch, d.Worktree = block(d.Cwd, maxPath), line(d.Branch, MaxNameLength), block(d.Worktree, maxPath)
	d.AgentType = line(d.AgentType, MaxNameLength)
	return d
}

func (d RunEndData) bound() Data {
	d.Reason = line(d.Reason, MaxReasonText)
	return d
}

func (d TurnStartData) bound() Data {
	d.Origin = line(d.Origin, maxWord)
	d.Cwd, d.Branch, d.Worktree = block(d.Cwd, maxPath), line(d.Branch, MaxNameLength), block(d.Worktree, maxPath)
	return d
}

func (d TurnEndData) bound() Data { return d }

func (d ToolData) bound() Data {
	d.Tool, d.Summary = line(d.Tool, MaxNameLength), line(d.Summary, MaxSummary)
	d.Paths, d.Path = lines(d.Paths, MaxToolPaths, maxPaths), line(d.Path, maxPaths)
	return d
}

func (d ActivityData) bound() Data {
	d.Paths = lines(d.Paths, MaxToolPaths, maxPaths)
	if len(d.Shell) > MaxActivitySpans {
		d.Shell = d.Shell[len(d.Shell)-MaxActivitySpans:]
	}
	return d
}

func (d PlanData) bound() Data {
	if d.Items == nil {
		return d
	}
	items := make([]PlanItem, 0, min(len(d.Items), MaxPlanItems))
	for _, item := range d.Items[:min(len(d.Items), MaxPlanItems)] {
		items = append(items, PlanItem{ID: line(item.ID, maxID), Text: line(item.Text, MaxPlanText), Status: line(item.Status, maxWord)})
	}
	d.Items = items
	return d
}

func (d PermissionData) bound() Data {
	d.Tool, d.Summary = line(d.Tool, MaxNameLength), line(d.Summary, MaxSummary)
	return d
}

func (d ResolvedData) bound() Data {
	d.Outcome, d.Tool = line(d.Outcome, maxWord), line(d.Tool, MaxNameLength)
	return d
}

func (d NotificationData) bound() Data {
	d.Type = line(d.Type, maxWord)
	return d
}

func (d CompactData) bound() Data {
	d.Phase = line(d.Phase, maxWord)
	return d
}

func (d AttachmentData) bound() Data {
	d.Ticket, d.File, d.Kind = line(d.Ticket, maxRef), line(d.File, maxRef), line(d.Kind, maxWord)
	return d
}

func (d TicketData) bound() Data {
	d.Ticket, d.Reason, d.By = line(d.Ticket, maxRef), line(d.Reason, MaxReasonText), line(d.By, maxRef)
	d.From, d.To, d.Home = line(d.From, maxWord), line(d.To, maxWord), line(d.Home, MaxNameLength)
	d.Fields = lines(d.Fields, maxField, maxWord)
	d.Comment = line(d.Comment, maxID)
	return d
}

func (d QuestionData) bound() Data {
	d.ID, d.Ticket, d.Kind = line(d.ID, maxID), line(d.Ticket, maxRef), line(d.Kind, maxWord)
	d.Text, d.Options = line(d.Text, MaxQuestionText), lines(d.Options, MaxOptions, MaxOptionText)
	return d
}

func (d AnswerData) bound() Data {
	d.ID, d.Answer, d.By = line(d.ID, maxID), block(d.Answer, MaxAnswerText), line(d.By, MaxNameLength)
	return d
}

func (d DeliveredData) bound() Data {
	d.ID = line(d.ID, maxID)
	return d
}

func (d CommentDeliveredData) bound() Data {
	d.Ticket, d.Comment, d.Type = line(d.Ticket, maxRef), line(d.Comment, maxID), line(d.Type, maxWord)
	return d
}

func (d CheckpointData) bound() Data {
	d.Ticket = line(d.Ticket, maxRef)
	return d
}
