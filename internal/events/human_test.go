package events

import (
	"testing"
	"time"
)

// A board write is recorded as the human's, naming the ticket and at most
// the fields or columns it changed: never their values (agent-protocol §3).
func TestByHumanRecordsTheBoardWrite(t *testing.T) {
	t.Parallel()

	moved := ByHuman(t0, "alpha", TicketMoved, TicketData{Ticket: "AL-2", From: "review", To: "done"})
	line, err := moved.MarshalLine()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"v":1,"ts":"2026-10-05T14:12:09.123Z","run":"human","agent":"human","kind":"ticket.moved","project":"alpha","data":{"ticket":"AL-2","by":"human","from":"review","to":"done"}}` + "\n"
	if string(line) != want {
		t.Errorf("line =\n%s\nwant\n%s", line, want)
	}

	log, _ := newRoot(t, "alpha")
	if err := log.Append(moved, ByHuman(t0, "alpha", TicketUpdated, TicketData{Ticket: "AL-3", Fields: []string{"criteria"}})); err != nil {
		t.Fatal(err)
	}
	var read []Event
	if err := log.Read("alpha", time.Time{}, func(e Event) { read = append(read, e) }); err != nil {
		t.Fatal(err)
	}
	if len(read) != 2 || read[1].Run != Human || read[1].Kind != TicketUpdated {
		t.Fatalf("read = %+v", read)
	}
}

func TestHumanActivityOf(t *testing.T) {
	t.Parallel()

	agent := "claude:s1"
	for _, test := range []struct {
		name  string
		event Event
		want  HumanActivity
		ok    bool
	}{
		{"a board move", ByHuman(t0, "alpha", TicketMoved, TicketData{Ticket: "AL-2", From: "review", To: "done"}),
			HumanActivity{Time: t0, Project: "alpha", Kind: TicketMoved, Ticket: "AL-2"}, true},
		{"a board edit", ByHuman(t0, "alpha", TicketUpdated, TicketData{Ticket: "AL-3", Fields: []string{"priority"}}),
			HumanActivity{Time: t0, Project: "alpha", Kind: TicketUpdated, Ticket: "AL-3"}, true},
		{"a new ticket on the board", ByHuman(t0, "beta", TicketCreated, TicketData{Ticket: "BE-2"}),
			HumanActivity{Time: t0, Project: "beta", Kind: TicketCreated, Ticket: "BE-2"}, true},
		// An answer is recorded on the asking run, by the person's name.
		{"an answer", Event{Time: t0, Run: agent, Agent: "claude", Kind: QuestionAnswered, Project: "alpha", Data: AnswerData{ID: "q-1", Answer: "Yes", By: "Robert"}},
			HumanActivity{Time: t0, Project: "alpha", Kind: QuestionAnswered}, true},
		{"an agent's move", Event{Time: t0, Run: agent, Agent: "claude", Kind: TicketMoved, Project: "alpha", Data: TicketData{Ticket: "AL-2", To: "review", By: agent}}, HumanActivity{}, false},
		{"an agent's edit", Event{Time: t0, Run: agent, Agent: "claude", Kind: TicketUpdated, Project: "alpha", Data: TicketData{Ticket: "AL-2", Fields: []string{"tags"}}}, HumanActivity{}, false},
		{"an agent's claim", Event{Time: t0, Run: agent, Agent: "claude", Kind: Claim, Project: "alpha", Data: TicketData{Ticket: "AL-2"}}, HumanActivity{}, false},
		{"a prompt", Event{Time: t0, Run: agent, Agent: "claude", Kind: TurnStart, Project: "alpha", Data: TurnStartData{}}, HumanActivity{}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Read back from a line, as readers see it.
			line, err := test.event.MarshalLine()
			if err != nil {
				t.Fatal(err)
			}
			read, parsed := parseLine(line[:len(line)-1])
			if !parsed {
				t.Fatalf("line does not parse: %s", line)
			}
			got, ok := HumanActivityOf(read)
			if ok != test.ok || got != test.want {
				t.Errorf("HumanActivityOf = %+v, %v; want %+v, %v", got, ok, test.want, test.ok)
			}
		})
	}
}
