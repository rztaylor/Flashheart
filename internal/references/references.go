package references

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/gitinfo"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

// maxCaption bounds a copy's caption, taken from the link text.
const maxCaption = 200

// gitTimeout bounds asking git whether it ignores a file.
const gitTimeout = 5 * time.Second

var (
	errNotInRepository = errors.New("it is not in a checkout of the project's repository")
	errIgnored         = errors.New("git ignores it, or could not say")
)

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
				c.ticket(project, ticket.ID)
			}
			if modified, ok := project.ReviewModified[ticket.ID]; ok && c.changed(ticket.ID+"/review", modified) && !first {
				c.review(project, ticket.ID, ticket.Folder)
			}
		}
	}
}

func (c *Copier) changed(key string, modified time.Time) bool {
	last, seen := c.seen[key]
	c.seen[key] = modified
	return !seen || !last.Equal(modified)
}

func (c *Copier) ticket(project board.Project, id string) {
	name, err := c.files.TicketFile(project.Name, id)
	if err != nil {
		return
	}
	data, hash, err := c.files.ReadTicket(project.Name, id)
	if err != nil || c.files.WroteLast(name, data) {
		return
	}
	if next := c.rewrite(project, id, string(data)); next != string(data) {
		_, err := c.files.UpdateTicket(project.Name, id, hash, func([]byte) ([]byte, error) { return []byte(next), nil })
		c.saved(id, err)
	}
}

func (c *Copier) review(project board.Project, id, folder string) {
	text, found, err := c.files.ReadReview(project.Name, folder)
	if err != nil || !found || c.files.WroteLast(path.Join(project.Name, "tickets", folder, "review.md"), []byte(text)) {
		return
	}
	if next := c.rewrite(project, id, text); next != text {
		_, err := c.files.UpdateReview(project.Name, id, store.Hash([]byte(text)), func([]byte) ([]byte, error) { return []byte(next), nil })
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
func (c *Copier) rewrite(project board.Project, id, text string) string {
	copies := map[string]string{}
	return board.RewriteLocalLinks(text, func(path, caption string) string {
		if stored, ok := copies[path]; ok {
			return stored
		}
		if _, allowed := board.AttachmentType(path); !allowed || c.inside(path) {
			return ""
		}
		var stored store.StoredFile
		source, err := c.permitted(project, path)
		if err == nil {
			stored, err = c.files.CopyIntoTicket(project.Name, id, store.FileCopy{
				Source: source, Caption: board.OneLine(caption, maxCaption), Kind: board.AttachmentKind(path),
				Run: "serve", MaxBytes: c.maxBytes,
			})
		}
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

// permitted returns the real path to copy for a link in one of project's
// tickets, or why it may not be copied. A synced edit is indistinguishable from the user's own,
// so only a file in a checkout of a repository the project records, whose
// name maps to the project (PRJ-2, PRJ-3), and that git does not ignore,
// is copied. Checking the checkout on disk means a forged repos list in a
// synced project.yaml cannot reach anything else on this machine.
func (c *Copier) permitted(project board.Project, link string) (string, error) {
	real, err := filepath.EvalSymlinks(link)
	if err != nil {
		return "", err
	}
	info := gitinfo.Resolve(filepath.Dir(real))
	relative, err := filepath.Rel(info.Worktree, real)
	if info.Repo == "" || err != nil || !slices.Contains(project.Repos, info.Repo) || inGitDir(relative) {
		return "", errNotInRepository
	}
	if name, err := c.files.FindProject(info.Project, info.Repo); err != nil || name != project.Name {
		return "", errNotInRepository
	}
	if ignored(info.Worktree, real) {
		return "", errIgnored
	}
	return real, nil
}

// inGitDir reports whether a worktree-relative path is inside .git, in any
// case, since macOS file systems usually ignore case.
func inGitDir(relative string) bool {
	return slices.ContainsFunc(strings.Split(relative, string(filepath.Separator)), func(part string) bool {
		return strings.EqualFold(part, ".git")
	})
}

// ignored asks git whether it ignores path. Anything but a clear "not
// ignored", including git being missing or failing, counts as ignored.
func ignored(worktree, path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	err := exec.CommandContext(ctx, "git", "-C", worktree, "check-ignore", "-q", "--", path).Run()
	var exit *exec.ExitError
	return !errors.As(err, &exit) || exit.ExitCode() != 1
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
