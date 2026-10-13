package events

import (
	"os"
	"slices"
	"testing"

	"github.com/rztaylor/flashheart/internal/store"
)

// sampleLog is the log of a copy of the sample board, whose project alpha
// has tickets, and its store.
func sampleLog(t *testing.T) (*Log, *store.Store) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../testdata/boards/sample")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s), s
}

// A comment a person sent to a session (FH-127) is delivered as it is in
// the ticket then, edits included, and recorded there as delivered; one
// sent to another run, delivered already or gone is not handed over; one
// whose ticket cannot be read goes as it was queued (nothing taken is
// lost) and the failure is reported.
func TestDeliverHandsOverASentCommentAsItIsNow(t *testing.T) {
	t.Parallel()

	log, s := sampleLog(t)
	robert := s.As("Robert")
	sent, hash, err := robert.AddComment("alpha", "AL-4", store.Merge, store.NewComment{Body: "Use the blue one.", SentTo: "claude:s"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := robert.EditComment("alpha", "AL-4", sent.ID, store.Unchanged(hash), "Use the green one."); err != nil {
		t.Fatal(err)
	}
	other, _, err := robert.AddComment("alpha", "AL-4", store.Merge, store.NewComment{Body: "Not yours.", SentTo: "claude:p"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []Entry{
		{Type: EntryComment, ID: sent.ID, Run: "claude:s", Ticket: "AL-4", Project: "alpha", Text: "Use the blue one.", By: "Robert"},
		{Type: EntryComment, ID: other.ID, Run: "claude:s", Ticket: "AL-4", Project: "alpha", Text: "Not yours.", By: "Robert"},
		{Type: EntryComment, ID: "deadbeef", Run: "claude:s", Ticket: "AL-4", Project: "alpha", Text: "Gone.", By: "Robert"},
		{Type: EntryComment, ID: "c-9", Run: "claude:s", Ticket: "AL-99", Project: "alpha", Text: "No ticket.", By: "Robert"},
		{Type: EntryComment, ID: "c-1", Run: "claude:s", Ticket: "AL-4", Text: "Queued before FH-127.", By: "Robert"},
	} {
		if err := log.Queue("alpha", e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Deliver(log, "alpha", []Mailbox{{Session: "claude:s"}}, nil, t0)
	if err == nil {
		t.Error("the unreadable ticket was not reported")
	}
	var texts []string
	for _, e := range got {
		texts = append(texts, e.Text)
	}
	if !slices.Equal(texts, []string{"Use the green one.", "No ticket.", "Queued before FH-127."}) {
		t.Fatalf("delivered %q", texts)
	}
	activity, _, _ := s.ReadTicketActivity("alpha", "AL-4-drag-and-drop")
	for _, c := range activity.Comments {
		if delivered := !c.Delivered.IsZero(); delivered != (c.ID == sent.ID) {
			t.Errorf("comment %s delivered %v", c.ID, c.Delivered)
		}
	}
	var recorded []string
	if err := log.Read("alpha", t0, func(e Event) {
		var d CommentDeliveredData
		if e.Kind == CommentDelivered && e.Decode(&d) == nil {
			recorded = append(recorded, d.Comment)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(recorded, []string{sent.ID, "c-9", "c-1"}) {
		t.Errorf("recorded %v", recorded)
	}
}
