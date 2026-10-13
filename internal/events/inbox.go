package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// EntryType says what a session inbox entry carries (agent-protocol §4,
// D54).
type EntryType string

// Inbox entry types. An answer is a board answer to one of the session's
// questions; a comment is a person's comment on a ticket sent to the
// session; a change request is a comment asking for a change in review
// (D47).
const (
	EntryAnswer        EntryType = "answer"
	EntryComment       EntryType = "comment"
	EntryChangeRequest EntryType = "change-request"
)

// Known reports a type this binary delivers. An entry of another type (a
// newer binary's) stays in the inbox.
func (t EntryType) Known() bool {
	return t == EntryAnswer || t == EntryComment || t == EntryChangeRequest
}

// MaxInboxText bounds an entry's text as Queue writes it; delivery shows
// less (protocol's note).
const MaxInboxText = 8000

// Entry is one item waiting in a session's inbox for its next prompt or
// tool result (HOOK-5). Run is the run it is for: the asking run of an
// answer (the session or one of its subagents), the run a comment or
// change request was sent to. ID is the question's id for an answer, the
// comment's id in the ticket's activity.md otherwise.
type Entry struct {
	Type   EntryType `json:"type"`
	ID     string    `json:"id"`
	Run    string    `json:"run"`
	Ticket string    `json:"ticket,omitempty"`
	// Project is the ticket's project for a comment a person sent from the
	// board (FH-127), whose delivery is recorded on the comment there;
	// the inbox is in the session's home project, which may be another.
	Project string `json:"project,omitempty"`
	// Question is the question an answer answers.
	Question string `json:"question,omitempty"`
	// Text is the answer, the comment or the change asked for.
	Text string `json:"text"`
	// By is who wrote it.
	By string `json:"by"`
	// Time is when it was queued.
	Time time.Time `json:"time,omitzero"`
}

// key is what one delivery hands over once.
func (e Entry) key() string { return string(e.Type) + "\x00" + e.ID }

// bound cuts and scrubs the entry's strings as the log does (SEC-3).
func (e Entry) bound() Entry {
	e.ID, e.Run, e.Ticket, e.Project = line(e.ID, maxID), line(e.Run, maxRef), line(e.Ticket, maxRef), line(e.Project, maxRef)
	e.Question, e.Text = line(e.Question, MaxQuestionText), block(e.Text, MaxInboxText)
	e.By = line(e.By, MaxNameLength)
	return e
}

// inboxLine is an inbox line as read. Lines written before D54 have no
// type (they are answers) and carry the answer in `answer`.
type inboxLine struct {
	Entry
	Answer string `json:"answer,omitempty"`
}

// parseEntry reads one inbox line; ok is false for a line that is not one.
func parseEntry(data []byte) (Entry, bool) {
	var l inboxLine
	if json.Unmarshal(data, &l) != nil || l.ID == "" {
		return Entry{}, false
	}
	e := l.Entry
	if e.Type == "" {
		e.Type = EntryAnswer
	}
	if e.Text == "" {
		e.Text = l.Answer
	}
	return e, true
}

// Mailbox is one session's inbox as a delivery takes it.
type Mailbox struct {
	Session string
	// AnswersOnly takes only answers and leaves comments and change
	// requests for the session they were sent to: a new session's recovery
	// note takes the previous session's answers, not its messages.
	AnswersOnly bool
	// Legacy also takes the session's inbox in the old answers/ location
	// (D54). The prompt hook, which may check one file (HOOK-1), leaves it
	// to the readers that can afford a second.
	Legacy bool
}

// Inbox is the session inbox as delivery uses it: taking a session's
// entries, handing over the comments people sent from the board as their
// tickets have them then, and recording that they reached their runs.
// *Log is one.
type Inbox interface {
	Take(project string, box Mailbox) ([]Entry, error)
	DeliverComment(e Entry, at time.Time) (Entry, bool, error)
	MarkDelivered(project string, list []Entry, at time.Time) error
}

// Deliver hands over what waits in sessions' inboxes (HOOK-5): it takes
// each mailbox, adds pending (answered questions the fold shows
// undelivered, whose inbox may be gone), keeps each entry once, hands
// over each comment sent from the board as its ticket has it now, marking
// it delivered there (FH-127: one sent to another run, delivered already
// or gone is left out), leaves out a change request deleted since it was
// queued (FH-140), and records each delivered (question.delivered
// for an answer, comment.delivered otherwise) at at. It returns
// everything it took even when an inbox, a ticket or the record fails, so
// the caller shows it: nothing taken may be lost, and an answer whose
// record failed is still undelivered in the log, so a later delivery
// shows it again.
func Deliver(in Inbox, project string, boxes []Mailbox, pending []Entry, at time.Time) ([]Entry, error) {
	var list []Entry
	var problems error
	seen := map[string]bool{}
	add := func(e Entry) {
		if e.ID != "" && !seen[e.key()] {
			seen[e.key()] = true
			list = append(list, e)
		}
	}
	for _, box := range boxes {
		if box.Session == "" {
			continue
		}
		taken, err := in.Take(project, box)
		problems = errors.Join(problems, err)
		for _, e := range taken {
			add(e)
		}
	}
	for _, e := range pending {
		add(e)
	}
	kept := list[:0]
	for _, e := range list {
		if (e.Type == EntryComment || e.Type == EntryChangeRequest) && e.Project != "" {
			current, ok, err := in.DeliverComment(e, at)
			if err != nil {
				// The ticket cannot say: hand it over as it was queued.
				problems = errors.Join(problems, err)
				current, ok = e, true
			}
			if !ok {
				continue
			}
			e = current
		}
		kept = append(kept, e)
	}
	list = kept
	if len(list) > 0 {
		problems = errors.Join(problems, in.MarkDelivered(project, list, at))
	}
	return list, problems
}

// Queue puts an entry in the inbox of the session of the run it is for.
// This is how a writer hands a session an answer, a comment or a change
// request; the meaning stays in the log and the ticket (STO-6).
func (l *Log) Queue(project string, e Entry) error {
	if !e.Type.Known() {
		return fmt.Errorf("inbox entry type %q is unknown", e.Type)
	}
	if e.ID == "" {
		return errors.New("inbox entry has no id")
	}
	run, err := ParseRunID(e.Run)
	if err != nil {
		return err
	}
	data, err := json.Marshal(e.bound())
	if err != nil {
		return err
	}
	return l.store.AppendInbox(project, run.Session().String(), append(data, '\n'))
}

// DeliverComment hands over a comment a person sent from the board as its
// ticket has it now (store.DeliverComment): with its current text, and
// marked delivered there. ok is false when it is not to be handed over:
// sent to another run, delivered already or no longer there. A change
// request goes as it was queued while its ticket still has it; one the
// person deleted since (FH-140) is not handed over.
func (l *Log) DeliverComment(e Entry, at time.Time) (Entry, bool, error) {
	if e.Type == EntryChangeRequest {
		_, ok, err := l.store.TicketComment(e.Project, e.Ticket, e.ID)
		return e, ok, err
	}
	comment, ok, err := l.store.DeliverComment(e.Project, e.Ticket, e.ID, e.Run, at)
	if err != nil || !ok {
		return e, ok, err
	}
	e.Text, e.By = block(comment.Body, MaxInboxText), line(comment.By, MaxNameLength)
	return e, true, nil
}

// Withdraw takes comment's entries (a comment or change request on ticket,
// in ticketProject) out of the inbox of run's session, in project, so a
// comment the person deleted (FH-140) does not wait there; the rest of the
// inbox stays. Nothing queued is no error. Delivery leaves out a comment
// that is gone in any case (Deliver); this keeps its text off the disk.
func (l *Log) Withdraw(project, run, ticketProject, ticket, comment string) error {
	id, err := ParseRunID(run)
	if err != nil {
		return err
	}
	_, err = l.store.TakeInbox(project, id.Session().String(), func(data []byte) bool {
		e, ok := parseEntry(data)
		return ok && (e.Type == EntryComment || e.Type == EntryChangeRequest) &&
			e.ID == comment && e.Ticket == ticket && e.Project == ticketProject
	})
	return err
}

// MarkDelivered records that entries taken from an inbox reached their
// runs: question.delivered for an answer, on the asking run;
// comment.delivered for a comment or change request, on the run it was
// sent to.
func (l *Log) MarkDelivered(project string, list []Entry, at time.Time) error {
	out := make([]Event, 0, len(list))
	for _, e := range list {
		event := Event{Time: at, Run: e.Run, Agent: RunOf(e.Run).Agent(), Project: project}
		if e.Type == EntryAnswer {
			event.Kind, event.Data = QuestionDelivered, DeliveredData{ID: e.ID}
		} else {
			event.Kind, event.Data = CommentDelivered, CommentDeliveredData{Ticket: e.Ticket, Comment: e.ID, Type: string(e.Type)}
		}
		out = append(out, event)
	}
	return l.Append(out...)
}

// Take empties a session's mailbox of the entries it asks for and returns
// them in the order they were queued, the old location's first, each
// once. Malformed lines go with them; entries of a type this binary does
// not know, and those AnswersOnly leaves, stay.
func (l *Log) Take(project string, box Mailbox) ([]Entry, error) {
	take := func(data []byte) bool {
		e, ok := parseEntry(data)
		if !ok {
			return true
		}
		return e.Type.Known() && (!box.AnswersOnly || e.Type == EntryAnswer)
	}
	var data []byte
	var problems error
	if box.Legacy {
		old, err := l.store.TakeLegacyInbox(project, box.Session, take)
		data, problems = old, err
	}
	current, err := l.store.TakeInbox(project, box.Session, take)
	data, problems = append(data, current...), errors.Join(problems, err)
	var list []Entry
	seen := map[string]bool{}
	for text := range strings.SplitSeq(string(data), "\n") {
		// An entry queued twice (a retried answer) is delivered once.
		if e, ok := parseEntry([]byte(text)); ok && e.Type.Known() && !seen[e.key()] {
			seen[e.key()] = true
			list = append(list, e)
		}
	}
	return list, problems
}
