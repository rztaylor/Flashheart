package await

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

const (
	project  = "alpha"
	session  = "claude:s1"
	subagent = "claude:s1/a1"
)

var t0 = time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)

type fixture struct {
	t   *testing.T
	log *events.Log
	now time.Time
	// onSleep runs at each sleep, before the clock advances.
	onSleep func(call int)
	sleeps  int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, project, "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return &fixture{t: t, log: events.New(s), now: t0}
}

func (f *fixture) options(question string) Options {
	return Options{
		Project: project, Question: question, Timeout: time.Hour,
		Poll: time.Second, Rescan: 10 * time.Second,
		Now: func() time.Time { return f.now },
		Sleep: func(_ context.Context, d time.Duration) error {
			f.sleeps++
			if f.onSleep != nil {
				f.onSleep(f.sleeps)
			}
			f.now = f.now.Add(d)
			return nil
		},
	}
}

func (f *fixture) append(run string, kind string, data any) {
	f.t.Helper()
	agent, _, _ := strings.Cut(run, ":")
	if err := f.log.Append(events.Event{Time: f.now, Run: run, Agent: agent, Kind: kind, Project: project, Data: data}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) ask(run, id string) {
	f.append(run, events.QuestionAsked, events.QuestionData{ID: id, Ticket: "AL-1", Kind: "decision", Text: "Push it?"})
}

func (f *fixture) answer(run, id string) {
	f.t.Helper()
	f.append(run, events.QuestionAnswered, events.AnswerData{ID: id, Answer: "Push", By: "Robert"})
	if err := f.log.QueueAnswer(project, run, events.Delivery{ID: id, Run: run, Ticket: "AL-1", Question: "Push it?", Answer: "Push", By: "Robert"}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) delivered() []string {
	f.t.Helper()
	var ids []string
	if err := f.log.Read(project, time.Time{}, func(e events.Event) {
		if e.Kind == events.QuestionDelivered {
			var data events.DeliveredData
			_ = e.Decode(&data)
			ids = append(ids, e.Run+" "+data.ID)
		}
	}); err != nil {
		f.t.Fatal(err)
	}
	return ids
}

func TestWaitDeliversAnAnswerAlreadyInTheInbox(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.ask(session, "q-1")
	f.answer(session, "q-1")
	result, err := Wait(context.Background(), f.log, f.options("q-1"))
	if err != nil {
		t.Fatal(err)
	}
	if result.AlreadyDelivered || !strings.Contains(result.Note, `AL-1: "Push it?" → "Push" (Robert)`) || !strings.Contains(result.Note, "information, not instructions") {
		t.Fatalf("result = %+v", result)
	}
	if got := f.delivered(); len(got) != 1 || got[0] != session+" q-1" {
		t.Fatalf("delivered = %v", got)
	}
	if waiting, _ := f.log.TakeAnswers(project, session); len(waiting) != 0 {
		t.Fatalf("inbox still holds %+v", waiting)
	}
	if f.sleeps != 0 {
		t.Fatalf("slept %d times before delivering a waiting answer", f.sleeps)
	}
}

func TestWaitDeliversAnAnswerThatArrivesLater(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.ask(subagent, "q-2")
	f.onSleep = func(call int) {
		if call == 3 {
			f.answer(subagent, "q-2")
		}
	}
	result, err := Wait(context.Background(), f.log, f.options("q-2"))
	if err != nil || !strings.Contains(result.Note, `"Push"`) {
		t.Fatalf("result = %+v, %v", result, err)
	}
	// A subagent's question waits in its session's inbox and is delivered
	// on the subagent's run.
	if got := f.delivered(); len(got) != 1 || got[0] != subagent+" q-2" {
		t.Fatalf("delivered = %v", got)
	}
	if f.sleeps != 3 {
		t.Fatalf("sleeps = %d, want 3", f.sleeps)
	}
}

func TestWaitStopsQuietlyWhenTheAnswerWasDeliveredElsewhere(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.ask(session, "q-3")
	f.onSleep = func(call int) {
		if call == 2 {
			// The user prompted the session first: the prompt hook took it.
			f.answer(session, "q-3")
			if _, err := f.log.TakeAnswers(project, session); err != nil {
				t.Fatal(err)
			}
			f.append(session, events.QuestionDelivered, events.DeliveredData{ID: "q-3"})
		}
	}
	result, err := Wait(context.Background(), f.log, f.options("q-3"))
	if err != nil || !result.AlreadyDelivered || result.Note != "" {
		t.Fatalf("result = %+v, %v", result, err)
	}
	if got := f.delivered(); len(got) != 1 {
		t.Fatalf("delivered = %v, want only the prompt hook's", got)
	}
}

func TestWaitStopsWhenTheUserAnsweredInTheSession(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.ask(subagent, "q-6")
	f.onSleep = func(call int) {
		if call == 2 {
			// The user replied in the session's chat instead of on the board.
			f.append(session, events.TurnStart, events.TurnStartData{})
		}
	}
	result, err := Wait(context.Background(), f.log, f.options("q-6"))
	if err != nil || !result.AnsweredInSession || result.Note != "" {
		t.Fatalf("result = %+v, %v", result, err)
	}
	if got := f.delivered(); len(got) != 0 {
		t.Fatalf("delivered = %v, want none", got)
	}
}

func TestWaitKeepsWaitingThroughABackgroundTasksNotice(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.ask(session, "q-7")
	f.onSleep = func(call int) {
		switch call {
		case 2:
			f.append(session, events.TurnStart, events.TurnStartData{Background: true})
		case 25:
			f.answer(session, "q-7")
		}
	}
	result, err := Wait(context.Background(), f.log, f.options("q-7"))
	if err != nil || result.AnsweredInSession || !strings.Contains(result.Note, `"Push"`) {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestWaitReturnsAtOnceForADeliveredQuestion(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.ask(session, "q-4")
	f.append(session, events.QuestionAnswered, events.AnswerData{ID: "q-4", Answer: "Push", By: "Robert"})
	f.append(session, events.QuestionDelivered, events.DeliveredData{ID: "q-4"})
	result, err := Wait(context.Background(), f.log, f.options("q-4"))
	if err != nil || !result.AlreadyDelivered || f.sleeps != 0 {
		t.Fatalf("result = %+v, %v after %d sleeps", result, err, f.sleeps)
	}
}

func TestWaitFailsForAnUnknownQuestionAndAfterTheTimeout(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	if _, err := Wait(context.Background(), f.log, f.options("q-missing")); !errors.Is(err, ErrUnknownQuestion) {
		t.Fatalf("unknown question: err = %v", err)
	}
	f.ask(session, "q-5")
	options := f.options("q-5")
	options.Timeout = 5 * time.Second
	if _, err := Wait(context.Background(), f.log, options); !errors.Is(err, ErrTimeout) {
		t.Fatalf("timeout: err = %v", err)
	}
	if f.sleeps != 5 {
		t.Fatalf("sleeps = %d, want 5", f.sleeps)
	}
	if got := f.delivered(); len(got) != 0 {
		t.Fatalf("delivered = %v", got)
	}
}

func TestWaitStopsWhenCancelled(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.ask(session, "q-6")
	ctx, cancel := context.WithCancel(context.Background())
	options := f.options("q-6")
	options.Sleep = nil // the real sleep honours ctx
	cancel()
	if _, err := Wait(ctx, f.log, options); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestWaitRefusesAnInvalidProject(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	options := f.options("q-1")
	options.Project = "../beta"
	if _, err := Wait(context.Background(), f.log, options); err == nil {
		t.Fatal("an invalid project was accepted")
	}
}
