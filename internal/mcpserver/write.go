package mcpserver

import (
	"regexp"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/scrub"
)

// Bounds on agent-written text (MCP-5, SEC-3).
const (
	maxItems    = 30
	maxItemText = 500
	maxNoteText = 2000
	maxReview   = 256 << 10
)

// agentKey is the key format agents may choose: 2–5 capitals or digits,
// starting with a letter (KEY-5).
var agentKey = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,4}$`)

// registerWrites registers the write tools, each with its run and claim
// policy (policy.go; agent-protocol §7.1's policy table). The handlers live
// by family: claims.go, handoff.go, tickets.go, workstreams.go, review.go
// and questions.go.
func (srv *server) registerWrites(server *mcp.Server) {
	tool(srv, server, "claim", "Claim a ticket before working on it: it moves to In progress, records your branch and holds the ticket for you while your session is active. Claiming another ticket releases this one. A ticket held by another live session, or blocked, needs force and a reason.", belongsToRun, srv.claim)
	tool(srv, server, "release", "Stop holding a ticket without finishing it (checkpoint first). The ticket stays in its column.", belongsToRun, srv.release)
	tool(srv, server, "checkpoint", "Record your handoff on a ticket: what is done, what is next, the files that matter and open questions. Rewrites the ticket's ## Handoff, which the next session resumes from. Do this at milestones and always before stopping after edits. Local files listed or linked outside the repository are copied into the ticket.", writesTicket, srv.checkpoint)
	tool(srv, server, "update_ticket", "Edit a ticket: set fields (title, type, priority, branch, pr, workstream, tags, depends-on, created), tick acceptance criteria by number (1 is the first) or text, and append to its notes. A title is for a person: what they would notice or gain, with names from the code left to the description or Context.", writesTicket, srv.updateTicket)
	tool(srv, server, "move", "Move a ticket to another column: backlog, up-next, in-progress or review. Moving to review warns about a missing review or unticked criteria. Agents never move tickets to done.", writesTicket, srv.move)
	tool(srv, server, "set_project_key", "Choose your project's ticket key (2–5 capital letters or digits, starting with a letter) before its first ticket. Pick what people call the project: FH for Flashheart.", writesProject, srv.setProjectKey)
	tool(srv, server, "create_ticket", "Create a ticket in your project with the next id. Write it for a person who has not read the code: the title says what they would notice or gain, the description gives the problem, why it matters and the proposed change in plain words, and implementation detail goes in context (the ticket's Context section). Feature tickets carry a test plan and bugs a reproduction (plan_or_repro). Returns the new id.", writesProject, srv.createTicket)
	tool(srv, server, "create_workstream", "Create a workstream in your project: a group of tickets with a shared goal, like an epic, worked on in sequence or in parallel. Its tickets wait only for the tickets their depends_on names, so give each ticket the depends_on it needs. Use one when work spans several tickets with a shared goal; leave single tickets alone.", writesTicket, srv.createWorkstream)
	tool(srv, server, "write_review", "Create or replace a ticket's review (the human verification guide, in the review template). Screenshots and other local files linked by absolute path are copied into the ticket.", writesTicket, srv.writeReview)
	tool(srv, server, "attach", "Copy a local file (screenshot, log or other: PNG, JPEG, GIF, WebP, PDF, text, markdown, JSON or log) into a ticket's files with a caption. Returns the stored name and a markdown snippet for the review's Evidence.", writesTicket, srv.attach)
	tool(srv, server, "ask_human", "Ask the human a question, decision, review or blocker. Your session shows as Needs you on the board; the answer arrives in a later prompt. Use it for any question that ends your turn too: a question asked only in chat leaves you in Waiting.", belongsToRun, srv.askHuman)
}

// items scrubs and bounds a list of agent-written items.
func items(list []string) []string {
	out := make([]string, 0, min(len(list), maxItems))
	for _, item := range list {
		if item = scrub.Text(item, maxItemText); item != "" && len(out) < maxItems {
			out = append(out, item)
		}
	}
	return out
}

// record appends events attributed to the caller's run; nothing is
// recorded for an unknown caller.
func (c *call) record(project string, list ...events.Event) error {
	if c.run == "" || len(list) == 0 {
		return nil
	}
	for index := range list {
		list[index].Time, list[index].Run, list[index].Agent, list[index].Project = c.now, c.run, agentOf(c.run), project
	}
	return c.srv.log.Append(list...)
}

// note is a dated ## Notes line written for an agent.
func (c *call) note(text string) string {
	return board.Note(c.now, protocol.ShortRun(c.by())+": "+text)
}
