package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
)

// A person deletes a comment posted by mistake (FH-140): its entry leaves
// activity.md under the lock once the hash the person read still holds,
// the ticket file is not touched, and the write's history names the
// comment by id, never its text.
func TestACommentIsDeletedWithItsHistoryButNotItsText(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	ticket := filepath.Join(root, "alpha", "tickets", al4Folder, al4Folder+".md")
	before, _ := os.ReadFile(ticket)
	robert := s.As("Robert")
	keep, _, err := robert.AddComment("alpha", "AL-4", Merge, NewComment{Body: "Keep this one."})
	if err != nil {
		t.Fatal(err)
	}
	mistake, hash, err := robert.AddComment("alpha", "AL-4", Merge, NewComment{Body: "Wrong ticket: the secret plan.", SentTo: "claude:s1"})
	if err != nil {
		t.Fatal(err)
	}

	// A stale hash is a conflict and writes nothing.
	name := filepath.Join(root, "alpha", "tickets", al4Folder, board.ActivityFile)
	current, _ := os.ReadFile(name)
	var conflict *ConflictError
	if _, _, err := robert.DeleteComment("alpha", "AL-4", mistake.ID, Unchanged("stale")); !errors.As(err, &conflict) || conflict.Hash != hash {
		t.Errorf("stale delete: %v", err)
	}
	if unchanged, _ := os.ReadFile(name); string(unchanged) != string(current) {
		t.Error("a refused delete changed activity.md")
	}

	deleted, next, err := robert.DeleteComment("alpha", "AL-4", mistake.ID, Unchanged(hash))
	if err != nil {
		t.Fatal(err)
	}
	if deleted.ID != mistake.ID || deleted.SentTo != "claude:s1" {
		t.Errorf("deleted = %+v", deleted)
	}
	activity, read, err := s.ReadTicketActivity("alpha", al4Folder)
	if err != nil || read != next {
		t.Fatalf("hash %q, want %q, %v", read, next, err)
	}
	if len(activity.Comments) != 1 || activity.Comments[0].ID != keep.ID {
		t.Errorf("comments = %+v", activity.Comments)
	}
	if len(activity.History) != 1 || activity.History[0].By != "Robert" ||
		!slices.EqualFunc(activity.History[0].Changes, []board.Change{{What: board.ChangeDeleted, Comments: []string{mistake.ID}}}, sameChange) {
		t.Errorf("history = %+v", activity.History)
	}
	if data, _ := os.ReadFile(name); strings.Contains(string(data), "secret plan") {
		t.Errorf("the deleted text is still in activity.md:\n%s", data)
	}
	if after, _ := os.ReadFile(ticket); string(after) != string(before) {
		t.Error("deleting a comment changed the ticket file")
	}

	// Once gone it cannot be delivered or deleted again.
	if _, ok, err := s.DeliverComment("alpha", "AL-4", mistake.ID, "claude:s1", fixedNow); ok || err != nil {
		t.Errorf("delivered after delete: %v, %v", ok, err)
	}
	if _, _, err := robert.DeleteComment("alpha", "AL-4", mistake.ID, Unchanged(next)); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted twice: %v, want ErrNotFound", err)
	}
}

// A comment delivered to its session, or a change request not yet
// resolved, is refused, and nothing is written; only a person deletes.
func TestDeletingACommentIsRefusedOnceReadOrWhileRequested(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	robert := s.As("Robert")
	sent, _, err := robert.AddComment("alpha", "AL-4", Merge, NewComment{Body: "Read already.", SentTo: "claude:s1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.DeliverComment("alpha", "AL-4", sent.ID, "claude:s1", fixedNow.Add(time.Minute)); !ok || err != nil {
		t.Fatalf("deliver: %v, %v", ok, err)
	}
	requested, err := robert.RequestChanges("alpha", "AL-2", nil, &NewComment{Body: "Rename the button."})
	if err != nil {
		t.Fatal(err)
	}
	request := requested.Comments[0].ID

	try := func(store *Store, id, folder string) error {
		_, hash, err := s.ReadTicketActivity("alpha", folder)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = store.DeleteComment("alpha", id, map[string]string{al4Folder: sent.ID, al2Folder: request}[folder], Unchanged(hash))
		return err
	}
	al2, al4 := filepath.Join(root, "alpha", "tickets", al2Folder, board.ActivityFile), filepath.Join(root, "alpha", "tickets", al4Folder, board.ActivityFile)
	was2, _ := os.ReadFile(al2)
	was4, _ := os.ReadFile(al4)
	if err := try(robert, "AL-4", al4Folder); !errors.Is(err, ErrDelivered) {
		t.Errorf("delivered: %v, want ErrDelivered", err)
	}
	if err := try(robert, "AL-2", al2Folder); !errors.Is(err, ErrOpenRequest) {
		t.Errorf("an open request: %v, want ErrOpenRequest", err)
	}
	if _, _, err := s.As("claude:s1").SetCommentState("alpha", "AL-2", request, board.CommentAddressed); err != nil {
		t.Fatal(err)
	}
	if err := try(robert, "AL-2", al2Folder); !errors.Is(err, ErrOpenRequest) {
		t.Errorf("an addressed request: %v, want ErrOpenRequest", err)
	}
	for name, writer := range map[string]*Store{"an agent": s.As("claude:s1"), "no one": s} {
		if err := try(writer, "AL-4", al4Folder); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: %v, want ErrInvalidInput", name, err)
		}
	}
	if now, _ := os.ReadFile(al4); string(now) != string(was4) {
		t.Error("a refused delete changed AL-4's activity")
	}
	if now, _ := os.ReadFile(al2); strings.Count(string(now), "· comment ·") != strings.Count(string(was2), "· comment ·") {
		t.Error("a refused delete removed AL-2's request")
	}

	// Resolved, the request can go.
	if _, _, err := robert.SetCommentState("alpha", "AL-2", request, board.CommentResolved); err != nil {
		t.Fatal(err)
	}
	if err := try(robert, "AL-2", al2Folder); err != nil {
		t.Errorf("a resolved request: %v", err)
	}
}

// TicketComment reads one comment of a live ticket as the file has it, for
// a delivery that must not hand over one deleted since it was queued.
func TestTicketCommentReadsOneComment(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	robert := s.As("Robert")
	comment, hash, err := robert.AddComment("alpha", "AL-4", Merge, NewComment{Body: "Hello."})
	if err != nil {
		t.Fatal(err)
	}
	read, ok, err := s.TicketComment("alpha", "AL-4", comment.ID)
	if err != nil || !ok || read.Body != "Hello." {
		t.Errorf("read = %+v, %v, %v", read, ok, err)
	}
	if _, _, err := robert.DeleteComment("alpha", "AL-4", comment.ID, Unchanged(hash)); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.TicketComment("alpha", "AL-4", comment.ID); ok || err != nil {
		t.Errorf("after delete: %v, %v", ok, err)
	}
	if _, _, err := s.TicketComment("alpha", "AL-99", comment.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing ticket: %v, want ErrNotFound", err)
	}
}

// A delete and a delivery of the same comment, in two processes, never
// both happen: the session reads it and the delete is refused, or it is
// deleted and the delivery hands over nothing (FH-140).
func TestADeleteAndADeliveryOfOneCommentDoNotBothHappen(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	s := open(t, root).As("Robert")
	comment, hash, err := s.AddComment("alpha", "AL-4", Merge, NewComment{Body: "Oops.", SentTo: "claude:s1"})
	if err != nil {
		t.Fatal(err)
	}
	outputs := runWriters(t, "activity",
		[]string{root, "delete", comment.ID, hash}, []string{root, "deliver", comment.ID, "claude:s1"})
	// Delivered first, the delete finds the file changed since the person
	// read it (a conflict), or, on its hash, delivered (refused).
	if !slices.ContainsFunc([][]string{{"conflict", "ok"}, {"ok", "refused"}, {"ok", "skipped"}}, func(want []string) bool { return slices.Equal(outputs, want) }) {
		t.Errorf("writers = %v, want one to happen and the other not", outputs)
	}
}
