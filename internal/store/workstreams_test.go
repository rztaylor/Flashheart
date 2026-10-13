package store

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// members returns a workstream's tickets list as the file has it.
func members(t *testing.T, root, slug string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "alpha", "workstreams", slug+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return mdfile.Parse(data).List("tickets")
}

func workstreamField(t *testing.T, s *Store, id string) string {
	t.Helper()
	data, _, err := s.ReadTicket("alpha", id)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := mdfile.Parse(data).String("workstream")
	return value
}

// noMembershipWarnings fails when the board warns that id's workstream field
// and the workstreams' lists disagree.
func noMembershipWarnings(t *testing.T, s *Store, id string) {
	t.Helper()
	b, _, err := s.ReadBoard()
	if err != nil {
		t.Fatal(err)
	}
	if warnings := board.Analyze(b).Warnings[board.Ref{Project: "alpha", ID: id}]; len(warnings) > 0 {
		t.Fatalf("%s warnings: %v", id, warnings)
	}
}

func TestSetTicketWorkstreamKeepsTheFieldAndListsTogether(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if _, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline", Goal: "Work without a server."}); err != nil {
		t.Fatal(err)
	}

	// Join: appended at the end of the list, with the other edit applied too.
	_, hash, _ := s.ReadTicket("alpha", "AL-5")
	if _, err := s.SetTicketWorkstream("alpha", "AL-5", Unchanged(hash), "board-ui", setStatus("up-next")); err != nil {
		t.Fatal(err)
	}
	if got := members(t, root, "board-ui"); !slices.Equal(got, []string{"AL-2", "AL-3", "AL-4", "AL-5"}) {
		t.Fatalf("board-ui tickets after join = %v", got)
	}
	if workstreamField(t, s, "AL-5") != "board-ui" {
		t.Fatal("field not set on join")
	}
	noMembershipWarnings(t, s, "AL-5")

	// Change: leaves the old list, joins the new one.
	if _, err := s.SetTicketWorkstream("alpha", "AL-5", Merge, "offline", nil); err != nil {
		t.Fatal(err)
	}
	if got := members(t, root, "board-ui"); !slices.Equal(got, []string{"AL-2", "AL-3", "AL-4"}) {
		t.Fatalf("board-ui tickets after change = %v", got)
	}
	if got := members(t, root, "offline"); !slices.Equal(got, []string{"AL-5"}) {
		t.Fatalf("offline tickets after change = %v", got)
	}
	noMembershipWarnings(t, s, "AL-5")

	// Joining again changes nothing.
	before := members(t, root, "offline")
	if _, err := s.SetTicketWorkstream("alpha", "AL-5", Merge, "offline", nil); err != nil {
		t.Fatal(err)
	}
	if got := members(t, root, "offline"); !slices.Equal(got, before) {
		t.Fatalf("rejoin changed the list: %v", got)
	}

	// Leave: out of every list, field cleared.
	if _, err := s.SetTicketWorkstream("alpha", "AL-5", Merge, "", nil); err != nil {
		t.Fatal(err)
	}
	if got := members(t, root, "offline"); len(got) != 0 {
		t.Fatalf("offline tickets after leave = %v", got)
	}
	if workstreamField(t, s, "AL-5") != "" {
		t.Fatal("field not cleared on leave")
	}
	noMembershipWarnings(t, s, "AL-5")

	// A member set to its own workstream keeps its place in the order.
	if _, err := s.SetTicketWorkstream("alpha", "AL-3", Merge, "board-ui", nil); err != nil {
		t.Fatal(err)
	}
	if got := members(t, root, "board-ui"); !slices.Equal(got, []string{"AL-2", "AL-3", "AL-4"}) {
		t.Fatalf("board-ui order changed: %v", got)
	}

	// A ticket a list holds without the field is repaired by setting it.
	if _, err := s.UpdateTicket("alpha", "AL-4", Merge, func(data []byte) ([]byte, error) { return mdfile.SetScalar(data, "workstream", "") }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetTicketWorkstream("alpha", "AL-4", Merge, "board-ui", nil); err != nil {
		t.Fatal(err)
	}
	noMembershipWarnings(t, s, "AL-4")
}

func TestSetTicketWorkstreamRefusesAMissingWorkstreamAndStaleHash(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	before, hash, _ := s.ReadTicket("alpha", "AL-5")
	_, err := s.SetTicketWorkstream("alpha", "AL-5", Unchanged(hash), "offline", nil)
	var missing *WorkstreamNotFoundError
	if !errors.As(err, &missing) || !errors.Is(err, ErrNotFound) || missing.Slug != "offline" || !slices.Equal(missing.Available, []string{"board-ui"}) {
		t.Fatalf("missing workstream error = %v", err)
	}
	if !strings.Contains(err.Error(), "board-ui") {
		t.Errorf("error does not name the available workstreams: %v", err)
	}
	if after, _, _ := s.ReadTicket("alpha", "AL-5"); string(after) != string(before) {
		t.Fatal("ticket written despite the refusal")
	}

	// A stale base touches neither file.
	if _, err := s.UpdateTicket("alpha", "AL-5", Merge, setStatus("up-next")); err != nil {
		t.Fatal(err)
	}
	_, err = s.SetTicketWorkstream("alpha", "AL-5", Unchanged(hash), "board-ui", nil)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale base error = %v", err)
	}
	if got := members(t, root, "board-ui"); slices.Contains(got, "AL-5") {
		t.Fatalf("workstream written despite the conflict: %v", got)
	}
}

func TestCreateTicketJoinsItsWorkstream(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	created, err := s.CreateTicket("alpha", NewTicket{Title: "Column scroll", Workstream: "board-ui"})
	if err != nil {
		t.Fatal(err)
	}
	if got := members(t, root, "board-ui"); !slices.Equal(got, []string{"AL-2", "AL-3", "AL-4", created.ID}) {
		t.Fatalf("board-ui tickets = %v", got)
	}
	noMembershipWarnings(t, s, created.ID)

	project, _ := s.ReadProject("alpha")
	_, err = s.CreateTicket("alpha", NewTicket{Title: "Nowhere", Workstream: "offline"})
	var missing *WorkstreamNotFoundError
	if !errors.As(err, &missing) {
		t.Fatalf("missing workstream error = %v", err)
	}
	if after, _ := s.ReadProject("alpha"); after.NextID != project.NextID || len(after.Tickets) != len(project.Tickets) {
		t.Fatal("a ticket was created despite the refusal")
	}
}

func TestCreateWorkstreamWritesTheTemplate(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	created, err := s.CreateWorkstream("alpha", NewWorkstream{
		Title: "Offline mode", Goal: "Work without a server.", Priority: "high",
		Tickets: []string{"AL-6", "AL-3"}, DependsOnWorkstreams: []string{"board-ui"}, Tags: []string{"sync"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Slug != "offline-mode" {
		t.Fatalf("slug = %q", created.Slug)
	}
	data, err := os.ReadFile(filepath.Join(root, "alpha", "workstreams", "offline-mode.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nslug: offline-mode\nstatus: active\npriority: high\ncreated: 2026-10-05\ntickets:\n  - AL-6\n  - AL-3\n" +
		"depends-on-workstreams: [board-ui]\ntags: [sync]\n---\n\n# Offline mode\n\n## Goal\n\nWork without a server.\n\n" +
		"## Scope\n\n## Success Criteria\n\n- [ ] Every ticket in the line reviewed\n\n## Notes\n\nNone.\n"
	if string(data) != want {
		t.Fatalf("workstream file:\n%s\nwant:\n%s", data, want)
	}
	if Hash(data) != created.Hash {
		t.Error("returned hash does not match the file")
	}
	// Initial tickets join it, leaving any workstream they were in.
	for _, id := range []string{"AL-6", "AL-3"} {
		if workstreamField(t, s, id) != "offline-mode" {
			t.Errorf("%s workstream field not set", id)
		}
		noMembershipWarnings(t, s, id)
	}
	if got := members(t, root, "board-ui"); !slices.Equal(got, []string{"AL-2", "AL-4"}) {
		t.Fatalf("board-ui tickets = %v", got)
	}
	parsed := board.ParseWorkstream("offline-mode", data)
	if parsed.Title != "Offline mode" || len(parsed.Repair) > 0 || len(parsed.Warnings) > 0 {
		t.Fatalf("parsed = %+v", parsed)
	}

	// Slugs stay unique; defaults fill priority.
	again, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline mode"})
	if err != nil || again.Slug != "offline-mode-2" {
		t.Fatalf("second create = %+v, %v", again, err)
	}
	if other, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "日本"}); err != nil || other.Slug != "workstream" {
		t.Fatalf("title without ASCII words = %+v, %v", other, err)
	}
	if got := mdfile.Parse(mustRead(t, filepath.Join(root, "alpha", "workstreams", "offline-mode-2.md"))); got.List("tickets") != nil {
		t.Fatalf("empty workstream lists tickets: %v", got.List("tickets"))
	} else if priority, _ := got.String("priority"); priority != "medium" {
		t.Fatalf("default priority = %q", priority)
	}
}

func TestCreateWorkstreamRefusesBadInputAndWritesNothing(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	for name, input := range map[string]NewWorkstream{
		"no title":           {Title: "  "},
		"bad priority":       {Title: "X", Priority: "urgent"},
		"missing ticket":     {Title: "X", Tickets: []string{"AL-99"}},
		"not an id":          {Title: "X", Tickets: []string{"../AL-1"}},
		"repeated ticket":    {Title: "X", Tickets: []string{"AL-5", "AL-5"}},
		"missing dependency": {Title: "X", DependsOnWorkstreams: []string{"nope"}},
	} {
		if _, err := s.CreateWorkstream("alpha", input); err == nil {
			t.Errorf("%s: created", name)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(root, "alpha", "workstreams"))
	if len(entries) != 1 {
		t.Fatalf("workstreams after refusals: %d files", len(entries))
	}
	if workstreamField(t, s, "AL-5") != "" {
		t.Fatal("a ticket was changed by a refused create")
	}
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// workstreamChanges checks field edits for a test.
func workstreamChanges(t *testing.T, changes ...board.FieldChange) board.WorkstreamChanges {
	t.Helper()
	checked, err := board.NewWorkstreamChanges(changes)
	if err != nil {
		t.Fatal(err)
	}
	return checked
}

// snapshot is every file under the project, for showing that a refused
// write wrote nothing.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(filepath.Join(root, "alpha"), func(name string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Base(name) == "lock" {
			return err
		}
		data, err := os.ReadFile(name)
		files[name] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// TestEditWorkstreamSetsItsFields: one write sets the title, goal,
// priority, tags and dependencies; the slug and file name stay when the
// title changes (FH-46).
func TestEditWorkstreamSetsItsFields(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if _, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline", Goal: "Work without a server."}); err != nil {
		t.Fatal(err)
	}
	_, hash, err := s.ReadWorkstream("alpha", "board-ui")
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.EditWorkstream("alpha", "board-ui", Unchanged(hash), WorkstreamEdit{Changes: workstreamChanges(t,
		board.FieldChange{Key: "title", Value: "Board and panel"},
		board.FieldChange{Key: "goal", Value: "Render, edit and open tickets."},
		board.FieldChange{Key: "priority", Value: "low"},
		board.FieldChange{Key: "tags", List: []string{"ui", "panel"}},
		board.FieldChange{Key: "depends-on-workstreams", List: []string{"offline"}},
	)})
	if err != nil {
		t.Fatal(err)
	}
	data := mustRead(t, filepath.Join(root, "alpha", "workstreams", "board-ui.md"))
	if Hash(data) != next {
		t.Error("returned hash does not match the file")
	}
	want := "---\nslug: board-ui\nstatus: active\npriority: low\ncreated: 2026-10-01\ntickets:\n  - AL-2\n  - AL-3\n  - AL-4\n" +
		"depends-on-workstreams: [offline]\ntags: [ui, panel]\n---\n\n# Board and panel\n\n## Goal\n\nRender, edit and open tickets.\n\n" +
		"## Scope\n\nColumns, card panel, drag and drop.\n\n## Success Criteria\n\n- [ ] All three tickets reviewed\n\n## Notes\n\nFixture workstream.\n"
	if string(data) != want {
		t.Fatalf("workstream file:\n%s\nwant:\n%s", data, want)
	}

	// The current values write nothing.
	again, err := s.EditWorkstream("alpha", "board-ui", Unchanged(next), WorkstreamEdit{Changes: workstreamChanges(t, board.FieldChange{Key: "priority", Value: "low"})})
	if err != nil || again != next {
		t.Fatalf("same value = %q, %v", again, err)
	}
	if after := mustRead(t, filepath.Join(root, "alpha", "workstreams", "board-ui.md")); string(after) != want {
		t.Fatal("an edit to the current value rewrote the file")
	}
}

// TestEditWorkstreamMovesTicketsInAndOut: tickets added join the end of
// its list and leave any other; tickets removed leave it and belong to no
// workstream; each ticket's field and every list agree after (FH-46).
func TestEditWorkstreamMovesTicketsInAndOut(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if _, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline", Tickets: []string{"AL-6"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{Add: []string{"AL-5", "AL-6", "AL-3"}, Remove: []string{"AL-2"}}); err != nil {
		t.Fatal(err)
	}
	if got := members(t, root, "board-ui"); !slices.Equal(got, []string{"AL-3", "AL-4", "AL-5", "AL-6"}) {
		t.Fatalf("board-ui tickets = %v", got)
	}
	if got := members(t, root, "offline"); len(got) != 0 {
		t.Fatalf("offline still lists %v", got)
	}
	for id, want := range map[string]string{"AL-2": "", "AL-3": "board-ui", "AL-5": "board-ui", "AL-6": "board-ui"} {
		if got := workstreamField(t, s, id); got != want {
			t.Errorf("%s workstream = %q, want %q", id, got, want)
		}
		noMembershipWarnings(t, s, id)
	}
}

// TestEditWorkstreamRefusesAndWritesNothing: a stale hash, an unknown
// workstream, ticket or dependency, a cycle, a ticket both added and
// removed, and removing a ticket that is not a member each refuse the
// whole edit, with nothing written.
func TestEditWorkstreamRefusesAndWritesNothing(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if _, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline", DependsOnWorkstreams: []string{"board-ui"}}); err != nil {
		t.Fatal(err)
	}
	_, hash, err := s.ReadWorkstream("alpha", "board-ui")
	if err != nil {
		t.Fatal(err)
	}
	// Another writer changes the workstream after hash was read.
	if _, err := s.UpdateWorkstream("alpha", "board-ui", Merge, func(data []byte) ([]byte, error) { return mdfile.SetScalar(data, "priority", "medium") }); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	title := workstreamChanges(t, board.FieldChange{Key: "title", Value: "Renamed"})
	for name, tc := range map[string]struct {
		slug string
		pre  Precondition
		edit WorkstreamEdit
		is   error
	}{
		"stale hash":         {"board-ui", Unchanged(hash), WorkstreamEdit{Changes: title, Add: []string{"AL-5"}}, ErrConflict},
		"no hash":            {"board-ui", Precondition{}, WorkstreamEdit{Changes: title}, ErrInvalidInput},
		"missing workstream": {"nope", Merge, WorkstreamEdit{Changes: title}, ErrNotFound},
		"bad slug":           {"../alpha", Merge, WorkstreamEdit{Changes: title}, ErrInvalidName},
		"missing ticket":     {"board-ui", Merge, WorkstreamEdit{Changes: title, Add: []string{"AL-5", "AL-99"}}, ErrNotFound},
		"not an id":          {"board-ui", Merge, WorkstreamEdit{Add: []string{"../AL-1"}}, ErrInvalidInput},
		"added twice":        {"board-ui", Merge, WorkstreamEdit{Add: []string{"AL-5", "AL-5"}}, ErrInvalidInput},
		"added and removed":  {"board-ui", Merge, WorkstreamEdit{Add: []string{"AL-5"}, Remove: []string{"AL-5"}}, ErrInvalidInput},
		"remove non-member":  {"board-ui", Merge, WorkstreamEdit{Changes: title, Remove: []string{"AL-5"}}, ErrInvalidInput},
		"missing dependency": {"board-ui", Merge, WorkstreamEdit{Changes: workstreamChanges(t, board.FieldChange{Key: "depends-on-workstreams", List: []string{"nope"}})}, ErrNotFound},
		"cycle":              {"board-ui", Merge, WorkstreamEdit{Changes: workstreamChanges(t, board.FieldChange{Key: "depends-on-workstreams", List: []string{"offline"}}), Add: []string{"AL-5"}}, ErrInvalidInput},
		"depends on itself":  {"board-ui", Merge, WorkstreamEdit{Changes: workstreamChanges(t, board.FieldChange{Key: "depends-on-workstreams", List: []string{"board-ui"}})}, ErrInvalidInput},
	} {
		_, err := s.EditWorkstream("alpha", tc.slug, tc.pre, tc.edit)
		if !errors.Is(err, tc.is) {
			t.Errorf("%s: error = %v, want %v", name, err, tc.is)
		}
		if after := snapshot(t, root); !maps.Equal(before, after) {
			t.Fatalf("%s: a refused edit wrote files", name)
		}
	}

	_, err = s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{Changes: workstreamChanges(t, board.FieldChange{Key: "depends-on-workstreams", List: []string{"offline"}})})
	var cycle *board.CycleError
	if !errors.As(err, &cycle) || !strings.Contains(err.Error(), "board-ui → offline → board-ui") {
		t.Fatalf("cycle error = %v", err)
	}
	_, err = s.EditWorkstream("alpha", "nope", Merge, WorkstreamEdit{Changes: title})
	var missing *WorkstreamNotFoundError
	if !errors.As(err, &missing) || !slices.Equal(missing.Available, []string{"board-ui", "offline"}) {
		t.Fatalf("missing workstream error = %v", err)
	}
}

// TestCreateWorkstreamRefusesACycle: a workstream edited by hand to wait
// for one not yet made cannot be made to wait for it back.
func TestCreateWorkstreamRefusesACycle(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if _, err := s.UpdateWorkstream("alpha", "board-ui", Merge, func(data []byte) ([]byte, error) {
		return mdfile.SetList(data, "depends-on-workstreams", []string{"offline"})
	}); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	_, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline", DependsOnWorkstreams: []string{"board-ui"}, Tickets: []string{"AL-5"}})
	var cycle *board.CycleError
	if !errors.As(err, &cycle) || !errors.Is(err, ErrInvalidInput) || !strings.Contains(err.Error(), "offline → board-ui → offline") {
		t.Fatalf("error = %v, want the cycle", err)
	}
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Fatal("a refused create wrote files")
	}
}

// TestEditWorkstreamWaitsForTheLock: while another writer holds the
// project lock the edit answers ErrBusy and writes nothing. Not parallel:
// it shortens the package-wide LockWait.
func TestEditWorkstreamWaitsForTheLock(t *testing.T) {
	old := LockWait
	LockWait = 200 * time.Millisecond
	t.Cleanup(func() { LockWait = old })

	s, root := writable(t)
	release, err := open(t, root).lock("alpha")
	if err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	_, err = s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{Add: []string{"AL-5"}})
	release()
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("error = %v, want ErrBusy", err)
	}
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Fatal("a busy edit wrote files")
	}
}

// TestWorkstreamDependencyHoldsBackTicketsAddedLater: a dependency set on
// a workstream blocks every member, a ticket that joins afterwards
// included, and the reason names both workstreams (board-format
// §Blocking rule 3).
func TestWorkstreamDependencyHoldsBackTicketsAddedLater(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	if _, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Offline", Tickets: []string{"AL-6"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{Changes: workstreamChanges(t, board.FieldChange{Key: "depends-on-workstreams", List: []string{"offline"}})}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{Add: []string{"AL-5"}}); err != nil {
		t.Fatal(err)
	}
	b, _, err := s.ReadBoard()
	if err != nil {
		t.Fatal(err)
	}
	analysis := board.Analyze(b)
	for _, id := range []string{"AL-4", "AL-5"} {
		var reasons []string
		for _, reason := range analysis.Blocked[board.Ref{Project: "alpha", ID: id}] {
			reasons = append(reasons, reason.Describe())
		}
		if !slices.Contains(reasons, "Its workstream board-ui depends on workstream offline, which has 1 ticket not yet in review or done") {
			t.Errorf("%s reasons = %v", id, reasons)
		}
	}
}
