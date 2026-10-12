package mcpserver

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/runs"
)

// This file owns run attribution, in the order of agent-protocol §7.1:
// which run a call is attributed to, and the run the server starts for a
// connection that has none. Whether a tool may act without a run is its
// policy's (policy.go).

var runPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}:[A-Za-z0-9_.-]{1,128}(/[A-Za-z0-9_.-]{1,128})?$`)

// Attribution is the optional run argument every tool accepts (MCP-4).
type Attribution struct {
	Run string `json:"run,omitempty" jsonschema:"your run id from the recovery note, if the tool asks for it"`
}

// fullSession is the length from which a session id is taken as complete
// rather than shortened (agent-protocol §2 shortens to 8 characters).
const fullSession = 16

// attribute resolves the caller's run (agent-protocol §7.1), first match
// wins:
//
//  1. the run argument, in full or shortened as recovery notes show it
//     (it must name one known run);
//  2. the run the server started for this connection, which stays its
//     caller for the connection's life and follows its branch;
//  3. the one live hook session of the caller's agent working in this
//     worktree on this branch;
//  4. several such sessions (candidates), or one of the caller's agent in
//     this worktree on another branch (nearby): no run, and the call's
//     policy decides whether it may go on;
//  5. for a write, a run started for the connection now.
//
// A run the server started is never another connection's caller.
func (c *call) attribute(inv *invocation, runArg string, write bool) error {
	if runArg != "" {
		return c.named(runArg)
	}
	cn := inv.connection()
	if run, _ := cn.current(); run != "" {
		c.run, c.started = run, true
		return c.followBranch()
	}
	c.hookRun(cn)
	if c.run != "" || len(c.candidates)+len(c.nearby) > 0 || !write {
		return nil
	}
	return c.own(cn)
}

// named resolves the run argument (step 1).
func (c *call) named(runArg string) error {
	arg := events.RunOf(runArg)
	var matches []string
	for _, r := range c.set.Runs() {
		if r.ID == runArg {
			c.run = r.ID
			return nil
		}
		id := events.RunOf(r.ID)
		if strings.HasPrefix(id.Session().String(), arg.Session().String()) && id.SubagentID() == arg.SubagentID() {
			matches = append(matches, r.ID)
		}
	}
	_, invalid := events.ParseRunID(runArg)
	switch {
	case len(matches) == 1:
		c.run = matches[0]
	case len(matches) > 1:
		return fail("ambiguous_run", "pass your full run id, one of "+strings.Join(matches, ", "), "run %q names %d runs", runArg, len(matches))
	case invalid != nil || len(arg.SessionID()) < fullSession:
		return fail("invalid_input", "pass the run id from your recovery note, or leave run out", "no run matches %q", runArg)
	default:
		// A full id not seen yet: a session whose hooks have not reported.
		c.run = runArg
	}
	return nil
}

// hookRun finds the caller among live hook sessions (steps 3 and 4).
func (c *call) hookRun(cn *conn) {
	if c.where.Worktree == "" && c.where.Repo == "" && c.srv.options.Cwd == "" {
		return
	}
	for _, r := range c.set.Runs() {
		if r.Kind != events.KindSession || r.Source == events.SourceMCP || c.set.State(r.ID, c.now, c.runs) == runs.Ended {
			continue
		}
		if cn != nil && cn.agent != "" && r.Agent != cn.agent {
			continue
		}
		switch {
		case c.here(r):
			c.candidates = append(c.candidates, r.ID)
		case c.where.Worktree != "" && r.Worktree == c.where.Worktree:
			c.nearby = append(c.nearby, r.ID)
		}
	}
	if len(c.candidates) == 1 {
		c.run, c.candidates = c.candidates[0], nil
	}
}

// here reports whether a run works where this server does.
func (c *call) here(r *runs.Run) bool {
	if c.where.Worktree != "" {
		return r.Worktree == c.where.Worktree && r.Branch == c.where.Branch
	}
	return r.Cwd != "" && r.Cwd == c.srv.options.Cwd
}

// own starts a run for a write whose connection has none and found no
// hook run (step 5): `<agent>:mcp-<random>` in the caller's project, with
// source mcp. A write with no project to record in starts none.
func (c *call) own(cn *conn) error {
	if cn == nil || c.project == "" || c.archived != "" {
		return nil
	}
	cn.mu.Lock()
	defer cn.mu.Unlock()
	if cn.run != "" {
		// Another call of this connection started it meanwhile.
		c.run, c.started = cn.run, true
		return nil
	}
	random := make([]byte, 6)
	_, _ = rand.Read(random)
	agent := cn.agent
	if agent == "" {
		agent = "agent"
	}
	run, err := events.SessionRun(agent, "mcp-"+hex.EncodeToString(random))
	if err != nil {
		return err
	}
	c.run = run.String()
	if err := c.recordStart(c.project); err != nil {
		c.run = ""
		return err
	}
	c.started = true
	cn.run, cn.project = c.run, c.project
	return nil
}

// recordStart records the started run's run.start where it works now.
func (c *call) recordStart(project string) error {
	list := []events.Event{{Kind: events.RunStart, Data: events.RunStartData{
		Kind: events.KindSession, Cwd: c.srv.options.Cwd, Branch: c.where.Branch, Worktree: c.where.Worktree, Source: events.SourceMCP,
	}}}
	if err := c.record(project, list...); err != nil {
		return err
	}
	c.seen = append(c.seen, list[0])
	c.set.Apply(list[0])
	return nil
}

// followBranch restates a started run's run.start when its agent has
// switched branches, so its link (RUN-5), handoff and Overview row follow,
// as a hook run's next prompt would.
func (c *call) followBranch() error {
	r := c.set.Get(c.run)
	if c.where.Branch == "" || r == nil || r.Branch == c.where.Branch || r.Project == "" {
		return nil
	}
	return c.recordStart(r.Project)
}

// related reports whether two runs are one session and its subagents.
func related(a, b string) bool {
	return events.RunOf(a).Session() == events.RunOf(b).Session()
}

func shortRuns(ids []string) string {
	short := make([]string, 0, len(ids))
	for _, id := range ids {
		short = append(short, protocol.ShortRun(id))
	}
	return strings.Join(short, ", ")
}
