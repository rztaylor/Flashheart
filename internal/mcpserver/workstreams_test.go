package mcpserver

import (
	"slices"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/store"
)

// workstream writes a minimal workstream file in demo.
func (e *env) workstream(slug string, tickets ...string) {
	e.t.Helper()
	list := "[]"
	if len(tickets) > 0 {
		list = ""
		for _, id := range tickets {
			list += "\n  - " + id
		}
	}
	write(e.t, e.root+"/demo/workstreams/"+slug+".md", "---\nslug: "+slug+"\nstatus: active\npriority: medium\ntickets: "+list+"\ndepends-on-workstreams: []\ntags: []\n---\n\n# "+slug+"\n")
}

func (e *env) members(slug string) []string {
	e.t.Helper()
	return mdfileList(e.t, e.readFile("demo/workstreams/"+slug+".md"), "tickets")
}

func (e *env) field(id, key string) string {
	e.t.Helper()
	value, _ := mdfile.Parse([]byte(e.file(id))).String(key)
	return value
}

func TestCreateTicketJoinsItsWorkstreamsList(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	first := e.ticket(store.NewTicket{Title: "First"})
	e.workstream("panel", first)
	e.startSession(session)
	contains(t, e.ok("create_ticket", map[string]any{"type": "feature", "title": "Second", "description": "", "criteria": []string{}, "priority": "low", "workstream": "panel"}), "ok ticket=DM-2")
	if got := e.members("panel"); !slices.Equal(got, []string{first, "DM-2"}) {
		t.Fatalf("panel tickets = %v", got)
	}
	if e.field("DM-2", "workstream") != "panel" {
		t.Fatal("field not set")
	}

	out := e.fails("create_ticket", map[string]any{"type": "feature", "title": "Third", "description": "", "criteria": []string{}, "priority": "low", "workstream": "nope"}, "not_found")
	contains(t, out, `workstream "nope"`, "panel", "create_workstream")
	if e.exists("demo/tickets/DM-3-third") {
		t.Fatal("ticket created despite the missing workstream")
	}
}

func TestUpdateTicketJoinsChangesAndLeavesWorkstreams(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	a := e.ticket(store.NewTicket{Title: "A"})
	b := e.ticket(store.NewTicket{Title: "B"})
	e.workstream("panel", a)
	e.workstream("sync")
	e.startSession(session)

	contains(t, e.ok("update_ticket", map[string]any{"ticket": b, "set": map[string]any{"workstream": "panel", "priority": "high"}}), "changed=priority,workstream")
	if got := e.members("panel"); !slices.Equal(got, []string{a, b}) {
		t.Fatalf("after join: panel = %v", got)
	}
	if e.field(b, "priority") != "high" || e.field(b, "workstream") != "panel" {
		t.Fatal("fields not set on join")
	}

	e.ok("update_ticket", map[string]any{"ticket": b, "set": map[string]any{"workstream": "sync"}})
	if got, sync := e.members("panel"), e.members("sync"); !slices.Equal(got, []string{a}) || !slices.Equal(sync, []string{b}) {
		t.Fatalf("after change: panel = %v, sync = %v", got, sync)
	}

	e.ok("update_ticket", map[string]any{"ticket": b, "set": map[string]any{"workstream": ""}})
	if got := e.members("sync"); len(got) != 0 || e.field(b, "workstream") != "" {
		t.Fatalf("after leave: sync = %v, field %q", got, e.field(b, "workstream"))
	}

	out := e.fails("update_ticket", map[string]any{"ticket": b, "set": map[string]any{"workstream": "nope"}}, "not_found")
	contains(t, out, "panel, sync")
}

func TestCreateWorkstream(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	a := e.ticket(store.NewTicket{Title: "A"})
	b := e.ticket(store.NewTicket{Title: "B"})
	held := e.ticket(store.NewTicket{Title: "Held"})
	e.workstream("old", a)
	e.events("claude:77777777-0000", events.Event{Kind: events.RunStart, Data: events.RunStartData{Kind: events.KindSession, Worktree: "/elsewhere"}},
		events.Event{Kind: events.Claim, Data: events.TicketData{Ticket: held}})
	e.startSession(session)

	out := e.ok("create_workstream", map[string]any{"title": "Card panel", "goal": "Open tickets beside the board.", "priority": "high",
		"tickets": []string{b, a}, "depends_on_workstreams": []string{"old"}, "tags": []string{"ui"}})
	contains(t, out, `Created workstream card-panel "Card panel" with DM-2, DM-1.`, "ok workstream=card-panel")
	file := e.readFile("demo/workstreams/card-panel.md")
	contains(t, file, "slug: card-panel", "priority: high", "depends-on-workstreams: [old]", "tags: [ui]", "# Card panel", "## Goal\n\nOpen tickets beside the board.")
	if got := e.members("card-panel"); !slices.Equal(got, []string{b, a}) {
		t.Fatalf("card-panel tickets = %v", got)
	}
	if got := e.members("old"); len(got) != 0 {
		t.Fatalf("old still lists %v", got)
	}
	for _, id := range []string{a, b} {
		if e.field(id, "workstream") != "card-panel" {
			t.Errorf("%s workstream field not set", id)
		}
	}
	if kinds := e.kinds(session); !slices.Contains(kinds, events.TicketUpdated) {
		t.Errorf("no ticket.updated event for the joined tickets: %v", kinds)
	}

	contains(t, e.ok("create_workstream", map[string]any{"title": "Card panel", "goal": "Again."}), "ok workstream=card-panel-2")
	e.fails("create_workstream", map[string]any{"title": "X", "goal": "g", "tickets": []string{"DM-99"}}, "not_found")
	e.fails("create_workstream", map[string]any{"title": "X", "goal": "g", "priority": "urgent"}, "invalid_input")
	e.fails("create_workstream", map[string]any{"title": "", "goal": "g"}, "invalid_input")
	e.fails("create_workstream", map[string]any{"title": "X", "goal": "g", "tickets": []string{held}}, "claimed")
	if e.exists("demo/workstreams/x.md") {
		t.Fatal("a refused create wrote a workstream")
	}
}

func TestBoardContextListsActiveWorkstreams(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	a := e.ticket(store.NewTicket{Title: "A"})
	b := e.ticket(store.NewTicket{Title: "B"})
	finished := e.ticket(store.NewTicket{Title: "Finished"})
	e.setStatus(finished, board.Done, "")
	e.workstream("panel", a, b)
	e.workstream("shipped", finished)
	e.startSession(session)

	out := e.ok("board_context", nil)
	contains(t, out, `Workstreams: panel "panel" (0 of 2 done, next DM-1).`)
	if contains2(out, "shipped") {
		t.Fatalf("completed workstream listed:\n%s", out)
	}
}
