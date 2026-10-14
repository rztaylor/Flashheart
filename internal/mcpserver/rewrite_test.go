package mcpserver

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/store"
)

func (e *env) rewritable() string {
	e.t.Helper()
	id := e.ticket(store.NewTicket{
		Title: "Card panel", Type: "feature", Description: "Open a panel.",
		Criteria: []string{"Opens", "Closes", "Remembers its tab"}, PlanOrRepro: "Unit tests.",
	})
	if _, err := e.store.UpdateTicket("demo", id, store.Merge, func(data []byte) ([]byte, error) {
		return mdfile.SetCheckbox(data, board.SectionCriteria, 1, true)
	}); err != nil {
		e.t.Fatal(err)
	}
	return id
}

// update_ticket rewrites named sections and edits criteria by operation
// (FH-69): ticks stay with their criteria, the replaced text is kept in
// the ticket's files and linked from a note naming the caller, and the
// history names the sections it changed.
func TestUpdateTicketRewritesSectionsAndCriteria(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.rewritable()
	e.startSession(session)
	out := e.ok("update_ticket", map[string]any{
		"ticket":   id,
		"sections": map[string]any{"description": "The scope changed: a drawer, not a panel.", "Context": "See D75."},
		"criteria": []any{
			map[string]any{"op": "reword", "item": "2", "text": "Closes with Escape"},
			map[string]any{"op": "remove", "item": "Remembers"},
			map[string]any{"op": "add", "text": "Slides in from the right", "position": 1},
		},
		"check":        []string{"1"},
		"append_notes": "Scope changed after the design review.",
	})
	contains(t, out, "ok ticket=DM-1 changed=criteria,description,context,notes", "kept=files/")
	file := e.file(id)
	contains(t, file,
		"## Description\n\nThe scope changed: a drawer, not a panel.\n",
		"## Acceptance Criteria\n\n- [ ] Slides in from the right\n- [x] Opens\n- [x] Closes with Escape\n\n",
		"## Context\n\nSee D75.\n",
		"claude:5b0c7e2a: Scope changed after the design review.",
		"claude:5b0c7e2a: Rewrote Description and Context; criteria: 2 reworded, 3 removed, one added as 1. The replaced text is in [files/")
	kept := regexp.MustCompile(`kept=(files/\S+)`).FindStringSubmatch(out)[1]
	folder := filepath.Join(e.root, filepath.Dir(e.ticketPath(id)))
	earlier, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(kept)))
	if err != nil {
		t.Fatal(err)
	}
	contains(t, string(earlier), "## Description\n\nOpen a panel.\n", "## Acceptance Criteria\n\n- [x] Opens\n- [x] Closes\n- [ ] Remembers its tab\n")
	if strings.Contains(string(earlier), "## Context") {
		t.Errorf("an added section was kept:\n%s", earlier)
	}
	history := e.history()
	last := history[len(history)-1]
	if last.By != session || !slices.ContainsFunc(last.Changes, func(c board.Change) bool {
		return c.What == board.ChangeEdited && slices.Equal(c.Fields, []string{"description", "acceptance-criteria", "context"})
	}) {
		t.Errorf("history = %+v", last)
	}
	if got := e.log(); got[len(got)-1].Kind != events.TicketUpdated {
		t.Errorf("last event = %v", got[len(got)-1].Kind)
	}
}

// ticketPath is a ticket file's path under the root.
func (e *env) ticketPath(id string) string {
	e.t.Helper()
	name, err := e.store.TicketFile("demo", id)
	if err != nil {
		e.t.Fatal(err)
	}
	return name
}

// Handoff and Notes are never rewritten, the criteria only by operation,
// and a section the ticket does not have is refused naming those it has
// (D75); each refusal writes nothing.
func TestUpdateTicketRefusesSectionsItDoesNotRewrite(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.rewritable()
	before := e.file(id)
	for _, tc := range []struct{ section, fix string }{
		{"Handoff", "checkpoint"},
		{"notes", "append_notes"},
		{"Acceptance Criteria", "criteria"},
		{"Descripton", "Description, Test Plan, Reproduction, Context"},
	} {
		out := e.fails("update_ticket", map[string]any{"ticket": id, "sections": map[string]any{tc.section: "x"}}, "invalid_input")
		if !strings.Contains(out, tc.fix) {
			t.Errorf("%s: %s\nwant it to name %q", tc.section, out, tc.fix)
		}
	}
	for _, criteria := range [][]any{
		{map[string]any{"op": "reword", "item": "9", "text": "x"}},
		{map[string]any{"op": "move", "item": "1"}},
		{map[string]any{"op": "tick", "item": "1"}},
		{map[string]any{"op": "add"}},
	} {
		e.fails("update_ticket", map[string]any{"ticket": id, "criteria": criteria}, "invalid_input")
	}
	if e.file(id) != before {
		t.Error("a refused rewrite changed the ticket")
	}
}

// A ticket that changes between update_ticket's read and its write is a
// conflict, and nothing of the call is written (STO-3).
func TestUpdateTicketAnswersConflictWhenTheTicketChangesInBetween(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.rewritable()
	e.connect()
	e.srv.betweenReadAndWrite = func() {
		if _, err := e.store.UpdateTicket("demo", id, store.Merge, func(data []byte) ([]byte, error) {
			return mdfile.AppendToSection(data, board.SectionNotes, "- 2026-10-06 · a person's note")
		}); err != nil {
			t.Error(err)
		}
	}
	out := e.fails("update_ticket", map[string]any{"ticket": id, "sections": map[string]any{"Description": "Rewritten."}, "check": []string{"1"}}, "conflict")
	contains(t, out, "get_ticket")
	file := e.file(id)
	if strings.Contains(file, "Rewritten.") || !strings.Contains(file, "- [ ] Opens") || !strings.Contains(file, "a person's note") {
		t.Errorf("ticket after the conflict:\n%s", file)
	}
	if e.exists(filepath.Join(filepath.Dir(e.ticketPath(id)), "files")) {
		t.Error("the kept text was written")
	}
}

// A rewrite of a ticket another live session holds answers claimed.
func TestUpdateTicketRewriteRespectsAnotherSessionsClaim(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.rewritable()
	e.rivalHolds(id)
	e.startSession(session)
	e.fails("update_ticket", map[string]any{"ticket": id, "sections": map[string]any{"Description": "Hijacked."}}, "claimed")
	e.fails("update_ticket", map[string]any{"ticket": id, "criteria": []any{map[string]any{"op": "remove", "item": "1"}}}, "claimed")
}
