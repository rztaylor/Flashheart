package runs

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
)

var t0 = time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)

const session = "claude:s1"

func at(minutes float64) time.Time { return t0.Add(time.Duration(minutes * float64(time.Minute))) }

func ev(minutes float64, run, kind string, data any) events.Event {
	return events.Event{Time: at(minutes), Run: run, Agent: "claude", Kind: kind, Project: "alpha", Data: data}
}

func start(minutes float64) events.Event {
	return ev(minutes, session, events.RunStart, events.RunStartData{Kind: events.KindSession, Cwd: "/src/alpha", Branch: "feature/x", Worktree: "/src/alpha", Source: "startup"})
}

func turn(minutes float64) events.Event {
	return ev(minutes, session, events.TurnStart, events.TurnStartData{})
}

func stop(minutes float64) events.Event {
	return ev(minutes, session, events.TurnEnd, events.TurnEndData{})
}

func tool(minutes float64, name, path string) events.Event {
	return ev(minutes, session, events.ToolUsed, events.ToolData{Tool: name, OK: true, Path: path})
}

func permission(minutes float64) events.Event {
	return ev(minutes, session, events.PermissionRequested, events.PermissionData{Tool: "Bash"})
}

func TestStateTable(t *testing.T) {
	t.Parallel()

	settings := DefaultSettings()
	cases := []struct {
		name   string
		events []events.Event
		now    float64
		want   State
	}{
		{"started, no prompt yet: waiting", []events.Event{start(0)}, 1, Waiting},
		{"prompt submitted: working", []events.Event{start(0), turn(1)}, 2, Working},
		{"tool use keeps it working", []events.Event{start(0), turn(1), tool(9, "Edit", "a.ts")}, 18, Working},
		{"no event for the quiet threshold: quiet", []events.Event{start(0), turn(1)}, 11, Quiet},
		{"quiet ends with new activity", []events.Event{start(0), turn(1), tool(30, "Read", "")}, 31, Working},
		{"turn finished: waiting", []events.Event{start(0), turn(1), stop(3)}, 60, Waiting},
		{"waiting never turns quiet", []events.Event{start(0), turn(1), stop(3)}, 600, Waiting},
		{"permission requested: needs you", []events.Event{start(0), turn(1), permission(2)}, 3, NeedsYou},
		{"needs you beats quiet", []events.Event{start(0), turn(1), permission(2)}, 300, NeedsYou},
		{"next tool result resolves the permission", []events.Event{start(0), turn(1), permission(2), tool(4, "Bash", "")}, 5, Working},
		{"next prompt resolves the permission", []events.Event{start(0), turn(1), permission(2), stop(3), turn(5)}, 6, Working},
		{"turn end resolves the permission", []events.Event{start(0), turn(1), permission(2), stop(3)}, 4, Waiting},
		{"denied permission resolves it", []events.Event{start(0), turn(1), permission(2), ev(3, session, events.PermissionResolved, events.ResolvedData{Outcome: "denied"})}, 4, Working},
		{"permission notification without a tool: needs you", []events.Event{start(0), turn(1), ev(2, session, events.PermissionRequested, events.PermissionData{})}, 3, NeedsYou},
		{"idle notification changes nothing", []events.Event{start(0), turn(1), stop(2), ev(3, session, events.Notification, events.NotificationData{Type: "idle"})}, 4, Waiting},
		{"session end: ended", []events.Event{start(0), turn(1), stop(2), ev(3, session, events.RunEnd, events.RunEndData{Reason: "prompt_input_exit"})}, 4, Ended},
		{"ended beats needs you", []events.Event{start(0), turn(1), permission(2), ev(3, session, events.RunEnd, events.RunEndData{})}, 4, Ended},
		{"stale twelve hours after the last event: ended", []events.Event{start(0), turn(1)}, 12*60 + 1, Ended},
		{"just under stale: quiet", []events.Event{start(0), turn(1)}, 12*60 - 1, Quiet},
		{"stale waiting run: ended", []events.Event{start(0), turn(1), stop(2)}, 12*60 + 2, Ended},
		{"resume after end reopens the run", []events.Event{start(0), ev(3, session, events.RunEnd, events.RunEndData{}), start(10)}, 11, Waiting},
		{"first seen mid-session from a prompt", []events.Event{turn(1)}, 2, Working},
		{"compaction is activity", []events.Event{start(0), turn(1), ev(15, session, events.Compact, events.CompactData{Phase: "pre"})}, 16, Working},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			set := NewSet()
			for _, e := range tc.events {
				set.Apply(e)
			}
			if got := set.State(session, at(tc.now), settings); got != tc.want {
				t.Fatalf("state at %vm = %s, want %s", tc.now, got, tc.want)
			}
		})
	}
}

func TestRunRecordsItsFacts(t *testing.T) {
	t.Parallel()

	set := NewSet()
	set.Apply(start(0))
	set.Apply(turn(1))
	set.Apply(ev(2, session, events.TurnStart, events.TurnStartData{Cwd: "/src/alpha", Branch: "feature/y", Worktree: "/src/alpha"}))
	set.Apply(tool(3, "Edit", "a.ts"))
	set.Apply(tool(4, "Write", "b.ts"))
	set.Apply(tool(5, "Read", ""))
	run := set.Get(session)
	if run == nil {
		t.Fatal("run missing")
	}
	if run.Agent != "claude" || run.Kind != events.KindSession || run.Project != "alpha" || run.Cwd != "/src/alpha" || run.Worktree != "/src/alpha" || run.Source != "startup" {
		t.Fatalf("run = %+v", run)
	}
	if run.Branch != "feature/y" {
		t.Fatalf("branch = %q; a prompt's branch replaces the start's", run.Branch)
	}
	if !run.Started.Equal(at(0)) || !run.LastActivity.Equal(at(5)) || run.Tools != 3 || run.Edits != 2 || !run.Dirty() {
		t.Fatalf("times and counts = %v %v tools %d edits %d", run.Started, run.LastActivity, run.Tools, run.Edits)
	}
	if !slices.Equal(run.Files, []string{"b.ts", "a.ts"}) {
		t.Fatalf("files = %v (most recent first)", run.Files)
	}

	// A checkpoint clears dirty; later edits set it again.
	set.Apply(ev(6, session, events.Checkpoint, events.CheckpointData{Ticket: "AL-3"}))
	if run.Dirty() || !run.LastCheckpoint.Equal(at(6)) {
		t.Fatal("checkpoint did not clear dirty")
	}
	set.Apply(tool(7, "Edit", "c.ts"))
	if !run.Dirty() || run.Edits != 1 {
		t.Fatalf("edits after checkpoint = %d", run.Edits)
	}
	// A failed edit is not an edit.
	set.Apply(ev(8, session, events.ToolUsed, events.ToolData{Tool: "Edit", OK: false, Path: "d.ts"}))
	if run.Edits != 1 {
		t.Fatalf("failed edit counted: %d", run.Edits)
	}

	set.Apply(ev(9, session, events.RunEnd, events.RunEndData{Reason: "other"}))
	if !run.EndedAt.Equal(at(9)) || run.EndReason != "other" {
		t.Fatalf("end = %v %q", run.EndedAt, run.EndReason)
	}
}

func TestPlanReplacesAndMerges(t *testing.T) {
	t.Parallel()

	set := NewSet()
	set.Apply(start(0))
	set.Apply(ev(1, session, events.PlanUpdated, events.PlanData{Items: []events.PlanItem{
		{Text: "Ticket tab", Status: events.PlanCompleted},
		{Text: "Review tab", Status: events.PlanInProgress},
		{Text: "Runs tab", Status: events.PlanPending},
	}}))
	run := set.Get(session)
	progress := run.Progress()
	if progress.Done != 1 || progress.Total != 3 || progress.Current != "Review tab" {
		t.Fatalf("progress = %+v", progress)
	}

	// Task tools merge by id; a later full list replaces everything.
	set.Apply(ev(2, session, events.PlanUpdated, events.PlanData{Items: []events.PlanItem{{Text: "Only", Status: events.PlanPending}}}))
	set.Apply(ev(3, session, events.PlanUpdated, events.PlanData{Merge: true, Items: []events.PlanItem{{ID: "1", Text: "Fix header", Status: events.PlanPending}}}))
	set.Apply(ev(4, session, events.PlanUpdated, events.PlanData{Merge: true, Items: []events.PlanItem{{ID: "2", Text: "Fix footer", Status: events.PlanPending}}}))
	set.Apply(ev(5, session, events.PlanUpdated, events.PlanData{Merge: true, Items: []events.PlanItem{{ID: "1", Status: events.PlanInProgress}}}))
	set.Apply(ev(6, session, events.PlanUpdated, events.PlanData{Merge: true, Items: []events.PlanItem{{ID: "2", Status: events.PlanDeleted}}}))
	set.Apply(ev(7, session, events.PlanUpdated, events.PlanData{Merge: true, Items: []events.PlanItem{{ID: "1", Text: "Fix header", Status: events.PlanCompleted}}}))
	want := []events.PlanItem{{Text: "Only", Status: events.PlanPending}, {ID: "1", Text: "Fix header", Status: events.PlanCompleted}}
	if !slices.Equal(run.Plan, want) {
		t.Fatalf("plan = %+v", run.Plan)
	}
	if p := run.Progress(); p.Done != 1 || p.Total != 2 || p.Current != "Only" {
		t.Fatalf("progress = %+v; with nothing in progress the next pending step is current", p)
	}

	// The plan stays bounded.
	var many []events.PlanItem
	for i := range 80 {
		many = append(many, events.PlanItem{ID: strconv.Itoa(i), Text: "x", Status: events.PlanPending})
	}
	set.Apply(ev(8, session, events.PlanUpdated, events.PlanData{Merge: true, Items: many}))
	if len(run.Plan) != events.MaxPlanItems {
		t.Fatalf("plan has %d items", len(run.Plan))
	}
}

func TestSubagentsNestAndFollowTheirParent(t *testing.T) {
	t.Parallel()

	child := session + "/a1"
	set := NewSet()
	set.Apply(start(0))
	set.Apply(turn(1))
	set.Apply(ev(2, child, events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: session, AgentType: "Explore"}))
	settings := DefaultSettings()

	run := set.Get(child)
	if run.Parent != session || run.AgentType != "Explore" || run.Kind != events.KindSubagent {
		t.Fatalf("child = %+v", run)
	}
	if !slices.Equal(set.Get(session).Children, []string{child}) {
		t.Fatalf("children = %v", set.Get(session).Children)
	}
	// A subagent works from its start without turns.
	if got := set.State(child, at(3), settings); got != Working {
		t.Fatalf("child state = %s", got)
	}
	// A subagent's tool use is activity for the parent too.
	set.Apply(ev(20, child, events.ToolUsed, events.ToolData{Tool: "Read", OK: true}))
	if got := set.State(session, at(21), settings); got != Working {
		t.Fatalf("parent state with a busy child = %s", got)
	}
	// A child waiting on permission makes its session need you.
	set.Apply(ev(22, child, events.PermissionRequested, events.PermissionData{Tool: "Bash"}))
	if got := set.State(session, at(23), settings); got != NeedsYou {
		t.Fatalf("parent state with a blocked child = %s", got)
	}
	set.Apply(ev(24, child, events.RunEnd, events.RunEndData{Reason: "completed"}))
	if got := set.State(child, at(25), settings); got != Ended {
		t.Fatalf("child after stop = %s", got)
	}
	if got := set.State(session, at(25), settings); got != Working {
		t.Fatalf("parent after child stop = %s", got)
	}
	// A child seen before its parent still nests once the parent appears,
	// and ends with its parent.
	orphan := "claude:s2/b1"
	set.Apply(ev(30, orphan, events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: "claude:s2"}))
	set.Apply(ev(31, "claude:s2", events.RunEnd, events.RunEndData{}))
	if got := set.State(orphan, at(32), settings); got != Ended {
		t.Fatalf("child of an ended session = %s", got)
	}
	if !slices.Contains(set.Get("claude:s2").Children, orphan) {
		t.Fatal("orphan did not nest")
	}
}

func TestTimelineIsBounded(t *testing.T) {
	t.Parallel()

	set := NewSet()
	set.Apply(start(0))
	for i := range MaxTimeline + 25 {
		set.Apply(tool(float64(i)/10, "Read", ""))
	}
	run := set.Get(session)
	if len(run.Timeline) != MaxTimeline || run.Tools != MaxTimeline+25 {
		t.Fatalf("timeline %d, tools %d", len(run.Timeline), run.Tools)
	}
	if run.Timeline[len(run.Timeline)-1].Kind != events.ToolUsed {
		t.Fatalf("last entry = %+v", run.Timeline[len(run.Timeline)-1])
	}
}

func TestLinks(t *testing.T) {
	t.Parallel()

	set := NewSet()
	set.Apply(start(0))
	set.Apply(ev(1, session+"/a1", events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: session}))
	inProgress := func(project, branch string) []string {
		if project == "alpha" && branch == "feature/x" {
			return []string{"AL-3"}
		}
		if branch == "shared" {
			return []string{"AL-1", "AL-2"}
		}
		return nil
	}
	if link := set.Link(session, inProgress); link != (Link{Ticket: "AL-3", By: LinkBranch}) {
		t.Fatalf("branch link = %+v", link)
	}
	if link := set.Link(session+"/a1", inProgress); link != (Link{Ticket: "AL-3", By: LinkBranch}) {
		t.Fatalf("subagent link = %+v", link)
	}
	// A claim wins over the branch; release drops it.
	set.Apply(ev(2, session, events.Claim, events.TicketData{Ticket: "AL-9"}))
	if link := set.Link(session, inProgress); link != (Link{Ticket: "AL-9", By: LinkClaim}) {
		t.Fatalf("claim link = %+v", link)
	}
	set.Apply(ev(3, session, events.Release, events.TicketData{Ticket: "AL-9"}))
	if link := set.Link(session, inProgress); link.By != LinkBranch {
		t.Fatalf("after release = %+v", link)
	}
	// Two in-progress tickets on the branch: no provisional link.
	set.Apply(ev(4, session, events.TurnStart, events.TurnStartData{Branch: "shared"}))
	if link := set.Link(session, inProgress); link != (Link{}) {
		t.Fatalf("ambiguous branch link = %+v", link)
	}
}

func TestNoHandoff(t *testing.T) {
	t.Parallel()

	settings := DefaultSettings()
	set := NewSet()
	set.Apply(start(0))
	set.Apply(tool(1, "Edit", "a.ts"))
	set.Apply(ev(2, session, events.RunEnd, events.RunEndData{}))
	linked := func(string, string) []string { return []string{"AL-3"} }
	unlinked := func(string, string) []string { return nil }
	if !set.NoHandoff(session, at(3), settings, linked) {
		t.Fatal("ended, linked and dirty run should be flagged no handoff")
	}
	if set.NoHandoff(session, at(3), settings, unlinked) {
		t.Fatal("unlinked run flagged")
	}
	set.Apply(start(4))
	if set.NoHandoff(session, at(5), settings, linked) {
		t.Fatal("live run flagged")
	}
}
