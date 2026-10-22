package questionhistory

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

const (
	inSession = "claude:aaaaaaaa-0000-0000-0000-000000000000"
	ended     = "claude:bbbbbbbb-0000-0000-0000-000000000000"
	onBoard   = "claude:cccccccc-0000-0000-0000-000000000000"
)

var now = time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)

func at(minute int) time.Time { return time.Date(2026, 10, 4, 13, minute, 0, 0, time.UTC) }

type fixture struct {
	t     *testing.T
	root  string
	files *store.Store
	board *index.Index
	log   bytes.Buffer
}

// newFixture serves a copy of the sample board with three sessions that
// asked about its tickets: one the user answered in its chat, one that
// ended with its question open, and one answered on the board by a binary
// that kept no history.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "boards", "sample"))); err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, root: root, files: store.New(root)}
	t.Cleanup(func() { f.files.Close() })
	f.files.SetClock(func() time.Time { return now })
	log := events.New(f.files)
	ev := func(minute int, run string, kind events.Kind, data any) events.Event {
		return events.Event{Time: at(minute), Run: run, Agent: "claude", Kind: kind, Project: "alpha", Data: data}
	}
	start := events.RunStartData{Kind: events.KindSession, Cwd: "/src/alpha"}
	if err := log.Append(
		ev(10, inSession, events.RunStart, start),
		ev(11, inSession, events.QuestionAsked, events.QuestionData{ID: "q-chat", Ticket: "AL-1", Kind: "decision", Text: "Keep the skeleton?", Options: []string{"Yes", "No"}}),
		ev(12, inSession, events.TurnStart, events.TurnStartData{}),
		ev(10, ended, events.RunStart, start),
		ev(11, ended, events.QuestionAsked, events.QuestionData{ID: "q-ended", Ticket: "AL-2", Kind: "question", Text: "Which column first?"}),
		ev(13, ended, events.RunEnd, events.RunEndData{}),
		ev(10, onBoard, events.RunStart, start),
		ev(15, onBoard, events.QuestionAsked, events.QuestionData{ID: "q-board", Ticket: "AL-1", Kind: "question", Text: "Ship it?"}),
		ev(16, onBoard, events.QuestionAnswered, events.AnswerData{ID: "q-board", Answer: "Not yet", By: "Robert"}),
		ev(17, onBoard, events.QuestionAsked, events.QuestionData{ID: "q-free", Kind: "question", Text: "About nothing?"}),
	); err != nil {
		t.Fatal(err)
	}
	f.board = index.New(f.files, index.Options{Now: func() time.Time { return now }, Events: log, Runs: runs.DefaultSettings()})
	return f
}

func (f *fixture) pass(r *Recorder) {
	f.t.Helper()
	snapshot, err := f.board.Rebuild()
	if err != nil {
		f.t.Fatal(err)
	}
	r.Pass(snapshot)
}

func (f *fixture) history(folder string) []board.History {
	f.t.Helper()
	activity, _, err := f.files.ReadTicketActivity("alpha", folder)
	if err != nil {
		f.t.Fatal(err)
	}
	return activity.History
}

type recorded struct {
	what, question, by string
	at                 time.Time
}

func entries(list []board.History) []recorded {
	var out []recorded
	for _, entry := range list {
		for _, change := range entry.Changes {
			if board.IsQuestionChange(change.What) {
				out = append(out, recorded{change.What, change.Question.ID, entry.By, entry.Time})
			}
		}
	}
	return out
}

func same(a, b []recorded) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index].what != b[index].what || a[index].question != b[index].question || a[index].by != b[index].by || !a[index].at.Equal(b[index].at) {
			return false
		}
	}
	return true
}

// Serve keeps in each ticket's history what the run window shows became
// of its questions (FH-190): asked, answered in the session's chat (by
// the asking run, when the prompt came), left open when the session ended
// (when serve saw it) and answered on the board (by the person, when), each
// once; a question about no ticket writes nothing.
func TestPassRecordsWhatBecameOfEachQuestion(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.pass(New(f.files, &f.log))

	wantAL1 := []recorded{
		{board.ChangeAsked, "q-chat", inSession, at(11)},
		{board.ChangeAnsweredInSession, "q-chat", inSession, at(12)},
		{board.ChangeAsked, "q-board", onBoard, at(15)},
		{board.ChangeAnswered, "q-board", "Robert", at(16)},
	}
	if got := entries(f.history("AL-1-project-skeleton")); !same(got, wantAL1) {
		t.Errorf("AL-1 history = %+v\nwant %+v", got, wantAL1)
	}
	wantAL2 := []recorded{
		{board.ChangeAsked, "q-ended", ended, at(11)},
		{board.ChangeSessionEnded, "q-ended", ended, now},
	}
	if got := entries(f.history("AL-2-board-columns")); !same(got, wantAL2) {
		t.Errorf("AL-2 history = %+v\nwant %+v", got, wantAL2)
	}
	answered := f.history("AL-1-project-skeleton")
	if q := answered[len(answered)-1].Changes[0].Question; q.Text != "Ship it?" || q.Answer != "Not yet" {
		t.Errorf("answered question = %+v", q)
	}
	if f.log.Len() != 0 {
		t.Errorf("log = %s", f.log.String())
	}

	// Another pass, or another serve process, writes nothing more.
	f.pass(New(f.files, &f.log))
	if got := entries(f.history("AL-1-project-skeleton")); !same(got, wantAL1) {
		t.Errorf("AL-1 history after a second pass = %+v", got)
	}
	if got := entries(f.history("AL-2-board-columns")); !same(got, wantAL2) {
		t.Errorf("AL-2 history after a second pass = %+v", got)
	}
}
