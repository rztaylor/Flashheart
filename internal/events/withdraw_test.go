package events

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/store"
)

// A comment deleted after it was queued (FH-140) is never handed over, on
// any path, since every path delivers through Deliver: a comment sent to
// the session and a change request, both looked up in their ticket at
// delivery, are left out once gone, and no comment.delivered is recorded
// for them. A change request still there goes as before.
func TestDeliverLeavesOutACommentDeletedAfterItWasQueued(t *testing.T) {
	t.Parallel()

	log, s := sampleLog(t)
	robert := s.As("Robert")
	sent, hash, err := robert.AddComment("alpha", "AL-4", store.Merge, store.NewComment{Body: "Wrong ticket.", SentTo: "claude:s"})
	if err != nil {
		t.Fatal(err)
	}
	plain, _, err := robert.AddComment("alpha", "AL-2", store.Merge, store.NewComment{Body: "Keep the colour."})
	if err != nil {
		t.Fatal(err)
	}
	requested, err := robert.RequestChanges("alpha", "AL-2", []string{plain.ID}, &store.NewComment{Body: "Rename it."})
	if err != nil {
		t.Fatal(err)
	}
	kept, gone := requested.Comments[0].ID, requested.Comments[1].ID
	for _, e := range []Entry{
		{Type: EntryComment, ID: sent.ID, Run: "claude:s", Ticket: "AL-4", Project: "alpha", Text: "Wrong ticket.", By: "Robert"},
		{Type: EntryChangeRequest, ID: kept, Run: "claude:s", Ticket: "AL-2", Project: "alpha", Text: "Keep the colour.", By: "Robert"},
		{Type: EntryChangeRequest, ID: gone, Run: "claude:s", Ticket: "AL-2", Project: "alpha", Text: "Rename it.", By: "Robert"},
	} {
		if err := log.Queue("alpha", e); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := robert.DeleteComment("alpha", "AL-4", sent.ID, store.Unchanged(hash)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := robert.SetCommentState("alpha", "AL-2", gone, "resolved"); err != nil {
		t.Fatal(err)
	}
	_, activityHash, _ := s.ReadTicketActivity("alpha", "AL-2-board-columns")
	if _, _, err := robert.DeleteComment("alpha", "AL-2", gone, store.Unchanged(activityHash)); err != nil {
		t.Fatal(err)
	}

	got, err := Deliver(log, "alpha", []Mailbox{{Session: "claude:s"}}, nil, t0)
	if err != nil || !slices.Equal(entryIDs(got), []string{"change-request:" + kept}) {
		t.Fatalf("delivered %+v, %v", got, err)
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
	if !slices.Equal(recorded, []string{kept}) {
		t.Errorf("recorded %v, want only %s", recorded, kept)
	}
}

// Withdraw takes a deleted comment's entries out of the session's inbox,
// so its text is not left waiting there, and leaves everything else.
func TestWithdrawTakesOnlyThatCommentFromTheInbox(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	for _, e := range []Entry{
		{Type: EntryAnswer, ID: "c1", Run: "claude:s", Ticket: "AL-4", Text: "An answer with the same id", By: "Robert"},
		{Type: EntryComment, ID: "c1", Run: "claude:s/a1", Ticket: "AL-4", Project: "alpha", Text: "The secret slip.", By: "Robert"},
		{Type: EntryComment, ID: "c1", Run: "claude:s", Ticket: "AL-5", Project: "alpha", Text: "Another ticket's c1.", By: "Robert"},
		{Type: EntryComment, ID: "c2", Run: "claude:s", Ticket: "AL-4", Project: "alpha", Text: "Stays.", By: "Robert"},
	} {
		if err := log.Queue("alpha", e); err != nil {
			t.Fatal(err)
		}
	}
	if err := log.Withdraw("alpha", "claude:s/a1", "alpha", "AL-4", "c1"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "alpha", ".flashheart", "inbox", "claude--s.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret slip") || strings.Count(string(data), "\n") != 3 {
		t.Errorf("inbox after withdraw:\n%s", data)
	}
	// Nothing queued, or no inbox at all, is no error.
	for _, run := range []string{"claude:s", "claude:other"} {
		if err := log.Withdraw("alpha", run, "alpha", "AL-4", "c1"); err != nil {
			t.Errorf("%s: %v", run, err)
		}
	}
	if err := log.Withdraw("alpha", "not a run", "alpha", "AL-4", "c1"); err == nil {
		t.Error("a bad run was accepted")
	}
}
