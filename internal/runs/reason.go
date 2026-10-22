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
	// answered it on the board and the answer waits for delivery. Ticket
	// is the ticket the question is about ("" when it is about none): its
	// card is where the human acts on it (About, FH-185).
	Question string `json:"question"`
	Answered bool   `json:"answered"`
	Ticket   string `json:"-"`
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

// About reports whether the reason needs you on a ticket's card (FH-185):
// a permission prompt is answered in the session, so it is its own card's;
// a question is answered on the card of the ticket it is about, and one
// about no ticket on the session's card. No reason is about any card.
func (r Reason) About(ticket string) bool {
	return r.Kind != "" && (r.Ticket == "" || r.Ticket == ticket)
}

// QuestionReason is the reason an open question gives the run it waits on
// (reason): a subagent's names the subagent, a session's own carries
// MCPOnly with Answered. asker is the view of the run that asked it. A
// card of the ticket a question is about words it as the run's Runs tab
// would, when the run is linked elsewhere (FH-185).
func QuestionReason(q Question, asker View) Reason {
	if asker.Parent != "" {
		return questionReason(q, asker.ID, asker.AgentType, false)
	}
	return questionReason(q, "", "", asker.MCPOnly)
}

// questionReason builds a question's reason; waitingOn and agentType name
// the subagent that asked it, empty for the run's own, and mcpOnly is the
// run's, kept only with Answered.
func questionReason(q Question, waitingOn, agentType string, mcpOnly bool) Reason {
	reason := Reason{Kind: ReasonQuestion, Question: q.Kind, Ticket: q.Ticket, Answered: q.Answered(), WaitingOn: waitingOn, AgentType: agentType}
	if reason.Answered && waitingOn == "" {
		reason.MCPOnly = mcpOnly
	}
	return reason
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
		return questionReason(q, "", "", false)
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
			return questionReason(q, child.ID, child.AgentType, false)
		}
	}
	if q, ok := r.firstOpen(true); ok {
		return questionReason(q, "", "", r.MCPOnly())
	}
	for _, child := range children {
		if q, ok := child.firstOpen(true); ok {
			return questionReason(q, child.ID, child.AgentType, false)
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
