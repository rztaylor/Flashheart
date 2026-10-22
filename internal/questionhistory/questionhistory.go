package questionhistory

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// Recorder records questions' outcomes in their tickets' history.
type Recorder struct {
	files *store.Store
	log   io.Writer
	// done holds the changes recorded or found already, by question and
	// change; failed those whose failure was logged.
	done   map[string]bool
	failed map[string]bool
}

// New returns a recorder writing through files and logging to log.
func New(files *store.Store, log io.Writer) *Recorder {
	return &Recorder{files: files, log: log, done: map[string]bool{}, failed: map[string]bool{}}
}

// Run makes a pass for every new revision until ctx ends.
func (r *Recorder) Run(ctx context.Context, b *index.Index) {
	var revision uint64
	for ctx.Err() == nil {
		if snapshot, err := b.Current(); err == nil && snapshot != nil {
			r.Pass(snapshot)
			revision = snapshot.Revision
		}
		b.Wait(ctx, revision)
	}
}

// Pass records what the snapshot's runs show of every question about a
// live ticket that its history does not have yet.
func (r *Recorder) Pass(snapshot *index.Snapshot) {
	for _, run := range snapshot.Runs {
		for _, q := range run.Questions {
			if q.Ticket == "" || snapshot.Archived(q.Ticket) {
				continue
			}
			project, _, ok := snapshot.FindTicket(q.Ticket)
			if !ok {
				continue
			}
			asked := board.QuestionRecord{ID: q.ID, Kind: q.Kind, Text: q.Text, Options: q.Options}
			r.record(project.Name, q, q.Run, q.Asked, board.ChangeAsked, asked)
			switch {
			case q.Answered():
				answered := asked
				answered.Answer = q.Answer
				r.record(project.Name, q, q.AnsweredBy, q.AnsweredAt, board.ChangeAnswered, answered)
			case q.AnsweredInSession:
				r.record(project.Name, q, q.Run, q.AnsweredInSessionAt, board.ChangeAnsweredInSession, board.QuestionRecord{ID: q.ID, Text: q.Text})
			case q.SessionEnded:
				r.record(project.Name, q, q.Run, time.Time{}, board.ChangeSessionEnded, board.QuestionRecord{ID: q.ID, Text: q.Text})
			}
		}
	}
}

// record writes one change of q by by, at at, unless it was written or
// found before.
func (r *Recorder) record(project string, q runs.Question, by string, at time.Time, what string, question board.QuestionRecord) {
	key := q.ID + "\x00" + what
	if r.done[key] {
		return
	}
	_, err := r.files.As(by).RecordQuestion(project, q.Ticket, at, board.Change{What: what, Question: question})
	switch {
	case err == nil, errors.Is(err, store.ErrNotFound):
		r.done[key] = true
	case !r.failed[key]:
		r.failed[key] = true
		fmt.Fprintf(r.log, "flashheart: keep question %s in %s's history: %v\n", q.ID, q.Ticket, err)
	}
}
