package mcpserver

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/scrub"
	"github.com/rztaylor/flashheart/internal/store"
)

// CheckpointInput is checkpoint's input (RUN-7).
type CheckpointInput struct {
	Attribution
	Ticket        string   `json:"ticket" jsonschema:"the ticket id"`
	Done          []string `json:"done,omitempty" jsonschema:"what is finished, in plain words"`
	Next          []string `json:"next,omitempty" jsonschema:"what to do next and why, a sentence or two each in plain words"`
	Files         []string `json:"files,omitempty" jsonschema:"files that matter, repository-relative; screenshots and logs outside the repository by absolute path are copied in"`
	OpenQuestions []string `json:"open_questions,omitempty" jsonschema:"in plain words"`
	Note          string   `json:"note,omitempty" jsonschema:"one paragraph appended to the ticket's notes"`
}

func (srv *server) checkpoint(c *call, input CheckpointInput) (string, error) {
	project, ticket, err := c.target(input.Ticket)
	if err != nil {
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
	section := board.RenderHandoff(board.HandoffNote{Updated: c.now, By: protocol.ShortRun(c.by()), Branch: branch, Done: done, Next: next, Files: files, Questions: questions})
	same := ticket.Handoff != nil && slices.Equal(ticket.Handoff.Done, done) && slices.Equal(ticket.Handoff.Next, next) &&
		slices.Equal(ticket.Handoff.Files, files) && slices.Equal(ticket.Handoff.Questions, questions)
	if !same || note != "" {
		_, err = c.files().UpdateTicket(project.Name, ticket.ID, store.Merge, func(data []byte) ([]byte, error) {
			if !same {
				var err error
				if data, err = mdfile.ReplaceSection(data, board.SectionHandoff, section); err != nil {
					return nil, err
				}
			}
			if note != "" {
				return mdfile.AppendToSection(data, board.SectionNotes, note)
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
	lines = append(lines, handoffAdvice(input.Done, input.Next, input.OpenQuestions)...)
	if len(copies.copied) > 0 {
		lines = append(lines, "Copied into the ticket: "+strings.Join(copies.copied, ", "))
	}
	for _, warning := range copies.warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, fmt.Sprintf("ok ticket=%s checkpoint by=%s", ticket.ID, protocol.ShortRun(c.by())))
	return strings.Join(lines, "\n") + "\n", nil
}

// glanceItem is the length past which a handoff item is no longer a
// sentence or two (advice only).
const glanceItem = 280

// handoffAdvice warns about each handoff item that reads like code (three
// or more names from the code) or runs long, so the agent can rewrite it
// for the person reviewing the ticket (FH-187); the handoff is saved
// either way.
func handoffAdvice(done, next, questions []string) []string {
	var warnings []string
	for _, list := range []struct {
		label string
		items []string
	}{{"Done item", items(done)}, {"Next item", items(next)}, {"Open question", items(questions)}} {
		for index, item := range list.items {
			name := fmt.Sprintf("%s %d", list.label, index+1)
			if words := codeWords(item); len(words) >= 3 {
				var quoted []string
				for _, word := range words {
					if word = fmt.Sprintf("%q", word); len(quoted) < 3 && !slices.Contains(quoted, word) {
						quoted = append(quoted, word)
					}
				}
				warnings = append(warnings, fmt.Sprintf("warning: %s reads like code (%s): write it as a sentence or two in plain words for the person reviewing the ticket, and put names from the code and exact rules in the ticket's Context (update_ticket sections).", name, strings.Join(quoted, ", ")))
			} else if length := utf8.RuneCountInString(item); length > glanceItem {
				warnings = append(warnings, fmt.Sprintf("warning: %s is %d characters: keep each item to a sentence or two (%d characters or fewer) in plain words for the person reviewing the ticket, and put the detail in the ticket's Context (update_ticket sections).", name, length, glanceItem))
			}
		}
	}
	return warnings
}
