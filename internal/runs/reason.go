package runs

// What a run needs you for (Reason.Kind).
const (
	ReasonPermission = "permission"
	ReasonQuestion   = "question"
)

// Reason is why a run in Needs you needs you: the one blocker its card,
// Runs tab and Overview word (FH-98, D39). Its zero value means the run
// does not need you. It carries typed fields, never words.
type Reason struct {
	// Kind is ReasonPermission or ReasonQuestion; empty when nothing
	// needs you.
	Kind string `json:"kind"`
	// Permission is the tool awaiting permission ("?" when the agent did
	// not say which).
	Permission string `json:"permission"`
	// Question is the open question's kind; Answered means the human
	// answered it on the board and the answer waits for delivery.
	Question string `json:"question"`
	Answered bool   `json:"answered"`
	// WaitingOn is the subagent whose prompt or question it is, and
	// AgentType that subagent's type ("" when unknown); both are empty
	// when the blocker is the run's own.
	WaitingOn string `json:"waitingOn"`
	AgentType string `json:"agentType"`
	// MCPOnly is set with Answered when the run is MCP-only
	// (Run.MCPOnly): the answer waits for its next Flashheart tool call,
	// not a prompt.
	MCPOnly bool `json:"mcpOnly"`
}

// reason derives why r needs you, first match wins: its own permission
// prompt, its own unanswered question, a live subagent's prompt, a
// subagent's unanswered question, then an answer waiting for delivery, its
// own before a subagent's. What the human can act on comes first, and the
// run's own blocker before its subagents', each of which is also listed on
// its own. Subagents are taken in the order they were first seen. State
// is Needs you exactly when this is not zero.
func (s *Set) reason(r *Run) Reason {
	children := s.children(r)
	if r.Permission != "" {
		return Reason{Kind: ReasonPermission, Permission: r.Permission}
	}
	if q, ok := r.firstOpen(false); ok {
		return Reason{Kind: ReasonQuestion, Question: q.Kind}
	}
	for _, child := range children {
		if !child.ended && child.Permission != "" {
			return Reason{Kind: ReasonPermission, Permission: child.Permission, WaitingOn: child.ID, AgentType: child.AgentType}
		}
	}
	// A subagent's question holds its session after the subagent ends:
	// its answer goes to the session (agent-protocol §4).
	for _, child := range children {
		if q, ok := child.firstOpen(false); ok {
			return Reason{Kind: ReasonQuestion, Question: q.Kind, WaitingOn: child.ID, AgentType: child.AgentType}
		}
	}
	if q, ok := r.firstOpen(true); ok {
		return Reason{Kind: ReasonQuestion, Question: q.Kind, Answered: true, MCPOnly: r.MCPOnly()}
	}
	for _, child := range children {
		if q, ok := child.firstOpen(true); ok {
			return Reason{Kind: ReasonQuestion, Question: q.Kind, Answered: true, WaitingOn: child.ID, AgentType: child.AgentType}
		}
	}
	return Reason{}
}

// firstOpen is the run's first open question, answered on the board or
// not as answered says.
func (r *Run) firstOpen(answered bool) (Question, bool) {
	for _, q := range r.Questions {
		if q.Open() && q.Answered() == answered {
			return q, true
		}
	}
	return Question{}, false
}
