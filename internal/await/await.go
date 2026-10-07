package await

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/store"
)

// Defaults for Options.
const (
	DefaultTimeout = 12 * time.Hour
	DefaultPoll    = time.Second
	DefaultRescan  = 15 * time.Second
	// lookback is how far back the question is searched for: runs are
	// derived from the last two days of events (agent-protocol §4).
	lookback = 48 * time.Hour
)

var (
	// ErrUnknownQuestion means the project's recent events hold no such
	// question.
	ErrUnknownQuestion = errors.New("no such question in the project's last two days of events")
	// ErrTimeout means no answer arrived within Options.Timeout.
	ErrTimeout = errors.New("no answer yet")
)

// Options say which question to wait for and how.
type Options struct {
	Project, Question string
	// Timeout bounds the wait; zero means DefaultTimeout.
	Timeout time.Duration
	// Poll is how often the inbox is checked (one stat when it is empty);
	// Rescan how often the log is read for delivery by another path.
	Poll, Rescan time.Duration
	// Now and Sleep are the clock; nil means the real one.
	Now   func() time.Time
	Sleep func(context.Context, time.Duration) error
}

// Result is what a finished wait found.
type Result struct {
	// Note is the answers note for the agent (information, not
	// instructions); empty when AlreadyDelivered.
	Note string
	// AlreadyDelivered means the answer reached the session another way.
	AlreadyDelivered bool
}

// Wait blocks until the question's answer is in its session's inbox, then
// takes the inbox, marks its answers delivered and returns them as a note.
// Other answers waiting for the same session are delivered with it.
func Wait(ctx context.Context, log *events.Log, o Options) (Result, error) {
	o = withDefaults(o)
	if !store.ValidProject(o.Project) {
		return Result{}, fmt.Errorf("project %q: %w", o.Project, store.ErrInvalidName)
	}
	start := o.Now()
	q, err := find(log, o.Project, o.Question, start.Add(-lookback))
	if err != nil {
		return Result{}, err
	}
	if q.run == "" {
		return Result{}, fmt.Errorf("%s: %w", o.Question, ErrUnknownQuestion)
	}
	session, _, _ := strings.Cut(q.run, "/")
	scanned := start
	for {
		if q.delivered {
			return Result{AlreadyDelivered: true}, nil
		}
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		waiting, err := log.TakeAnswers(o.Project, session)
		if err != nil {
			return Result{}, err
		}
		if len(waiting) > 0 {
			// Taken answers are returned even if marking them fails, so they
			// are never lost (as the prompt hook does).
			return Result{Note: protocol.AnswersNote(toAnswers(waiting))}, log.MarkDelivered(o.Project, waiting, o.Now())
		}
		if o.Now().Sub(start) >= o.Timeout {
			return Result{}, fmt.Errorf("%s after %s: %w", o.Question, o.Timeout, ErrTimeout)
		}
		if err := o.Sleep(ctx, o.Poll); err != nil {
			return Result{}, err
		}
		if o.Now().Sub(scanned) >= o.Rescan {
			if q, err = find(log, o.Project, o.Question, start.Add(-lookback)); err != nil {
				return Result{}, err
			}
			scanned = o.Now()
		}
	}
}

func withDefaults(o Options) Options {
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.Poll <= 0 {
		o.Poll = DefaultPoll
	}
	if o.Rescan <= 0 {
		o.Rescan = DefaultRescan
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Sleep == nil {
		o.Sleep = sleep
	}
	return o
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// question is what the log says about one question.
type question struct {
	run       string
	delivered bool
}

func find(log *events.Log, project, id string, since time.Time) (question, error) {
	var q question
	err := log.Read(project, since, func(e events.Event) {
		switch e.Kind {
		case events.QuestionAsked:
			var data events.QuestionData
			if e.Decode(&data) == nil && data.ID == id {
				q.run = e.Run
			}
		case events.QuestionDelivered:
			var data events.DeliveredData
			if e.Decode(&data) == nil && data.ID == id {
				q.delivered = true
			}
		}
	})
	return q, err
}

func toAnswers(list []events.Delivery) []protocol.Answer {
	out := make([]protocol.Answer, 0, len(list))
	for _, d := range list {
		out = append(out, protocol.Answer{Question: d.Question, Answer: d.Answer, By: d.By, Ticket: d.Ticket})
	}
	return out
}
