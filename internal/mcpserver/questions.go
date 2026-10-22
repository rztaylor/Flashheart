package mcpserver

import (
	"crypto/rand"
	"fmt"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/scrub"
)

// AskHumanInput is ask_human's input (RUN-8).
type AskHumanInput struct {
	Attribution
	Ticket  string   `json:"ticket,omitempty" jsonschema:"the ticket it is about, if any"`
	Kind    string   `json:"kind" jsonschema:"question, decision, review or blocked"`
	Text    string   `json:"text" jsonschema:"the question, self-contained, at most 1,000 characters"`
	Options []string `json:"options,omitempty" jsonschema:"answers to choose from, when there are clear choices"`
}

func (srv *server) askHuman(c *call, input AskHumanInput) (string, error) {
	run := c.run // never "": the tool belongs to a run (belongsToRun)
	kind := strings.TrimSpace(input.Kind)
	if !slices.Contains(events.QuestionKinds, kind) {
		return "", fail("invalid_input", "use question, decision, review or blocked", "%q is not a question kind", input.Kind)
	}
	text := scrub.Text(input.Text, events.MaxQuestionText)
	if text == "" {
		return "", fail("invalid_input", "write the question in text", "the question is empty")
	}
	if len(input.Options) > events.MaxOptions {
		return "", fail("invalid_input", fmt.Sprintf("offer at most %d options", events.MaxOptions), "too many options")
	}
	var options []string
	for _, option := range input.Options {
		if option = scrub.Text(option, events.MaxOptionText); option != "" {
			options = append(options, option)
		}
	}
	ticketID, ticketProject := "", ""
	if strings.TrimSpace(input.Ticket) != "" {
		in, ticket, err := c.target(input.Ticket)
		if err != nil {
			return "", err
		}
		ticketID, ticketProject = ticket.ID, in.Name
	}
	project := c.project
	if project == "" {
		if r := c.set.Get(run); r != nil {
			project = r.Project
		}
	}
	if project == "" && c.archived != "" {
		return "", c.archivedError()
	}
	if project == "" {
		return "", fail("not_found", "run the session inside a project's repository", "there is no project to record the question in")
	}
	id := questionID()
	if err := c.record(project, events.Event{Kind: events.QuestionAsked, Data: events.QuestionData{ID: id, Ticket: ticketID, Kind: kind, Text: text, Options: options}}); err != nil {
		return "", err
	}
	// The ticket keeps the question in its history (FH-190). The question
	// is asked once it is in the log, so a failure here is only reported.
	kept := ""
	if ticketID != "" {
		change := board.Change{What: board.ChangeAsked, Question: board.QuestionRecord{ID: id, Kind: kind, Text: text, Options: options}}
		if _, err := c.files().RecordQuestion(ticketProject, ticketID, c.now, change); err != nil {
			kept = fmt.Sprintf("The question could not be added to %s's history: %v.\n", ticketID, err)
		}
	}
	// The await command wakes an agent that can run background commands
	// as soon as the answer is given (agent-protocol §7.5).
	command := protocol.AwaitCommand(srv.options.Binary, srv.options.Root, project, id)
	arrives := "the answer will arrive in a later prompt, or the user replies in this chat, which settles it"
	if started, _ := c.inv.connection().current(); started == run {
		// No prompt hook delivers to a run the server started (HOOK-5), or
		// sees a reply in its chat (§4).
		arrives = "the answer will arrive with the result of your next flashheart tool call. A reply in this chat does not reach the board, so if the user answers here, ask them to answer it on the board too"
	}
	// Only a Claude run is told how Claude Code runs it.
	how := ""
	if agentOf(run) == "claude" {
		how = " (Claude Code: Bash with run_in_background)"
	}
	return fmt.Sprintf("Asked the human (%s). Your session shows as Needs you on the board; %s. Carry on with other work if you can.\n"+
		"%sTo get the answer as soon as it is given, run this as a background command that wakes you when it exits%s:\n%s\nok question=%s\n", kind, arrives, kept, how, command, id), nil
}

func questionID() string {
	return "q-" + rand.Text()[:16]
}
