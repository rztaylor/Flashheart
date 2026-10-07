package mcpserver

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/scrub"
	"github.com/rztaylor/flashheart/internal/store"
)

// Bounds on agent-written text (MCP-5, SEC-3).
const (
	maxItems       = 30
	maxItemText    = 500
	maxNoteText    = 2000
	maxDescription = 16 << 10
	maxReview      = 256 << 10
)

// agentKey is the key format agents may choose: 2–5 capitals or digits,
// starting with a letter (KEY-5).
var agentKey = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,4}$`)

func (srv *server) registerWrites(server *mcp.Server) {
	tool(server, "claim", "Claim a ticket before working on it: it moves to In progress, records your branch and holds the ticket for you while your session is active. Claiming another ticket releases this one. A ticket held by another live session, or blocked, needs force and a reason.", srv.claim)
	tool(server, "release", "Stop holding a ticket without finishing it (checkpoint first). The ticket stays in its column.", srv.release)
	tool(server, "checkpoint", "Record your handoff on a ticket: what is done, what is next, the files that matter and open questions. Rewrites the ticket's ## Handoff, which the next session resumes from. Do this at milestones and always before stopping after edits. Local files listed or linked outside the repository are copied into the ticket.", srv.checkpoint)
	tool(server, "update_ticket", "Edit a ticket: set fields (title, type, priority, branch, workstream, tags, depends-on, created), tick acceptance criteria by number (1 is the first) or text, and append to its notes.", srv.updateTicket)
	tool(server, "move", "Move a ticket to another column: backlog, up-next, in-progress or review. Moving to review warns about a missing review or unticked criteria. Agents never move tickets to done.", srv.move)
	tool(server, "set_project_key", "Choose your project's ticket key (2–5 capital letters or digits, starting with a letter) before its first ticket. Pick what people call the project: FH for Flashheart.", srv.setProjectKey)
	tool(server, "create_ticket", "Create a ticket in your project with the next id. Feature tickets carry a test plan and bugs a reproduction (plan_or_repro). Returns the new id.", srv.createTicket)
	tool(server, "create_workstream", "Create a workstream in your project: an ordered group of tickets with a shared goal, where each ticket waits for the ones before it. Use one when work spans several dependent tickets; leave single tickets alone. The tickets you list join it in that order.", srv.createWorkstream)
	tool(server, "write_review", "Create or replace a ticket's review (the human verification guide, in the review template). Screenshots and other local files linked by absolute path are copied into the ticket.", srv.writeReview)
	tool(server, "ask_human", "Ask the human a question, decision, review or blocker. Your session shows as Needs you on the board; the answer arrives in a later prompt. Use it for any question that ends your turn too: a question asked only in chat leaves you in Waiting.", srv.askHuman)
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
	return fmt.Sprintf("- %s · %s: %s", c.now.Format("2006-01-02"), protocol.ShortRun(c.by()), text)
}

// related reports whether two runs are one session and its subagents.
func related(a, b string) bool {
	sa, _, _ := strings.Cut(a, "/")
	sb, _, _ := strings.Cut(b, "/")
	return sa == sb
}

// ClaimInput is claim's input.
type ClaimInput struct {
	Attribution
	Ticket string `json:"ticket" jsonschema:"the ticket id, like FH-42"`
	Force  bool   `json:"force,omitempty" jsonschema:"take a ticket another live session holds, or start a blocked one; needs reason"`
	Reason string `json:"reason,omitempty" jsonschema:"why force is right; recorded in the ticket's notes"`
}

func (srv *server) claim(input ClaimInput) (string, error) {
	c, err := srv.begin(input.Run)
	if err != nil {
		return "", err
	}
	run, err := c.requireRun()
	if err != nil {
		return "", err
	}
	project, ticket, err := c.find(input.Ticket)
	if err != nil {
		return "", err
	}
	if ticket.NeedsRepair() {
		return "", fail("needs_repair", "ask the human to repair it on the board", "%s needs repair: %s", ticket.ID, strings.Join(ticket.Repair, "; "))
	}
	if ticket.Column == board.Done {
		return "", fail("invalid_input", "create a ticket for new work on it, or ask the human to reopen it", "%s is done", ticket.ID)
	}
	reason := scrub.Text(input.Reason, events.MaxReasonText)
	if input.Force && reason == "" {
		return "", fail("invalid_input", "say why in reason", "force needs a reason")
	}
	var notes []string
	holder := c.holder(ticket.ID)
	if holder != nil && !related(holder.ID, run) {
		if !input.Force {
			return "", fail("claimed", "choose another ticket, or pass force with a reason if that session is gone",
				"%s is held by %s (%s)", ticket.ID, protocol.ShortRun(holder.ID), c.state(holder))
		}
		notes = append(notes, c.note(fmt.Sprintf("Claimed from %s (%s): %s", protocol.ShortRun(holder.ID), c.state(holder), reason)))
	}
	reasons := c.blocked(project, ticket)
	startsBlocked := len(reasons) > 0 && ticket.Column != board.InProgress
	if startsBlocked {
		if !input.Force {
			return "", fail("blocked", "work on what blocks it first, or pass force with a reason to start it anyway",
				"%s is blocked: %s", ticket.ID, strings.Join(reasons, "; "))
		}
		notes = append(notes, "- "+c.now.Format("2006-01-02")+" · "+board.BlockedStartNote(reason))
	}

	var list []events.Event
	// One explicit claim per run: claiming another releases the first, in
	// the log of the released ticket's project, where its holder is read.
	if current := c.set.Get(run); current != nil && current.Claim != "" && current.Claim != ticket.ID {
		released := events.Event{Kind: events.Release, Data: events.TicketData{Ticket: current.Claim, Reason: "claimed " + ticket.ID}}
		if err := c.record(c.projectOf(current.Claim, project.Name), released); err != nil {
			return "", err
		}
	}
	column := ticket.Column
	moves := column == board.Backlog || column == board.UpNext
	if moves {
		column = board.InProgress
	}
	setBranch := ticket.Branch == "" && c.where.Branch != ""
	if moves || setBranch || len(notes) > 0 {
		_, err := srv.store.UpdateTicket(project.Name, ticket.ID, "", func(data []byte) ([]byte, error) {
			var err error
			if moves {
				if data, err = mdfile.SetScalar(data, "status", string(column)); err != nil {
					return nil, err
				}
			}
			if setBranch {
				if data, err = mdfile.SetScalar(data, "branch", c.where.Branch); err != nil {
					return nil, err
				}
			}
			for _, note := range notes {
				if data, err = mdfile.AppendToSection(data, "Notes", note); err != nil {
					return nil, err
				}
			}
			return data, nil
		})
		if err != nil {
			return "", err
		}
	}
	list = append(list, events.Event{Kind: events.Claim, Data: events.TicketData{Ticket: ticket.ID, Force: input.Force, Reason: reason}})
	if moves {
		list = append(list, events.Event{Kind: events.TicketMoved, Data: events.TicketData{Ticket: ticket.ID, From: string(ticket.Column), To: string(column), By: run}})
	}
	if err := c.record(project.Name, list...); err != nil {
		return "", err
	}

	lines := []string{fmt.Sprintf("Claimed %s %q (%s).", ticket.ID, scrub.Limit(ticket.Title, 200), column)}
	if ticket.Handoff != nil {
		lines = append(lines, handoffLines(ticket.Handoff.Markdown, 10, func(s string) string { return scrub.Limit(s, 300) })...)
	}
	var open []string
	for _, criterion := range ticket.Criteria {
		if !criterion.Done {
			open = append(open, criterion.Text)
		}
	}
	if len(open) > 0 {
		lines = append(lines, fmt.Sprintf("Unticked criteria (%d of %d):", len(open), len(ticket.Criteria)))
		lines = append(lines, bullets(open, 15, func(s string) string { return scrub.Limit(s, 300) })...)
	}
	lines = append(lines, "Read it in full with get_ticket. "+dataNote, fmt.Sprintf("ok ticket=%s column=%s", ticket.ID, column))
	return strings.Join(lines, "\n") + "\n", nil
}

// ReleaseInput is release's input.
type ReleaseInput struct {
	Attribution
	Ticket string `json:"ticket" jsonschema:"the ticket id"`
	Reason string `json:"reason,omitempty"`
}

func (srv *server) release(input ReleaseInput) (string, error) {
	c, err := srv.begin(input.Run)
	if err != nil {
		return "", err
	}
	run, err := c.requireRun()
	if err != nil {
		return "", err
	}
	project, ticket, err := c.find(input.Ticket)
	if err != nil {
		return "", err
	}
	if r := c.set.Get(run); r == nil || r.Claim != ticket.ID {
		return fmt.Sprintf("You do not hold %s; nothing to release.\nok ticket=%s\n", ticket.ID, ticket.ID), nil
	}
	if err := c.record(project.Name, events.Event{Kind: events.Release, Data: events.TicketData{Ticket: ticket.ID, Reason: scrub.Text(input.Reason, events.MaxReasonText)}}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Released %s; it stays in %s.\nok ticket=%s\n", ticket.ID, ticket.Column, ticket.ID), nil
}

// CheckpointInput is checkpoint's input (RUN-7).
type CheckpointInput struct {
	Attribution
	Ticket        string   `json:"ticket" jsonschema:"the ticket id"`
	Done          []string `json:"done,omitempty" jsonschema:"what is finished"`
	Next          []string `json:"next,omitempty" jsonschema:"what the next session should do first"`
	Files         []string `json:"files,omitempty" jsonschema:"files that matter, repository-relative; screenshots and logs outside the repository by absolute path are copied in"`
	OpenQuestions []string `json:"open_questions,omitempty"`
	Note          string   `json:"note,omitempty" jsonschema:"one paragraph appended to the ticket's notes"`
}

func items(list []string) []string {
	out := make([]string, 0, min(len(list), maxItems))
	for _, item := range list {
		if item = scrub.Text(item, maxItemText); item != "" && len(out) < maxItems {
			out = append(out, item)
		}
	}
	return out
}

func (srv *server) checkpoint(input CheckpointInput) (string, error) {
	c, err := srv.begin(input.Run)
	if err != nil {
		return "", err
	}
	if len(c.candidates) > 1 {
		if _, err := c.requireRun(); err != nil {
			return "", err
		}
	}
	project, ticket, err := c.find(input.Ticket)
	if err != nil {
		return "", err
	}
	if err := c.guard(ticket.ID); err != nil {
		return "", err
	}
	done, next, questions := items(input.Done), items(input.Next), items(input.OpenQuestions)
	if len(done)+len(next) == 0 {
		return "", fail("invalid_input", "say what is done and what is next", "a checkpoint needs done or next items")
	}
	copies := c.copier(project.Name, ticket.ID)
	for _, list := range [][]string{done, next, questions} {
		for index := range list {
			list[index] = copies.markdown(list[index])
		}
	}
	var files []string
	for _, entry := range items(input.Files) {
		files = append(files, copies.file(entry))
	}
	note := ""
	if text := strings.TrimSpace(scrub.Secrets(input.Note)); text != "" {
		note = c.note(copies.markdown(board.OneLine(text, maxNoteText)))
	}
	branch := c.where.Branch
	if r := c.set.Get(c.run); r != nil && r.Branch != "" {
		branch = r.Branch
	}
	section := handoffSection(c.now, protocol.ShortRun(c.by()), branch, done, next, files, questions)
	same := ticket.Handoff != nil && slices.Equal(board.HandoffList(ticket.Handoff.Markdown, "Done"), done) &&
		slices.Equal(board.HandoffList(ticket.Handoff.Markdown, "Next"), next) &&
		slices.Equal(board.HandoffList(ticket.Handoff.Markdown, "Files"), orNone(files)) &&
		slices.Equal(board.HandoffList(ticket.Handoff.Markdown, "Open questions"), orNone(questions))
	if !same || note != "" {
		_, err = srv.store.UpdateTicket(project.Name, ticket.ID, "", func(data []byte) ([]byte, error) {
			if !same {
				var err error
				if data, err = mdfile.ReplaceSection(data, "Handoff", section); err != nil {
					return nil, err
				}
			}
			if note != "" {
				return mdfile.AppendToSection(data, "Notes", note)
			}
			return data, nil
		})
		if err != nil {
			return "", err
		}
	}
	// The checkpoint clears the run's edits where they are recorded (its own
	// project) and where the ticket's runs are read (the ticket's project).
	checkpointed := events.Event{Kind: events.Checkpoint, Data: events.CheckpointData{Ticket: ticket.ID, Done: len(done), Next: len(next), Files: len(files), Questions: len(questions)}}
	for _, where := range c.projectsFor(project.Name) {
		if err := c.record(where, checkpointed); err != nil {
			return "", err
		}
	}
	var lines []string
	if same && note == "" {
		lines = append(lines, "The handoff already says this; nothing changed.")
	}
	if len(copies.copied) > 0 {
		lines = append(lines, "Copied into the ticket: "+strings.Join(copies.copied, ", "))
	}
	for _, warning := range copies.warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, fmt.Sprintf("ok ticket=%s checkpoint by=%s", ticket.ID, protocol.ShortRun(c.by())))
	return strings.Join(lines, "\n") + "\n", nil
}

func orNone(list []string) []string {
	if len(list) == 0 {
		return []string{"None"}
	}
	return list
}

// handoffSection renders ## Handoff (board-format §Handoff section).
func handoffSection(now time.Time, by, branch string, done, next, files, questions []string) string {
	header := fmt.Sprintf("_Updated %s by %s (run)", now.UTC().Format("2006-01-02 15:04 UTC"), by)
	if branch != "" {
		header += " on " + branch
	}
	lines := []string{header + "._", ""}
	for _, part := range []struct {
		label string
		items []string
	}{{"Done", done}, {"Next", next}, {"Files", orNone(files)}, {"Open questions", orNone(questions)}} {
		if len(part.items) == 0 {
			continue
		}
		lines = append(lines, "**"+part.label+"**", "")
		for _, item := range part.items {
			lines = append(lines, "- "+item)
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// UpdateTicketInput is update_ticket's input.
type UpdateTicketInput struct {
	Attribution
	Ticket      string         `json:"ticket" jsonschema:"the ticket id"`
	Set         map[string]any `json:"set,omitempty" jsonschema:"fields to set: title, type, priority, created, branch, workstream (strings); tags, depends-on, depends-on-workstreams (lists of strings)"`
	Check       []string       `json:"check,omitempty" jsonschema:"acceptance criteria to tick, by number (1 is the first) or by their text"`
	AppendNotes string         `json:"append_notes,omitempty" jsonschema:"a paragraph appended to the ticket's notes"`
}

func (srv *server) updateTicket(input UpdateTicketInput) (string, error) {
	c, err := srv.begin(input.Run)
	if err != nil {
		return "", err
	}
	if len(c.candidates) > 1 {
		if _, err := c.requireRun(); err != nil {
			return "", err
		}
	}
	project, ticket, err := c.find(input.Ticket)
	if err != nil {
		return "", err
	}
	if err := c.guard(ticket.ID); err != nil {
		return "", err
	}
	type change struct {
		key    string
		value  string
		list   []string
		isList bool
	}
	var changes []change
	for key, raw := range input.Set {
		ch := change{key: key, isList: slices.Contains(board.ListFields, key)}
		switch {
		case key == "status":
			return "", fail("invalid_input", "use move to change a ticket's column", "status is changed with move")
		case ch.isList:
			values, ok := stringList(raw)
			if !ok {
				return "", fail("invalid_input", "pass a list of strings", "%s must be a list of strings", key)
			}
			values = items(values)
			for _, value := range values {
				if err := board.ValidateField(key, value); err != nil {
					return "", fail("invalid_input", "correct the value", "%v", err)
				}
			}
			ch.list = values
		case slices.Contains(board.ScalarFields, key):
			value, ok := raw.(string)
			if !ok {
				return "", fail("invalid_input", "pass a string", "%s must be a string", key)
			}
			ch.value = scrub.Text(value, 300)
			if err := board.ValidateField(key, ch.value); err != nil {
				return "", fail("invalid_input", "correct the value", "%v", err)
			}
		default:
			return "", fail("invalid_input", "settable fields: "+strings.Join(settable, ", "), "%q cannot be set", key)
		}
		changes = append(changes, ch)
	}
	slices.SortFunc(changes, func(a, b change) int { return strings.Compare(a.key, b.key) })
	// Criteria are found in the same bytes whose hash guards the write, so a
	// reordering in between is a conflict, not a wrong tick (STO-3).
	data, base, err := srv.store.ReadTicket(project.Name, ticket.ID)
	if err != nil {
		return "", err
	}
	current := board.ParseTicket(ticket.Folder, data)
	var checks []int
	for _, item := range input.Check {
		index, err := criterionIndex(current.Criteria, item)
		if err != nil {
			return "", err
		}
		checks = append(checks, index)
	}
	note := ""
	if text := strings.TrimSpace(scrub.Secrets(input.AppendNotes)); text != "" {
		note = c.note(c.copier(project.Name, ticket.ID).markdown(board.OneLine(text, maxNoteText)))
	}
	if len(changes) == 0 && len(checks) == 0 && note == "" {
		return "", fail("invalid_input", "pass set, check or append_notes", "nothing to change")
	}
	// A workstream change also rewrites the workstreams' tickets lists.
	workstream := slices.IndexFunc(changes, func(ch change) bool { return ch.key == "workstream" })
	edit := func(data []byte) ([]byte, error) {
		var err error
		for _, ch := range changes {
			switch {
			case ch.key == "workstream":
				continue
			case ch.key == "title":
				data, err = mdfile.SetTitle(data, ch.value)
			case ch.isList:
				data, err = mdfile.SetList(data, ch.key, ch.list)
			default:
				data, err = mdfile.SetScalar(data, ch.key, ch.value)
			}
			if err != nil {
				return nil, err
			}
		}
		for _, index := range checks {
			if data, err = mdfile.SetCheckbox(data, "Acceptance Criteria", index, true); err != nil {
				return nil, err
			}
		}
		if note != "" {
			return mdfile.AppendToSection(data, "Notes", note)
		}
		return data, nil
	}
	if workstream >= 0 {
		_, err = srv.store.SetTicketWorkstream(project.Name, ticket.ID, base, changes[workstream].value, edit)
	} else {
		_, err = srv.store.UpdateTicket(project.Name, ticket.ID, base, edit)
	}
	if err != nil {
		return "", err
	}
	var fields []string
	for _, ch := range changes {
		fields = append(fields, ch.key)
	}
	if len(checks) > 0 {
		fields = append(fields, "criteria")
	}
	if note != "" {
		fields = append(fields, "notes")
	}
	if err := c.record(project.Name, events.Event{Kind: events.TicketUpdated, Data: events.TicketData{Ticket: ticket.ID, Fields: fields}}); err != nil {
		return "", err
	}
	return fmt.Sprintf("ok ticket=%s changed=%s\n", ticket.ID, strings.Join(fields, ",")), nil
}

func stringList(raw any) ([]string, bool) {
	switch value := raw.(type) {
	case string:
		var list []string
		for _, item := range strings.Split(value, ",") {
			if item = strings.TrimSpace(item); item != "" {
				list = append(list, item)
			}
		}
		return list, true
	case []any:
		list := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			if text = strings.TrimSpace(text); text != "" {
				list = append(list, text)
			}
		}
		return list, true
	}
	return nil, false
}

// criterionIndex finds a criterion by 1-based number or by text.
func criterionIndex(criteria []board.Criterion, item string) (int, error) {
	item = strings.TrimSpace(item)
	if number, err := strconv.Atoi(item); err == nil {
		if number < 1 || number > len(criteria) {
			return 0, fail("invalid_input", fmt.Sprintf("number the criteria 1 to %d", len(criteria)), "there is no criterion %d", number)
		}
		return number - 1, nil
	}
	var matches []int
	for index, criterion := range criteria {
		if strings.EqualFold(strings.TrimSpace(criterion.Text), item) {
			return index, nil
		}
		if item != "" && strings.HasPrefix(strings.ToLower(criterion.Text), strings.ToLower(item)) {
			matches = append(matches, index)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return 0, fail("invalid_input", "pass the criterion's number (1 is the first) or its full text", "no single criterion matches %q", item)
}

// MoveInput is move's input.
type MoveInput struct {
	Attribution
	Ticket string `json:"ticket" jsonschema:"the ticket id"`
	To     string `json:"to" jsonschema:"backlog, up-next, in-progress or review"`
}

func (srv *server) move(input MoveInput) (string, error) {
	c, err := srv.begin(input.Run)
	if err != nil {
		return "", err
	}
	if len(c.candidates) > 1 {
		if _, err := c.requireRun(); err != nil {
			return "", err
		}
	}
	to, ok := board.ParseColumn(strings.TrimSpace(input.To))
	if !ok {
		return "", fail("invalid_input", "use backlog, up-next, in-progress or review", "%q is not a column", input.To)
	}
	if to == board.Done {
		return "", fail("invalid_input", "move it to review; the human moves it to done after verifying", "agents do not move tickets to done")
	}
	project, ticket, err := c.find(input.Ticket)
	if err != nil {
		return "", err
	}
	if err := c.guard(ticket.ID); err != nil {
		return "", err
	}
	if to == board.InProgress && ticket.Column != board.InProgress {
		if reasons := c.blocked(project, ticket); len(reasons) > 0 {
			return "", fail("blocked", "claim it with force and a reason to start it anyway", "%s is blocked: %s", ticket.ID, strings.Join(reasons, "; "))
		}
	}
	if _, err := srv.store.UpdateTicket(project.Name, ticket.ID, "", func(data []byte) ([]byte, error) {
		return mdfile.SetScalar(data, "status", string(to))
	}); err != nil {
		return "", err
	}
	var warnings []string
	if to == board.Review && ticket.Column != board.Review {
		warnings = board.ReviewWarnings(ticket, project.Reviews[ticket.ID])
	}
	if ticket.Column != to {
		if err := c.record(project.Name, events.Event{Kind: events.TicketMoved, Data: events.TicketData{Ticket: ticket.ID, From: string(ticket.Column), To: string(to), By: c.by()}}); err != nil {
			return "", err
		}
	}
	var lines []string
	for _, warning := range warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, fmt.Sprintf("ok ticket=%s column=%s", ticket.ID, to))
	return strings.Join(lines, "\n") + "\n", nil
}

// SetProjectKeyInput is set_project_key's input.
type SetProjectKeyInput struct {
	Attribution
	Key     string `json:"key" jsonschema:"2–5 capital letters or digits, starting with a letter"`
	Project string `json:"project,omitempty" jsonschema:"default: your project"`
}

func (c *call) checkKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if !agentKey.MatchString(key) {
		return "", fail("invalid_input", "use 2–5 capital letters or digits starting with a letter, like FH", "%q is not a key", key)
	}
	return key, nil
}

func (srv *server) setProjectKey(input SetProjectKeyInput) (string, error) {
	c, err := srv.begin(input.Run)
	if err != nil {
		return "", err
	}
	key, err := c.checkKey(input.Key)
	if err != nil {
		return "", err
	}
	project, err := c.projectArg(input.Project)
	if err != nil {
		return "", err
	}
	if err := srv.store.SetProjectKey(project.Name, key); err != nil {
		return "", err
	}
	return fmt.Sprintf("Project %s now uses key %s; its first ticket will be %s-%d.\nok key=%s project=%s\n", project.Name, key, key, max(project.NextID, 1), key, project.Name), nil
}

// CreateTicketInput is create_ticket's input.
type CreateTicketInput struct {
	Attribution
	Type        string   `json:"type" jsonschema:"feature, bug, test, refactor, infra, docs or spike"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Criteria    []string `json:"criteria" jsonschema:"testable acceptance criteria"`
	Priority    string   `json:"priority" jsonschema:"high, medium or low"`
	Status      string   `json:"status,omitempty" jsonschema:"backlog (default) or up-next"`
	Workstream  string   `json:"workstream,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty" jsonschema:"ids of tickets this one needs first"`
	Tags        []string `json:"tags,omitempty"`
	PlanOrRepro string   `json:"plan_or_repro,omitempty" jsonschema:"the Test Plan (features) or Reproduction (bugs)"`
	ProjectKey  string   `json:"project_key,omitempty" jsonschema:"only for a project with no key yet: the key to record first"`
}

func (srv *server) createTicket(input CreateTicketInput) (string, error) {
	c, err := srv.begin(input.Run)
	if err != nil {
		return "", err
	}
	if len(c.candidates) > 1 {
		if _, err := c.requireRun(); err != nil {
			return "", err
		}
	}
	project, err := c.projectArg("")
	if err != nil {
		return "", err
	}
	status := board.Column(strings.TrimSpace(input.Status))
	if status != "" && status != board.Backlog && status != board.UpNext {
		return "", fail("invalid_input", "use backlog or up-next; claim the ticket to start it", "new tickets start in backlog or up-next, not %q", input.Status)
	}
	key := ""
	if input.ProjectKey != "" {
		if key, err = c.checkKey(input.ProjectKey); err != nil {
			return "", err
		}
		if !project.KeyDerived || project.OwnsIDs() {
			if key != project.Key {
				return "", fail("key_fixed", "leave project_key out; this project's key is "+project.Key, "project %s already uses key %s", project.Name, project.Key)
			}
			key = ""
		}
	}
	if len(input.Description) > maxDescription || len(input.PlanOrRepro) > maxDescription {
		return "", fail("too_large", "keep the description and plan under 16 KiB; link to longer documents", "the text is too long")
	}
	for _, id := range input.DependsOn {
		if _, _, ok := board.ParseID(id); !ok {
			return "", fail("invalid_input", "pass ticket ids like FH-42", "depends_on %q is not a ticket id", id)
		}
	}
	created, err := srv.store.CreateTicket(project.Name, store.NewTicket{
		Title: scrub.Text(input.Title, 300), Type: input.Type, Priority: input.Priority, Status: status,
		Workstream: strings.TrimSpace(input.Workstream), Description: demoteHeadings(scrub.Secrets(input.Description)),
		Criteria: items(input.Criteria), DependsOn: input.DependsOn, Tags: items(input.Tags),
		Session: c.by(), PlanOrRepro: demoteHeadings(scrub.Secrets(input.PlanOrRepro)), Key: key,
	})
	if err != nil {
		return "", err
	}
	if err := c.record(project.Name, events.Event{Kind: events.TicketCreated, Data: events.TicketData{Ticket: created.ID, By: c.by()}}); err != nil {
		return "", err
	}
	column := status
	if column == "" {
		column = board.Backlog
	}
	text := fmt.Sprintf("Created %s %q in %s.", created.ID, scrub.Text(input.Title, 200), column)
	if created.Key != "" {
		text += fmt.Sprintf(" Project %s now uses key %s.", project.Name, created.Key)
	}
	return text + fmt.Sprintf("\nok ticket=%s\n", created.ID), nil
}

// CreateWorkstreamInput is create_workstream's input.
type CreateWorkstreamInput struct {
	Attribution
	Title                string   `json:"title" jsonschema:"what the line of work is called; its slug is made from it"`
	Goal                 string   `json:"goal" jsonschema:"the shared outcome the tickets deliver"`
	Priority             string   `json:"priority,omitempty" jsonschema:"high, medium (default) or low"`
	Tickets              []string `json:"tickets,omitempty" jsonschema:"ids of your project's tickets, in the order they should be done"`
	DependsOnWorkstreams []string `json:"depends_on_workstreams,omitempty" jsonschema:"slugs of workstreams that must finish first"`
	Tags                 []string `json:"tags,omitempty"`
}

func (srv *server) createWorkstream(input CreateWorkstreamInput) (string, error) {
	c, err := srv.begin(input.Run)
	if err != nil {
		return "", err
	}
	if len(c.candidates) > 1 {
		if _, err := c.requireRun(); err != nil {
			return "", err
		}
	}
	project, err := c.projectArg("")
	if err != nil {
		return "", err
	}
	if len(input.Goal) > maxDescription {
		return "", fail("too_large", "keep the goal under 16 KiB; link to longer documents", "the goal is too long")
	}
	if len(input.Tickets) > maxItems {
		return "", fail("invalid_input", fmt.Sprintf("list at most %d tickets; add more with update_ticket", maxItems), "too many tickets")
	}
	var tickets []string
	for _, id := range input.Tickets {
		owner, ticket, err := c.find(id)
		if err != nil {
			return "", err
		}
		if owner.Name != project.Name {
			return "", fail("invalid_input", "list tickets of project "+project.Name, "%s belongs to project %s", ticket.ID, owner.Name)
		}
		if err := c.guard(ticket.ID); err != nil {
			return "", err
		}
		tickets = append(tickets, ticket.ID)
	}
	var dependencies []string
	for _, slug := range items(input.DependsOnWorkstreams) {
		dependencies = append(dependencies, strings.TrimSpace(slug))
	}
	title := scrub.Text(input.Title, 300)
	created, err := srv.store.CreateWorkstream(project.Name, store.NewWorkstream{
		Title: title, Goal: demoteHeadings(scrub.Secrets(input.Goal)), Priority: strings.TrimSpace(input.Priority),
		Tickets: tickets, DependsOnWorkstreams: dependencies, Tags: items(input.Tags),
	})
	if err != nil {
		return "", err
	}
	var list []events.Event
	for _, id := range tickets {
		list = append(list, events.Event{Kind: events.TicketUpdated, Data: events.TicketData{Ticket: id, Fields: []string{"workstream"}}})
	}
	if err := c.record(project.Name, list...); err != nil {
		return "", err
	}
	text := fmt.Sprintf("Created workstream %s %q", created.Slug, scrub.Limit(title, 200))
	if len(tickets) > 0 {
		text += " with " + strings.Join(tickets, ", ")
	}
	text += fmt.Sprintf(". Add tickets with create_ticket or update_ticket workstream=%s; each waits for the ones before it.", created.Slug)
	return text + fmt.Sprintf("\nok workstream=%s\n", created.Slug), nil
}

// WriteReviewInput is write_review's input.
type WriteReviewInput struct {
	Attribution
	Ticket   string `json:"ticket" jsonschema:"the ticket id"`
	Markdown string `json:"markdown" jsonschema:"the whole review in the review template; link screenshots by absolute path"`
}

func (srv *server) writeReview(input WriteReviewInput) (string, error) {
	c, err := srv.begin(input.Run)
	if err != nil {
		return "", err
	}
	if len(c.candidates) > 1 {
		if _, err := c.requireRun(); err != nil {
			return "", err
		}
	}
	if strings.TrimSpace(input.Markdown) == "" {
		return "", fail("invalid_input", "write the review in the review template", "the review is empty")
	}
	if len(input.Markdown) > maxReview {
		return "", fail("too_large", "keep the review under 256 KiB", "the review is too long")
	}
	project, ticket, err := c.find(input.Ticket)
	if err != nil {
		return "", err
	}
	if err := c.guard(ticket.ID); err != nil {
		return "", err
	}
	copies := c.copier(project.Name, ticket.ID)
	text := copies.markdown(scrub.Secrets(input.Markdown))
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if err := srv.store.WriteReview(project.Name, ticket.ID, []byte(text)); err != nil {
		return "", err
	}
	if err := c.record(project.Name, events.Event{Kind: events.ReviewWritten, Data: events.TicketData{Ticket: ticket.ID}}); err != nil {
		return "", err
	}
	lines := []string{fmt.Sprintf("Review written: tickets/%s/review.md", ticket.Folder)}
	if len(copies.copied) > 0 {
		lines = append(lines, "Copied into the ticket: "+strings.Join(copies.copied, ", "))
	}
	for _, warning := range copies.warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, fmt.Sprintf("ok ticket=%s review", ticket.ID))
	return strings.Join(lines, "\n") + "\n", nil
}

// AskHumanInput is ask_human's input (RUN-8).
type AskHumanInput struct {
	Attribution
	Ticket  string   `json:"ticket,omitempty" jsonschema:"the ticket it is about, if any"`
	Kind    string   `json:"kind" jsonschema:"question, decision, review or blocked"`
	Text    string   `json:"text" jsonschema:"the question, self-contained, at most 1,000 characters"`
	Options []string `json:"options,omitempty" jsonschema:"answers to choose from, when there are clear choices"`
}

func (srv *server) askHuman(input AskHumanInput) (string, error) {
	c, err := srv.begin(input.Run)
	if err != nil {
		return "", err
	}
	run, err := c.requireRun()
	if err != nil {
		return "", err
	}
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
	ticketID := ""
	if strings.TrimSpace(input.Ticket) != "" {
		_, ticket, err := c.find(input.Ticket)
		if err != nil {
			return "", err
		}
		ticketID = ticket.ID
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
	return fmt.Sprintf("Asked the human (%s). Your session shows as Needs you on the board; the answer will arrive in a later prompt. Carry on with other work if you can.\nok question=%s\n", kind, id), nil
}

func questionID() string {
	return "q-" + rand.Text()[:16]
}

// settable lists the fields update_ticket sets (status changes with move).
var settable = slices.DeleteFunc(append(slices.Clone(board.ScalarFields), board.ListFields...), func(field string) bool { return field == "status" })

// demoteHeadings turns agent-written level-1 and level-2 headings outside
// code fences into level 3, so text placed inside a ticket section cannot
// start a section of its own (## Handoff, ## Notes) that later edits and
// recovery notes would read.
func demoteHeadings(text string) string {
	lines := strings.Split(text, "\n")
	fenced := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, prefix := range []string{"## ", "# "} {
			if strings.HasPrefix(line, prefix) {
				lines[index] = "### " + strings.TrimPrefix(line, prefix)
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}
