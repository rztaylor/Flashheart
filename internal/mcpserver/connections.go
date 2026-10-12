package mcpserver

import (
	"strings"
	"sync"
	"time"

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
	// tools and failed count the run's tool calls since its last activity
	// record, written at recorded (agent-protocol §3, protocol 2).
	tools, failed int
	recorded      time.Time
}

// invocation is one tool call: its connection, and the run the call was
// attributed to once its call began.
type invocation struct {
	conn *conn
	run  string
	// every is activity_seconds, read when the call began.
	every time.Duration
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
	// Closing the connection ends the session, so it ends the run the
	// server started for it, if any; a server that dies leaves that to the
	// stale rule. The connection is first seen in a tool call, which ends
	// before the session's Wait returns, so Serve waits for this too.
	srv.ending.Add(1)
	go func() {
		defer srv.ending.Done()
		_ = session.Wait()
		srv.disconnected(cn)
	}()
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

// connection is the call's client connection, nil when it has none.
func (inv *invocation) connection() *conn {
	if inv == nil {
		return nil
	}
	return inv.conn
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

// disconnected ends a closed connection's run (run.end, reason
// disconnected) and forgets the connection.
func (srv *server) disconnected(cn *conn) {
	srv.mu.Lock()
	delete(srv.conns, cn.session)
	srv.mu.Unlock()
	cn.mu.Lock()
	defer cn.mu.Unlock()
	if cn.run == "" {
		return
	}
	// Activity not yet recorded goes first, as hooks flush it before an
	// end. Failing open: the stale rule ends a run whose end was not
	// recorded.
	now := srv.options.Now().UTC()
	list := []events.Event{{Time: now, Run: cn.run, Agent: agentOf(cn.run), Kind: events.RunEnd, Project: cn.project,
		Data: events.RunEndData{Reason: events.RunEndDisconnected}}}
	if activity, ok := cn.take(); ok {
		list = append([]events.Event{{Time: now, Run: cn.run, Agent: agentOf(cn.run), Kind: events.Activity, Project: cn.project, Data: activity}}, list...)
	}
	_ = srv.log.Append(list...)
}

// take returns the run's activity since its last record and starts a new
// count; ok is false when there is none. The caller holds cn.mu.
func (cn *conn) take() (events.ActivityData, bool) {
	activity := events.ActivityData{Tools: cn.tools, Failed: cn.failed}
	cn.tools, cn.failed = 0, 0
	return activity, activity.Tools > 0
}

// called counts a tool call of the connection's started run as its
// activity, recorded as hooks record tool results: at most one activity
// record per activity_seconds, which renews its claim's lease
// (agent-protocol §3, §6). It returns the answers waiting for the run,
// marked delivered: with no prompt hook, the next tool result carries them
// (HOOK-5). A call attributed to another run leaves the started run alone.
func (srv *server) called(inv *invocation, ok bool) string {
	cn := inv.conn
	if cn == nil {
		return ""
	}
	now := srv.options.Now().UTC()
	cn.mu.Lock()
	run, project := cn.run, cn.project
	if run == "" || inv.run != run {
		cn.mu.Unlock()
		return ""
	}
	cn.tools++
	if !ok {
		cn.failed++
	}
	if now.Sub(cn.recorded) >= inv.every {
		activity, _ := cn.take()
		cn.recorded = now
		// Activity is best effort: the call itself has already succeeded
		// or failed on its own terms.
		_ = srv.log.Append(events.Event{Time: now, Run: run, Agent: agentOf(run), Kind: events.Activity, Project: project, Data: activity})
	}
	cn.mu.Unlock()
	waiting, err := srv.log.TakeAnswers(project, run)
	if err != nil || len(waiting) == 0 {
		return ""
	}
	// Taken from the inbox, they are shown even if marking them fails;
	// board_context would then show them again.
	_ = srv.log.MarkDelivered(project, waiting, now)
	return "\n" + protocol.AnswersNote(answersOf(waiting))
}
