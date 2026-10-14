package mcpserver

import (
	"maps"
	"slices"
	"testing"
	"time"

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
	contains(t, out, `Created workstream card-panel "Card panel" with DM-2, DM-1.`, "in any order", "ok workstream=card-panel")
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

	// A project's workstream titles are unique, ignoring case and spacing
	// (FH-163, D76); another title with the same slug takes the next one.
	contains(t, e.fails("create_workstream", map[string]any{"title": "card  PANEL", "goal": "Again."}, "invalid_input"),
		`a workstream called "Card panel" is already in demo (card-panel)`, "choose another title", "update_workstream workstream=card-panel")
	contains(t, e.ok("create_workstream", map[string]any{"title": "Card panel!", "goal": "Again."}), "ok workstream=card-panel-2")
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

// TestUpdateWorkstream: one call sets a workstream's fields and moves
// tickets in and out, keeping each ticket's field and every list in step;
// the slug stays when the title changes (FH-46).
func TestUpdateWorkstream(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	a := e.ticket(store.NewTicket{Title: "A"})
	b := e.ticket(store.NewTicket{Title: "B"})
	c := e.ticket(store.NewTicket{Title: "C"})
	e.workstream("panel", a, b)
	e.workstream("sync", c)
	e.workstream("storage")
	e.startSession(session)

	out := e.ok("update_workstream", map[string]any{"workstream": "panel",
		"set": map[string]any{"title": "Card panel", "goal": "Open tickets beside the board.", "priority": "high",
			"tags": []any{"ui"}, "depends-on-workstreams": []any{"storage"}},
		"add_tickets": []string{c}, "remove_tickets": []string{a}})
	contains(t, out, `Updated workstream panel "Card panel"`, "added DM-3", "removed DM-1", "ok workstream=panel changed=depends-on-workstreams,goal,priority,tags,title,tickets")
	file := e.readFile("demo/workstreams/panel.md")
	contains(t, file, "slug: panel", "priority: high", "depends-on-workstreams: [storage]", "tags: [ui]", "# Card panel", "## Goal\n\nOpen tickets beside the board.")
	if e.exists("demo/workstreams/card-panel.md") {
		t.Fatal("the title change renamed the file")
	}
	if got := e.members("panel"); !slices.Equal(got, []string{b, c}) {
		t.Fatalf("panel tickets = %v", got)
	}
	if got := e.members("sync"); len(got) != 0 {
		t.Fatalf("sync still lists %v", got)
	}
	for id, want := range map[string]string{a: "", c: "panel"} {
		if got := e.field(id, "workstream"); got != want {
			t.Errorf("%s workstream = %q, want %q", id, got, want)
		}
	}
	updated := 0
	for _, ev := range e.log() {
		var data events.TicketData
		if ev.Kind == events.TicketUpdated && ev.Run == session && ev.Decode(&data) == nil && slices.Equal(data.Fields, []string{"workstream"}) {
			updated++
		}
	}
	if updated != 2 {
		t.Errorf("%d ticket.updated events for the moved tickets, want 2", updated)
	}
	// A project named by key is the same project.
	contains(t, e.ok("update_workstream", map[string]any{"workstream": "panel", "project": "DM", "set": map[string]any{"priority": "low"}}), "changed=priority")
}

// TestUpdateWorkstreamRenamesItsSlug: rename_to moves the workstream to a
// new slug and rewrites every reference to it, members' workstream fields
// and every depends-on-workstreams, in the same call as its other edits;
// each rewritten ticket gets a ticket.updated naming the fields (FH-158).
func TestUpdateWorkstreamRenamesItsSlug(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	a := e.ticket(store.NewTicket{Title: "A"})
	b := e.ticket(store.NewTicket{Title: "B"})
	c := e.ticket(store.NewTicket{Title: "C"})
	e.startSession(session)
	e.ok("create_workstream", map[string]any{"title": "panel", "goal": "Open tickets.", "tickets": []string{a, b}})
	e.ok("create_workstream", map[string]any{"title": "sync", "goal": "Sync.", "depends_on_workstreams": []string{"panel"}})
	e.ok("update_ticket", map[string]any{"ticket": c, "set": map[string]any{"depends-on-workstreams": []string{"panel"}}})

	out := e.ok("update_workstream", map[string]any{"workstream": "panel", "rename_to": "card-panel", "set": map[string]any{"title": "Card panel"}})
	contains(t, out, `Updated workstream card-panel "Card panel"`, "renamed from panel", "rewrote DM-1, DM-2, DM-3", "ok workstream=card-panel changed=slug,title")
	if e.exists("demo/workstreams/panel.md") {
		t.Fatal("the old file is still there")
	}
	contains(t, e.readFile("demo/workstreams/card-panel.md"), "slug: card-panel", "# Card panel", "  - "+a, "  - "+b)
	contains(t, e.readFile("demo/workstreams/sync.md"), "depends-on-workstreams: [card-panel]")
	for _, id := range []string{a, b} {
		if got := e.field(id, "workstream"); got != "card-panel" {
			t.Errorf("%s workstream = %q", id, got)
		}
	}
	if got := mdfileList(t, e.file(c), "depends-on-workstreams"); !slices.Equal(got, []string{"card-panel"}) {
		t.Errorf("%s depends-on-workstreams = %v", c, got)
	}
	// The rename's events are the last for each ticket.
	updated := map[string][]string{}
	for _, ev := range e.log() {
		var data events.TicketData
		if ev.Kind == events.TicketUpdated && ev.Run == session && ev.Decode(&data) == nil {
			updated[data.Ticket] = data.Fields
		}
	}
	if !slices.Equal(updated[a], []string{"workstream"}) || !slices.Equal(updated[c], []string{"depends-on-workstreams"}) {
		t.Errorf("ticket.updated fields = %v", updated)
	}
	// The new slug is the workstream's from now on.
	contains(t, e.ok("update_workstream", map[string]any{"workstream": "card-panel", "set": map[string]any{"priority": "low"}}), "ok workstream=card-panel changed=priority")
	contains(t, e.fails("update_workstream", map[string]any{"workstream": "panel", "set": map[string]any{"priority": "low"}}, "not_found"), "card-panel, sync")
}

// TestUpdateWorkstreamRefusesAndWritesNothing: an unknown field,
// workstream, ticket or dependency, a ticket of another project or held by
// another session, a cycle and an empty call each refuse with a clear
// error and change no file.
func TestUpdateWorkstreamRefusesAndWritesNothing(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	a := e.ticket(store.NewTicket{Title: "A"})
	held := e.ticket(store.NewTicket{Title: "Held"})
	e.workstream("panel", a)
	e.workstream("storage")
	write(t, e.root+"/demo/workstreams/sync.md", "---\nslug: sync\nstatus: active\npriority: medium\ntickets: []\ndepends-on-workstreams: [panel]\ntags: []\n---\n\n# sync\n")
	write(t, e.root+"/other/project.yaml", "key: OT\nname: other\n")
	other, err := e.store.CreateTicket("other", store.NewTicket{Title: "Elsewhere"})
	if err != nil {
		t.Fatal(err)
	}
	e.rivalHolds(held)
	e.startSession(session)
	files := map[string]string{}
	for _, name := range []string{"demo/workstreams/panel.md", "demo/workstreams/sync.md", "demo/workstreams/storage.md", "demo/project.yaml"} {
		files[name] = e.readFile(name)
	}
	ticketFiles := map[string]string{a: e.file(a), held: e.file(held)}

	for _, tc := range []struct {
		args map[string]any
		code string
		says []string
	}{
		{map[string]any{"workstream": "panel", "set": map[string]any{"ordered": true}}, "invalid_input", []string{`"ordered"`, "title, goal, priority, tags, depends-on-workstreams"}},
		{map[string]any{"workstream": "panel", "set": map[string]any{"slug": "renamed"}}, "invalid_input", []string{`"slug"`, "rename_to"}},
		{map[string]any{"workstream": "panel", "rename_to": "storage", "add_tickets": []string{a}}, "invalid_input", []string{"storage already exists", "choose another slug"}},
		{map[string]any{"workstream": "panel", "rename_to": "Card Panel"}, "invalid_input", []string{`"Card Panel"`, "lower-case letters"}},
		{map[string]any{"workstream": "panel", "rename_to": "panel"}, "invalid_input", []string{"nothing to change"}},
		{map[string]any{"workstream": "panel", "set": map[string]any{"priority": "urgent"}}, "invalid_input", []string{"priority must be one of"}},
		{map[string]any{"workstream": "panel", "set": map[string]any{"tags": 3}}, "invalid_input", []string{"list of strings"}},
		{map[string]any{"workstream": "nope", "set": map[string]any{"priority": "low"}}, "not_found", []string{`workstream "nope"`, "panel, storage, sync", "create_workstream"}},
		{map[string]any{"workstream": "panel", "add_tickets": []string{"DM-99"}}, "not_found", []string{"DM-99"}},
		{map[string]any{"workstream": "panel", "add_tickets": []string{other.ID}}, "invalid_input", []string{other.ID, "project other"}},
		{map[string]any{"workstream": "panel", "add_tickets": []string{held}}, "claimed", []string{held}},
		{map[string]any{"workstream": "panel", "remove_tickets": []string{held}}, "claimed", []string{held}},
		{map[string]any{"workstream": "storage", "remove_tickets": []string{a}}, "invalid_input", []string{a + " is not in workstream storage"}},
		{map[string]any{"workstream": "panel", "set": map[string]any{"depends-on-workstreams": []any{"nope"}}}, "not_found", []string{`workstream "nope"`}},
		{map[string]any{"workstream": "panel", "set": map[string]any{"depends-on-workstreams": []any{"sync"}}, "add_tickets": []string{a}}, "invalid_input", []string{"cycle", "panel → sync → panel"}},
		{map[string]any{"workstream": "panel"}, "invalid_input", []string{"nothing to change"}},
		{map[string]any{"workstream": "panel", "set": map[string]any{"title": "Storage"}, "add_tickets": []string{a}}, "invalid_input", []string{`a workstream called "storage" is already in demo (storage)`, "choose another title"}},
	} {
		contains(t, e.fails("update_workstream", tc.args, tc.code), tc.says...)
	}
	for name, data := range files {
		if e.readFile(name) != data {
			t.Errorf("a refused call changed %s", name)
		}
	}
	for id, data := range ticketFiles {
		if e.file(id) != data {
			t.Errorf("a refused call changed %s", id)
		}
	}
	if len(e.history()) != 0 {
		t.Errorf("refused calls wrote history %+v", e.history())
	}
}

// TestWorkstreamDependencyBlocksTicketsAddedLater: a dependency set on a
// workstream holds back the tickets that join it afterwards, by
// create_ticket or update_workstream, and list_tickets, get_ticket and
// board_context say why (FH-46).
func TestWorkstreamDependencyBlocksTicketsAddedLater(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	first := e.ticket(store.NewTicket{Title: "Schema"})
	e.workstream("storage", first)
	e.workstream("panel")
	e.startSession(session)

	e.ok("update_workstream", map[string]any{"workstream": "panel", "set": map[string]any{"depends-on-workstreams": []any{"storage"}}})
	contains(t, e.ok("create_ticket", map[string]any{"type": "feature", "title": "Later", "description": "", "criteria": []string{}, "priority": "low", "workstream": "panel"}), "ok ticket=DM-2")
	joined := e.ticket(store.NewTicket{Title: "Joined"})
	e.ok("update_workstream", map[string]any{"workstream": "panel", "add_tickets": []string{joined}})

	reason := "Its workstream panel depends on workstream storage, which has 1 ticket not yet in review or done"
	blocked := e.ok("list_tickets", map[string]any{"blocked": true})
	contains(t, blocked, "DM-2 feature", joined+" ", reason)
	contains(t, e.ok("get_ticket", map[string]any{"ticket": joined}), reason)
	e.ok("claim", map[string]any{"ticket": joined, "force": true, "reason": "testing the reason"})
	contains(t, e.ok("board_context", nil), "Blocked: "+reason, `panel "panel" (0 of 2 done`)
}

// TestCreateTicketInAnotherProject: create_ticket files a ticket in the
// project its project input names, by name or key, with that project's
// next id and workstreams (FH-46).
func TestCreateTicketInAnotherProject(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	e.ticket(store.NewTicket{Title: "Here"})
	write(t, e.root+"/other/project.yaml", "key: OT\nname: other\n")
	write(t, e.root+"/other/workstreams/sync.md", "---\nslug: sync\nstatus: active\npriority: medium\ntickets: []\ndepends-on-workstreams: []\ntags: []\n---\n\n# sync\n")
	e.startSession(session)

	with := func(extra map[string]any) map[string]any {
		args := map[string]any{"type": "bug", "title": "Sync drops edits", "description": "Edits made offline are lost.", "criteria": []string{"No edit is lost"}, "priority": "high"}
		maps.Copy(args, extra)
		return args
	}
	contains(t, e.ok("create_ticket", with(map[string]any{"project": "OT", "workstream": "sync"})), "Created OT-1", "ok ticket=OT-1")
	contains(t, e.ok("create_ticket", with(map[string]any{"project": "other"})), "ok ticket=OT-2")
	data, _, err := e.store.ReadTicket("other", "OT-1")
	if err != nil {
		t.Fatal(err)
	}
	if workstream, _ := mdfile.Parse(data).String("workstream"); workstream != "sync" {
		t.Fatalf("OT-1 workstream = %q", workstream)
	}
	if got := mdfileList(t, e.readFile("other/workstreams/sync.md"), "tickets"); !slices.Equal(got, []string{"OT-1"}) {
		t.Fatalf("sync tickets = %v", got)
	}
	if e.exists("demo/tickets/DM-2-sync-drops-edits") {
		t.Fatal("the ticket was also filed in the caller's project")
	}
	// The other project's log records the caller's ticket.created.
	var created []string
	if err := events.New(e.store).Read("other", time.Time{}, func(ev events.Event) {
		var data events.TicketData
		if ev.Kind == events.TicketCreated && ev.Run == session && ev.Decode(&data) == nil {
			created = append(created, data.Ticket)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(created, []string{"OT-1", "OT-2"}) {
		t.Fatalf("ticket.created in other = %v", created)
	}
	contains(t, e.fails("create_ticket", with(map[string]any{"project": "nope"}), "not_found"), `no project "nope"`, "demo", "other")
	contains(t, e.fails("create_ticket", with(map[string]any{"project": "OT", "workstream": "panel"}), "not_found"), `workstream "panel"`, "sync")
}

func TestWorkstreamOrderDoesNotBlock(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	a := e.ticket(store.NewTicket{Title: "First"})
	b := e.ticket(store.NewTicket{Title: "Second"})
	c := e.ticket(store.NewTicket{Title: "Third", DependsOn: []string{a, b}})
	e.workstream("epic", a, b, c)
	e.startSession(session)

	contains(t, e.ok("board_context", nil), `epic "epic" (0 of 3 done, next DM-1)`)
	if out := e.ok("get_ticket", map[string]any{"ticket": b}); contains2(out, "Blocked") {
		t.Fatalf("a later ticket in a workstream is blocked by order:\n%s", out)
	}
	contains(t, e.ok("get_ticket", map[string]any{"ticket": c}), "Depends on DM-1", "Depends on DM-2")
	blocked := e.ok("list_tickets", map[string]any{"blocked": true})
	contains(t, blocked, c)
	if contains2(blocked, b+" feature") {
		t.Fatalf("list_tickets shows a workstream ticket blocked by order:\n%s", blocked)
	}
}
