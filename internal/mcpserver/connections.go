package mcpserver

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/protocol"
)

// conn is one client connection, which is one agent session: the agent its
// client named at initialize and, once a write found no run, the run the
// server started for it (agent-protocol §7.1).
type conn struct {
	session *mcp.ServerSession
	// agent is the agent id the client's name gives, "" when it gave none.
	agent string

	mu sync.Mutex
	// run is the started run, "" until a write needs one; project is where
	// its events are recorded.
	run, project string
}

// invocation is one tool call: its connection and tool, and the run the
// call was attributed to once its call began.
type invocation struct {
	conn *conn
	tool string
	run  string
}

// connection returns the state of a client connection, created at its
// first tool call.
func (srv *server) connection(session *mcp.ServerSession) *conn {
	if session == nil {
		return nil
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if cn, ok := srv.conns[session]; ok {
		return cn
	}
	cn := &conn{session: session}
	if params := session.InitializeParams(); params != nil && params.ClientInfo != nil {
		cn.agent = agentName(params.ClientInfo.Name)
	}
	srv.conns[session] = cn
	return cn
}

// agentName maps a client's name to an agent id (agent-protocol §2):
// Claude Code calls itself claude-code and Codex codex-mcp-client; another
// client keeps its own name, made fit for a run id.
func agentName(client string) string {
	name := strings.ToLower(strings.TrimSpace(client))
	switch {
	case name == "":
		return ""
	case strings.Contains(name, "claude"):
		return "claude"
	case strings.Contains(name, "codex"):
		return "codex"
	}
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	agent := strings.TrimRight(b.String()[:min(b.Len(), 32)], "-")
	if agent == "" || agent[0] < 'a' || agent[0] > 'z' {
		return "agent"
	}
	return agent
}

// current is the connection's started run, "" when there is none.
func (cn *conn) current() (run, project string) {
	if cn == nil {
		return "", ""
	}
	cn.mu.Lock()
	defer cn.mu.Unlock()
	return cn.run, cn.project
}

// own attributes a call no other run claims to its connection's run, and
// for a write that finds none, starts one: `<agent>:mcp-<random>` in the
// caller's project, with source mcp. A run found from hooks, or named by
// the run argument, always wins (agent-protocol §7.1).
func (c *call) own(inv *invocation, write bool) error {
	if inv == nil || inv.conn == nil || c.run != "" || len(c.candidates) > 0 {
		return nil
	}
	cn := inv.conn
	cn.mu.Lock()
	defer cn.mu.Unlock()
	if cn.run != "" {
		c.run = cn.run
		return nil
	}
	if !write || c.project == "" || c.archived != "" {
		return nil
	}
	random := make([]byte, 6)
	_, _ = rand.Read(random)
	agent := cn.agent
	if agent == "" {
		agent = "agent"
	}
	c.run = agent + ":mcp-" + hex.EncodeToString(random)
	started := events.Event{Kind: events.RunStart, Data: events.RunStartData{
		Kind: events.KindSession, Cwd: c.srv.options.Cwd, Branch: c.where.Branch, Worktree: c.where.Worktree, Source: events.SourceMCP,
	}}
	list := []events.Event{started}
	if err := c.record(c.project, list...); err != nil {
		c.run = ""
		return err
	}
	c.seen = append(c.seen, list[0])
	c.set.Apply(list[0])
	cn.run, cn.project = c.run, c.project
	// Closing the connection ends the session, so it ends the run; a
	// server that dies leaves it to the stale rule.
	c.srv.ending.Add(1)
	go func() {
		defer c.srv.ending.Done()
		_ = cn.session.Wait()
		c.srv.disconnected(cn)
	}()
	return nil
}

// disconnected ends a closed connection's run (run.end, reason
// disconnected) and forgets the connection.
func (srv *server) disconnected(cn *conn) {
	srv.mu.Lock()
	delete(srv.conns, cn.session)
	srv.mu.Unlock()
	run, project := cn.current()
	if run == "" {
		return
	}
	// Failing open: the stale rule ends a run whose end was not recorded.
	_ = srv.log.Append(events.Event{Time: srv.options.Now().UTC(), Run: run, Agent: agentOf(run), Kind: events.RunEnd, Project: project,
		Data: events.RunEndData{Reason: events.RunEndDisconnected}})
}

// called records a tool call of the connection's started run as its
// activity, which renews its claim's lease (agent-protocol §6), and returns
// the answers waiting for it, marked delivered: with no prompt hook, the
// next tool result carries them (HOOK-5). A call attributed to another run
// leaves the started run alone.
func (srv *server) called(inv *invocation, ok bool) string {
	if inv.conn == nil {
		return ""
	}
	run, project := inv.conn.current()
	if run == "" || inv.run != "" && inv.run != run {
		return ""
	}
	now := srv.options.Now().UTC()
	// Activity is best effort: the call itself has already succeeded or
	// failed on its own terms.
	_ = srv.log.Append(events.Event{Time: now, Run: run, Agent: agentOf(run), Kind: events.ToolUsed, Project: project,
		Data: events.ToolData{Tool: inv.tool, OK: ok}})
	waiting, err := srv.log.TakeAnswers(project, run)
	if err != nil || len(waiting) == 0 {
		return ""
	}
	answers := make([]protocol.Answer, 0, len(waiting))
	for _, d := range waiting {
		answers = append(answers, protocol.Answer{Question: d.Question, Answer: d.Answer, By: d.By, Ticket: d.Ticket})
	}
	// Taken from the inbox, they are shown even if marking them fails;
	// board_context would then show them again.
	_ = srv.log.MarkDelivered(project, waiting, now)
	return "\n" + protocol.AnswersNote(answers)
}
