package mcpserver

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/scrub"
	"github.com/rztaylor/flashheart/internal/store"
)

// UpdateTicketInput is update_ticket's input.
type UpdateTicketInput struct {
	Attribution
	Ticket      string         `json:"ticket" jsonschema:"the ticket id"`
	Set         map[string]any `json:"set,omitempty" jsonschema:"fields to set: title, type, priority, created, branch, workstream (strings); tags, depends-on, depends-on-workstreams (lists of strings)"`
	Check       []string       `json:"check,omitempty" jsonschema:"acceptance criteria to tick, by number (1 is the first) or by their text"`
	AppendNotes string         `json:"append_notes,omitempty" jsonschema:"a paragraph appended to the ticket's notes"`
}

func (srv *server) updateTicket(c *call, input UpdateTicketInput) (string, error) {
	project, ticket, err := c.target(input.Ticket)
	if err != nil {
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
			if data, err = mdfile.SetCheckbox(data, board.SectionCriteria, index, true); err != nil {
				return nil, err
			}
		}
		if note != "" {
			return mdfile.AppendToSection(data, board.SectionNotes, note)
		}
		return data, nil
	}
	if workstream >= 0 {
		_, err = srv.store.SetTicketWorkstream(project.Name, ticket.ID, store.Unchanged(base), changes[workstream].value, edit)
	} else {
		_, err = srv.store.UpdateTicket(project.Name, ticket.ID, store.Unchanged(base), edit)
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
	text := ""
	for _, ch := range changes {
		if ch.key != "title" {
			continue
		}
		if advice := titleAdvice(ch.value); advice != "" {
			text = "warning: " + advice + "\n"
		}
	}
	return text + fmt.Sprintf("ok ticket=%s changed=%s\n", ticket.ID, strings.Join(fields, ",")), nil
}

// settable lists the fields update_ticket sets (status changes with move).
var settable = slices.DeleteFunc(append(slices.Clone(board.ScalarFields), board.ListFields...), func(field string) bool { return field == "status" })

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

func (srv *server) move(c *call, input MoveInput) (string, error) {
	to, rejected, ok := board.ParseStatus(strings.TrimSpace(input.To))
	if !ok {
		return "", fail("invalid_input", "use backlog, up-next, in-progress or review", "%q is not a column", input.To)
	}
	if rejected {
		return "", fail("invalid_input", "ask the human to reject it on the board", "agents do not reject tickets")
	}
	if to == board.Done {
		return "", fail("invalid_input", "move it to review; the human moves it to done after verifying", "agents do not move tickets to done")
	}
	project, ticket, err := c.target(input.Ticket)
	if err != nil {
		return "", err
	}
	if to == board.InProgress && ticket.Column != board.InProgress {
		if reasons := c.blocked(project, ticket); len(reasons) > 0 {
			return "", fail("blocked", "claim it with force and a reason to start it anyway", "%s is blocked: %s", ticket.ID, strings.Join(reasons, "; "))
		}
	}
	if _, err := srv.store.UpdateTicket(project.Name, ticket.ID, store.Merge, func(data []byte) ([]byte, error) {
		return mdfile.SetScalar(data, "status", string(to))
	}); err != nil {
		return "", err
	}
	var warnings []string
	if to == board.Review && ticket.Column != board.Review {
		review := ""
		if project.Reviews[ticket.ID] {
			review, _, _ = srv.store.ReadReview(project.Name, ticket.Folder)
		}
		warnings = board.ReviewWarnings(ticket, review, project.Reviews[ticket.ID])
	}
	if ticket.Column != to {
		if err := c.record(project.Name, events.Event{Kind: events.TicketMoved, Data: events.TicketData{Ticket: ticket.ID, From: ticket.Status(), To: string(to), By: c.by()}}); err != nil {
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

// CreateTicketInput is create_ticket's input.
type CreateTicketInput struct {
	Attribution
	Type        string   `json:"type" jsonschema:"feature, bug, test, refactor, infra, docs or spike"`
	Title       string   `json:"title" jsonschema:"what a person would notice or gain, in plain words and under 80 characters; no file, function or test names"`
	Description string   `json:"description" jsonschema:"for someone who has not read the code: the problem, why it matters and the proposed change, in plain words"`
	Criteria    []string `json:"criteria" jsonschema:"testable acceptance criteria"`
	Priority    string   `json:"priority" jsonschema:"high, medium or low"`
	Status      string   `json:"status,omitempty" jsonschema:"backlog (default) or up-next"`
	Workstream  string   `json:"workstream,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty" jsonschema:"ids of tickets this one needs first"`
	Tags        []string `json:"tags,omitempty"`
	PlanOrRepro string   `json:"plan_or_repro,omitempty" jsonschema:"the Test Plan (features) or Reproduction (bugs)"`
	Context     string   `json:"context,omitempty" jsonschema:"implementation detail for whoever builds it (files, functions, design notes), saved under ## Context below the description"`
	ProjectKey  string   `json:"project_key,omitempty" jsonschema:"only for a project with no key yet: the key to record first"`
}

func (srv *server) createTicket(c *call, input CreateTicketInput) (string, error) {
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
	if len(input.Description) > board.MaxSectionText || len(input.PlanOrRepro) > board.MaxSectionText || len(input.Context) > board.MaxSectionText {
		return "", fail("too_large", fmt.Sprintf("keep the description, plan and context under %d KiB each; link to longer documents", board.MaxSectionText>>10), "the text is too long")
	}
	for _, id := range input.DependsOn {
		if _, _, ok := board.ParseID(id); !ok {
			return "", fail("invalid_input", "pass ticket ids like FH-42", "depends_on %q is not a ticket id", id)
		}
	}
	created, err := srv.store.CreateTicket(project.Name, store.NewTicket{
		Title: scrub.Text(input.Title, 300), Type: input.Type, Priority: input.Priority, Status: status,
		Workstream: strings.TrimSpace(input.Workstream), Description: scrub.Secrets(input.Description),
		Criteria: items(input.Criteria), DependsOn: input.DependsOn, Tags: items(input.Tags),
		Session: c.by(), PlanOrRepro: scrub.Secrets(input.PlanOrRepro), Context: scrub.Secrets(input.Context), Key: key,
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
	if advice := titleAdvice(input.Title); advice != "" {
		text += "\nwarning: " + advice
	}
	return text + fmt.Sprintf("\nok ticket=%s\n", created.ID), nil
}

// maxTitle is the length past which a title no longer reads at a glance on
// a card.
const maxTitle = 80

// codeLike matches a word of a title that is a name from the code rather
// than words a person uses: a path or file name, snake_case, a call,
// package.Name, lowerCamel (but not iPhone or macOS) or UpperCamel of
// three or more humps (but not GitHub or JavaScript).
var codeLike = regexp.MustCompile("`|/.*\\.[A-Za-z]+$|^[a-z][\\w-]+\\.[a-z]{1,4}$|[A-Za-z0-9]_[A-Za-z0-9]|\\(\\)|^[a-z]+\\.[A-Z]|^[a-z]{2,}[A-Z][a-z]|^(?:[A-Z][a-z0-9]+){3,}")

// titleAdvice says how to improve a title that reads like code or runs
// long (FH-113); the title is saved either way. "" when it reads well.
func titleAdvice(title string) string {
	title = strings.Join(strings.Fields(title), " ")
	for _, word := range strings.Fields(title) {
		if word = strings.Trim(word, `"',;:!?.“”‘’`); codeLike.MatchString(word) {
			return fmt.Sprintf("The title reads like code (%q): name what a person would notice or gain, and keep names from the code for the description or Context.", word)
		}
	}
	if length := utf8.RuneCountInString(title); length > maxTitle {
		return fmt.Sprintf("The title is %d characters: keep it to %d or fewer so it reads at a glance on a card.", length, maxTitle)
	}
	return ""
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

func (srv *server) setProjectKey(c *call, input SetProjectKeyInput) (string, error) {
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
