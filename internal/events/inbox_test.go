package events

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func entryIDs(list []Entry) []string {
	var ids []string
	for _, e := range list {
		ids = append(ids, string(e.Type)+":"+e.ID)
	}
	return ids
}

// A session's inbox holds typed entries (D54): answers, comments and
// change requests, each taken once and recorded delivered by its own kind.
func TestTheInboxCarriesEachEntryType(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	queued := []Entry{
		{Type: EntryAnswer, ID: "q-1", Run: "claude:s/a1", Ticket: "AL-1", Question: "Q?", Text: "A", By: "Robert", Time: t0},
		{Type: EntryComment, ID: "c-1", Run: "claude:s", Ticket: "AL-1", Text: "Use the blue one.", By: "Robert", Time: t0},
		{Type: EntryChangeRequest, ID: "c-2", Run: "claude:s", Ticket: "AL-1", Text: "Rename the button.", By: "Robert", Time: t0},
	}
	for _, e := range queued {
		if err := log.Queue("alpha", e); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "alpha", ".flashheart", "inbox", "claude--s.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"type":"change-request"`) || !strings.Contains(string(data), `"type":"comment"`) {
		t.Fatalf("inbox = %s", data)
	}
	got, err := Deliver(log, "alpha", []Mailbox{{Session: "claude:s"}}, nil, t0)
	if err != nil || !slices.Equal(got, queued) {
		t.Fatalf("delivered %+v, %v", got, err)
	}
	var kinds []string
	if err := log.Read("alpha", t0, func(e Event) {
		switch e.Kind {
		case QuestionDelivered:
			var d DeliveredData
			_ = e.Decode(&d)
			kinds = append(kinds, string(e.Kind)+" "+e.Run+" "+d.ID)
		case CommentDelivered:
			var d CommentDeliveredData
			_ = e.Decode(&d)
			kinds = append(kinds, string(e.Kind)+" "+e.Run+" "+d.Ticket+" "+d.Comment+" "+d.Type)
		}
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"question.delivered claude:s/a1 q-1",
		"comment.delivered claude:s AL-1 c-1 comment",
		"comment.delivered claude:s AL-1 c-2 change-request",
	}
	if !slices.Equal(kinds, want) {
		t.Fatalf("recorded %v", kinds)
	}
	if again, err := Deliver(log, "alpha", []Mailbox{{Session: "claude:s"}}, nil, t0); err != nil || len(again) != 0 {
		t.Fatalf("second delivery = %+v, %v", again, err)
	}
}

func TestQueueRefusesAnEntryItCannotDeliver(t *testing.T) {
	t.Parallel()

	log, _ := newRoot(t, "alpha")
	for name, e := range map[string]Entry{
		"no type":      {ID: "q-1", Run: "claude:s", Text: "A"},
		"unknown type": {Type: "memo", ID: "m-1", Run: "claude:s", Text: "A"},
		"no id":        {Type: EntryComment, Run: "claude:s", Text: "A"},
		"bad run":      {Type: EntryComment, ID: "c-1", Run: "nobody", Text: "A"},
	} {
		if err := log.Queue("alpha", e); err == nil {
			t.Errorf("%s: queued", name)
		}
	}
}

// Queue bounds and scrubs what it writes, as the log does (SEC-3).
func TestQueueBoundsTheEntry(t *testing.T) {
	t.Parallel()

	log, _ := newRoot(t, "alpha")
	long := strings.Repeat("x", MaxInboxText+100)
	if err := log.Queue("alpha", Entry{Type: EntryComment, ID: "c-1", Run: "claude:s", Text: long, By: "Robert"}); err != nil {
		t.Fatal(err)
	}
	got, err := log.Take("alpha", Mailbox{Session: "claude:s"})
	if err != nil || len(got) != 1 || len(got[0].Text) > MaxInboxText+len("…") {
		t.Fatalf("taken %d entries, text %d bytes, %v", len(got), len(got[0].Text), err)
	}
}

// Before D54 the inbox was answers/ and its lines had no type and an
// `answer` field; such an inbox is still delivered, as answers (D54).
func TestAnInboxInTheOldLocationIsStillDelivered(t *testing.T) {
	t.Parallel()

	old := `{"id":"q-1","run":"claude:s","ticket":"AL-1","question":"Q?","answer":"A","by":"Robert"}` + "\n"
	write := func(t *testing.T, root, dir string) {
		t.Helper()
		folder := filepath.Join(root, "alpha", ".flashheart", dir)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "claude--s.jsonl"), []byte(old), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	answer := Entry{Type: EntryAnswer, ID: "q-1", Run: "claude:s", Ticket: "AL-1", Question: "Q?", Text: "A", By: "Robert"}

	t.Run("an old line in the inbox reads as an answer", func(t *testing.T) {
		t.Parallel()
		log, root := newRoot(t, "alpha")
		write(t, root, "inbox")
		got, err := Deliver(log, "alpha", []Mailbox{{Session: "claude:s"}}, nil, t0)
		if err != nil || !slices.Equal(got, []Entry{answer}) {
			t.Fatalf("delivered %+v, %v", got, err)
		}
	})
	t.Run("a reader that checks the old location delivers it", func(t *testing.T) {
		t.Parallel()
		log, root := newRoot(t, "alpha")
		write(t, root, "answers")
		comment := Entry{Type: EntryComment, ID: "c-1", Run: "claude:s", Text: "Hi", By: "Robert"}
		// Old and new both present: both are delivered, the old first,
		// each entry once.
		if err := os.MkdirAll(filepath.Join(root, "alpha", ".flashheart", "inbox"), 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"type":"comment","id":"c-1","run":"claude:s","text":"Hi","by":"Robert"}` + "\n" + old
		if err := os.WriteFile(filepath.Join(root, "alpha", ".flashheart", "inbox", "claude--s.jsonl"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := Deliver(log, "alpha", []Mailbox{{Session: "claude:s", Legacy: true}}, nil, t0)
		if err != nil || !slices.Equal(entryIDs(got), []string{"answer:q-1", "comment:c-1"}) || got[1] != comment {
			t.Fatalf("delivered %+v, %v", got, err)
		}
		for _, dir := range []string{"inbox", "answers"} {
			if _, err := os.Stat(filepath.Join(root, "alpha", ".flashheart", dir, "claude--s.jsonl")); !os.IsNotExist(err) {
				t.Errorf("%s inbox left: %v", dir, err)
			}
		}
	})
	t.Run("a reader that checks one file leaves the old location", func(t *testing.T) {
		t.Parallel()
		log, root := newRoot(t, "alpha")
		write(t, root, "answers")
		if got, err := Deliver(log, "alpha", []Mailbox{{Session: "claude:s"}}, nil, t0); err != nil || len(got) != 0 {
			t.Fatalf("delivered %+v, %v", got, err)
		}
		// Queueing the next entry moves the old inbox in front of it.
		if err := log.Queue("alpha", Entry{Type: EntryComment, ID: "c-1", Run: "claude:s", Text: "Hi", By: "Robert"}); err != nil {
			t.Fatal(err)
		}
		got, err := Deliver(log, "alpha", []Mailbox{{Session: "claude:s"}}, nil, t0)
		if err != nil || !slices.Equal(entryIDs(got), []string{"answer:q-1", "comment:c-1"}) {
			t.Fatalf("delivered %+v, %v", got, err)
		}
	})
}

// A previous session's inbox, taken by a new session's recovery note,
// gives up only its answers: comments and change requests were sent to
// that session and stay for it (AnswersOnly).
func TestAnswersOnlyLeavesCommentsAndChangeRequests(t *testing.T) {
	t.Parallel()

	log, _ := newRoot(t, "alpha")
	for _, e := range []Entry{
		{Type: EntryComment, ID: "c-1", Run: "claude:p", Text: "Hi", By: "Robert"},
		{Type: EntryAnswer, ID: "q-1", Run: "claude:p", Question: "Q?", Text: "A", By: "Robert"},
		{Type: EntryChangeRequest, ID: "c-2", Run: "claude:p", Text: "Fix it", By: "Robert"},
	} {
		if err := log.Queue("alpha", e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Deliver(log, "alpha", []Mailbox{{Session: "claude:p", AnswersOnly: true}}, nil, t0)
	if err != nil || !slices.Equal(entryIDs(got), []string{"answer:q-1"}) {
		t.Fatalf("delivered %+v, %v", got, err)
	}
	left, err := log.Take("alpha", Mailbox{Session: "claude:p"})
	if err != nil || !slices.Equal(entryIDs(left), []string{"comment:c-1", "change-request:c-2"}) {
		t.Fatalf("left %+v, %v", left, err)
	}
}

// An entry of a type this binary does not know (a newer binary queued it)
// stays in the inbox rather than being taken and lost; malformed lines go.
func TestTakeLeavesUnknownTypesAndDropsMalformedLines(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	folder := filepath.Join(root, "alpha", ".flashheart", "inbox")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "not json\n" + `{"type":"memo","id":"m-1","run":"claude:s","text":"later"}` + "\n" + `{"type":"comment","id":"c-1","run":"claude:s","text":"Hi","by":"Robert"}` + "\n"
	if err := os.WriteFile(filepath.Join(folder, "claude--s.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := log.Take("alpha", Mailbox{Session: "claude:s"})
	if err != nil || !slices.Equal(entryIDs(got), []string{"comment:c-1"}) {
		t.Fatalf("taken %+v, %v", got, err)
	}
	left, err := os.ReadFile(filepath.Join(folder, "claude--s.jsonl"))
	if err != nil || string(left) != `{"type":"memo","id":"m-1","run":"claude:s","text":"later"}`+"\n" {
		t.Fatalf("left %q, %v", left, err)
	}
}
