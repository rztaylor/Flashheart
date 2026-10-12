package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/protocol"
)

// runRule is when a tool needs to know its caller's run (agent-protocol
// §7.1, the policy table).
type runRule int

const (
	// runNone: the tool works with no resolvable run (reads say
	// run=unknown).
	runNone runRule = iota
	// runIfAmbiguous: the tool refuses with ambiguous_run while several
	// sessions could be the caller; with no run it writes as "unknown".
	runIfAmbiguous
	// runRequired: the tool belongs to a run and refuses without one.
	runRequired
)

// policy is what a tool declares at registration: whether it writes (so
// it may create the caller's project and start a run, §7.1 step 6), when
// it needs the caller's run, and whether the ticket it is given is guarded
// against another live session's claim (§7.2, claimed).
type policy struct {
	write  bool
	run    runRule
	guards bool
}

// Policies, named for the tools that use them.
var (
	reads = policy{}
	// belongsToRun: claim, release and ask_human are the run's own acts.
	belongsToRun = policy{write: true, run: runRequired}
	// writesTicket: a write to a ticket another session may hold.
	writesTicket = policy{write: true, run: runIfAmbiguous, guards: true}
	// writesProject: a write to the caller's project, not to a held ticket.
	writesProject = policy{write: true, run: runIfAmbiguous}
)

// attributed is a tool input: every one carries the run argument.
type attributed interface{ runArg() string }

func (a Attribution) runArg() string { return a.Run }

// tool registers a handler under its policy. Before the handler runs, the
// call reads the board, resolves the caller (attribute) and checks the
// policy's run rule, so a handler starts from a call it may act for.
// Errors become tool errors with a fix. A call of a run the server started
// is that run's activity, and its result carries the run's waiting
// answers.
func tool[In attributed](srv *server, server *mcp.Server, name, description string, p policy, handle func(*call, In) (string, error)) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description}, func(_ context.Context, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, any, error) {
		inv := &invocation{conn: srv.connection(req.Session)}
		if req.Params != nil {
			inv.session, inv.thread = codexSession(req.Params.Meta, inv.conn)
		}
		text, err := srv.enter(inv, p, input.runArg(), func(c *call) (string, error) { return handle(c, input) })
		if err != nil {
			text = asToolError(err).Error()
		}
		text += srv.called(inv, err == nil)
		return &mcp.CallToolResult{IsError: err != nil, Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}

// enter begins a call under a policy and hands it to the handler.
func (srv *server) enter(inv *invocation, p policy, runArg string, handle func(*call) (string, error)) (string, error) {
	c, err := srv.start(inv, runArg, p.write)
	if err != nil {
		return "", err
	}
	c.policy = p
	if err := c.checkRun(); err != nil {
		return "", err
	}
	return handle(c)
}

// checkRun applies the policy's run rule to the resolved caller.
func (c *call) checkRun() error {
	if c.run != "" {
		return nil
	}
	switch c.policy.run {
	case runRequired:
		return c.noRun()
	case runIfAmbiguous:
		if len(c.candidates) > 1 {
			return c.noRun()
		}
	}
	return nil
}

// noRun explains why the caller has no run, and how to name it.
func (c *call) noRun() error {
	// No run is recorded in an archived project's repository, or outside git.
	if c.archived != "" {
		return c.archivedError()
	}
	if c.outside {
		return noProjectError()
	}
	if len(c.candidates) > 1 {
		return fail("ambiguous_run", "pass run (your run id from the recovery note): one of "+shortRuns(c.candidates),
			"%d sessions are working in this worktree, so the caller is ambiguous", len(c.candidates))
	}
	if len(c.nearby) > 0 {
		return fail("ambiguous_run", "pass run (your run id from the recovery note): one of "+shortRuns(c.nearby),
			"a session works in this worktree on another branch than %s, so the caller is ambiguous", c.where.Branch)
	}
	return fail("ambiguous_run", "pass run, your run id from the recovery note ([Flashheart] run=…)",
		"no live session was found working in %s", c.srv.options.Cwd)
}

// target finds the ticket a tool acts on and, when its policy guards
// tickets, refuses one held by another live session.
func (c *call) target(id string) (*board.Project, board.Ticket, error) {
	project, ticket, err := c.find(id)
	if err != nil {
		return nil, board.Ticket{}, err
	}
	if c.policy.guards {
		if err := c.guard(ticket.ID); err != nil {
			return nil, board.Ticket{}, err
		}
	}
	return project, ticket, nil
}

// guard refuses a write to a ticket held by another live session (one not
// the caller's own session or subagent, agent-protocol §7.2).
func (c *call) guard(id string) error {
	if holder := c.holder(id); holder != nil && !related(holder.ID, c.run) {
		return fail("claimed", "work on the ticket you hold, or claim this one first", "%s is held by %s (%s)", id, protocol.ShortRun(holder.ID), c.state(holder))
	}
	return nil
}
