package api

import (
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// countingSource counts the board reads behind the API's index (FH-96).
type countingSource struct {
	index.Source
	reads atomic.Int64
}

func (c *countingSource) ReadBoard() (board.Board, string, error) {
	c.reads.Add(1)
	return c.Source.ReadBoard()
}

// A write from the UI reads the board once, for the revision it answers
// with; answering a question checks it against freshly folded runs, not a
// second board read (FH-96).
func TestAWriteReadsTheBoardOnce(t *testing.T) {
	t.Parallel()

	root := copyBoard(t, "sample")
	files := store.New(root)
	t.Cleanup(func() { files.Close() })
	log := events.New(files)
	asked := events.Event{Time: time.Date(2026, 10, 4, 13, 28, 0, 0, time.UTC), Run: sampleSession, Agent: "claude", Kind: events.QuestionAsked, Project: "alpha",
		Data: events.QuestionData{ID: "q-1", Ticket: "AL-1", Kind: "decision", Text: "Keep the old skeleton?"}}
	if err := log.Append(asked); err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC) }
	source := &countingSource{Source: files}
	handler := New(Options{
		Info:   Info{Root: root, Theme: "system"},
		Board:  index.New(source, index.Options{Now: clock, Events: log, Runs: runs.DefaultSettings()}),
		Files:  files,
		Writer: files,
		Events: log,
		Now:    clock,
	})

	detail := ticketDetail(t, handler, "AL-2")
	reads := source.reads.Load()
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", MoveRequest{Base: detail.Hash, To: "done"}, http.StatusOK, nil)
	if got := source.reads.Load() - reads; got != 1 {
		t.Errorf("a move read the board %d times, want 1", got)
	}

	reads = source.reads.Load()
	send(t, handler, http.MethodPost, "/api/questions/q-1/answer", map[string]string{"answer": "No"}, http.StatusOK, nil)
	if got := source.reads.Load() - reads; got != 1 {
		t.Errorf("an answer read the board %d times, want 1", got)
	}
	var response RunsResponse
	getJSON(t, handler, "/api/runs", http.StatusOK, &response)
	if q := response.Runs[0].Questions[0]; q.Answer != "No" {
		t.Fatalf("question after answer = %+v", q)
	}
}
