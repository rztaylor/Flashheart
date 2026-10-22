package mcpserver

import (
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/store"
)

func historyOf(t *testing.T, e *env, id string) []board.History {
	t.Helper()
	ticket, err := e.store.TicketFile("demo", id)
	if err != nil {
		t.Fatal(err)
	}
	activity, _, err := e.store.ReadTicketActivity("demo", path.Base(path.Dir(ticket)))
	if err != nil {
		t.Fatal(err)
	}
	return activity.History
}

// ask_human about a ticket keeps the question in the ticket's history,
// naming the asking run, the time, the question and its options (FH-190);
// a question about no ticket writes nothing to any ticket.
func TestAskHumanKeepsTheQuestionInTheTicketsHistory(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "A"})
	other := e.ticket(store.NewTicket{Title: "B"})
	e.startSession(session)
	before := len(historyOf(t, e, id))
	out := e.ok("ask_human", map[string]any{"ticket": id, "kind": "decision", "text": "Which schema?", "options": []string{"v1", "v2"}})
	question := strings.TrimSpace(out[strings.Index(out, "question=")+len("question="):])

	history := historyOf(t, e, id)
	if len(history) != before+1 {
		t.Fatalf("history = %+v", history)
	}
	asked := history[len(history)-1]
	if asked.By != session || !asked.Time.Equal(e.now) || len(asked.Changes) != 1 {
		t.Fatalf("asked = %+v", asked)
	}
	change := asked.Changes[0]
	if change.What != board.ChangeAsked || change.Question.ID != question || change.Question.Kind != "decision" ||
		change.Question.Text != "Which schema?" || !slices.Equal(change.Question.Options, []string{"v1", "v2"}) {
		t.Errorf("change = %+v", change)
	}

	unrelated := len(historyOf(t, e, other))
	e.ok("ask_human", map[string]any{"kind": "question", "text": "Ship it?"})
	if len(historyOf(t, e, id)) != before+1 || len(historyOf(t, e, other)) != unrelated {
		t.Error("a question about no ticket wrote to a ticket")
	}
}
