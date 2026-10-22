package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
		Writer: files.As("Robert"),
		Events: log,
		// Who answers: the configured name, else the account (FH-8).
		AnsweredBy: "Robert",
		// The answer is stamped with the time it was given, not the index's.
		Now: func() time.Time { return time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC) },
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
	if !detail.NeedsYou || len(detail.Questions) != 1 || detail.Questions[0].ID != "q-1" || detail.Questions[0].RunShort != "claude:3f2a9c1e" {
		t.Fatalf("AL-1 needsYou %v questions %+v", detail.NeedsYou, detail.Questions)
	}
	var board BoardResponse
	getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
	for _, card := range board.Cards {
		if card.ID == "AL-1" && (!card.NeedsYou || card.OpenQuestions != 1) {
			t.Fatalf("AL-1 card = %+v", card)
		}
		// AL-3's session waits on a permission prompt, which it names first.
		if card.ID == "AL-3" && (card.Live == nil || card.Live.Reason != reasonJSON(runs.Reason{Kind: runs.ReasonPermission, Permission: "Bash"})) {
			t.Fatalf("AL-3 live = %+v", card.Live)
		}
	}
}

func TestAnsweringAQuestionRecordsAndQueuesIt(t *testing.T) {
	t.Parallel()

	handler, root, log := questionsAPI(t)
	send(t, handler, http.MethodPost, "/api/questions/q-1/answer", map[string]string{"answer": "  No  "}, http.StatusOK, nil)

	ticket, _ := os.ReadFile(filepath.Join(root, "alpha", "tickets", "AL-1-project-skeleton", "AL-1-project-skeleton.md"))
	if !strings.Contains(string(ticket), `- 2026-10-05 · Question from claude:3f2a9c1e — "Keep the old skeleton?" Answer (Robert): "No".`) {
		t.Fatalf("ticket notes:\n%s", ticket)
	}
	// The ticket's history keeps the answer with its question, by the
	// person, from the same write as the note (FH-190).
	history := ticketDetail(t, handler, "AL-1").History
	if len(history) == 0 {
		t.Fatal("no history")
	}
	answered := history[len(history)-1]
	if answered.By != "Robert" || len(answered.Changes) != 1 || answered.Changes[0].What != "answered" {
		t.Fatalf("history = %+v", answered)
	}
	if q := answered.Changes[0].Question; q == nil || q.ID != "q-1" || q.Text != "Keep the old skeleton?" || q.Answer != "No" {
		t.Fatalf("question = %+v", q)
	}
	waiting, err := log.Take("alpha", events.Mailbox{Session: sampleSession})
	if err != nil || len(waiting) != 1 || waiting[0].Type != events.EntryAnswer || waiting[0].Text != "No" || waiting[0].Run != sampleSession || waiting[0].Question != "Keep the old skeleton?" {
		t.Fatalf("inbox = %+v, %v", waiting, err)
	}
	var response RunsResponse
	getJSON(t, handler, "/api/runs", http.StatusOK, &response)
	if q := response.Runs[0].Questions[0]; q.Answer != "No" || q.AnsweredBy != "Robert" || q.Delivered {
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

func TestOnlyOneOfTwoConcurrentAnswersIsAccepted(t *testing.T) {
	t.Parallel()

	handler, _, log := questionsAPI(t)
	codes := make(chan int, 2)
	var wait sync.WaitGroup
	for _, answer := range []string{"Yes", "No"} {
		wait.Go(func() {
			body, _ := json.Marshal(map[string]string{"answer": answer})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/questions/q-1/answer", bytes.NewReader(body)))
			codes <- recorder.Code
		})
	}
	wait.Wait()
	close(codes)
	accepted := 0
	for code := range codes {
		if code == http.StatusOK {
			accepted++
		}
	}
	waiting, _ := log.Take("alpha", events.Mailbox{Session: sampleSession})
	if accepted != 1 || len(waiting) != 1 {
		t.Fatalf("accepted %d answers, inbox holds %d", accepted, len(waiting))
	}
}

// A question of an ended session is shown and answerable on its ticket but
// does not put the ticket in Needs you (FH-8).
func TestQuestionsOfEndedSessionsDoNotNeedYou(t *testing.T) {
	t.Parallel()

	handler, _, log := questionsAPI(t)
	if err := log.Append(events.Event{Time: time.Date(2026, 10, 4, 13, 29, 0, 0, time.UTC), Run: sampleSession, Agent: "claude", Kind: events.RunEnd, Project: "alpha", Data: events.RunEndData{Reason: "other"}}); err != nil {
		t.Fatal(err)
	}
	var board BoardResponse
	getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
	for _, card := range board.Cards {
		if card.ID == "AL-1" && (card.NeedsYou || card.OpenQuestions != 0) {
			t.Errorf("AL-1 card = needsYou %v, openQuestions %d", card.NeedsYou, card.OpenQuestions)
		}
	}
	detail := ticketDetail(t, handler, "AL-1")
	if len(detail.Questions) != 1 || !detail.Questions[0].SessionEnded {
		t.Fatalf("AL-1 questions = %+v", detail.Questions)
	}
	send(t, handler, http.MethodPost, "/api/questions/q-1/answer", map[string]string{"answer": "No"}, http.StatusOK, nil)
}

func TestAQuestionAnsweredInTheSessionNoLongerNeedsYou(t *testing.T) {
	t.Parallel()

	handler, _, log := questionsAPI(t)
	// The user replied in the session's chat instead of on the board.
	if err := log.Append(events.Event{Time: time.Date(2026, 10, 4, 13, 29, 0, 0, time.UTC), Run: sampleSession, Agent: "claude", Kind: events.TurnStart, Project: "alpha", Data: events.TurnStartData{}}); err != nil {
		t.Fatal(err)
	}
	var board BoardResponse
	getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
	for _, card := range board.Cards {
		if card.ID == "AL-1" && (card.NeedsYou || card.OpenQuestions != 0) {
			t.Errorf("AL-1 card = needsYou %v, openQuestions %d", card.NeedsYou, card.OpenQuestions)
		}
	}
	if detail := ticketDetail(t, handler, "AL-1"); detail.NeedsYou || len(detail.Questions) != 0 {
		t.Fatalf("AL-1 needsYou %v questions %+v", detail.NeedsYou, detail.Questions)
	}
	// The run keeps the question, marked as answered in the session.
	var response RunsResponse
	getJSON(t, handler, "/api/runs", http.StatusOK, &response)
	if q := response.Runs[0].Questions[0]; q.ID != "q-1" || !q.AnsweredInSession || !q.Delivered {
		t.Fatalf("question = %+v", q)
	}
	var refusal struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	send(t, handler, http.MethodPost, "/api/questions/q-1/answer", map[string]string{"answer": "No"}, http.StatusConflict, &refusal)
	if refusal.Error.Code != "answered_in_session" {
		t.Fatalf("refusal = %+v", refusal)
	}
}

// A subagent that asked a question and finished leaves it with its session
// (agent-protocol §4): while the session is live the question needs you, so
// the session's reason, the card's question count and the Overview's
// decisions (the runs API's open questions) agree (FH-112). Once the
// session ends, none of them counts it.
func TestASubagentsQuestionNeedsYouWhileItsSessionIsLive(t *testing.T) {
	t.Parallel()

	child := sampleSession + "/e1"
	lines := []string{
		// The session's tool use settles its permission prompt.
		`{"v":1,"ts":"2026-10-04T13:27:00.000Z","run":"` + sampleSession + `","agent":"claude","kind":"tool.used","project":"alpha","data":{"tool":"Bash","ok":true}}`,
		`{"v":1,"ts":"2026-10-04T13:28:00.000Z","run":"` + child + `","agent":"claude","kind":"run.start","project":"alpha","data":{"kind":"subagent","parent":"` + sampleSession + `","agent_type":"Explore"}}`,
		`{"v":1,"ts":"2026-10-04T13:28:30.000Z","run":"` + child + `","agent":"claude","kind":"question.asked","project":"alpha","data":{"id":"q-7","ticket":"AL-3","kind":"decision","text":"Which layout?"}}`,
		`{"v":1,"ts":"2026-10-04T13:29:00.000Z","run":"` + child + `","agent":"claude","kind":"run.end","project":"alpha","data":{"reason":"completed"}}`,
	}
	ended := `{"v":1,"ts":"2026-10-04T13:29:30.000Z","run":"` + sampleSession + `","agent":"claude","kind":"run.end","project":"alpha","data":{"reason":"other"}}`
	now := time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC)

	type agreement struct {
		reason    NeedsReasonJSON
		cardCount int
		overview  int
	}
	observe := func(handler http.Handler) agreement {
		t.Helper()
		var seen agreement
		var response RunsResponse
		getJSON(t, handler, "/api/runs", http.StatusOK, &response)
		for _, run := range response.Runs {
			if run.ID == sampleSession {
				seen.reason = run.Reason
			}
			// The Overview's Needs your decision lists a run's questions
			// that are neither delivered, answered in the session nor
			// from an ended session (frontend model/overview.ts).
			for _, q := range run.Questions {
				if q.ID == "q-7" && !q.Delivered && !q.AnsweredInSession && !q.SessionEnded {
					seen.overview++
				}
			}
		}
		var board BoardResponse
		getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &board)
		seen.cardCount = cardByID(t, board.Cards, "AL-3").OpenQuestions
		return seen
	}

	live := observe(runsAPI(t, now, lines...))
	want := agreement{reason: reasonJSON(runs.Reason{Kind: runs.ReasonQuestion, Question: "decision", WaitingOn: child, AgentType: "Explore"}), cardCount: 1, overview: 1}
	if live != want {
		t.Fatalf("with the session live: %+v, want %+v", live, want)
	}
	if over := observe(runsAPI(t, now, append(lines, ended)...)); over != (agreement{}) {
		t.Fatalf("with the session ended: %+v, want nothing", over)
	}
}
