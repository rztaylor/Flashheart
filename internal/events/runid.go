package events

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// RunID is a run's identity (agent-protocol §2): <agent>:<session> for a
// session, <agent>:<session>/<subagent> for one of its subagents. It owns
// the grammar; nothing else cuts run ids apart. Events keep the id as a
// string; RunOf reads one back.
type RunID struct {
	agent, session, subagent string
}

var (
	// runAgent is an agent id: Claude Code's claude, Codex's codex, or an
	// MCP client's name made fit (mcpserver).
	runAgent = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	// runPart is a session or subagent id as agents send them.
	runPart = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
)

// ErrInvalidRunID is returned for a run id outside the grammar.
var ErrInvalidRunID = errors.New("invalid run id")

// SessionRun is the run id of an agent's session.
func SessionRun(agent, session string) (RunID, error) {
	return RunID{agent: agent, session: session}.validate()
}

// Subagent is the run id of one of the session's subagents.
func (r RunID) Subagent(id string) (RunID, error) {
	return RunID{agent: r.agent, session: r.session, subagent: id}.validate()
}

// ParseRunID reads a run id that must follow the grammar.
func ParseRunID(s string) (RunID, error) {
	r := RunOf(s)
	if r.String() != s {
		return RunID{}, fmt.Errorf("%w %q", ErrInvalidRunID, s)
	}
	return r.validate()
}

// RunOf reads a run id leniently, for ids already recorded: a malformed
// one keeps its parts as they fall, so String gives it back unchanged, and
// one with no agent (the human's board writes) is its own session.
func RunOf(s string) RunID {
	session, subagent, _ := strings.Cut(s, "/")
	agent, id, found := strings.Cut(session, ":")
	if !found {
		return RunID{session: session, subagent: subagent}
	}
	return RunID{agent: agent, session: id, subagent: subagent}
}

func (r RunID) validate() (RunID, error) {
	switch {
	case !runAgent.MatchString(r.agent):
		return RunID{}, fmt.Errorf("%w: agent %q", ErrInvalidRunID, r.agent)
	case !runPart.MatchString(r.session):
		return RunID{}, fmt.Errorf("%w: session %q", ErrInvalidRunID, r.session)
	case r.subagent != "" && !runPart.MatchString(r.subagent):
		return RunID{}, fmt.Errorf("%w: subagent %q", ErrInvalidRunID, r.subagent)
	}
	return r, nil
}

// String is the run id as recorded.
func (r RunID) String() string {
	s := r.session
	if r.agent != "" {
		s = r.agent + ":" + s
	}
	if r.subagent != "" {
		s += "/" + r.subagent
	}
	return s
}

// Agent is the agent id, "" for a run without one.
func (r RunID) Agent() string { return r.agent }

// SessionID is the agent's own session id.
func (r RunID) SessionID() string { return r.session }

// SubagentID is the subagent's id, "" for a session.
func (r RunID) SubagentID() string { return r.subagent }

// IsSubagent reports a subagent's run.
func (r RunID) IsSubagent() bool { return r.subagent != "" }

// Session is the session run: the run itself, or a subagent's parent.
func (r RunID) Session() RunID { return RunID{agent: r.agent, session: r.session} }

// Short is the agent and the first 8 characters of the session id, as
// recovery notes name runs (agent-protocol §2); an id without an agent is
// returned whole.
func (r RunID) Short() string {
	if r.agent == "" {
		return r.String()
	}
	return r.agent + ":" + r.session[:min(len(r.session), 8)]
}
