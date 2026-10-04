package board

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func ticketMD(front, body string) []byte {
	return []byte("---\n" + front + "---\n" + body)
}

const fullFront = "type: feature\nproject: alpha\ncreated: 2026-10-02\npriority: high\n"

func TestParseTicketReadsFieldsAndBody(t *testing.T) {
	t.Parallel()

	data := ticketMD(fullFront+"branch: feature/x\nworkstream: board-ui\ndepends-on: [feat--a, beta/feat--b]\ndepends-on-workstreams: ws\ntags: [ui]\nupdated: 2026-10-04T14:12:09Z\n",
		"\n# Card panel\n\n## Description\n\nFirst paragraph of the\ndescription.\n\nSecond paragraph.\n\n## Acceptance Criteria\n\n- [x] One\n- [ ] Two\n\n## Handoff\n\n_Updated by run._\n\n**Done**\n- Thing\n**Next**\n- Review tab\n- Then tests\n**Files**\n- a.go\n")
	ticket := ParseTicket("feat--card-panel", InProgress, data)

	if ticket.Title != "Card panel" || ticket.Type != "feature" || ticket.Priority != "high" || ticket.Branch != "feature/x" {
		t.Errorf("ticket = %+v", ticket)
	}
	if !reflect.DeepEqual(ticket.DependsOn, []string{"feat--a", "beta/feat--b"}) || !reflect.DeepEqual(ticket.DependsOnWorkstreams, []string{"ws"}) {
		t.Errorf("dependencies = %q %q", ticket.DependsOn, ticket.DependsOnWorkstreams)
	}
	if ticket.Excerpt != "First paragraph of the description." {
		t.Errorf("Excerpt = %q", ticket.Excerpt)
	}
	if want := []Criterion{{"One", true}, {"Two", false}}; !reflect.DeepEqual(ticket.Criteria, want) {
		t.Errorf("Criteria = %+v", ticket.Criteria)
	}
	if ticket.Handoff == nil || !reflect.DeepEqual(ticket.Handoff.Next, []string{"Review tab", "Then tests"}) || !strings.Contains(ticket.Handoff.Markdown, "**Files**") {
		t.Errorf("Handoff = %+v", ticket.Handoff)
	}
	if len(ticket.Warnings) != 0 || ticket.NeedsRepair() {
		t.Errorf("warnings=%q repair=%q", ticket.Warnings, ticket.Repair)
	}
}

func TestParseTicketWarnings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		slug  string
		front string
		want  string
	}{
		{"missing required", "feat--x", "type: feature\n", "missing project, created, priority"},
		{"bad priority", "feat--x", strings.Replace(fullFront, "high", "urgent", 1), `priority "urgent" should be high, medium or low`},
		{"type mismatch", "bug--x", fullFront, `type "feature" does not match the bug-- prefix`},
		{"unknown type", "feat--x", strings.Replace(fullFront, "feature", "chore", 1), `type "chore" is not a known ticket type`},
		{"bad date", "feat--x", strings.Replace(fullFront, "2026-10-02", "Oct 2", 1), `created "Oct 2" should be YYYY-MM-DD`},
		{"bad filename", "notes", fullFront, "filename should be <type>--<slug>.md"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ticket := ParseTicket(test.slug, Todo, ticketMD(test.front, "# T\n"))
			if !slices.ContainsFunc(ticket.Warnings, func(w string) bool { return strings.Contains(w, test.want) }) {
				t.Errorf("warnings = %q, want %q", ticket.Warnings, test.want)
			}
			if ticket.NeedsRepair() {
				t.Errorf("warnings must not mark a ticket as needing repair: %q", ticket.Repair)
			}
		})
	}
}

func TestBrokenFrontmatterNeedsRepairButStillShows(t *testing.T) {
	t.Parallel()

	ticket := ParseTicket("docs--broken", Todo, []byte("---\ntype: docs\npriority: [unclosed\n---\n\n# Broken frontmatter\n\nBody.\n"))
	if !ticket.NeedsRepair() || !strings.Contains(ticket.Repair[0], "frontmatter does not parse") {
		t.Fatalf("Repair = %q", ticket.Repair)
	}
	if ticket.Title != "Broken frontmatter" || ticket.Type != "docs" || !strings.Contains(ticket.Body, "Body.") {
		t.Errorf("broken ticket lost its title, type or body: %+v", ticket)
	}
}

func TestTitleFallsBackToSlug(t *testing.T) {
	t.Parallel()

	if got := ParseTicket("feat--untitled", Todo, ticketMD(fullFront, "no heading\n")).Title; got != "feat--untitled" {
		t.Errorf("Title = %q", got)
	}
}

func TestExcerptIsBoundedPlainText(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("word ", 100)
	ticket := ParseTicket("feat--x", Todo, ticketMD(fullFront, "# T\n\n## Description\n\n**Bold** "+long+"\n"))
	if n := len([]rune(ticket.Excerpt)); n > ExcerptRunes+1 {
		t.Errorf("Excerpt has %d runes", n)
	}
	if !strings.HasSuffix(ticket.Excerpt, "…") || strings.Contains(ticket.Excerpt, "**") {
		t.Errorf("Excerpt = %q", ticket.Excerpt)
	}
}

func TestParseWorkstream(t *testing.T) {
	t.Parallel()

	ws := ParseWorkstream("board-ui", []byte("---\nslug: board-ui\nstatus: active\npriority: high\ncreated: 2026-10-01\ntickets:\n  - feat--a\n  - feat--b\ndepends-on-workstreams: [base]\ntags: [ui]\n---\n\n# Board UI\n\n## Goal\n\nRender.\n"))
	if ws.Title != "Board UI" || ws.Status != "active" || !reflect.DeepEqual(ws.Tickets, []string{"feat--a", "feat--b"}) || !reflect.DeepEqual(ws.DependsOnWorkstreams, []string{"base"}) {
		t.Errorf("workstream = %+v", ws)
	}
	broken := ParseWorkstream("bad", []byte("---\ntickets: [oops\n---\n# Bad\n"))
	if !broken.NeedsRepair() || broken.Title != "Bad" {
		t.Errorf("broken workstream = %+v", broken)
	}
	if mismatch := ParseWorkstream("a", []byte("---\nslug: b\n---\n")); !slices.ContainsFunc(mismatch.Warnings, func(w string) bool { return strings.Contains(w, `slug "b" does not match`) }) {
		t.Errorf("warnings = %q", mismatch.Warnings)
	}
}

func TestMarkDuplicates(t *testing.T) {
	t.Parallel()

	project := Project{Name: "p", Tickets: []Ticket{
		ParseTicket("feat--a", Todo, ticketMD(fullFront, "# A\n")),
		ParseTicket("feat--a", Done, ticketMD(fullFront, "# A again\n")),
		ParseTicket("feat--b", Todo, ticketMD(fullFront, "# B\n")),
	}}
	MarkDuplicates(&project)
	if !project.Tickets[0].NeedsRepair() || !project.Tickets[1].NeedsRepair() || project.Tickets[2].NeedsRepair() {
		t.Fatalf("repairs = %q / %q / %q", project.Tickets[0].Repair, project.Tickets[1].Repair, project.Tickets[2].Repair)
	}
	if !strings.Contains(project.Tickets[0].Repair[0], "also in done") || !strings.Contains(project.Tickets[1].Repair[0], "also in todo") {
		t.Errorf("repairs = %q / %q", project.Tickets[0].Repair, project.Tickets[1].Repair)
	}
}

// t builds a ticket for blocking tables.
func tk(slug string, column Column, extra string) Ticket {
	return ParseTicket(slug, column, ticketMD(fullFront+extra, "# "+slug+"\n"))
}

func ws(slug string, tickets []string, deps ...string) Workstream {
	front := "tickets: [" + strings.Join(tickets, ", ") + "]\n"
	if len(deps) > 0 {
		front += "depends-on-workstreams: [" + strings.Join(deps, ", ") + "]\n"
	}
	return ParseWorkstream(slug, []byte("---\n"+front+"---\n# "+slug+"\n"))
}

func reasonsOf(analysis Analysis, project, slug string) []Reason {
	return analysis.Blocked[Ref{Project: project, Slug: slug}]
}

func TestBlockingRules(t *testing.T) {
	t.Parallel()

	alpha := Project{
		Name: "alpha",
		Tickets: []Ticket{
			tk("feat--done", Done, ""),
			tk("feat--review", ReadyToReview, ""),
			tk("feat--wip", InProgress, ""),
			tk("feat--free", Todo, "depends-on: [feat--done, feat--review, feat--archived]\n"),
			tk("feat--dep", Todo, "depends-on: [feat--wip]\n"),
			tk("feat--missing", Todo, "depends-on: [feat--nope]\n"),
			tk("feat--cross", Todo, "depends-on: [beta/feat--b-wip, beta/feat--b-done, gamma/feat--x]\n"),
			tk("feat--w1", Done, "workstream: one\n"),
			tk("feat--w2", InProgress, "workstream: one\n"),
			tk("feat--w3", Todo, "workstream: one\n"),
			tk("feat--wsdep", Todo, "depends-on-workstreams: [one, ghost, two]\n"),
			tk("feat--viaws", Todo, "workstream: three\n"),
			tk("feat--lost", Todo, "workstream: nowhere\n"),
			tk("feat--doneblocked", Done, "depends-on: [feat--wip]\n"),
		},
		Workstreams: []Workstream{
			ws("one", []string{"feat--w1", "feat--w2", "feat--w3"}),
			ws("two", []string{"feat--done", "feat--review"}),
			ws("three", []string{"feat--viaws"}, "one"),
		},
		Archived: []string{"feat--archived"},
	}
	beta := Project{Name: "beta", Tickets: []Ticket{tk("feat--b-wip", InProgress, ""), tk("feat--b-done", Done, "")}}
	analysis := Analyze(Board{Projects: []Project{alpha, beta}})

	for _, free := range []string{"feat--free", "feat--w2", "feat--done", "feat--doneblocked"} {
		if reasons := reasonsOf(analysis, "alpha", free); len(reasons) != 0 {
			t.Errorf("%s reasons = %+v, want none", free, reasons)
		}
	}
	tests := []struct {
		slug string
		want []Reason
	}{
		{"feat--dep", []Reason{{Kind: TicketDependency, Ticket: Ref{"alpha", "feat--wip"}, Column: InProgress}}},
		{"feat--missing", []Reason{{Kind: TicketDependency, Ticket: Ref{"alpha", "feat--nope"}, Missing: true}}},
		{"feat--cross", []Reason{
			{Kind: TicketDependency, Ticket: Ref{"beta", "feat--b-wip"}, Column: InProgress},
			{Kind: TicketDependency, Ticket: Ref{"gamma", "feat--x"}, Missing: true},
		}},
		{"feat--w3", []Reason{{Kind: WorkstreamOrder, Workstream: "one", Ticket: Ref{"alpha", "feat--w2"}, Column: InProgress}}},
		{"feat--wsdep", []Reason{
			{Kind: WorkstreamDependency, Workstream: "one", Pending: 2},
			{Kind: WorkstreamDependency, Workstream: "ghost", Missing: true},
		}},
		{"feat--viaws", []Reason{{Kind: WorkstreamDependency, Workstream: "one", Pending: 2, Via: "three"}}},
		{"feat--lost", []Reason{{Kind: WorkstreamDependency, Workstream: "nowhere", Missing: true, Via: "nowhere"}}},
	}
	for _, test := range tests {
		if got := reasonsOf(analysis, "alpha", test.slug); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s reasons =\n  %+v\nwant\n  %+v", test.slug, got, test.want)
		}
	}
}

func TestDuplicatedDependencyIsSatisfiedOnlyWhenEveryCopyIs(t *testing.T) {
	t.Parallel()

	project := Project{Name: "p", Tickets: []Ticket{
		tk("feat--twice", Done, ""),
		tk("feat--twice", Todo, ""),
		tk("feat--waits", Todo, "depends-on: [feat--twice]\n"),
	}}
	MarkDuplicates(&project)
	analysis := Analyze(Board{Projects: []Project{project}})
	if reasons := reasonsOf(analysis, "p", "feat--waits"); len(reasons) != 1 || reasons[0].Column != Todo {
		t.Errorf("reasons = %+v", reasons)
	}
}

func TestWorkstreamStatus(t *testing.T) {
	t.Parallel()

	project := Project{
		Name: "p",
		Tickets: []Ticket{
			tk("feat--a", Done, ""), tk("feat--b", ReadyToReview, ""),
			tk("feat--c", InProgress, ""), tk("feat--d", Todo, "depends-on: [feat--c]\n"),
			tk("feat--e", Todo, ""),
		},
		Workstreams: []Workstream{
			ws("complete", []string{"feat--a", "feat--b"}),
			ws("moving", []string{"feat--a", "feat--c", "feat--e"}),
			ws("stuck", []string{"feat--d", "feat--e"}),
			ws("waiting", []string{"feat--e"}, "moving"),
			ws("empty", nil),
			ws("broken-ref", []string{"feat--ghost"}),
		},
	}
	analysis := Analyze(Board{Projects: []Project{project}})
	tests := map[string]WorkstreamState{
		"complete":   {Status: StatusCompleted, Done: 2, Total: 2},
		"moving":     {Status: StatusActive, Done: 1, Total: 3, Next: "feat--c"},
		"stuck":      {Status: StatusBlocked, Done: 0, Total: 2, Next: "feat--d", Reasons: []Reason{{Kind: TicketDependency, Ticket: Ref{"p", "feat--c"}, Column: InProgress}}},
		"waiting":    {Status: StatusBlocked, Done: 0, Total: 1, Next: "feat--e", Reasons: []Reason{{Kind: WorkstreamDependency, Workstream: "moving", Pending: 2}}},
		"empty":      {Status: StatusActive},
		"broken-ref": {Status: StatusBlocked, Done: 0, Total: 1, Next: "feat--ghost", Reasons: []Reason{{Kind: TicketDependency, Ticket: Ref{"p", "feat--ghost"}, Missing: true}}},
	}
	for slug, want := range tests {
		if got := analysis.Workstreams[Ref{Project: "p", Slug: slug}]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %+v, want %+v", slug, got, want)
		}
	}
}

func TestMembershipMismatchWarns(t *testing.T) {
	t.Parallel()

	project := Project{
		Name:        "p",
		Tickets:     []Ticket{tk("feat--claims", Todo, "workstream: one\n"), tk("feat--listed", Todo, "")},
		Workstreams: []Workstream{ws("one", []string{"feat--listed"})},
	}
	analysis := Analyze(Board{Projects: []Project{project}})
	if got := analysis.Warnings[Ref{"p", "feat--claims"}]; len(got) != 1 || !strings.Contains(got[0], "not listed in workstream one") {
		t.Errorf("claims warnings = %q", got)
	}
	if got := analysis.Warnings[Ref{"p", "feat--listed"}]; len(got) != 1 || !strings.Contains(got[0], `workstream one lists this ticket`) {
		t.Errorf("listed warnings = %q", got)
	}
}

func TestSampleFixtureTickets(t *testing.T) {
	t.Parallel()

	dir := filepath.Join("..", "..", "testdata", "boards", "sample", "alpha")
	read := func(column Column, slug string) Ticket {
		data, err := os.ReadFile(filepath.Join(dir, string(column), slug+".md"))
		if err != nil {
			t.Fatal(err)
		}
		return ParseTicket(slug, column, data)
	}
	broken := read(Todo, "docs--broken-frontmatter")
	if !broken.NeedsRepair() || broken.Title != "Broken frontmatter" {
		t.Errorf("broken fixture = %+v", broken)
	}
	panel := read(InProgress, "feat--card-panel")
	if panel.Handoff == nil || !reflect.DeepEqual(panel.Handoff.Next, []string{"Review tab"}) || panel.Workstream != "board-ui" {
		t.Errorf("card panel fixture = %+v", panel)
	}
	if overflow := read(Todo, "bug--column-overflow"); len(overflow.Warnings) != 0 || overflow.Branch != "" {
		t.Errorf("column overflow fixture warnings = %q branch = %q", overflow.Warnings, overflow.Branch)
	}
}

func TestAttachmentTypeAllowList(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]string{"a.PNG": "image/png", "b.jpeg": "image/jpeg", "c.log": "text/plain; charset=utf-8"} {
		if got, ok := AttachmentType(name); !ok || got != want {
			t.Errorf("AttachmentType(%q) = %q, %v", name, got, ok)
		}
	}
	for _, name := range []string{"x.svg", "x.html", "x.htm", "x", "x.exe", "index.yaml"} {
		if _, ok := AttachmentType(name); ok {
			t.Errorf("AttachmentType(%q) allowed", name)
		}
	}
}

func TestReasonDescriptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reason Reason
		want   string
	}{
		{Reason{Kind: TicketDependency, Ticket: Ref{"alpha", "feat--a"}, Column: Todo}, "Depends on feat--a, which is in To do"},
		{Reason{Kind: TicketDependency, Ticket: Ref{"beta", "feat--b"}, Column: InProgress}, "Depends on beta/feat--b, which is In progress"},
		{Reason{Kind: TicketDependency, Ticket: Ref{"alpha", "feat--x"}, Missing: true}, "Depends on feat--x, which does not exist"},
		{Reason{Kind: WorkstreamDependency, Workstream: "core", Pending: 1}, "Depends on workstream core, which has 1 ticket not yet in review or done"},
		{Reason{Kind: WorkstreamDependency, Workstream: "core", Pending: 3, Via: "ui"}, "Its workstream ui depends on workstream core, which has 3 tickets not yet in review or done"},
		{Reason{Kind: WorkstreamDependency, Workstream: "ghost", Missing: true}, "Depends on workstream ghost, which does not exist"},
		{Reason{Kind: WorkstreamDependency, Workstream: "ghost", Missing: true, Via: "ghost"}, "Belongs to workstream ghost, which does not exist"},
		{Reason{Kind: WorkstreamDependency, Workstream: "ghost", Missing: true, Via: "ui"}, "Its workstream ui depends on workstream ghost, which does not exist"},
		{Reason{Kind: WorkstreamOrder, Workstream: "ui", Ticket: Ref{"alpha", "feat--a"}, Column: InProgress}, "Comes after feat--a in workstream ui, which is In progress"},
		{Reason{Kind: WorkstreamOrder, Workstream: "ui", Ticket: Ref{"alpha", "feat--z"}, Missing: true}, "Comes after feat--z in workstream ui, which does not exist"},
	}
	for _, test := range tests {
		if got := test.reason.Describe("alpha"); got != test.want {
			t.Errorf("Describe(%+v) =\n  %q\nwant\n  %q", test.reason, got, test.want)
		}
	}
}
