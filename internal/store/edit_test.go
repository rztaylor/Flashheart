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

func fieldChanges(t *testing.T, changes ...board.FieldChange) board.FieldChanges {
	t.Helper()
	checked, err := board.NewFieldChanges(changes)
	if err != nil {
		t.Fatal(err)
	}
	return checked
}

// TestEditTicket: the one typed ticket edit the board and agents share
// (EDIT-6, FH-85): field changes, then the caller's body edit, under one
// precondition; a workstream change also rewrites the workstreams' lists
// in the same write, and a refused write changes no file.
func TestEditTicket(t *testing.T) {
	t.Parallel()

	appendNote := func(data []byte) ([]byte, error) {
		// The body edit sees the fields already changed.
		if title := board.ParseTicket("AL-5-x", data).Title; title != "Titles wrap in narrow columns" {
			return nil, errors.New("the body edit ran before the fields: " + title)
		}
		return mdfile.AppendToSection(data, board.SectionNotes, "- 2026-10-05 · noted")
	}
	cases := []struct {
		name       string
		changes    []board.FieldChange
		edit       Edit
		stale      bool
		wantErr    error
		wantNote   bool
		workstream string
		members    []string
	}{
		{
			name:    "fields and a body edit",
			changes: []board.FieldChange{{Key: "title", Value: "Titles wrap in narrow columns"}, {Key: "tags", List: []string{"ui", "css"}}},
			edit:    appendNote, wantNote: true, members: []string{"AL-2", "AL-3", "AL-4"},
		},
		{
			name:    "a workstream joins its list in the same write",
			changes: []board.FieldChange{{Key: "workstream", Value: "board-ui"}, {Key: "title", Value: "Titles wrap in narrow columns"}},
			edit:    appendNote, wantNote: true, workstream: "board-ui", members: []string{"AL-2", "AL-3", "AL-4", "AL-5"},
		},
		{
			name:    "a stale hash writes nothing",
			changes: []board.FieldChange{{Key: "workstream", Value: "board-ui"}},
			stale:   true, wantErr: ErrConflict, members: []string{"AL-2", "AL-3", "AL-4"},
		},
		{
			name:    "a missing workstream writes nothing",
			changes: []board.FieldChange{{Key: "workstream", Value: "offline"}, {Key: "priority", Value: "high"}},
			wantErr: ErrNotFound, members: []string{"AL-2", "AL-3", "AL-4"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, root := writable(t)
			before, hash, err := s.ReadTicket("alpha", "AL-5")
			if err != nil {
				t.Fatal(err)
			}
			if tc.stale {
				hash = Hash([]byte("something else"))
			}
			next, err := s.EditTicket("alpha", "AL-5", Unchanged(hash), fieldChanges(t, tc.changes...), tc.edit)
			after, current, readErr := s.ReadTicket("alpha", "AL-5")
			if readErr != nil {
				t.Fatal(readErr)
			}
			if got := members(t, root, "board-ui"); !slices.Equal(got, tc.members) {
				t.Fatalf("board-ui lists %v, want %v", got, tc.members)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				if string(after) != string(before) {
					t.Fatalf("a refused edit changed the ticket:\n%s", after)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if next != current {
				t.Fatalf("returned hash %s, file has %s", next, current)
			}
			ticket := board.ParseTicket("AL-5-x", after)
			if ticket.Title != "Titles wrap in narrow columns" || ticket.Workstream != tc.workstream {
				t.Fatalf("ticket: title %q workstream %q", ticket.Title, ticket.Workstream)
			}
			if notes, _ := mdfile.FindSection(string(after), board.SectionNotes); tc.wantNote && notes.Content == "" {
				t.Fatalf("no note:\n%s", after)
			}
			if updated, _ := mdfile.Parse(after).String("updated"); updated != "2026-10-05T14:12:09Z" {
				t.Fatalf("updated = %q", updated)
			}
			noMembershipWarnings(t, s, "AL-5")
		})
	}
}

// TestEditTicketWithNothingToChangeWritesNothing: no fields and no body
// edit leave the file as it was.
func TestEditTicketWithNothingToChangeWritesNothing(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	name, err := s.TicketFile("alpha", "AL-5")
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	_, hash, _ := s.ReadTicket("alpha", "AL-5")
	got, err := s.EditTicket("alpha", "AL-5", Unchanged(hash), board.FieldChanges{}, nil)
	if err != nil || got != hash {
		t.Fatalf("EditTicket = %s, %v; want %s", got, err, hash)
	}
	after, _ := os.ReadFile(filepath.Join(root, name))
	if string(after) != string(before) {
		t.Fatalf("an empty edit wrote the ticket:\n%s", after)
	}
}
