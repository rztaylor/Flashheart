package board

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestCommentTransitions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		from, to CommentState
		person   bool
		ok       bool
	}{
		// A person asks for a change, reopens an addressed or resolved one,
		// and resolves a request (FH-126).
		{CommentOpen, CommentChangeRequested, true, true},
		{CommentAddressed, CommentChangeRequested, true, true},
		{CommentResolved, CommentChangeRequested, true, true},
		{CommentChangeRequested, CommentChangeRequested, true, true},
		{CommentAddressed, CommentResolved, true, true},
		{CommentChangeRequested, CommentResolved, true, true},
		{CommentResolved, CommentResolved, true, true},
		{CommentOpen, CommentResolved, true, false},
		{CommentChangeRequested, CommentAddressed, true, false},
		{CommentAddressed, CommentOpen, true, false},
		// An agent only marks a change request addressed.
		{CommentChangeRequested, CommentAddressed, false, true},
		{CommentAddressed, CommentAddressed, false, true},
		{CommentOpen, CommentAddressed, false, false},
		{CommentResolved, CommentAddressed, false, false},
		{CommentAddressed, CommentResolved, false, false},
		{CommentOpen, CommentChangeRequested, false, false},
	} {
		err := CommentTransition(tc.from, tc.to, tc.person)
		if (err == nil) != tc.ok {
			t.Errorf("%s → %s (person %v): err = %v, want ok %v", tc.from, tc.to, tc.person, err, tc.ok)
		}
		if err != nil && !errors.Is(err, ErrCommentState) {
			t.Errorf("%s → %s: err = %v, want ErrCommentState", tc.from, tc.to, err)
		}
	}
}

func TestSetCommentStateRewritesOnlyItsState(t *testing.T) {
	t.Parallel()

	next, comment, err := SetCommentState([]byte(activitySample), "4e5f6a7b", CommentAddressed, false)
	if err != nil {
		t.Fatal(err)
	}
	if comment.State != CommentAddressed {
		t.Errorf("state = %s", comment.State)
	}
	want := strings.Replace(activitySample, "state: change-requested", "state: addressed", 1)
	if string(next) != want {
		t.Errorf("file =\n%s\nwant\n%s", next, want)
	}
	crlf := strings.ReplaceAll(activitySample, "\n", "\r\n")
	if next, _, err := SetCommentState([]byte(crlf), "4e5f6a7b", CommentAddressed, false); err != nil || string(next) != strings.ReplaceAll(want, "\n", "\r\n") {
		t.Errorf("crlf: err %v, file\n%q", err, next)
	}

	if _, _, err := SetCommentState([]byte(activitySample), "00000000", CommentAddressed, false); !errors.Is(err, ErrNoComment) {
		t.Errorf("missing comment: err = %v", err)
	}
	if _, _, err := SetCommentState([]byte(activitySample), "4e5f6a7b", CommentResolved, false); !errors.Is(err, ErrCommentState) {
		t.Errorf("agent resolving: err = %v", err)
	}
}

func TestCommentChangesNameTheComments(t *testing.T) {
	t.Parallel()

	file := func(states ...string) []byte {
		data := []byte(nil)
		for index, state := range states {
			id := []string{"aaaa0001", "aaaa0002", "aaaa0003"}[index]
			data = AppendComment(data, "FH-42", Comment{ID: id, Time: at, By: "Robert", State: CommentState(state), Body: "secret text " + id})
		}
		return data
	}
	for _, tc := range []struct {
		name          string
		before, after []byte
		want          []Change
	}{
		{"no file", nil, nil, nil},
		{"a new open comment", nil, file("open"), nil},
		{"a new change request", file("open"), file("open", "change-requested"), []Change{
			{What: ChangeRequested, Comments: []string{"aaaa0002"}},
		}},
		{"requested", file("open", "open"), file("change-requested", "change-requested"), []Change{
			{What: ChangeRequested, Comments: []string{"aaaa0001", "aaaa0002"}},
		}},
		{"addressed", file("change-requested"), file("addressed"), []Change{{What: ChangeAddressed, Comments: []string{"aaaa0001"}}}},
		{"resolved and reopened", file("addressed", "addressed", "resolved"), file("resolved", "change-requested", "change-requested"), []Change{
			{What: ChangeReopened, Comments: []string{"aaaa0002", "aaaa0003"}},
			{What: ChangeResolved, Comments: []string{"aaaa0001"}},
		}},
		{"text edits are not history", file("open"), []byte(strings.Replace(string(file("open")), "secret text", "other text", 1)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := CommentChanges(tc.before, tc.after)
			if !slices.EqualFunc(got, tc.want, equalChange) {
				t.Errorf("changes = %+v\nwant      %+v", got, tc.want)
			}
			if len(got) == 0 {
				return
			}
			rendered := AppendHistory(nil, "FH-42", History{ID: "bbbb0000", Time: at, By: "Robert", Changes: got})
			if strings.Contains(string(rendered), "secret") {
				t.Errorf("history names comment text:\n%s", rendered)
			}
			parsed := ParseActivity(rendered).History[0].Changes
			if !slices.EqualFunc(parsed, got, equalChange) {
				t.Errorf("read back %+v from\n%s", parsed, rendered)
			}
		})
	}
}

func TestRequestsCountTheOpenAndAddressed(t *testing.T) {
	t.Parallel()

	comments := []Comment{
		{ID: "a", By: "Robert", State: CommentChangeRequested},
		{ID: "b", By: "Robert", State: CommentAddressed},
		{ID: "c", By: "Robert", State: CommentResolved},
		{ID: "d", By: "Robert", State: CommentOpen},
		{ID: "e", By: "Robert", State: CommentChangeRequested},
		// Not a person's: written by hand as a run, never the user's request.
		{ID: "f", By: "claude:abc", State: CommentChangeRequested},
	}
	if got := CountRequests(comments); got != (RequestCount{Open: 2, Addressed: 1}) {
		t.Errorf("count = %+v", got)
	}
	open := OpenRequests(comments)
	if len(open) != 2 || open[0].ID != "a" || open[1].ID != "e" {
		t.Errorf("open = %+v", open)
	}
	if got := RequestsIn([]byte(activitySample)); got != (RequestCount{Open: 1}) {
		t.Errorf("sample count = %+v", got)
	}
	if got := RequestsIn([]byte("# Activity: FH-1\n")); got != (RequestCount{}) {
		t.Errorf("empty count = %+v", got)
	}
}

func TestReviewWarningsNameOpenChangeRequests(t *testing.T) {
	t.Parallel()

	ticket := Ticket{Criteria: []Criterion{{Text: "x", Done: true}}}
	review := "## Evidence\nNo visible change: backend.\n"
	if got := ReviewWarnings(ticket, review, true, nil); len(got) != 0 {
		t.Fatalf("warnings without requests = %v", got)
	}
	open := []Comment{
		{ID: "4e5f6a7b", By: "Robert", State: CommentChangeRequested, Body: "Rename the button\nto Save, please."},
		{ID: "9c0d1e2f", By: "Robert", State: CommentChangeRequested, Body: strings.Repeat("long ", 40)},
	}
	got := ReviewWarnings(ticket, review, true, open)
	if len(got) != 1 {
		t.Fatalf("warnings = %v", got)
	}
	for _, want := range []string{"2 change requests are still open", "4e5f6a7b", `"Rename the button to Save, please."`, "(Robert)", "9c0d1e2f", "…"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("warning %q lacks %q", got[0], want)
		}
	}
	one := ReviewWarnings(ticket, review, true, open[:1])
	if len(one) != 1 || !strings.HasPrefix(one[0], "1 change request is still open: ") {
		t.Errorf("one = %v", one)
	}
}
