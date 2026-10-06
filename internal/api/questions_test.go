package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// questionsAPI serves the sample board with a writable store and log, and
// the sample session's question about AL-1 (it is linked to AL-3).
func questionsAPI(t *testing.T) (http.Handler, string, *events.Log) {
	t.Helper()
	root := copyBoard(t, "sample")
	files := store.New(root)
	t.Cleanup(func() { files.Close() })
	log := events.New(files)
	asked := events.Event{Time: time.Date(2026, 10, 4, 13, 28, 0, 0, time.UTC), Run: sampleSession, Agent: "claude", Kind: events.QuestionAsked, Project: "alpha",
		Data: events.QuestionData{ID: "q-1", Ticket: "AL-1", Kind: "decision", Text: "Keep the old skeleton?", Options: []string{"Yes", "No"}}}
	if err := log.Append(asked); err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC) }
	files.SetClock(clock)
	return New(Options{
		Info:   Info{Root: root, Theme: "system"},
		Board:  index.New(files, index.Options{Now: clock, Events: log, Runs: runs.DefaultSettings()}),
		Files:  files,
		Writer: files,
		Events: log,
	}), root, log
}

func TestQuestionsAreShownOnRunsCardsAndTickets(t *testing.T) {
	t.Parallel()

	handler, _, _ := questionsAPI(t)
	var response RunsResponse
	getJSON(t, handler, "/api/runs", http.StatusOK, &response)
	questions := response.Runs[0].Questions
	if len(questions) != 1 || questions[0].ID != "q-1" || questions[0].Ticket != "AL-1" || questions[0].Text != "Keep the old skeleton?" || len(questions[0].Options) != 2 {
		t.Fatalf("questions = %+v", questions)
	}
	// AL-1 is not the session's ticket, but its open question needs you (VIEW-2).
	detail := ticketDetail(t, handler, "AL-1")
	if !detail.NeedsYou || len(detail.Questions) != 1 || detail.Questions[0].ID != "q-1" {
		t.Fatalf("AL-1 needsYou %v questions %+v", detail.NeedsYou, detail.Questions)
	}
	var board BoardResponse
	getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
	for _, card := range board.Cards {
		if card.ID == "AL-1" && (!card.NeedsYou || card.OpenQuestions != 1) {
			t.Fatalf("AL-1 card = %+v", card)
		}
		// AL-3's session waits on a permission prompt, which it names first.
		if card.ID == "AL-3" && (card.Live == nil || card.Live.Permission != "Bash" || card.Live.Question != "") {
			t.Fatalf("AL-3 live = %+v", card.Live)
		}
	}
}

func TestAnsweringAQuestionRecordsAndQueuesIt(t *testing.T) {
	t.Parallel()

	handler, root, log := questionsAPI(t)
	send(t, handler, http.MethodPost, "/api/questions/q-1/answer", map[string]string{"answer": "  No  "}, http.StatusOK, nil)

	ticket, _ := os.ReadFile(filepath.Join(root, "alpha", "tickets", "AL-1-project-skeleton", "AL-1-project-skeleton.md"))
	if !strings.Contains(string(ticket), `- 2026-10-04 · Question from claude:3f2a9c1e — "Keep the old skeleton?" Answer (human): "No".`) {
		t.Fatalf("ticket notes:\n%s", ticket)
	}
	waiting, err := log.TakeAnswers("alpha", sampleSession)
	if err != nil || len(waiting) != 1 || waiting[0].Answer != "No" || waiting[0].Run != sampleSession || waiting[0].Question != "Keep the old skeleton?" {
		t.Fatalf("inbox = %+v, %v", waiting, err)
	}
	var response RunsResponse
	getJSON(t, handler, "/api/runs", http.StatusOK, &response)
	if q := response.Runs[0].Questions[0]; q.Answer != "No" || q.AnsweredBy != "human" || q.Delivered {
		t.Fatalf("question after answer = %+v", q)
	}

	send(t, handler, http.MethodPost, "/api/questions/q-1/answer", map[string]string{"answer": "Yes"}, http.StatusConflict, nil)
	send(t, handler, http.MethodPost, "/api/questions/q-9/answer", map[string]string{"answer": "Yes"}, http.StatusNotFound, nil)
	send(t, handler, http.MethodPost, "/api/questions/q-1/answer", map[string]string{"answer": " "}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPost, "/api/questions/q-1/answer", map[string]string{"answer": strings.Repeat("x", 1001)}, http.StatusBadRequest, nil)
}

func TestAnsweringNeedsTheWriteSide(t *testing.T) {
	t.Parallel()

	handler := runsAPI(t, time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC))
	send(t, handler, http.MethodPost, "/api/questions/q-1/answer", map[string]string{"answer": "Yes"}, http.StatusNotImplemented, nil)
}
