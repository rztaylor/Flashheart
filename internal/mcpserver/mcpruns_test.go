package mcpserver

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// codexClient is the name Codex gives its MCP client at initialize.
const codexClient = "codex-mcp-client"

// started lists the runs the server started by their first run.start
// with source mcp.
func (e *env) started() []events.Event {
	e.t.Helper()
	var list []events.Event
	for _, ev := range e.log() {
		var data events.RunStartData
		if ev.Kind == events.RunStart && ev.Decode(&data) == nil && data.Source == events.SourceMCP &&
			!slices.ContainsFunc(list, func(seen events.Event) bool { return seen.Run == ev.Run }) {
			list = append(list, ev)
		}
	}
	return list
}

func (e *env) fold() *runs.Set {
	set := runs.NewSet()
	for _, ev := range e.log() {
		set.Apply(ev)
	}
	return set
}

func (e *env) okOn(client *mcp.ClientSession, tool string, args map[string]any) string {
	e.t.Helper()
	text, failed := e.callOn(client, tool, args)
	if failed {
		e.t.Fatalf("%s(%v) failed:\n%s", tool, args, text)
	}
	return text
}

// An agent without hooks gets a run from its first write: claim, release
// and ask_human work, and its writes name the run (agent-protocol §7.1).
func TestAWriteWithoutAHookRunStartsOne(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	codex := e.dial(codexClient)

	// Reading starts nothing.
	contains(t, e.okOn(codex, "board_context", nil), "run=unknown")
	if len(e.log()) != 0 {
		t.Fatalf("a read recorded %v", e.log())
	}

	contains(t, e.okOn(codex, "claim", map[string]any{"ticket": id}), "ok ticket=DM-1 column=in-progress")
	list := e.started()
	if len(list) != 1 {
		t.Fatalf("started runs = %v", list)
	}
	start := list[0]
	var data events.RunStartData
	if err := start.Decode(&data); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(start.Run, "codex:mcp-") || start.Agent != "codex" || start.Project != "demo" ||
		data.Kind != events.KindSession || data.Cwd != e.cwd || data.Branch != "feature/demo" || data.Worktree != e.cwd {
		t.Fatalf("run.start = %+v %+v", start, data)
	}
	run := start.Run
	if kinds := e.kinds(run); !slices.Equal(kinds, []events.Kind{events.RunStart, events.Claim, events.TicketMoved, events.Activity}) {
		t.Fatalf("events = %v", kinds)
	}
	short := run[:len("codex:mcp-0123")]
	contains(t, e.okOn(codex, "board_context", nil), "run="+short, "Your ticket DM-1")
	contains(t, e.okOn(codex, "checkpoint", map[string]any{"ticket": id, "done": []string{"Panel"}, "next": []string{"Tests"}}), "by="+short)
	contains(t, e.file(id), "by "+short+" (run)")
	contains(t, e.okOn(codex, "release", map[string]any{"ticket": id}), "Released DM-1")
	asked := e.okOn(codex, "ask_human", map[string]any{"ticket": id, "kind": "question", "text": "Which schema?"})
	contains(t, asked, "the answer will arrive with the result of your next flashheart tool call", "ok question=q-")
	// No prompt hook sees a reply in the chat, so only the board settles it.
	contains(t, asked, "answer it on the board")
	if strings.Contains(asked, "later prompt") || strings.Contains(asked, "settles it") {
		t.Fatalf("a run without hooks was told a prompt or a chat reply settles it:\n%s", asked)
	}
	if state := e.fold().State(run, e.now, runs.DefaultSettings()); state != runs.NeedsYou {
		t.Fatalf("state = %s", state)
	}
	if len(e.started()) != 1 {
		t.Fatalf("one connection started %d runs", len(e.started()))
	}
	for _, ev := range e.log() {
		if ev.Run != run {
			t.Fatalf("event not attributed to the started run: %+v", ev)
		}
	}
}

// A run found from hooks always wins: no run is started for its agent.
// Another agent's hook session in the same worktree is not the caller.
func TestAHookRunWinsOverStartingOne(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	first := e.ticket(store.NewTicket{Title: "First"})
	second := e.ticket(store.NewTicket{Title: "Second"})
	e.startSession(session)
	e.ok("claim", map[string]any{"ticket": first})
	if len(e.started()) != 0 {
		t.Fatalf("started %v beside a hook run", e.started())
	}
	codex := e.dial(codexClient)
	e.okOn(codex, "claim", map[string]any{"ticket": second})
	list := e.started()
	if len(list) != 1 || list[0].Agent != "codex" {
		t.Fatalf("started = %v", list)
	}
	if r := e.fold().Get(session); r.Claim != first {
		t.Fatalf("the hook run's claim = %q", r.Claim)
	}
	// The stamped run argument wins too, and is not the started run's
	// activity.
	e.okOn(codex, "update_ticket", map[string]any{"ticket": first, "run": session, "append_notes": "Checked."})
	if len(e.started()) != 1 {
		t.Fatal("a call naming its run started another")
	}
	contains(t, e.file(first), "claude:5b0c7e2a: Checked.")
	last := e.log()[len(e.log())-1]
	if last.Run != session || last.Kind != events.TicketUpdated {
		t.Fatalf("last event = %+v", last)
	}
}

// codexSession records a live Codex session from hooks in the checkout,
// on branch.
func (e *env) codexSession(run, branch string) {
	e.t.Helper()
	data := events.RunStartData{Kind: events.KindSession, Cwd: e.cwd, Branch: branch, Worktree: e.cwd, Source: "startup"}
	if err := events.New(e.store).Append(events.Event{Time: e.now, Run: run, Agent: "codex", Kind: events.RunStart, Project: "demo", Data: data}); err != nil {
		e.t.Fatal(err)
	}
}

// A connection keeps the run it started: a hook session that shows up in
// the worktree later is another session (one server serves one session).
func TestAConnectionKeepsItsRunWhenAHookSessionAppears(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	codex := e.dial(codexClient)
	e.okOn(codex, "claim", map[string]any{"ticket": id})
	run := e.started()[0].Run
	e.codexSession("codex:019a0000-hooks", "feature/demo")
	contains(t, e.okOn(codex, "checkpoint", map[string]any{"ticket": id, "done": []string{"Panel"}, "next": []string{"Tests"}}), "by="+run[:len("codex:mcp-0123")])
	if r := e.fold().Get(run); r.Claim != id {
		t.Fatalf("the started run's claim = %q", r.Claim)
	}
}

// A live hook session of the caller's agent in this worktree, on another
// branch than the server reads (the agent switched branches mid-turn), is
// the caller's run, not a reason to start one: the write asks for run.
func TestNoRunStartsBesideAHookSessionOnAnotherBranch(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	e.codexSession("codex:019a0000-hooks", "main")
	codex := e.dial(codexClient)
	out, failed := e.callOn(codex, "claim", map[string]any{"ticket": id})
	if !failed || !strings.HasPrefix(out, "error ambiguous_run:") || !strings.Contains(out, "codex:019a0000") {
		t.Fatalf("claim = %s", out)
	}
	if len(e.started()) != 0 {
		t.Fatalf("started %v beside a hook session", e.started())
	}
	contains(t, e.okOn(codex, "claim", map[string]any{"ticket": id, "run": "codex:019a0000"}), "ok ticket=DM-1")
}

// A started run follows its agent to another branch, as a hook run does at
// its next prompt.
func TestAStartedRunFollowsABranchSwitch(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	codex := e.dial(codexClient)
	e.okOn(codex, "ask_human", map[string]any{"kind": "question", "text": "Which branch?"})
	run := e.started()[0].Run
	write(t, filepath.Join(e.cwd, ".git", "HEAD"), "ref: refs/heads/feature/next\n")
	e.now = e.now.Add(time.Minute)
	e.okOn(codex, "claim", map[string]any{"ticket": id})
	e.okOn(codex, "checkpoint", map[string]any{"ticket": id, "done": []string{"Panel"}, "next": []string{"Tests"}})
	contains(t, e.file(id), "branch: feature/next", "(run) on feature/next._")
	r := e.fold().Get(run)
	if r.Branch != "feature/next" || len(e.started()) != 1 {
		t.Fatalf("run branch = %q, started %d runs", r.Branch, len(e.started()))
	}
	// The timeline shows one connection and the switch (FH-71).
	var starts []string
	for _, entry := range r.Timeline {
		if entry.Kind == events.RunStart {
			starts = append(starts, entry.Detail+" "+entry.Branch)
		}
	}
	if !slices.Equal(starts, []string{"mcp ", "mcp feature/next"}) {
		t.Fatalf("run.start entries = %q", starts)
	}
}

// Two connections in one worktree are two sessions, so two runs, and
// neither makes the other ambiguous.
func TestTwoConnectionsGetTwoRuns(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	first := e.ticket(store.NewTicket{Title: "First"})
	second := e.ticket(store.NewTicket{Title: "Second"})
	one, two := e.dial(codexClient), e.dial(codexClient)
	e.okOn(one, "claim", map[string]any{"ticket": first})
	e.okOn(two, "claim", map[string]any{"ticket": second})
	e.okOn(one, "ask_human", map[string]any{"kind": "question", "text": "One?"})
	list := e.started()
	if len(list) != 2 || list[0].Run == list[1].Run {
		t.Fatalf("started = %v", list)
	}
	set := e.fold()
	if set.Get(list[0].Run).Claim != first || set.Get(list[1].Run).Claim != second {
		t.Fatalf("claims = %q, %q", set.Get(list[0].Run).Claim, set.Get(list[1].Run).Claim)
	}
	// Each holds its own ticket against the other.
	out, failed := e.callOn(two, "claim", map[string]any{"ticket": first})
	if !failed || !strings.Contains(out, "error claimed:") {
		t.Fatalf("claim of the other connection's ticket:\n%s", out)
	}
	if len(set.Get(list[0].Run).Questions) != 1 || len(set.Get(list[1].Run).Questions) != 0 {
		t.Fatal("the question went to the wrong run")
	}
}

// A client that names another agent never takes a hook run of Claude
// Code's as its own.
func TestAnotherAgentStartsItsOwnRun(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	e.startSession(session)
	zed := e.dial("Zed")
	contains(t, e.okOn(zed, "claim", map[string]any{"ticket": id}), "ok ticket=DM-1")
	if list := e.started(); len(list) != 1 || !strings.HasPrefix(list[0].Run, "zed:mcp-") || list[0].Agent != "zed" {
		t.Fatalf("started = %v", list)
	}
}
