package await

import (
	"context"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// A comment the person sent from the board while the session waits
// (FH-127) wakes it, is handed over as the user's words and is marked
// delivered on its ticket.
func TestWaitDeliversACommentSentFromTheBoard(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	s, err := store.Open(f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	created, err := s.CreateTicket(project, store.NewTicket{Title: "Colour the header", Key: "AL"})
	if err != nil {
		t.Fatal(err)
	}
	f.ask(session, "q-1")
	var comment string
	f.onSleep = func(call int) {
		if call != 2 {
			return
		}
		posted, _, err := s.As("Robert").AddComment(project, created.ID, store.Merge, store.NewComment{Body: "Use the blue one.", SentTo: session})
		if err != nil {
			t.Fatal(err)
		}
		comment = posted.ID
		if err := f.log.Queue(project, events.Entry{Type: events.EntryComment, ID: comment, Run: session, Ticket: created.ID, Project: project, Text: posted.Body, By: "Robert"}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Wait(context.Background(), f.log, f.options("q-1"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Note, `- `+created.ID+` (Robert): "Use the blue one."`) {
		t.Fatalf("note:\n%s", result.Note)
	}
	activity, _, err := s.ReadTicketActivity(project, created.Folder)
	if err != nil || len(activity.Comments) != 1 || activity.Comments[0].ID != comment || activity.Comments[0].Delivered.IsZero() {
		t.Fatalf("activity = %+v, %v", activity.Comments, err)
	}
}

// A comment the person deleted after it was queued (FH-140) does not end
// the wait and is never handed over: the wait goes on to the answer.
func TestWaitNeverHandsOverADeletedComment(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	s, err := store.Open(f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	created, err := s.CreateTicket(project, store.NewTicket{Title: "Colour the header", Key: "AL"})
	if err != nil {
		t.Fatal(err)
	}
	f.ask(session, "q-1")
	f.onSleep = func(call int) {
		switch call {
		case 2:
			robert := s.As("Robert")
			posted, hash, err := robert.AddComment(project, created.ID, store.Merge, store.NewComment{Body: "Posted by mistake.", SentTo: session})
			if err != nil {
				t.Fatal(err)
			}
			if err := f.log.Queue(project, events.Entry{Type: events.EntryComment, ID: posted.ID, Run: session, Ticket: created.ID, Project: project, Text: posted.Body, By: "Robert"}); err != nil {
				t.Fatal(err)
			}
			if _, _, err := robert.DeleteComment(project, created.ID, posted.ID, store.Unchanged(hash)); err != nil {
				t.Fatal(err)
			}
		case 4:
			f.answer(session, "q-1")
		}
	}
	result, err := Wait(context.Background(), f.log, f.options("q-1"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Note, "Posted by mistake") || !strings.Contains(result.Note, `"Push it?" → "Push"`) {
		t.Fatalf("note:\n%s", result.Note)
	}
}
