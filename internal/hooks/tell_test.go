package hooks

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// toldFromBoard posts a comment on a new ticket in demo, sent to the
// session, and queues it as the board does (FH-127). It returns the
// ticket's id and the comment's.
func toldFromBoard(t *testing.T, root, session, body string) (string, string) {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	created, err := s.CreateTicket("demo", store.NewTicket{Title: "Colour the header", Key: "DM"})
	if err != nil {
		t.Fatal(err)
	}
	id := "fake:" + session
	comment, _, err := s.As("Robert").AddComment("demo", created.ID, store.Merge, store.NewComment{Body: body, SentTo: id})
	if err != nil {
		t.Fatal(err)
	}
	if err := events.New(s).Queue("demo", events.Entry{Type: events.EntryComment, ID: comment.ID, Run: id, Ticket: created.ID, Project: "demo", Text: body, By: "Robert"}); err != nil {
		t.Fatal(err)
	}
	return created.ID, comment.ID
}

// deliveredOn reports when the comment was marked delivered in its
// ticket's activity, zero when it was not.
func deliveredOn(t *testing.T, root, ticket, comment string) time.Time {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	file, err := s.TicketFile("demo", ticket)
	if err != nil {
		t.Fatal(err)
	}
	activity, _, err := s.ReadTicketActivity("demo", filepath.Base(filepath.Dir(file)))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range activity.Comments {
		if c.ID == comment {
			return c.Delivered
		}
	}
	t.Fatalf("no comment %s", comment)
	return time.Time{}
}

// A comment the person sent from the board reaches its session with the
// next prompt, framed as the user's words, and is marked delivered on the
// ticket; a later session in the worktree is not handed it (FH-127).
func TestPromptDeliversACommentSentFromTheBoard(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	run(t, root, "SessionStart", "s1 "+cwd, fake{})
	ticket, comment := toldFromBoard(t, root, "s1", "Use the blue one.")
	if out := run(t, root, "SessionStart", "s2 "+cwd, fake{}); strings.Contains(out, "Use the blue one") {
		t.Fatalf("the next session's recovery note = %q", out)
	}
	if !deliveredOn(t, root, ticket, comment).IsZero() {
		t.Fatal("delivered to the next session")
	}
	out := run(t, root, "Prompt", "s1 "+cwd, fake{})
	for _, part := range []string{
		`[Flashheart] Comments from the user on the board (their own words):`,
		`- ` + ticket + ` (Robert): "Use the blue one."`,
	} {
		if !strings.Contains(out, part) {
			t.Fatalf("prompt output missing %q:\n%s", part, out)
		}
	}
	if deliveredOn(t, root, ticket, comment).IsZero() {
		t.Error("the comment is not marked delivered")
	}
	if got := commentsDelivered(t, root); !slices.Equal(got, []string{"fake:s1 " + comment + " comment"}) {
		t.Errorf("comments delivered = %v", got)
	}
}

// A comment the person deleted after it was queued (FH-140) is not handed
// over with the next prompt, nor recorded as delivered.
func TestPromptNeverHandsOverADeletedComment(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	run(t, root, "SessionStart", "s1 "+cwd, fake{})
	ticket, comment := toldFromBoard(t, root, "s1", "Posted by mistake.")
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	file, err := s.TicketFile("demo", ticket)
	if err != nil {
		t.Fatal(err)
	}
	_, hash, err := s.ReadTicketActivity("demo", filepath.Base(filepath.Dir(file)))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.As("Robert").DeleteComment("demo", ticket, comment, store.Unchanged(hash)); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if out := run(t, root, "Prompt", "s1 "+cwd, fake{}); strings.Contains(out, "Posted by mistake") || strings.Contains(out, "Comments from the user") {
		t.Errorf("prompt output:\n%s", out)
	}
	if got := commentsDelivered(t, root); len(got) != 0 {
		t.Errorf("comments delivered = %v", got)
	}
}
