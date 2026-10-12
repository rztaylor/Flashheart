package mcpserver

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/scrub"
	"github.com/rztaylor/flashheart/internal/store"
)

// WriteReviewInput is write_review's input.
type WriteReviewInput struct {
	Attribution
	Ticket   string `json:"ticket" jsonschema:"the ticket id"`
	Markdown string `json:"markdown" jsonschema:"the whole review in the review template; link screenshots by absolute path"`
}

func (srv *server) writeReview(c *call, input WriteReviewInput) (string, error) {
	if strings.TrimSpace(input.Markdown) == "" {
		return "", fail("invalid_input", "write the review in the review template", "the review is empty")
	}
	if len(input.Markdown) > maxReview {
		return "", fail("too_large", "keep the review under 256 KiB", "the review is too long")
	}
	project, ticket, err := c.target(input.Ticket)
	if err != nil {
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

// AttachInput is attach's input (REV-1, REV-2).
type AttachInput struct {
	Attribution
	Ticket  string `json:"ticket" jsonschema:"the ticket id"`
	Path    string `json:"path" jsonschema:"absolute path of a local file to copy into the ticket"`
	Caption string `json:"caption" jsonschema:"what it shows, in a few words"`
	Kind    string `json:"kind,omitempty" jsonschema:"screenshot, log or other; default: from the file type"`
}

func (srv *server) attach(c *call, input AttachInput) (string, error) {
	path := strings.TrimSpace(input.Path)
	if !filepath.IsAbs(path) {
		return "", fail("invalid_input", "pass the file's absolute path", "%q is not an absolute path", path)
	}
	kind := strings.TrimSpace(input.Kind)
	if kind == "" {
		kind = kindOf(path)
	}
	if !slices.Contains(board.AttachmentKinds, kind) {
		return "", fail("invalid_input", "use kind screenshot, log or other", "%q is not a kind", kind)
	}
	project, ticket, err := c.target(input.Ticket)
	if err != nil {
		return "", err
	}
	caption := scrub.Text(input.Caption, 200)
	stored, err := srv.store.CopyIntoTicket(project.Name, ticket.ID, store.FileCopy{
		Source: path, Caption: caption, Kind: kind, Run: c.by(), MaxBytes: c.settings.Attachments.MaxBytes,
	})
	if err != nil {
		return "", err
	}
	if err := c.record(project.Name, events.Event{Kind: events.AttachmentAdded, Data: events.AttachmentData{Ticket: ticket.ID, File: stored.File, Kind: kind}}); err != nil {
		return "", err
	}
	name := "files/" + stored.File
	label := caption
	if label == "" {
		label = filepath.Base(path)
	}
	label = strings.NewReplacer(`[`, `\[`, `]`, `\]`).Replace(label)
	snippet := fmt.Sprintf("[%s](%s)", label, name)
	if contentType, _ := board.AttachmentType(path); strings.HasPrefix(contentType, "image/") {
		snippet = "!" + snippet
	}
	verb := "Attached"
	if stored.Reused {
		verb = "Already attached (identical copy)"
	}
	return fmt.Sprintf("%s %s to %s.\nUse in the review: %s\nok ticket=%s file=%s\n", verb, name, ticket.ID, snippet, ticket.ID, name), nil
}
