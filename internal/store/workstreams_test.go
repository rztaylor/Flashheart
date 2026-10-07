package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
	if _, err := s.SetTicketWorkstream("alpha", "AL-5", hash, "board-ui", setStatus("up-next")); err != nil {
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
	if _, err := s.SetTicketWorkstream("alpha", "AL-5", "", "offline", nil); err != nil {
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
	if _, err := s.SetTicketWorkstream("alpha", "AL-5", "", "offline", nil); err != nil {
		t.Fatal(err)
	}
	if got := members(t, root, "offline"); !slices.Equal(got, before) {
		t.Fatalf("rejoin changed the list: %v", got)
	}

	// Leave: out of every list, field cleared.
	if _, err := s.SetTicketWorkstream("alpha", "AL-5", "", "", nil); err != nil {
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
	if _, err := s.SetTicketWorkstream("alpha", "AL-3", "", "board-ui", nil); err != nil {
		t.Fatal(err)
	}
	if got := members(t, root, "board-ui"); !slices.Equal(got, []string{"AL-2", "AL-3", "AL-4"}) {
		t.Fatalf("board-ui order changed: %v", got)
	}

	// A ticket a list holds without the field is repaired by setting it.
	if _, err := s.UpdateTicket("alpha", "AL-4", "", func(data []byte) ([]byte, error) { return mdfile.SetScalar(data, "workstream", "") }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetTicketWorkstream("alpha", "AL-4", "", "board-ui", nil); err != nil {
		t.Fatal(err)
	}
	noMembershipWarnings(t, s, "AL-4")
}

func TestSetTicketWorkstreamRefusesAMissingWorkstreamAndStaleHash(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	before, hash, _ := s.ReadTicket("alpha", "AL-5")
	_, err := s.SetTicketWorkstream("alpha", "AL-5", hash, "offline", nil)
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
	if _, err := s.UpdateTicket("alpha", "AL-5", "", setStatus("up-next")); err != nil {
		t.Fatal(err)
	}
	_, err = s.SetTicketWorkstream("alpha", "AL-5", hash, "board-ui", nil)
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
	// Only an ordered workstream says so; unordered is the default.
	ordered, err := s.CreateWorkstream("alpha", NewWorkstream{Title: "Release train", Ordered: true})
	if err != nil {
		t.Fatal(err)
	}
	chain := mustRead(t, filepath.Join(root, "alpha", "workstreams", ordered.Slug+".md"))
	if !strings.Contains(string(chain), "created: 2026-10-05\nordered: true\ntickets:") || !board.ParseWorkstream(ordered.Slug, chain).Ordered {
		t.Fatalf("ordered workstream file:\n%s", chain)
	}
	if strings.Contains(string(data), "ordered") {
		t.Fatalf("unordered workstream writes the ordered field:\n%s", data)
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
