package references

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

// maxCaption bounds a copy's caption, taken from the link text.
const maxCaption = 200

// Copier remembers what it has seen of the board between passes.
type Copier struct {
	files    *store.Store
	roots    []string
	maxBytes int64
	log      io.Writer
	started  bool
	// seen maps a ticket id (or id + "/review") to the modification time
	// last seen.
	seen map[string]time.Time
	// failed holds the links already logged as not copied.
	failed map[string]bool
}

// New returns a copier for the board at root, copying files of at most
// maxBytes and logging links it cannot copy to log.
func New(files *store.Store, root string, maxBytes int64, log io.Writer) *Copier {
	roots := []string{filepath.Clean(root)}
	if real, err := filepath.EvalSymlinks(root); err == nil && real != roots[0] {
		roots = append(roots, real)
	}
	return &Copier{files: files, roots: roots, maxBytes: maxBytes, log: log, seen: map[string]time.Time{}, failed: map[string]bool{}}
}

// Run makes a pass for every new revision until ctx ends.
func (c *Copier) Run(ctx context.Context, b *index.Index) {
	var revision uint64
	for ctx.Err() == nil {
		if snapshot, err := b.Current(); err == nil && snapshot != nil {
			c.Pass(snapshot)
			revision = snapshot.Revision
		}
		b.Wait(ctx, revision)
	}
}

// Pass handles every ticket and review changed since the last pass; the
// first pass only records what it sees.
func (c *Copier) Pass(snapshot *index.Snapshot) {
	first := !c.started
	c.started = true
	for _, project := range snapshot.Board.Projects {
		for _, ticket := range project.Tickets {
			if ticket.ID == "" {
				continue
			}
			if c.changed(ticket.ID, ticket.Modified) && !first {
				c.ticket(project.Name, ticket.ID)
			}
			if modified, ok := project.ReviewModified[ticket.ID]; ok && c.changed(ticket.ID+"/review", modified) && !first {
				c.review(project.Name, ticket.ID, ticket.Folder)
			}
		}
	}
}

func (c *Copier) changed(key string, modified time.Time) bool {
	last, seen := c.seen[key]
	c.seen[key] = modified
	return !seen || !last.Equal(modified)
}

func (c *Copier) ticket(project, id string) {
	name, err := c.files.TicketFile(project, id)
	if err != nil {
		return
	}
	data, hash, err := c.files.ReadTicket(project, id)
	if err != nil || c.files.WroteLast(name, data) {
		return
	}
	if next := c.rewrite(project, id, string(data)); next != string(data) {
		_, err := c.files.UpdateTicket(project, id, hash, func([]byte) ([]byte, error) { return []byte(next), nil })
		c.saved(id, err)
	}
}

func (c *Copier) review(project, id, folder string) {
	text, found, err := c.files.ReadReview(project, folder)
	if err != nil || !found || c.files.WroteLast(path.Join(project, "tickets", folder, "review.md"), []byte(text)) {
		return
	}
	if next := c.rewrite(project, id, text); next != text {
		_, err := c.files.UpdateReview(project, id, store.Hash([]byte(text)), func([]byte) ([]byte, error) { return []byte(next), nil })
		c.saved(id, err)
	}
}

// saved logs a failed write; a conflict means the file changed again, and
// the next pass sees that change.
func (c *Copier) saved(id string, err error) {
	var conflict *store.ConflictError
	if err != nil && !errors.As(err, &conflict) {
		fmt.Fprintf(c.log, "flashheart: point %s's links at their copies: %v\n", id, err)
	}
}

// rewrite copies the files text links to and returns it with the links
// pointing at the copies.
func (c *Copier) rewrite(project, id, text string) string {
	copies := map[string]string{}
	return board.RewriteLocalLinks(text, func(path, caption string) string {
		if stored, ok := copies[path]; ok {
			return stored
		}
		if _, allowed := board.AttachmentType(path); !allowed || c.inside(path) {
			return ""
		}
		stored, err := c.files.CopyIntoTicket(project, id, store.FileCopy{
			Source: path, Caption: board.OneLine(caption, maxCaption), Kind: board.AttachmentKind(path),
			Run: "serve", MaxBytes: c.maxBytes,
		})
		if err != nil {
			if key := id + "\x00" + path; !c.failed[key] {
				c.failed[key] = true
				fmt.Fprintf(c.log, "flashheart: %s links to %s, which was not copied (%v); the link stays as it is\n", id, path, err)
			}
			return ""
		}
		copies[path] = "files/" + stored.File
		return copies[path]
	})
}

// inside reports whether path is in the board root, which needs no copy.
func (c *Copier) inside(path string) bool {
	candidates := []string{filepath.Clean(path)}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		candidates = append(candidates, real)
	}
	for _, root := range c.roots {
		for _, candidate := range candidates {
			if relative, err := filepath.Rel(root, candidate); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}
