package mcpserver

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/store"
)

// markdownLink matches a link or image whose target is an absolute local
// path or a file:// URL, bare or in angle brackets (for paths with spaces):
// [text](/abs/path.png "title"), ![shot](</abs/my shot.png>).
var markdownLink = regexp.MustCompile(`(!?\[[^\]\n]*\]\()(?:<((?:file://)?/[^>\n]+)>|((?:file://)?/[^)\s]+))((?:\s+"[^"\n]*")?\))`)

// copier copies the local files a write refers to into a ticket (REV-5):
// agents and their tools clean up their own files, so the ticket keeps
// copies.
type copier struct {
	c        *call
	project  string
	ticket   string
	copied   []string
	warnings []string
}

func (c *call) copier(project, ticket string) *copier {
	return &copier{c: c, project: project, ticket: ticket}
}

// inRepository reports whether a path is inside the caller's checkout.
func (k *copier) inRepository(path string) (string, bool) {
	if k.c.where.Worktree == "" {
		return "", false
	}
	relative, err := filepath.Rel(k.c.where.Worktree, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

// copy stores one file and returns its ticket-relative path, or "" with a
// warning when it cannot be copied.
func (k *copier) copy(path, kind string) string {
	stored, err := k.c.srv.store.CopyIntoTicket(k.project, k.ticket, store.FileCopy{
		Source: path, Kind: kind, Run: k.c.by(), MaxBytes: k.c.settings.Attachments.MaxBytes,
	})
	if err != nil {
		reason := err.Error()
		var te *toolError
		if errors.As(err, &te) {
			reason = te.Message
		}
		k.warnings = append(k.warnings, fmt.Sprintf("%s was not copied (%s); it stays as text", path, reason))
		return ""
	}
	k.copied = append(k.copied, "files/"+stored.File)
	return "files/" + stored.File
}

// kindOf guesses an attachment kind from a file name.
func kindOf(path string) string {
	contentType, _ := board.AttachmentType(path)
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "screenshot"
	case strings.HasPrefix(contentType, "text/plain"):
		return "log"
	}
	return "other"
}

// markdown rewrites local links and images in text to copies in files/,
// line by line outside code fences, so a review can show example markdown.
// Links into the repository to files that are not attachment types (source
// code) are left alone; repository screenshots and logs are copied like any
// other evidence.
func (k *copier) markdown(text string) string {
	lines := strings.Split(text, "\n")
	fenced := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if !fenced {
			lines[index] = markdownLink.ReplaceAllStringFunc(line, k.link)
		}
	}
	return strings.Join(lines, "\n")
}

func (k *copier) link(match string) string {
	parts := markdownLink.FindStringSubmatch(match)
	path := strings.TrimPrefix(parts[2]+parts[3], "file://")
	if _, inside := k.inRepository(path); inside {
		if _, allowed := board.AttachmentType(path); !allowed {
			return match
		}
	}
	if stored := k.copy(path, kindOf(path)); stored != "" {
		return parts[1] + stored + parts[4]
	}
	return match
}

// file handles one entry of checkpoint's files[]: repository paths become
// repository-relative; other absolute paths are copied in.
func (k *copier) file(entry string) string {
	entry = strings.TrimSpace(entry)
	if !filepath.IsAbs(entry) {
		return entry
	}
	if relative, inside := k.inRepository(entry); inside {
		return relative
	}
	if stored := k.copy(entry, kindOf(entry)); stored != "" {
		return stored + " (copied from " + entry + ")"
	}
	return entry
}
