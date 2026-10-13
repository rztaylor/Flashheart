package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A comment a person sends to a session (FH-127) names its run; delivery
// marks it delivered under the lock and returns its text as it is then,
// once; after that the person cannot edit it.
func TestASentCommentIsDeliveredOnceAndThenLocked(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	robert := s.As("Robert")
	comment, hash, err := robert.AddComment("alpha", "AL-4", Merge, NewComment{Body: "Use the blue one.", SentTo: "claude:s1"})
	if err != nil || comment.SentTo != "claude:s1" {
		t.Fatalf("add: %+v, %v", comment, err)
	}
	// Edited before delivery: the session gets the new text.
	if _, _, err = robert.EditComment("alpha", "AL-4", comment.ID, Unchanged(hash), "Use the green one."); err != nil {
		t.Fatal(err)
	}
	ticket := filepath.Join(root, "alpha", "tickets", al4Folder, al4Folder+".md")
	before, _ := os.ReadFile(ticket)

	at := fixedNow.Add(time.Minute)
	if _, ok, err := s.DeliverComment("alpha", "AL-4", comment.ID, "claude:other", at); ok || err != nil {
		t.Errorf("delivered to another run: %v, %v", ok, err)
	}
	delivered, ok, err := s.DeliverComment("alpha", "AL-4", comment.ID, "claude:s1", at)
	if err != nil || !ok || delivered.Body != "Use the green one." || !delivered.Delivered.Equal(at) {
		t.Fatalf("deliver: %+v, %v, %v", delivered, ok, err)
	}
	if _, ok, err := s.DeliverComment("alpha", "AL-4", comment.ID, "claude:s1", at.Add(time.Minute)); ok || err != nil {
		t.Errorf("delivered twice: %v, %v", ok, err)
	}
	activity := activityOf(t, root, "tickets", al4Folder)
	if read := activity.Comments[0]; !read.Delivered.Equal(at) || read.SentTo != "claude:s1" || len(activity.History) != 0 {
		t.Errorf("read = %+v, history %+v", read, activity.History)
	}
	if after, _ := os.ReadFile(ticket); string(after) != string(before) {
		t.Error("delivery changed the ticket file")
	}

	_, current, _ := s.ReadTicketActivity("alpha", al4Folder)
	if _, _, err := robert.EditComment("alpha", "AL-4", comment.ID, Unchanged(current), "Too late."); !errors.Is(err, ErrDelivered) {
		t.Errorf("edit after delivery: %v, want ErrDelivered", err)
	}
	if _, _, err := robert.UnsendComment("alpha", "AL-4", comment.ID); !errors.Is(err, ErrDelivered) {
		t.Errorf("unsend after delivery: %v, want ErrDelivered", err)
	}
	if _, ok, err := s.DeliverComment("alpha", "AL-4", "deadbeef", "claude:s1", at); ok || err != nil {
		t.Errorf("missing comment: %v, %v", ok, err)
	}
	if _, _, err := s.DeliverComment("alpha", "AL-99", comment.ID, "claude:s1", at); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing ticket: %v, want ErrNotFound", err)
	}
}

// A comment that could not be queued is unsent, so the board does not say
// it waits for a session; only a run id is a run to send to.
func TestUnsendAndTheRunSentTo(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	robert := s.As("Robert")
	comment, _, err := robert.AddComment("alpha", "AL-4", Merge, NewComment{Body: "Hi", SentTo: "claude:s1"})
	if err != nil {
		t.Fatal(err)
	}
	unsent, _, err := robert.UnsendComment("alpha", "AL-4", comment.ID)
	if err != nil || unsent.SentTo != "" || activityOf(t, root, "tickets", al4Folder).Comments[0].SentTo != "" {
		t.Errorf("unsend: %+v, %v", unsent, err)
	}
	if _, _, err := robert.AddComment("alpha", "AL-4", Merge, NewComment{Body: "Hi", SentTo: "Robert"}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("sent to a person: %v, want ErrInvalidInput", err)
	}
	if _, _, err := s.As("claude:s1").UnsendComment("alpha", "AL-4", comment.ID); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("an agent unsends: %v, want ErrInvalidInput", err)
	}
	if len(activityOf(t, root, "tickets", al4Folder).Comments) != 1 {
		t.Error("a refused comment was written")
	}
}
