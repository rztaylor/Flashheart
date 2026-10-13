package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

const al2Folder = "AL-2-board-columns"

func ticketStatus(t *testing.T, root, folder string) (status, branch string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "alpha", "tickets", folder, folder+".md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := mdfile.Parse(data)
	status, _ = doc.String("status")
	branch, _ = doc.String("branch")
	return status, branch
}

func TestRequestChangesMovesTheTicketBackAndMarksTheComments(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	robert := s.As("Robert")
	first, _, err := robert.AddComment("alpha", "AL-2", Merge, NewComment{Body: "The header wraps."})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := robert.AddComment("alpha", "AL-2", Merge, NewComment{Body: "Looks good otherwise."}); err != nil {
		t.Fatal(err)
	}

	result, err := robert.RequestChanges("alpha", "AL-2", []string{first.ID}, &NewComment{Body: "Rename the button to Save.", Quote: "Board columns", QuoteFrom: "Title"})
	if err != nil {
		t.Fatal(err)
	}
	if status, branch := ticketStatus(t, root, al2Folder); status != "in-progress" || branch != "feature/board-columns" {
		t.Errorf("ticket is %s on %q, want in-progress on its branch", status, branch)
	}
	if len(result.Comments) != 2 || result.Comments[0].ID != first.ID || result.Comments[1].Body != "Rename the button to Save." {
		t.Fatalf("requested = %+v", result.Comments)
	}
	activity, hash, err := s.ReadTicketActivity("alpha", al2Folder)
	if err != nil || hash != result.ActivityHash {
		t.Fatalf("activity hash %q, want %q (%v)", hash, result.ActivityHash, err)
	}
	var states []board.CommentState
	for _, comment := range activity.Comments {
		states = append(states, comment.State)
	}
	if want := []board.CommentState{board.CommentChangeRequested, board.CommentOpen, board.CommentChangeRequested}; !slices.Equal(states, want) {
		t.Errorf("states = %v, want %v", states, want)
	}
	if len(activity.History) != 1 {
		t.Fatalf("history = %+v, want one entry", activity.History)
	}
	want := []board.Change{
		{What: board.ChangeMoved, From: "review", To: "in-progress"},
		{What: board.ChangeRequested, Comments: []string{first.ID, result.Comments[1].ID}},
	}
	if entry := activity.History[0]; entry.By != "Robert" || !slices.EqualFunc(entry.Changes, want, sameChange) {
		t.Errorf("history = %+v, want %+v", entry, want)
	}
}

func TestRequestChangesIsRefusedOutsideReview(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	robert := s.As("Robert")
	comment, _, err := robert.AddComment("alpha", "AL-4", Merge, NewComment{Body: "x"})
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(root, "alpha", "tickets", al4Folder, board.ActivityFile)
	before, _ := os.ReadFile(name)
	for label, call := range map[string]func() error{
		"not in review": func() error {
			_, err := robert.RequestChanges("alpha", "AL-4", []string{comment.ID}, nil)
			if !errors.Is(err, ErrNotInReview) {
				t.Errorf("not in review: %v", err)
			}
			return err
		},
		"nothing asked": func() error {
			_, err := robert.RequestChanges("alpha", "AL-2", nil, nil)
			return err
		},
		"an agent": func() error {
			_, err := s.As("claude:5b0c7e2a").RequestChanges("alpha", "AL-2", nil, &NewComment{Body: "x"})
			return err
		},
		"a missing comment": func() error {
			_, err := robert.RequestChanges("alpha", "AL-2", []string{"deadbeef"}, nil)
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("missing comment: %v", err)
			}
			return err
		},
	} {
		if call() == nil {
			t.Errorf("%s: accepted", label)
		}
	}
	if after, _ := os.ReadFile(name); string(after) != string(before) {
		t.Error("a refused request changed activity.md")
	}
	if status, _ := ticketStatus(t, root, al2Folder); status != "review" {
		t.Errorf("AL-2 is %s after refused requests", status)
	}
}

func TestChangeRequestsAreAddressedResolvedAndReopened(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	robert, agent := s.As("Robert"), s.As("claude:5b0c7e2a")
	requested, err := robert.RequestChanges("alpha", "AL-2", nil, &NewComment{Body: "Rename the button."})
	if err != nil {
		t.Fatal(err)
	}
	id := requested.Comments[0].ID

	// The agent marks it addressed with a note, in one write.
	data, base, err := s.ReadTicket("alpha", "AL-2")
	if err != nil || len(data) == 0 {
		t.Fatal(err)
	}
	note := func(data []byte) ([]byte, error) {
		return mdfile.AppendToSection(data, board.SectionNotes, board.NoteLine(fixedNow, "claude:5b0c7e2a", "Renamed it."))
	}
	if _, err := agent.EditTicketAddressing("alpha", "AL-2", Unchanged(base), board.FieldChanges{}, note, []string{id}); err != nil {
		t.Fatal(err)
	}
	// Again is no change.
	if _, err := agent.EditTicketAddressing("alpha", "AL-2", Merge, board.FieldChanges{}, nil, []string{id}); err != nil {
		t.Fatal(err)
	}
	comment, _, err := robert.SetCommentState("alpha", "AL-2", id, board.CommentResolved)
	if err != nil || comment.State != board.CommentResolved {
		t.Fatalf("resolve: %+v, %v", comment, err)
	}
	if comment, _, err = robert.SetCommentState("alpha", "AL-2", id, board.CommentChangeRequested); err != nil || comment.State != board.CommentChangeRequested {
		t.Fatalf("reopen: %+v, %v", comment, err)
	}

	activity := activityOf(t, root, "tickets", al2Folder)
	var got [][]board.Change
	var by []string
	for _, entry := range activity.History {
		got = append(got, entry.Changes)
		by = append(by, entry.By)
	}
	want := [][]board.Change{
		{{What: board.ChangeMoved, From: "review", To: "in-progress"}, {What: board.ChangeRequested, Comments: []string{id}}},
		{{What: board.ChangeAddressed, Comments: []string{id}}},
		{{What: board.ChangeResolved, Comments: []string{id}}},
		{{What: board.ChangeReopened, Comments: []string{id}}},
	}
	if !slices.EqualFunc(got, want, func(a, b []board.Change) bool { return slices.EqualFunc(a, b, sameChange) }) {
		t.Errorf("history = %+v\nwant      %+v", got, want)
	}
	if !slices.Equal(by, []string{"Robert", "claude:5b0c7e2a", "Robert", "Robert"}) {
		t.Errorf("by = %v", by)
	}

	// Refused: an agent resolving, a person addressing, an agent
	// addressing a plain comment, and an unknown id; none writes.
	plain, _, err := robert.AddComment("alpha", "AL-2", Merge, NewComment{Body: "Just a remark."})
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(root, "alpha", "tickets", al2Folder, board.ActivityFile)
	ticketName := filepath.Join(root, "alpha", "tickets", al2Folder, al2Folder+".md")
	before, _ := os.ReadFile(name)
	ticketBefore, _ := os.ReadFile(ticketName)
	for label, call := range map[string]func() error{
		"agent resolves": func() error {
			_, _, err := agent.SetCommentState("alpha", "AL-2", id, board.CommentResolved)
			return err
		},
		"person addresses": func() error {
			_, _, err := robert.SetCommentState("alpha", "AL-2", id, board.CommentAddressed)
			return err
		},
		"agent addresses a remark": func() error {
			_, err := agent.EditTicketAddressing("alpha", "AL-2", Merge, board.FieldChanges{}, note, []string{plain.ID})
			return err
		},
		"no one": func() error {
			_, _, err := s.SetCommentState("alpha", "AL-2", id, board.CommentResolved)
			return err
		},
	} {
		if err := call(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: %v, want ErrInvalidInput", label, err)
		}
	}
	if _, err := agent.EditTicketAddressing("alpha", "AL-2", Merge, board.FieldChanges{}, nil, []string{"deadbeef"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: %v", err)
	}
	if after, _ := os.ReadFile(name); string(after) != string(before) {
		t.Error("a refused state change wrote activity.md")
	}
	if after, _ := os.ReadFile(ticketName); string(after) != string(ticketBefore) {
		t.Error("a refused addressing wrote the ticket (its note)")
	}
}

func TestTheBoardCountsChangeRequests(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	robert := s.As("Robert")
	if _, err := robert.RequestChanges("alpha", "AL-2", nil, &NewComment{Body: "One."}); err != nil {
		t.Fatal(err)
	}
	// A second comment asked for while the ticket is in progress (as a
	// reopened request is), then addressed.
	comment, _, err := robert.AddComment("alpha", "AL-2", Merge, NewComment{Body: "Two."})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := robert.SetCommentState("alpha", "AL-2", comment.ID, board.CommentChangeRequested); err != nil {
		t.Fatal(err)
	}
	if _, err := s.As("claude:5b0c7e2a").EditTicketAddressing("alpha", "AL-2", Merge, board.FieldChanges{}, nil, []string{comment.ID}); err != nil {
		t.Fatal(err)
	}
	b, _, err := s.ReadBoard()
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range b.Projects {
		for _, ticket := range project.Tickets {
			want := board.RequestCount{}
			if ticket.ID == "AL-2" {
				want = board.RequestCount{Open: 1, Addressed: 1}
			}
			if ticket.Requests != want {
				t.Errorf("%s requests = %+v, want %+v", ticket.ID, ticket.Requests, want)
			}
		}
	}
}
