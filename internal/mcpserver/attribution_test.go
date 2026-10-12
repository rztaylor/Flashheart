package mcpserver

import (
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/gitinfo"
	"github.com/rztaylor/flashheart/internal/runs"
)

// hookRun is a hook session's run.start in the env's worktree.
func hookRun(run, agent, branch, source, cwd string) events.Event {
	return events.Event{Time: t0, Run: run, Agent: agent, Kind: events.RunStart, Project: "demo",
		Data: events.RunStartData{Kind: events.KindSession, Cwd: cwd, Branch: branch, Worktree: cwd, Source: source}}
}

const startedRun = "claude:mcp-0123456789ab"

// Attribution precedence (agent-protocol §7.1), apart from any tool: the
// run argument, then the connection's started run, then the one hook
// session here; several, or one on another branch, leave the caller
// unresolved; a write with none of these starts a run.
func TestAttributionPrecedence(t *testing.T) {
	t.Parallel()

	const other = "claude:77777777-0000"
	type hook struct{ run, agent, branch, source string }
	here := func(run string) hook { return hook{run, "claude", "feature/demo", "startup"} }
	cases := []struct {
		name       string
		hooks      []hook
		ended      string // a hook run that has ended
		agent      string // the connection's agent; "-" for no connection
		started    string // the connection's started run
		arg        string
		write      bool
		run        string // the resolved run; "mcp" for a run started now
		fromConn   bool
		candidates int
		nearby     int
		code       string
	}{
		{name: "argument beats the started run and hooks", hooks: []hook{here(other)}, agent: "claude", started: startedRun, arg: session, run: session},
		{name: "shortened argument names its one run", hooks: []hook{here(session), here(other)}, agent: "claude", arg: "claude:5b0c7e2a", run: session},
		{name: "shortened argument naming two is ambiguous", hooks: []hook{here("claude:5b0c7e2a-aaaa"), here("claude:5b0c7e2a-bbbb")}, agent: "claude", arg: "claude:5b0c7e2a", code: "ambiguous_run"},
		{name: "shortened argument naming none is a mistake", agent: "claude", arg: "claude:deadbeef", code: "invalid_input"},
		{name: "full argument not seen yet is taken", agent: "claude", arg: "claude:0123456789abcdef-new", run: "claude:0123456789abcdef-new"},
		{name: "started run beats a hook run", hooks: []hook{here(session)}, agent: "claude", started: startedRun, run: startedRun, fromConn: true},
		{name: "one hook run here is the caller", hooks: []hook{here(session)}, agent: "claude", run: session},
		{name: "one hook run here without a connection", hooks: []hook{here(session)}, agent: "-", run: session},
		{name: "two hook runs here are candidates", hooks: []hook{here(session), here(other)}, agent: "claude", write: true, candidates: 2},
		{name: "a hook run on another branch is nearby", hooks: []hook{{session, "claude", "main", "startup"}}, agent: "claude", write: true, nearby: 1},
		{name: "another agent's hook run is not the caller", hooks: []hook{here(session)}, agent: "codex"},
		{name: "a run the server started elsewhere is not the caller", hooks: []hook{{"claude:mcp-ffffffffffff", "claude", "feature/demo", events.SourceMCP}}, agent: "claude"},
		{name: "an ended hook run is not the caller", hooks: []hook{here(session)}, ended: session, agent: "claude"},
		{name: "a read with no run starts none", agent: "claude"},
		{name: "a write with no run starts one", agent: "claude", write: true, run: "mcp", fromConn: true},
		{name: "a write without a connection starts none", agent: "-", write: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			e := newEnv(t, "DM")
			srv, _, err := newServer(Options{Root: e.root, Cwd: e.cwd, Now: func() time.Time { return e.now }})
			if err != nil {
				t.Fatal(err)
			}
			c := &call{srv: srv, now: e.now, runs: runs.DefaultSettings(), set: runs.NewSet(), folded: map[string]bool{}, loaded: map[string]bool{},
				project: "demo", where: gitinfo.Info{Repo: e.cwd, Worktree: e.cwd, Branch: "feature/demo"}}
			for _, h := range tc.hooks {
				ev := hookRun(h.run, h.agent, h.branch, h.source, e.cwd)
				c.seen = append(c.seen, ev)
				c.set.Apply(ev)
			}
			if tc.ended != "" {
				c.set.Apply(events.Event{Time: t0, Run: tc.ended, Agent: "claude", Kind: events.RunEnd, Project: "demo", Data: events.RunEndData{Reason: "exit"}})
			}
			inv := &invocation{}
			if tc.agent != "-" {
				inv.conn = &conn{agent: tc.agent, run: tc.started, project: "demo"}
			}

			err = c.attribute(inv, tc.arg, tc.write)
			if tc.code != "" {
				if te := asToolError(err); err == nil || te.Code != tc.code {
					t.Fatalf("attribute = %v, want %s", err, tc.code)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case tc.run == "mcp":
				if !strings.HasPrefix(c.run, "claude:mcp-") || inv.conn.run != c.run || len(e.log()) != 1 {
					t.Fatalf("run = %q, connection's = %q, log = %v", c.run, inv.conn.run, e.log())
				}
			case c.run != tc.run:
				t.Fatalf("run = %q, want %q", c.run, tc.run)
			case tc.run == "" && len(e.log()) != 0:
				t.Fatalf("an unresolved caller recorded %v", e.log())
			}
			if c.started != tc.fromConn || len(c.candidates) != tc.candidates || len(c.nearby) != tc.nearby {
				t.Fatalf("started %v, candidates %v, nearby %v", c.started, c.candidates, c.nearby)
			}
		})
	}
}
