package api

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/protocol"
)

// AnswerRequest is POST /api/questions/{id}/answer (RUN-8, CARD-6).
type AnswerRequest struct {
	Answer string `json:"answer"`
}

func (b boardAPI) answer(w http.ResponseWriter, r *http.Request) {
	if b.writer(w) == nil {
		return
	}
	if b.events == nil {
		writeError(w, http.StatusNotImplemented, "read_only", "This server does not record answers")
		return
	}
	var request AnswerRequest
	if !decode(w, r, &request) {
		return
	}
	answer := strings.TrimSpace(request.Answer)
	if answer == "" || utf8.RuneCountInString(answer) > events.MaxAnswerText || strings.ContainsRune(answer, 0) {
		writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("An answer is 1 to %d characters", events.MaxAnswerText))
		return
	}
	// Check and record under one lock, against a fresh snapshot, so two
	// tabs answering at once cannot both be accepted.
	b.answering.Lock()
	defer b.answering.Unlock()
	snapshot, err := b.board.Rebuild()
	if err != nil || snapshot == nil {
		writeError(w, http.StatusServiceUnavailable, "board_unavailable", "The board could not be read")
		return
	}
	now := b.now().UTC()
	answeredBy := b.answeredBy
	question, run, ok := snapshot.Question(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "No such question in the last two days")
		return
	}
	if question.Answered() {
		writeError(w, http.StatusConflict, "already_answered", "This question has already been answered")
		return
	}
	if question.AnsweredInSession {
		writeError(w, http.StatusConflict, "answered_in_session", "This question was answered in the session's own chat")
		return
	}
	// The inbox carries the answer to the run (HOOK-5) and the log records
	// it; the ticket keeps it for whoever reads the ticket later (STO-6).
	// Queueing first means a failure leaves the question unanswered, so it
	// can be answered again; an answer queued twice is delivered once.
	err = b.events.QueueAnswer(run.Project, run.ID, events.Delivery{ID: question.ID, Run: run.ID, Ticket: question.Ticket, Question: question.Text, Answer: answer, By: answeredBy})
	if err == nil {
		err = b.events.Append(events.Event{Time: now, Run: run.ID, Agent: run.Agent, Kind: events.QuestionAnswered, Project: run.Project,
			Data: events.AnswerData{ID: question.ID, Answer: answer, By: answeredBy}})
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	var warnings []string
	if question.Ticket != "" {
		if project, ticket, found := snapshot.FindTicket(question.Ticket); found {
			note := fmt.Sprintf("- %s · Question from %s — %q Answer (%s): %q.", now.Format("2006-01-02"), protocol.ShortRun(run.ID), question.Text, answeredBy, answer)
			if _, err := b.write.UpdateTicket(project.Name, ticket.ID, "", func(data []byte) ([]byte, error) {
				return mdfile.AppendToSection(data, "Notes", note)
			}); err != nil {
				warnings = append(warnings, "The answer was sent, but it could not be added to "+ticket.ID+"'s notes: "+err.Error())
			}
		}
	}
	b.saved(w, http.StatusOK, "", warnings)
}
