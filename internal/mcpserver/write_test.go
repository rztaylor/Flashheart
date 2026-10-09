package mcpserver

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

func status(t *testing.T, file string) string {
	t.Helper()
	value, _ := mdfile.Parse([]byte(file)).String("status")
	return value
}

func TestClaimStartsTheTicketAndHoldsIt(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel", Criteria: []string{"Opens"}})
	e.startSession(session)
	out := e.ok("claim", map[string]any{"ticket": id})
	contains(t, out, `Claimed DM-1 "Card panel" (in-progress).`, "Unticked criteria (1 of 1):\n- Opens", "ok ticket=DM-1 column=in-progress")
	file := e.file(id)
	if status(t, file) != "in-progress" || !strings.Contains(file, "branch: feature/demo") {
		t.Fatalf("ticket after claim:\n%s", file)
	}
	if kinds := e.kinds(session); !slices.Equal(kinds[len(kinds)-2:], []string{events.Claim, events.TicketMoved}) {
		t.Fatalf("events = %v", kinds)
	}
	// Claiming what you hold changes nothing (MCP-5).
	e.ok("claim", map[string]any{"ticket": id})
	if got := e.kinds(session); len(got) != 5 || got[4] != events.Claim {
		t.Fatalf("second claim events = %v", got)
	}
	contains(t, e.ok("board_context", nil), "Your ticket DM-1 \"Card panel\" (in-progress, medium, linked by claim).")
}

func TestClaimRespectsOtherHoldersAndBlocking(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	first := e.ticket(store.NewTicket{Title: "First"})
	second := e.ticket(store.NewTicket{Title: "Second", DependsOn: []string{first}})
	const rival = "claude:77777777-0000"
	e.events(rival, events.Event{Kind: events.RunStart, Data: events.RunStartData{Kind: events.KindSession, Branch: "feature/other", Worktree: "/elsewhere"}},
		events.Event{Kind: events.Claim, Data: events.TicketData{Ticket: first}})
	e.startSession(session)

	out := e.fails("claim", map[string]any{"ticket": first}, "claimed")
	contains(t, out, "held by claude:77777777")
	e.fails("claim", map[string]any{"ticket": first, "force": true}, "invalid_input")
	e.ok("claim", map[string]any{"ticket": first, "force": true, "reason": "the other session crashed"})
	contains(t, e.file(first), "Claimed from claude:77777777", "the other session crashed")

	out = e.fails("claim", map[string]any{"ticket": second}, "blocked")
	contains(t, out, "Depends on DM-1")
	e.ok("claim", map[string]any{"ticket": second, "force": true, "reason": "the dependency is nearly done"})
	contains(t, e.file(second), "Started while blocked: the dependency is nearly done")
	// Claiming the second released the first.
	kinds := e.kinds(session)
	if !slices.Contains(kinds, events.Release) {
		t.Fatalf("events = %v", kinds)
	}

	// The rival's lease lapses after lease_minutes of silence.
	e.now = e.now.Add(31 * time.Minute)
	e.startSession(session)
	e.ok("claim", map[string]any{"ticket": first})

	e.fails("claim", map[string]any{"ticket": "DM-99"}, "not_found")
	write(t, filepath.Join(e.root, "demo", "tickets", "DM-7-broken", "DM-7-broken.md"), "---\nid: DM-7\nstatus: [unclosed\n---\n# Broken\n")
	e.fails("claim", map[string]any{"ticket": "DM-7"}, "needs_repair")
}

func TestReleaseEndsTheClaimOnly(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "A"})
	e.startSession(session)
	contains(t, e.ok("release", map[string]any{"ticket": id}), "nothing to release")
	e.ok("claim", map[string]any{"ticket": id})
	contains(t, e.ok("release", map[string]any{"ticket": id, "reason": "switching"}), "Released DM-1; it stays in in-progress.")
	if status(t, e.file(id)) != "in-progress" {
		t.Fatal("release moved the ticket")
	}
	contains(t, e.ok("board_context", nil), "Your ticket DM-1") // still linked by branch
}

func TestCheckpointRewritesTheHandoff(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	e.startSession(session)
	e.ok("claim", map[string]any{"ticket": id})
	e.events(session, events.Event{Kind: events.ToolUsed, Data: events.ToolData{Tool: "Edit", OK: true, Path: "src/a.ts"}})
	shot := filepath.Join(t.TempDir(), "panel.png")
	write(t, shot, "\x89PNG")
	out := e.ok("checkpoint", map[string]any{
		"ticket": id, "done": []string{"Added the failing test"}, "next": []string{"Wire the panel"},
		"files":          []string{filepath.Join(e.cwd, "src", "Panel.tsx"), shot},
		"open_questions": []string{}, "note": "Chose the slide-over layout.",
	})
	contains(t, out, "Copied into the ticket: files/20261006T1000-panel.png", "ok ticket=DM-1 checkpoint by=claude:5b0c7e2a")
	file := e.file(id)
	contains(t, file, "## Handoff\n\n_Updated 2026-10-06 10:00 UTC by claude:5b0c7e2a (run) on feature/demo._\n\n**Done**\n\n- Added the failing test\n\n**Next**\n\n- Wire the panel\n\n**Files**\n\n- src/Panel.tsx\n- files/20261006T1000-panel.png (copied from "+shot+")\n\n**Open questions**\n\n- None\n",
		"2026-10-06 · claude:5b0c7e2a: Chose the slide-over layout.")
	set := runs.NewSet()
	for _, ev := range e.log() {
		set.Apply(ev)
	}
	if set.Get(session).Dirty() {
		t.Fatal("checkpoint did not clear dirty")
	}
	// The same checkpoint again changes nothing (MCP-5).
	e.now = e.now.Add(time.Minute)
	contains(t, e.ok("checkpoint", map[string]any{"ticket": id, "done": []string{"Added the failing test"}, "next": []string{"Wire the panel"},
		"files": []string{"src/Panel.tsx", "files/20261006T1000-panel.png (copied from " + shot + ")"}}), "nothing changed")
	if !strings.Contains(e.file(id), "10:00 UTC") {
		t.Fatal("identical checkpoint rewrote the handoff")
	}
	e.fails("checkpoint", map[string]any{"ticket": id}, "invalid_input")
}

func TestCheckpointLeavesOtherHoldersAlone(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "A"})
	e.events("claude:77777777-0000", events.Event{Kind: events.RunStart, Data: events.RunStartData{Kind: events.KindSession, Worktree: "/elsewhere"}},
		events.Event{Kind: events.Claim, Data: events.TicketData{Ticket: id}})
	e.startSession(session)
	e.fails("checkpoint", map[string]any{"ticket": id, "done": []string{"x"}}, "claimed")
	// A subagent of the holder may checkpoint its session's ticket (§10).
	sub := "claude:77777777-0000/a1"
	e.events(sub, events.Event{Kind: events.RunStart, Data: events.RunStartData{Kind: events.KindSubagent, Parent: "claude:77777777-0000"}})
	e.ok("checkpoint", map[string]any{"ticket": id, "done": []string{"x"}, "run": sub})
}

func TestUpdateTicket(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "A", Criteria: []string{"Opens beside the board", "Keeps the column in view", "Closes on Escape"}})
	e.startSession(session)
	out := e.ok("update_ticket", map[string]any{"ticket": id, "set": map[string]any{"priority": "high", "tags": []string{"ui", "panel"}, "title": "Card panel"},
		"check": []string{"1", "keeps the column"}, "append_notes": "Decided on a slide-over."})
	contains(t, out, "ok ticket=DM-1 changed=priority,tags,title,criteria,notes")
	file := e.file(id)
	contains(t, file, "priority: high", "tags: [ui, panel]", "# Card panel", "- [x] Opens beside the board", "- [x] Keeps the column in view", "- [ ] Closes on Escape", "claude:5b0c7e2a: Decided on a slide-over.")
	e.fails("update_ticket", map[string]any{"ticket": id, "set": map[string]any{"status": "done"}}, "invalid_input")
	e.fails("update_ticket", map[string]any{"ticket": id, "set": map[string]any{"id": "DM-9"}}, "invalid_input")
	e.fails("update_ticket", map[string]any{"ticket": id, "set": map[string]any{"priority": "urgent"}}, "invalid_input")
	e.fails("update_ticket", map[string]any{"ticket": id, "check": []string{"4"}}, "invalid_input")
	e.fails("update_ticket", map[string]any{"ticket": id, "check": []string{"nothing like it"}}, "invalid_input")
	e.fails("update_ticket", map[string]any{"ticket": id}, "invalid_input")
}

func TestMove(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	first := e.ticket(store.NewTicket{Title: "First", Criteria: []string{"a"}})
	second := e.ticket(store.NewTicket{Title: "Second", DependsOn: []string{first}})
	e.startSession(session)
	out := e.ok("move", map[string]any{"ticket": first, "to": "review"})
	contains(t, out, "warning: There is no review file yet.", "warning: 1 acceptance criterion is not ticked.", "ok ticket=DM-1 column=review")
	if status(t, e.file(first)) != "review" {
		t.Fatal("not moved")
	}
	e.fails("move", map[string]any{"ticket": first, "to": "done"}, "invalid_input")
	e.fails("move", map[string]any{"ticket": first, "to": "todo"}, "invalid_input")
	e.ok("move", map[string]any{"ticket": first, "to": "backlog"})
	e.fails("move", map[string]any{"ticket": second, "to": "in-progress"}, "blocked")
	e.ok("move", map[string]any{"ticket": second, "to": "up-next"})

	// A review without a screenshot or a stated reason warns; one with
	// either does not (FH-29).
	e.ok("write_review", map[string]any{"ticket": second, "markdown": "# Review: Second\n\n## Summary\nDone.\n"})
	contains(t, e.ok("move", map[string]any{"ticket": second, "to": "review"}), "warning: The review shows no evidence")
	e.ok("move", map[string]any{"ticket": second, "to": "up-next"})
	e.ok("write_review", map[string]any{"ticket": second, "markdown": "# Review: Second\n\n## Evidence\nNo visible change: an internal rename.\n"})
	if out := e.ok("move", map[string]any{"ticket": second, "to": "review"}); strings.Contains(out, "no evidence") {
		t.Errorf("move = %s", out)
	}
}

// An agent's ticket writes are its run's, never the human's (FH-49).
func TestAgentWritesAreNeverTheHumans(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	e.startSession(session)
	created := e.ok("create_ticket", map[string]any{"title": "First", "type": "feature", "priority": "medium", "description": "x", "criteria": []string{"a"}})
	contains(t, created, "DM-1")
	e.ok("claim", map[string]any{"ticket": "DM-1"})
	e.ok("update_ticket", map[string]any{"ticket": "DM-1", "set": map[string]any{"priority": "high"}, "check": []string{"1"}})
	e.ok("move", map[string]any{"ticket": "DM-1", "to": "review"})
	kinds := map[string]bool{}
	for _, ev := range e.log() {
		kinds[ev.Kind] = true
		if activity, ok := events.HumanActivityOf(ev); ok || ev.Run == events.Human {
			t.Errorf("an agent's %s is the human's: %+v", ev.Kind, activity)
		}
	}
	for _, kind := range []string{events.TicketCreated, events.TicketUpdated, events.TicketMoved} {
		if !kinds[kind] {
			t.Errorf("no %s recorded", kind)
		}
	}
}

func TestProjectKeys(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")
	write(t, e.root+"/other/project.yaml", "key: DE\n")
	write(t, e.root+"/other/tickets/DE-1-x/DE-1-x.md", "---\nid: DE-1\nstatus: backlog\n---\n# X\n")
	e.startSession(session)
	out := e.fails("set_project_key", map[string]any{"key": "DE"}, "key_taken")
	contains(t, out, "keys in use: DE")
	e.fails("set_project_key", map[string]any{"key": "de"}, "invalid_input")
	e.fails("set_project_key", map[string]any{"key": "TOOLONG"}, "invalid_input")
	contains(t, e.ok("set_project_key", map[string]any{"key": "DMO"}), "ok key=DMO project=demo", "first ticket will be DMO-1")
	contains(t, e.ok("create_ticket", map[string]any{"type": "feature", "title": "First", "description": "d", "criteria": []string{"c"}, "priority": "high"}), "ok ticket=DMO-1")
	e.fails("set_project_key", map[string]any{"key": "DMP"}, "key_fixed")
	contains(t, e.ok("set_project_key", map[string]any{"key": "DMO"}), "ok key=DMO") // idempotent
}

func TestCreateTicket(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")
	e.startSession(session)
	out := e.ok("create_ticket", map[string]any{"type": "bug", "title": "Header overlaps", "description": "On narrow screens.",
		"criteria": []string{"No overlap at 1280"}, "priority": "high", "status": "up-next", "plan_or_repro": "Open at 1280; the header covers the band.",
		"project_key": "DM", "tags": []string{"ui"}})
	contains(t, out, `Created DM-1 "Header overlaps" in up-next. Project demo now uses key DM.`, "ok ticket=DM-1")
	file := e.file("DM-1")
	contains(t, file, "status: up-next", "type: bug", "session: "+session, "## Reproduction\n\nOpen at 1280; the header covers the band.", "- [ ] No overlap at 1280")
	if kinds := e.kinds(session); kinds[len(kinds)-1] != events.TicketCreated {
		t.Fatalf("events = %v", kinds)
	}
	contains(t, e.ok("create_ticket", map[string]any{"type": "feature", "title": "Second", "description": "", "criteria": []string{}, "priority": "low"}), "ok ticket=DM-2")
	e.fails("create_ticket", map[string]any{"type": "feature", "title": "x", "description": "", "criteria": []string{}, "priority": "low", "project_key": "ZZ"}, "key_fixed")
	e.fails("create_ticket", map[string]any{"type": "epic", "title": "x", "description": "", "criteria": []string{}, "priority": "low"}, "invalid_input")
	e.fails("create_ticket", map[string]any{"type": "feature", "title": "x", "description": "", "criteria": []string{}, "priority": "low", "status": "in-progress"}, "invalid_input")
	e.fails("create_ticket", map[string]any{"type": "feature", "title": "x", "description": strings.Repeat("x", 17<<10), "criteria": []string{}, "priority": "low"}, "too_large")
}

func TestCreateTicketWithoutAKeyDerivesAFreeOne(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")
	write(t, e.root+"/other/project.yaml", "key: DEM\n")
	write(t, e.root+"/other/tickets/DEM-1-x/DEM-1-x.md", "---\nid: DEM-1\nstatus: backlog\n---\n# X\n")
	e.startSession(session)
	contains(t, e.ok("create_ticket", map[string]any{"type": "feature", "title": "A", "description": "", "criteria": []string{}, "priority": "low"}), "ok ticket=DEM2-1", "now uses key DEM2")
}

func TestWriteReviewKeepsScreenshotsAfterTheOriginalsGo(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	e.startSession(session)
	shots := t.TempDir()
	shot := filepath.Join(shots, "board dark.png")
	write(t, shot, "\x89PNG dark")
	review := "# Review: Card panel\n\n## Summary\n\nDone.\n\n![Board, dark](<" + shot + ">)\n\n[log](file://" + filepath.Join(shots, "missing.log") + ")\n\n![vector](" + filepath.Join(shots, "logo.svg") + ")\n"
	write(t, filepath.Join(shots, "logo.svg"), "<svg/>")
	out := e.ok("write_review", map[string]any{"ticket": id, "markdown": review})
	contains(t, out, "Review written: tickets/DM-1-card-panel/review.md", "Copied into the ticket: files/20261006T1000-board-dark.png", "missing.log was not copied", "logo.svg was not copied", "ok ticket=DM-1 review")
	if err := os.RemoveAll(shots); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(e.root, "demo", "tickets", "DM-1-card-panel")
	written, err := os.ReadFile(filepath.Join(folder, "review.md"))
	if err != nil {
		t.Fatal(err)
	}
	contains(t, string(written), "![Board, dark](files/20261006T1000-board-dark.png)")
	if data, err := os.ReadFile(filepath.Join(folder, "files", "20261006T1000-board-dark.png")); err != nil || string(data) != "\x89PNG dark" {
		t.Fatalf("copy = %q, %v", data, err)
	}
	e.fails("write_review", map[string]any{"ticket": id, "markdown": "  "}, "invalid_input")
}

func TestAskHumanPutsTheRunInNeedsYou(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "A"})
	e.fails("ask_human", map[string]any{"kind": "question", "text": "Which schema?"}, "ambiguous_run")
	e.startSession(session)
	out := e.ok("ask_human", map[string]any{"ticket": id, "kind": "decision", "text": "Which schema?", "options": []string{"v1", "v2"}})
	contains(t, out, "the answer will arrive in a later prompt", "ok question=q-")
	question := strings.TrimSpace(out[strings.Index(out, "question=")+len("question="):])
	// The await command wakes the session when the answer arrives (§7.5).
	contains(t, out, "run_in_background", "\nflashheart await "+question+" --project demo --root "+e.root+"\n")
	set := runs.NewSet()
	for _, ev := range e.log() {
		set.Apply(ev)
	}
	if state := set.State(session, e.now, runs.DefaultSettings()); state != runs.NeedsYou {
		t.Fatalf("state = %s", state)
	}
	q := set.Get(session).Questions[0]
	if q.Ticket != id || q.Kind != "decision" || q.Text != "Which schema?" || !slices.Equal(q.Options, []string{"v1", "v2"}) {
		t.Fatalf("question = %+v", q)
	}
	e.fails("ask_human", map[string]any{"kind": "chat", "text": "hi"}, "invalid_input")
	e.fails("ask_human", map[string]any{"kind": "question", "text": " "}, "invalid_input")
	e.fails("ask_human", map[string]any{"kind": "question", "text": "x", "ticket": "DM-99"}, "not_found")
}

func TestBoardContextDeliversAnswers(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	e.startSession(session)
	out := e.ok("ask_human", map[string]any{"kind": "question", "text": "Ship it?"})
	id := strings.TrimSpace(out[strings.Index(out, "question=")+len("question="):])
	e.events(session, events.Event{Kind: events.QuestionAnswered, Data: events.AnswerData{ID: id, Answer: "Not yet", By: "Robert"}})
	if err := events.New(e.store).QueueAnswer("demo", session, events.Delivery{ID: id, Run: session, Question: "Ship it?", Answer: "Not yet", By: "Robert"}); err != nil {
		t.Fatal(err)
	}
	contains(t, e.ok("board_context", nil), `- "Ship it?" → "Not yet" (Robert)`)
	if waiting, _ := events.New(e.store).TakeAnswers("demo", session); len(waiting) != 0 {
		t.Fatalf("inbox still holds %+v", waiting)
	}
	if strings.Contains(e.ok("board_context", nil), "Ship it?") {
		t.Fatal("answer delivered twice")
	}
}
