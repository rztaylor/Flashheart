package board

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// A comment sent to a session (FH-127) names the run it was sent to and,
// once handed over, when; both round-trip through the file as attributes.
func TestASentCommentKeepsItsRunAndDelivery(t *testing.T) {
	t.Parallel()

	delivered := at.Add(time.Hour)
	data := AppendComment(nil, "FH-42", Comment{ID: "4e5f6a7b", Time: at, By: "Robert", SentTo: "claude:s1", Body: "Use the blue one."})
	if !strings.Contains(string(data), "state: open\nsent-to: claude:s1\n\nUse the blue one.") {
		t.Fatalf("written:\n%s", data)
	}
	next, comment, ok, err := DeliverComment(data, "4e5f6a7b", "claude:s1", delivered)
	if err != nil || !ok {
		t.Fatalf("deliver: %v, %v", ok, err)
	}
	if comment.Body != "Use the blue one." || !comment.Delivered.Equal(delivered) || comment.SentTo != "claude:s1" {
		t.Errorf("comment = %+v", comment)
	}
	if !strings.Contains(string(next), "sent-to: claude:s1\ndelivered: 2026-10-13T10:10:53Z\n") {
		t.Errorf("written:\n%s", next)
	}
	reread := ParseActivity(next).Comments[0]
	if reread.SentTo != "claude:s1" || !reread.Delivered.Equal(delivered) {
		t.Errorf("reread = %+v", reread)
	}
	if string(ParseActivity(next).Bytes()) != string(next) {
		t.Error("the file does not round-trip")
	}
	// The delivery is not history: no write to the ticket made it.
	if changes := CommentChanges(data, next); len(changes) != 0 {
		t.Errorf("changes = %+v", changes)
	}
}

// A comment is delivered once, and only to the run it was sent to; one
// that is not there is delivered to no one.
func TestDeliverCommentHandsItOverOnce(t *testing.T) {
	t.Parallel()

	sent := AppendComment(nil, "FH-42", Comment{ID: "c1", Time: at, By: "Robert", SentTo: "claude:s1", Body: "Hi"})
	once, _, _, err := DeliverComment(sent, "c1", "claude:s1", at)
	if err != nil {
		t.Fatal(err)
	}
	plain := AppendComment(nil, "FH-42", Comment{ID: "c1", Time: at, By: "Robert", Body: "Hi"})
	for name, test := range map[string]struct {
		data []byte
		id   string
		run  string
	}{
		"delivered already": {once, "c1", "claude:s1"},
		"another run":       {sent, "c1", "claude:s2"},
		"never sent":        {plain, "c1", "claude:s1"},
	} {
		next, _, ok, err := DeliverComment(test.data, test.id, test.run, at.Add(time.Hour))
		if err != nil || ok || string(next) != string(test.data) {
			t.Errorf("%s: ok %v, err %v, changed %v", name, ok, err, string(next) != string(test.data))
		}
	}
	if _, _, _, err := DeliverComment(sent, "c9", "claude:s1", at); !errors.Is(err, ErrNoComment) {
		t.Errorf("missing comment: %v, want ErrNoComment", err)
	}
}

// Editing a comment keeps where it was sent; a person cannot edit one
// once it is delivered.
func TestADeliveredCommentIsLocked(t *testing.T) {
	t.Parallel()

	sent := AppendComment(nil, "FH-42", Comment{ID: "c1", Time: at, By: "Robert", SentTo: "claude:s1", Body: "Hi"})
	edited, comment, err := EditComment(sent, "c1", func(c *Comment) { c.Body = "Hello" })
	if err != nil || comment.SentTo != "claude:s1" || !strings.Contains(string(edited), "sent-to: claude:s1\n") {
		t.Fatalf("edit: %+v, %v\n%s", comment, err, edited)
	}
	if err := CanEditComment(ParseActivity(edited).Comments[0]); err != nil {
		t.Errorf("an undelivered comment is locked: %v", err)
	}
	delivered, _, _, _ := DeliverComment(edited, "c1", "claude:s1", at)
	if err := CanEditComment(ParseActivity(delivered).Comments[0]); !errors.Is(err, ErrCommentDelivered) {
		t.Errorf("a delivered comment: %v, want ErrCommentDelivered", err)
	}
}
