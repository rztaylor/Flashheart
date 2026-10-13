package board

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// TestNewFieldChanges: the one change-set rule for typed ticket edits from
// the board and agents (EDIT-6, FH-85): only editable fields, values
// trimmed, list items checked before empty ones are dropped, sorted by key.
func TestNewFieldChanges(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		in      []FieldChange
		want    []FieldChange
		wantErr string
		notEdit bool
	}{
		{name: "none", in: nil, want: []FieldChange{}},
		{
			name: "trimmed and sorted by key",
			in:   []FieldChange{{Key: "type", Value: " bug "}, {Key: "title", Value: "  Fix the header\t"}, {Key: "branch", Value: "feature/x"}},
			want: []FieldChange{{Key: "branch", Value: "feature/x"}, {Key: "title", Value: "Fix the header"}, {Key: "type", Value: "bug"}},
		},
		{
			name: "list items trimmed, empty ones dropped",
			in:   []FieldChange{{Key: "tags", List: []string{" ui ", "", "  "}}},
			want: []FieldChange{{Key: "tags", List: []string{"ui"}}},
		},
		{
			name: "an empty list clears the field",
			in:   []FieldChange{{Key: "depends-on", List: nil}},
			want: []FieldChange{{Key: "depends-on", List: []string{}}},
		},
		{name: "an empty value clears a scalar", in: []FieldChange{{Key: "pr", Value: " "}}, want: []FieldChange{{Key: "pr", Value: ""}}},
		{name: "status is a scalar field", in: []FieldChange{{Key: "status", Value: "review"}}, want: []FieldChange{{Key: "status", Value: "review"}}},
		{name: "workstream is a scalar field", in: []FieldChange{{Key: "workstream", Value: "board-ui"}}, want: []FieldChange{{Key: "workstream", Value: "board-ui"}}},
		{name: "a bad value", in: []FieldChange{{Key: "priority", Value: "urgent"}}, wantErr: "priority must be one of high, medium, low"},
		{name: "a bad list item", in: []FieldChange{{Key: "depends-on", List: []string{"FH-1", "fh-2"}}}, wantErr: `"fh-2" is not a ticket id like FH-42`},
		{name: "an empty id is checked before it is dropped", in: []FieldChange{{Key: "depends-on", List: []string{""}}}, wantErr: `"" is not a ticket id like FH-42`},
		{name: "a long title", in: []FieldChange{{Key: "title", Value: strings.Repeat("x", MaxTitle+1)}}, wantErr: "title must be 1 to 300 characters"},
		{name: "a title of the most characters", in: []FieldChange{{Key: "title", Value: strings.Repeat("x", MaxTitle)}}, want: []FieldChange{{Key: "title", Value: strings.Repeat("x", MaxTitle)}}},
		{name: "not editable", in: []FieldChange{{Key: "id", Value: "FH-9"}}, notEdit: true},
		{name: "set twice", in: []FieldChange{{Key: "tags", List: []string{"a"}}, {Key: "tags", List: []string{"b"}}}, wantErr: "tags is set twice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewFieldChanges(tc.in)
			switch {
			case tc.notEdit:
				if !errors.Is(err, ErrNotEditable) {
					t.Fatalf("err = %v, want ErrNotEditable", err)
				}
				return
			case tc.wantErr != "":
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			case err != nil:
				t.Fatal(err)
			}
			if !slices.EqualFunc(got.changes, tc.want, func(a, b FieldChange) bool {
				return a.Key == b.Key && a.Value == b.Value && slices.Equal(a.List, b.List) && (a.List == nil) == (b.List == nil)
			}) {
				t.Fatalf("changes = %#v, want %#v", got.changes, tc.want)
			}
			if keys := got.Keys(); len(keys) != len(tc.want) {
				t.Fatalf("keys = %v", keys)
			}
		})
	}
}

func TestEditableField(t *testing.T) {
	t.Parallel()

	for key, want := range map[string][2]bool{
		"title": {false, true}, "status": {false, true}, "workstream": {false, true}, "pr": {false, true},
		"tags": {true, true}, "depends-on": {true, true}, "depends-on-workstreams": {true, true},
		"id": {false, false}, "updated": {false, false}, "": {false, false},
	} {
		if list, ok := EditableField(key); list != want[0] || ok != want[1] {
			t.Errorf("EditableField(%q) = %v, %v; want %v, %v", key, list, ok, want[0], want[1])
		}
	}
}

// TestFieldChangesApply: the title is the H1, the rest frontmatter; the
// workstream field is left to store, which changes it with the
// workstreams' tickets lists.
func TestFieldChangesApply(t *testing.T) {
	t.Parallel()

	ticket := []byte("---\nid: FH-1\nstatus: backlog\npriority: medium\nworkstream: \ntags: [a]\n---\n\n# Old title\n\n## Description\n\nText.\n")
	changes, err := NewFieldChanges([]FieldChange{
		{Key: "title", Value: "New   title"}, {Key: "priority", Value: "high"},
		{Key: "tags", List: []string{"ui", "api"}}, {Key: "workstream", Value: "board-ui"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := changes.Apply(ticket)
	if err != nil {
		t.Fatal(err)
	}
	doc := mdfile.Parse(got)
	parsed := ParseTicket("FH-1-x", got)
	if priority, _ := doc.String("priority"); priority != "high" || parsed.Title != "New title" || !slices.Equal(doc.List("tags"), []string{"ui", "api"}) {
		t.Fatalf("applied:\n%s", got)
	}
	if workstream, _ := doc.String("workstream"); workstream != "" {
		t.Fatalf("workstream = %q; it is store's to set", workstream)
	}
	if value, ok := changes.Value("workstream"); !ok || value != "board-ui" {
		t.Fatalf("Value(workstream) = %q, %v", value, ok)
	}
	if _, ok := changes.Value("type"); ok {
		t.Fatal("Value(type) reported a change that is not there")
	}
	if keys := changes.Keys(); !slices.Equal(keys, []string{"priority", "tags", "title", "workstream"}) {
		t.Fatalf("keys = %v", keys)
	}
	if empty, _ := NewFieldChanges(nil); empty.Len() != 0 || changes.Len() != 4 {
		t.Fatalf("Len = %d, %d", empty.Len(), changes.Len())
	}
	same, err := FieldChanges{}.Apply(ticket)
	if err != nil || string(same) != string(ticket) {
		t.Fatalf("no changes changed the file: %v\n%s", err, same)
	}
}

// TestStartsBlocked: the blocked-start gate (EDIT-2) the board's move, an
// agent's move and an agent's claim share: entering In progress from
// another column while blocked.
func TestStartsBlocked(t *testing.T) {
	t.Parallel()

	cases := []struct {
		from, to Column
		blocked  bool
		want     bool
	}{
		{Backlog, InProgress, true, true},
		{UpNext, InProgress, true, true},
		{Review, InProgress, true, true},
		{Backlog, InProgress, false, false},
		{InProgress, InProgress, true, false},
		{Backlog, UpNext, true, false},
		{InProgress, Review, true, false},
	}
	for _, tc := range cases {
		if got := StartsBlocked(tc.from, tc.to, tc.blocked); got != tc.want {
			t.Errorf("StartsBlocked(%s, %s, %v) = %v", tc.from, tc.to, tc.blocked, got)
		}
	}
}
