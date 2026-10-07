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

const fullFront = "id: FH-42\nstatus: in-progress\ntype: feature\npriority: high\ncreated: 2026-10-02\n"

func TestParseIDAndKeys(t *testing.T) {
	t.Parallel()

	if key, number, ok := ParseID("FH-42"); !ok || key != "FH" || number != 42 {
		t.Errorf("ParseID(FH-42) = %q %d %v", key, number, ok)
	}
	for _, bad := range []string{"fh-42", "F-1", "FH-0", "FH-01", "FH42", "FH-4a", "ABCDEFGHIJK-1", ""} {
		if _, _, ok := ParseID(bad); ok {
			t.Errorf("ParseID(%q) accepted", bad)
		}
	}
	for key, want := range map[string]bool{"FH": true, "OPS2": true, "A": false, "2FH": false, "fh": false, "ABCDEFGHIJK": false} {
		if ValidKey(key) != want {
			t.Errorf("ValidKey(%q) = %v", key, !want)
		}
	}
	for name, want := range map[string]string{"flashheart": "FLA", "board-ui": "BU", "my_cool app": "MCA", "a": "AX", "9lives": "P9L", "ng": "NG"} {
		if got := DeriveKey(name); got != want || !ValidKey(got) {
			t.Errorf("DeriveKey(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestParseTicketReadsFieldsAndBody(t *testing.T) {
	t.Parallel()

	data := ticketMD(fullFront+"branch: feature/x\nworkstream: board-ui\ndepends-on: [FH-12, NG-3]\ndepends-on-workstreams: ws\ntags: [ui]\nupdated: 2026-10-04T14:12:09Z\n",
		"\n# Card panel\n\n## Description\n\nFirst paragraph of the\ndescription.\n\nSecond paragraph.\n\n## Acceptance Criteria\n\n- [x] One\n- [ ] Two\n\n## Handoff\n\n_Updated by run._\n\n**Done**\n- Thing\n**Next**\n- Review tab\n- Then tests\n**Files**\n- a.go\n")
	ticket := ParseTicket("FH-42-card-panel", data)

	if ticket.ID != "FH-42" || ticket.Slug != "card-panel" || ticket.Folder != "FH-42-card-panel" || ticket.Column != InProgress {
		t.Errorf("identity = %q %q %q %q", ticket.ID, ticket.Slug, ticket.Folder, ticket.Column)
	}
	if ticket.Title != "Card panel" || ticket.Type != "feature" || ticket.Priority != "high" || ticket.Branch != "feature/x" {
		t.Errorf("ticket = %+v", ticket)
	}
	if !reflect.DeepEqual(ticket.DependsOn, []string{"FH-12", "NG-3"}) || !reflect.DeepEqual(ticket.DependsOnWorkstreams, []string{"ws"}) {
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

func TestParseTicketWarningsAndRepairs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		folder string
		front  string
		warn   string
		repair string
		id     string
		column Column
	}{
		{"missing required", "FH-42-x", "id: FH-42\nstatus: backlog\n", "missing type, priority, created", "", "FH-42", Backlog},
		{"bad priority", "FH-42-x", strings.Replace(fullFront, "high", "urgent", 1), `priority "urgent" should be high, medium or low`, "", "FH-42", InProgress},
		{"unknown type", "FH-42-x", strings.Replace(fullFront, "feature", "chore", 1), `type "chore" is not a known ticket type`, "", "FH-42", InProgress},
		{"bad date", "FH-42-x", strings.Replace(fullFront, "2026-10-02", "Oct 2", 1), `created "Oct 2" should be YYYY-MM-DD`, "", "FH-42", InProgress},
		{"missing status", "FH-42-x", strings.Replace(fullFront, "status: in-progress\n", "", 1), "missing status; shown in Backlog", "", "FH-42", Backlog},
		{"unknown status", "FH-42-x", strings.Replace(fullFront, "in-progress", "doing", 1), "", `status "doing" is not one of backlog, up-next, in-progress, review, done`, "FH-42", Backlog},
		{"id from folder", "FH-42-x", strings.Replace(fullFront, "id: FH-42\n", "", 1), "missing id; using FH-42 from the folder name", "", "FH-42", InProgress},
		{"id mismatch", "FH-43-x", fullFront, "id FH-42 does not match the folder name FH-43-x", "", "FH-42", InProgress},
		{"invalid id", "notes", strings.Replace(fullFront, "FH-42", "fh42", 1), "", `id "fh42" is not a ticket id like FH-42`, "", InProgress},
		{"no id at all", "notes", strings.Replace(fullFront, "id: FH-42\n", "", 1), "", "no ticket id: add id: <KEY>-<number> to the frontmatter", "", InProgress},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ticket := ParseTicket(test.folder, ticketMD(test.front, "# T\n"))
			if test.warn != "" && !slices.ContainsFunc(ticket.Warnings, func(w string) bool { return strings.Contains(w, test.warn) }) {
				t.Errorf("warnings = %q, want %q", ticket.Warnings, test.warn)
			}
			if test.repair == "" && ticket.NeedsRepair() {
				t.Errorf("unexpected repair %q", ticket.Repair)
			}
			if test.repair != "" && !slices.ContainsFunc(ticket.Repair, func(r string) bool { return strings.Contains(r, test.repair) }) {
				t.Errorf("repair = %q, want %q", ticket.Repair, test.repair)
			}
			if ticket.ID != test.id || ticket.Column != test.column {
				t.Errorf("id=%q column=%q, want %q %q", ticket.ID, ticket.Column, test.id, test.column)
			}
		})
	}
}

func TestBrokenFrontmatterNeedsRepairButStillShows(t *testing.T) {
	t.Parallel()

	ticket := ParseTicket("FH-9-broken", []byte("---\nid: FH-9\npriority: [unclosed\n---\n\n# Broken frontmatter\n\nBody.\n"))
	if !ticket.NeedsRepair() || !strings.Contains(ticket.Repair[0], "frontmatter does not parse") {
		t.Fatalf("Repair = %q", ticket.Repair)
	}
	if ticket.ID != "FH-9" || ticket.Column != Backlog || ticket.Title != "Broken frontmatter" || !strings.Contains(ticket.Body, "Body.") {
		t.Errorf("broken ticket = %+v", ticket)
	}
}

func TestTitleFallsBackToID(t *testing.T) {
	t.Parallel()

	if got := ParseTicket("FH-42-x", ticketMD(fullFront, "no heading\n")).Title; got != "FH-42" {
		t.Errorf("Title = %q", got)
	}
}

func TestExcerptIsBoundedPlainText(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("word ", 100)
	ticket := ParseTicket("FH-42-x", ticketMD(fullFront, "# T\n\n## Description\n\n**Bold** "+long+"\n"))
	if n := len([]rune(ticket.Excerpt)); n > ExcerptRunes+1 {
		t.Errorf("Excerpt has %d runes", n)
	}
	if !strings.HasSuffix(ticket.Excerpt, "…") || strings.Contains(ticket.Excerpt, "**") {
		t.Errorf("Excerpt = %q", ticket.Excerpt)
	}
}

func TestParseWorkstream(t *testing.T) {
	t.Parallel()

	ws := ParseWorkstream("board-ui", []byte("---\nslug: board-ui\nstatus: active\npriority: high\ncreated: 2026-10-01\ntickets:\n  - FH-1\n  - FH-2\ndepends-on-workstreams: [base]\ntags: [ui]\n---\n\n# Board UI\n\n## Goal\n\nRender.\n"))
	if ws.Title != "Board UI" || ws.Status != "active" || !reflect.DeepEqual(ws.Tickets, []string{"FH-1", "FH-2"}) || !reflect.DeepEqual(ws.DependsOnWorkstreams, []string{"base"}) {
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

func TestParseWorkstreamOrdered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		front   string
		ordered bool
		warning string
	}{
		{"", false, ""},
		{"ordered: false\n", false, ""},
		{"ordered: true\n", true, ""},
		{"ordered: True\n", true, ""},
		{"ordered:\n", false, ""},
		{"ordered: sometimes\n", false, `ordered "sometimes" is not true or false`},
	}
	for _, test := range tests {
		got := ParseWorkstream("w", []byte("---\n"+test.front+"---\n# W\n"))
		if got.Ordered != test.ordered {
			t.Errorf("%q: Ordered = %v, want %v", test.front, got.Ordered, test.ordered)
		}
		if test.warning == "" && len(got.Warnings) > 0 {
			t.Errorf("%q: unexpected warnings %q", test.front, got.Warnings)
		}
		if test.warning != "" && !slices.ContainsFunc(got.Warnings, func(w string) bool { return strings.Contains(w, test.warning) }) {
			t.Errorf("%q: warnings = %q, want %q", test.front, got.Warnings, test.warning)
		}
	}
}

func TestCheckProjectMarksDuplicatesAndForeignKeys(t *testing.T) {
	t.Parallel()

	project := Project{Name: "p", Key: "FH", Tickets: []Ticket{
		ParseTicket("FH-1-a", ticketMD("id: FH-1\nstatus: backlog\n", "# A\n")),
		ParseTicket("FH-1-a-copy", ticketMD("id: FH-1\nstatus: done\n", "# A again\n")),
		ParseTicket("FH-2-b", ticketMD("id: FH-2\nstatus: backlog\n", "# B\n")),
		ParseTicket("NG-3-c", ticketMD("id: NG-3\nstatus: backlog\n", "# C\n")),
	}}
	CheckProject(&project)
	if !project.Tickets[0].NeedsRepair() || !project.Tickets[1].NeedsRepair() || project.Tickets[2].NeedsRepair() {
		t.Fatalf("repairs = %q / %q / %q", project.Tickets[0].Repair, project.Tickets[1].Repair, project.Tickets[2].Repair)
	}
	if !strings.Contains(project.Tickets[0].Repair[0], "FH-1-a-copy") {
		t.Errorf("duplicate repair does not name the other folder: %q", project.Tickets[0].Repair)
	}
	if !slices.ContainsFunc(project.Tickets[3].Warnings, func(w string) bool { return strings.Contains(w, "uses key NG but this project's key is FH") }) {
		t.Errorf("foreign key warnings = %q", project.Tickets[3].Warnings)
	}
}

func TestCheckKeysWarnsOnDuplicateKeys(t *testing.T) {
	t.Parallel()

	b := Board{Projects: []Project{{Name: "a", Key: "FH"}, {Name: "b", Key: "FH"}, {Name: "c", Key: "NG"}}}
	CheckKeys(&b)
	for _, index := range []int{0, 1} {
		if len(b.Projects[index].Warnings) != 1 || !strings.Contains(b.Projects[index].Warnings[0], "key FH is also used by") {
			t.Errorf("project %s warnings = %q", b.Projects[index].Name, b.Projects[index].Warnings)
		}
	}
	if len(b.Projects[2].Warnings) != 0 {
		t.Errorf("project c warnings = %q", b.Projects[2].Warnings)
	}
}

// tk builds a ticket for blocking tables.
func tk(id string, column Column, extra string) Ticket {
	return ParseTicket(id+"-x", ticketMD("id: "+id+"\nstatus: "+string(column)+"\ntype: feature\npriority: high\ncreated: 2026-10-02\n"+extra, "# "+id+"\n"))
}

// ws builds an unordered workstream (the default) for blocking tables.
func ws(slug string, tickets []string, deps ...string) Workstream {
	return workstreamOf("", slug, tickets, deps)
}

// chain builds an ordered workstream (ordered: true).
func chain(slug string, tickets []string, deps ...string) Workstream {
	return workstreamOf("ordered: true\n", slug, tickets, deps)
}

func workstreamOf(extra, slug string, tickets, deps []string) Workstream {
	front := extra + "tickets: [" + strings.Join(tickets, ", ") + "]\n"
	if len(deps) > 0 {
		front += "depends-on-workstreams: [" + strings.Join(deps, ", ") + "]\n"
	}
	return ParseWorkstream(slug, []byte("---\n"+front+"---\n# "+slug+"\n"))
}

func reasonsOf(analysis Analysis, project, id string) []Reason {
	return analysis.Blocked[Ref{Project: project, ID: id}]
}

func TestBlockingRules(t *testing.T) {
	t.Parallel()

	alpha := Project{
		Name: "alpha", Key: "AL",
		Tickets: []Ticket{
			tk("AL-1", Done, ""),
			tk("AL-2", Review, ""),
			tk("AL-3", InProgress, ""),
			tk("AL-4", Backlog, "depends-on: [AL-1, AL-2, AL-99]\n"),
			tk("AL-5", UpNext, "depends-on: [AL-3]\n"),
			tk("AL-6", Backlog, "depends-on: [AL-404]\n"),
			tk("AL-7", Backlog, "depends-on: [BE-1, BE-2, GA-1]\n"),
			tk("AL-8", Done, "workstream: one\n"),
			tk("AL-9", InProgress, "workstream: one\n"),
			tk("AL-10", UpNext, "workstream: one\n"),
			tk("AL-11", Backlog, "depends-on-workstreams: [one, ghost, two]\n"),
			tk("AL-12", Backlog, "workstream: three\n"),
			tk("AL-13", Backlog, "workstream: nowhere\n"),
			tk("AL-14", Done, "depends-on: [AL-3]\n"),
			tk("AL-15", InProgress, "workstream: epic\n"),
			tk("AL-16", Backlog, "workstream: epic\n"),
			tk("AL-17", UpNext, "workstream: epic\ndepends-on: [AL-15]\n"),
			tk("AL-18", Backlog, "workstream: later\n"),
		},
		Workstreams: []Workstream{
			chain("one", []string{"AL-8", "AL-9", "AL-10"}),
			ws("epic", []string{"AL-15", "AL-16", "AL-17"}),
			ws("later", []string{"AL-18"}, "epic"),
			ws("two", []string{"AL-1", "AL-2"}),
			ws("three", []string{"AL-12"}, "one"),
		},
		Archived: []string{"AL-99"},
	}
	beta := Project{Name: "beta", Key: "BE", Tickets: []Ticket{tk("BE-1", InProgress, ""), tk("BE-2", Done, "")}}
	analysis := Analyze(Board{Projects: []Project{alpha, beta}})

	// An unordered workstream's later tickets do not wait for earlier ones.
	for _, free := range []string{"AL-4", "AL-9", "AL-1", "AL-14", "AL-15", "AL-16"} {
		if reasons := reasonsOf(analysis, "alpha", free); len(reasons) != 0 {
			t.Errorf("%s reasons = %+v, want none", free, reasons)
		}
	}
	tests := []struct {
		id   string
		want []Reason
	}{
		{"AL-5", []Reason{{Kind: TicketDependency, Ticket: Ref{"alpha", "AL-3"}, Column: InProgress}}},
		{"AL-6", []Reason{{Kind: TicketDependency, Ticket: Ref{"", "AL-404"}, Missing: true}}},
		{"AL-7", []Reason{
			{Kind: TicketDependency, Ticket: Ref{"beta", "BE-1"}, Column: InProgress},
			{Kind: TicketDependency, Ticket: Ref{"", "GA-1"}, Missing: true},
		}},
		{"AL-10", []Reason{{Kind: WorkstreamOrder, Workstream: "one", Ticket: Ref{"alpha", "AL-9"}, Column: InProgress}}},
		{"AL-11", []Reason{
			{Kind: WorkstreamDependency, Workstream: "one", Pending: 2},
			{Kind: WorkstreamDependency, Workstream: "ghost", Missing: true},
		}},
		{"AL-12", []Reason{{Kind: WorkstreamDependency, Workstream: "one", Pending: 2, Via: "three"}}},
		{"AL-13", []Reason{{Kind: WorkstreamDependency, Workstream: "nowhere", Missing: true, Via: "nowhere"}}},
		{"AL-17", []Reason{{Kind: TicketDependency, Ticket: Ref{"alpha", "AL-15"}, Column: InProgress}}},
		{"AL-18", []Reason{{Kind: WorkstreamDependency, Workstream: "epic", Pending: 3, Via: "later"}}},
	}
	for _, test := range tests {
		if got := reasonsOf(analysis, "alpha", test.id); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s reasons =\n  %+v\nwant\n  %+v", test.id, got, test.want)
		}
	}
}

func TestDuplicatedDependencyIsSatisfiedOnlyWhenEveryCopyIs(t *testing.T) {
	t.Parallel()

	project := Project{Name: "p", Key: "PP", Tickets: []Ticket{
		tk("PP-1", Done, ""),
		tk("PP-1", Backlog, ""),
		tk("PP-2", Backlog, "depends-on: [PP-1]\n"),
	}}
	CheckProject(&project)
	analysis := Analyze(Board{Projects: []Project{project}})
	if reasons := reasonsOf(analysis, "p", "PP-2"); len(reasons) != 1 || reasons[0].Column != Backlog {
		t.Errorf("reasons = %+v", reasons)
	}
}

func TestWorkstreamStatus(t *testing.T) {
	t.Parallel()

	project := Project{
		Name: "p", Key: "PP",
		Tickets: []Ticket{
			tk("PP-1", Done, ""), tk("PP-2", Review, ""),
			tk("PP-3", InProgress, ""), tk("PP-4", UpNext, "depends-on: [PP-3]\n"),
			tk("PP-5", Backlog, ""),
			tk("PP-6", Backlog, "depends-on: [PP-3]\n"),
			tk("PP-7", Backlog, ""),
		},
		Workstreams: []Workstream{
			ws("complete", []string{"PP-1", "PP-2"}),
			ws("moving", []string{"PP-1", "PP-3", "PP-5"}),
			chain("stuck", []string{"PP-4", "PP-5"}),
			ws("epic", []string{"PP-4", "PP-7"}),
			ws("epic-stuck", []string{"PP-2", "PP-4", "PP-6"}),
			ws("waiting", []string{"PP-5"}, "moving"),
			ws("empty", nil),
			ws("broken-ref", []string{"PP-404"}),
		},
	}
	analysis := Analyze(Board{Projects: []Project{project}})
	tests := map[string]WorkstreamState{
		"complete": {Status: StatusCompleted, Done: 2, Total: 2},
		"moving":   {Status: StatusActive, Done: 1, Total: 3, Next: "PP-3"},
		"stuck":    {Status: StatusBlocked, Done: 0, Total: 2, Next: "PP-4", Reasons: []Reason{{Kind: TicketDependency, Ticket: Ref{"p", "PP-3"}, Column: InProgress}}},
		// Unordered: next is the first unfinished member that is not blocked,
		// and the workstream is blocked only when every unfinished one is.
		"epic":       {Status: StatusActive, Done: 0, Total: 2, Next: "PP-7"},
		"epic-stuck": {Status: StatusBlocked, Done: 1, Total: 3, Next: "PP-4", Reasons: []Reason{{Kind: TicketDependency, Ticket: Ref{"p", "PP-3"}, Column: InProgress}}},
		"waiting":    {Status: StatusBlocked, Done: 0, Total: 1, Next: "PP-5", Reasons: []Reason{{Kind: WorkstreamDependency, Workstream: "moving", Pending: 2}}},
		"empty":      {Status: StatusActive},
		"broken-ref": {Status: StatusBlocked, Done: 0, Total: 1, Next: "PP-404", Reasons: []Reason{{Kind: TicketDependency, Ticket: Ref{"", "PP-404"}, Missing: true}}},
	}
	for slug, want := range tests {
		if got := analysis.Workstreams[Ref{Project: "p", ID: slug}]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %+v, want %+v", slug, got, want)
		}
	}
}

func TestMembershipMismatchWarns(t *testing.T) {
	t.Parallel()

	project := Project{
		Name: "p", Key: "PP",
		Tickets:     []Ticket{tk("PP-1", Backlog, "workstream: one\n"), tk("PP-2", Backlog, "")},
		Workstreams: []Workstream{ws("one", []string{"PP-2"})},
	}
	analysis := Analyze(Board{Projects: []Project{project}})
	if got := analysis.Warnings[Ref{"p", "PP-1"}]; len(got) != 1 || !strings.Contains(got[0], "not listed in workstream one") {
		t.Errorf("claims warnings = %q", got)
	}
	if got := analysis.Warnings[Ref{"p", "PP-2"}]; len(got) != 1 || !strings.Contains(got[0], `workstream one lists this ticket`) {
		t.Errorf("listed warnings = %q", got)
	}
}

func TestReasonDescriptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reason Reason
		want   string
	}{
		{Reason{Kind: TicketDependency, Ticket: Ref{"alpha", "AL-1"}, Column: Backlog}, "Depends on AL-1, which is in Backlog"},
		{Reason{Kind: TicketDependency, Ticket: Ref{"beta", "BE-2"}, Column: UpNext}, "Depends on BE-2, which is Up next"},
		{Reason{Kind: TicketDependency, Ticket: Ref{"beta", "BE-3"}, Column: InProgress}, "Depends on BE-3, which is In progress"},
		{Reason{Kind: TicketDependency, Ticket: Ref{"", "AL-9"}, Missing: true}, "Depends on AL-9, which does not exist"},
		{Reason{Kind: WorkstreamDependency, Workstream: "core", Pending: 1}, "Depends on workstream core, which has 1 ticket not yet in review or done"},
		{Reason{Kind: WorkstreamDependency, Workstream: "core", Pending: 3, Via: "ui"}, "Its workstream ui depends on workstream core, which has 3 tickets not yet in review or done"},
		{Reason{Kind: WorkstreamDependency, Workstream: "ghost", Missing: true}, "Depends on workstream ghost, which does not exist"},
		{Reason{Kind: WorkstreamDependency, Workstream: "ghost", Missing: true, Via: "ghost"}, "Belongs to workstream ghost, which does not exist"},
		{Reason{Kind: WorkstreamOrder, Workstream: "ui", Ticket: Ref{"alpha", "AL-1"}, Column: InProgress}, "Comes after AL-1 in workstream ui, which is In progress"},
	}
	for _, test := range tests {
		if got := test.reason.Describe(); got != test.want {
			t.Errorf("Describe(%+v) =\n  %q\nwant\n  %q", test.reason, got, test.want)
		}
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

func TestSampleFixtureTickets(t *testing.T) {
	t.Parallel()

	dir := filepath.Join("..", "..", "testdata", "boards", "sample", "alpha", "tickets")
	read := func(folder string) Ticket {
		data, err := os.ReadFile(filepath.Join(dir, folder, folder+".md"))
		if err != nil {
			t.Fatal(err)
		}
		return ParseTicket(folder, data)
	}
	broken := read("AL-7-broken-frontmatter")
	if !broken.NeedsRepair() || broken.Title != "Broken frontmatter" || broken.ID != "AL-7" {
		t.Errorf("broken fixture = %+v", broken)
	}
	panel := read("AL-3-card-panel")
	if panel.Handoff == nil || !reflect.DeepEqual(panel.Handoff.Next, []string{"Review tab"}) || panel.Workstream != "board-ui" || panel.Column != InProgress {
		t.Errorf("card panel fixture = %+v", panel)
	}
}

func TestInProgressOnBranch(t *testing.T) {
	t.Parallel()

	project := Project{Tickets: []Ticket{
		{ID: "AL-1", Column: InProgress, Branch: "feature/x"},
		{ID: "AL-2", Column: Backlog, Branch: "feature/x"},
		{ID: "AL-3", Column: InProgress, Branch: "main"},
		{ID: "AL-4", Column: InProgress, Branch: "feature/x", Repair: []string{"bad frontmatter"}},
		{ID: "AL-5", Column: InProgress},
	}}
	if got := project.InProgressOnBranch("feature/x"); len(got) != 1 || got[0] != "AL-1" {
		t.Fatalf("InProgressOnBranch(feature/x) = %v", got)
	}
	if got := project.InProgressOnBranch(""); got != nil {
		t.Fatalf("InProgressOnBranch(\"\") = %v", got)
	}
}
