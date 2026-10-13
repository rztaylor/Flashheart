package board

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// Deleting a comment removes its entry and nothing else: the file reads as
// if the comment had never been appended, wherever it was (FH-140).
func TestDeleteCommentRemovesOnlyItsEntry(t *testing.T) {
	t.Parallel()

	created := AppendHistory(nil, "FH-42", History{ID: "0a1b2c3d", Time: at, By: "Robert", Changes: []Change{{What: ChangeCreated, To: "backlog"}}})
	first := Comment{ID: "c1", Time: at, By: "Robert", Body: "Posted on the wrong ticket."}
	second := Comment{ID: "c2", Time: at, By: "Robert", Body: "Second."}
	both := AppendComment(AppendComment(created, "FH-42", first), "FH-42", second)
	// The sample's comment, a change request resolved first, holds a
	// fenced heading; the entries after it are of a kind this version
	// does not read.
	sample, _, err := SetCommentState([]byte(activitySample), "4e5f6a7b", CommentResolved, true)
	if err != nil {
		t.Fatal(err)
	}
	start, end := strings.Index(activitySample, "### 2026-10-13T09:12:00Z · comment"), strings.Index(activitySample, "### 2026-10-13T09:13:00Z")
	for _, tc := range []struct {
		name string
		data []byte
		id   string
		want []byte
	}{
		{"the first of two", both, "c1", AppendComment(created, "FH-42", second)},
		{"the last", both, "c2", AppendComment(created, "FH-42", first)},
		{"the only entry", AppendComment(nil, "FH-42", first), "c1", []byte("# Activity: FH-42\n")},
		{"in a CRLF file", crlf(both), "c2", crlf(AppendComment(created, "FH-42", first))},
		{"keeping unknown entries", sample, "4e5f6a7b", []byte(activitySample[:start] + activitySample[end:])},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			next, deleted, err := DeleteComment(tc.data, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			if deleted.ID != tc.id {
				t.Errorf("deleted = %+v", deleted)
			}
			if string(next) != string(tc.want) {
				t.Errorf("after delete:\n%q\nwant\n%q", next, tc.want)
			}
			if slices.ContainsFunc(ParseActivity(next).Comments, func(c Comment) bool { return c.ID == tc.id }) {
				t.Error("the comment is still there")
			}
		})
	}
}

func crlf(data []byte) []byte {
	return []byte(strings.ReplaceAll(string(data), "\n", "\r\n"))
}

// A comment the file does not have, or an entry that is not a comment, is
// ErrNoComment, and nothing changes.
func TestDeleteCommentNeedsAComment(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"c9", "0a1b2c3d", "8c9d0e1f"} {
		if _, _, err := DeleteComment([]byte(activitySample), id); !errors.Is(err, ErrNoComment) {
			t.Errorf("%s: %v, want ErrNoComment", id, err)
		}
	}
}

// A comment can be deleted until a session has read it, and a change
// request only once it is resolved (FH-140); each refusal says why.
func TestWhichCommentsCanBeDeleted(t *testing.T) {
	t.Parallel()

	delivered := at.Add(time.Hour)
	for _, tc := range []struct {
		name    string
		comment Comment
		want    error
	}{
		{"a comment", Comment{State: CommentOpen}, nil},
		{"one sent and still waiting", Comment{State: CommentOpen, SentTo: "claude:s1"}, nil},
		{"a resolved request", Comment{State: CommentResolved}, nil},
		{"an open request", Comment{State: CommentChangeRequested}, ErrDeleteRequest},
		{"an addressed request", Comment{State: CommentAddressed}, ErrDeleteRequest},
		{"one delivered", Comment{State: CommentOpen, SentTo: "claude:s1", Delivered: delivered}, ErrDeleteDelivered},
		{"a resolved request delivered", Comment{State: CommentResolved, SentTo: "claude:s1", Delivered: delivered}, ErrDeleteDelivered},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.comment.ID, tc.comment.Time, tc.comment.By, tc.comment.Body = "c1", at, "Robert", "Text"
			err := CanDeleteComment(tc.comment)
			if !errors.Is(err, tc.want) || (tc.want == nil) != (err == nil) {
				t.Fatalf("CanDeleteComment = %v, want %v", err, tc.want)
			}
			data := AppendComment(nil, "FH-42", tc.comment)
			next, _, err := DeleteComment(data, "c1")
			if tc.want != nil {
				if !errors.Is(err, tc.want) || next != nil {
					t.Errorf("DeleteComment = %v, wrote %v; want %v", err, next != nil, tc.want)
				}
				return
			}
			if err != nil {
				t.Errorf("DeleteComment: %v", err)
			}
		})
	}
}

// History records a deleted comment by its id, never its text, and reads
// back.
func TestHistoryNamesADeletedCommentNotItsText(t *testing.T) {
	t.Parallel()

	before := AppendComment(AppendComment(nil, "FH-42", Comment{ID: "c1", Time: at, By: "Robert", Body: "secret text one"}),
		"FH-42", Comment{ID: "c2", Time: at, By: "Robert", State: CommentResolved, Body: "secret text two"})
	after, _, err := DeleteComment(before, "c1")
	if err != nil {
		t.Fatal(err)
	}
	after, _, err = DeleteComment(after, "c2")
	if err != nil {
		t.Fatal(err)
	}
	changes := CommentChanges(before, after)
	want := []Change{{What: ChangeDeleted, Comments: []string{"c1", "c2"}}}
	if !slices.EqualFunc(changes, want, equalChange) {
		t.Fatalf("changes = %+v, want %+v", changes, want)
	}
	rendered := AppendHistory(after, "FH-42", History{ID: "bbbb0000", Time: at, By: "Robert", Changes: changes})
	if strings.Contains(string(rendered), "secret") {
		t.Errorf("history keeps the text:\n%s", rendered)
	}
	if !strings.Contains(string(rendered), "\n- deleted: comment c1, comment c2\n") {
		t.Errorf("history:\n%s", rendered)
	}
	if parsed := ParseActivity(rendered).History[0].Changes; !slices.EqualFunc(parsed, want, equalChange) {
		t.Errorf("read back %+v", parsed)
	}
}
