package store

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Workstream membership (board-format §Blocking): a workstream's tickets:
// list defines membership and order, and a ticket's workstream: field names
// the one workstream that lists it. Every write here changes both together
// under the project lock, preparing every file before writing any, so a
// refused write leaves all of them as they were.

// WorkstreamNotFoundError names a workstream the project does not have and
// the ones it does, so the caller can choose.
type WorkstreamNotFoundError struct {
	Project, Slug string
	Available     []string
}

func (e *WorkstreamNotFoundError) Error() string {
	available := "none"
	if len(e.Available) > 0 {
		available = strings.Join(e.Available, ", ")
	}
	return fmt.Sprintf("workstream %q is not in %s; its workstreams: %s", e.Slug, e.Project, available)
}

// Unwrap lets errors.Is match ErrNotFound.
func (e *WorkstreamNotFoundError) Unwrap() error { return ErrNotFound }

// workstreamSlugs lists the project's workstream slugs in sorted order.
// Callers hold the project lock.
func (s *Store) workstreamSlugs(project string) ([]string, error) {
	entries, err := s.ReadDir(path.Join(project, "workstreams"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var slugs []string
	for _, entry := range entries {
		name := entry.Name()
		slug, ok := strings.CutSuffix(name, ".md")
		if !ok || entry.IsDir() || strings.HasPrefix(name, ".") || !ValidName(slug) {
			continue
		}
		slugs = append(slugs, slug)
	}
	slices.Sort(slugs)
	return slugs, nil
}

// membership stages the workstream files so that ids are listed by target
// alone ("" for none): appended to its list in order when missing, removed
// from every other list. A broken workstream file other than the target is
// left alone, since its list cannot be read. Callers hold the project lock.
func (s *Store) membership(t *tx, project string, ids []string, target string) error {
	slugs, err := s.workstreamSlugs(project)
	if err != nil {
		return err
	}
	if target != "" && !slices.Contains(slugs, target) {
		return &WorkstreamNotFoundError{Project: project, Slug: target, Available: slugs}
	}
	for _, slug := range slugs {
		name := path.Join(project, "workstreams", slug+".md")
		data, err := t.read(name)
		if err != nil {
			return err
		}
		doc := mdfile.Parse(data)
		if doc.FrontmatterError != nil {
			if slug == target {
				return fmt.Errorf("workstream %s: %w", slug, doc.FrontmatterError)
			}
			continue
		}
		current := doc.List("tickets")
		next := slices.DeleteFunc(slices.Clone(current), func(id string) bool { return slug != target && slices.Contains(ids, id) })
		if slug == target {
			for _, id := range ids {
				if !slices.Contains(next, id) {
					next = append(next, id)
				}
			}
		}
		if slices.Equal(current, next) {
			continue
		}
		if data, err = mdfile.SetList(data, "tickets", next); err != nil {
			return fmt.Errorf("workstream %s: %w", slug, err)
		}
		t.put(name, data)
	}
	return nil
}

// joinTicket stages edit (which may be nil) to a ticket, once pre holds,
// with its workstream field set. Callers hold the project lock.
func (s *Store) joinTicket(t *tx, project, id string, pre Precondition, workstream string, edit Edit) (string, error) {
	name, err := s.TicketFile(project, id)
	if err != nil {
		return "", err
	}
	return t.edit(name, pre, func(current []byte) ([]byte, error) {
		next := current
		if edit != nil {
			if next, err = edit(current); err != nil {
				return nil, err
			}
		}
		return mdfile.SetScalar(next, "workstream", workstream)
	}, true)
}

// SetTicketWorkstream applies edit (nil for none) to a ticket and makes
// workstream ("" for none) the one workstream that lists it: the ticket's
// field and every workstream's tickets list change together under the
// project lock (board-format §Blocking), once pre holds for the ticket. A
// workstream the project does not have is a *WorkstreamNotFoundError and
// nothing is written. It returns the ticket's hash after the call.
func (s *Store) SetTicketWorkstream(project, id string, pre Precondition, workstream string, edit Edit) (string, error) {
	if err := s.checkTicket(project, id); err != nil {
		return "", err
	}
	if workstream != "" && !board.ValidSlug(workstream) {
		return "", fmt.Errorf("workstream %q: %w", workstream, ErrInvalidName)
	}
	release, err := s.lock(project)
	if err != nil {
		return "", err
	}
	defer release()
	// The lists, then the ticket (board-format.md, Multi-file writes); the
	// ticket's precondition is checked before anything is staged.
	name, err := s.TicketFile(project, id)
	if err != nil {
		return "", err
	}
	t := s.begin()
	if _, err := t.require(name, pre); err != nil {
		return "", err
	}
	if err := s.membership(t, project, []string{id}, workstream); err != nil {
		return "", err
	}
	hash, err := s.joinTicket(t, project, id, pre, workstream, edit)
	if err != nil {
		return "", err
	}
	return hash, t.commit()
}

// NewWorkstream is the content of a workstream created from the template.
type NewWorkstream = board.NewWorkstream

// CreatedWorkstream reports a new workstream.
type CreatedWorkstream struct {
	Slug string
	Hash string
}

// CreateWorkstream writes workstreams/<slug>.md from the board-format
// template with a slug made unique from the title, and moves its initial
// tickets into it, all under the project lock. Nothing is written when any
// input is refused.
func (s *Store) CreateWorkstream(project string, input NewWorkstream) (CreatedWorkstream, error) {
	if err := s.checkProject(project); err != nil {
		return CreatedWorkstream{}, err
	}
	if err := input.Normalize(); err != nil {
		return CreatedWorkstream{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	release, err := s.lock(project)
	if err != nil {
		return CreatedWorkstream{}, err
	}
	defer release()

	slugs, err := s.workstreamSlugs(project)
	if err != nil {
		return CreatedWorkstream{}, err
	}
	for _, dependency := range input.DependsOnWorkstreams {
		if !slices.Contains(slugs, dependency) {
			return CreatedWorkstream{}, &WorkstreamNotFoundError{Project: project, Slug: dependency, Available: slugs}
		}
	}
	base := slugify(input.Title, "workstream")
	slug := base
	for n := 2; slices.Contains(slugs, slug) || s.Exists(path.Join(project, "workstreams", slug+".md")); n++ {
		slug = base + "-" + strconv.Itoa(n)
	}
	data, err := board.RenderWorkstream(slug, input, s.now())
	if err != nil {
		return CreatedWorkstream{}, err
	}
	// The new file, then the other lists, which give up the initial tickets
	// (the new file already lists them), then the tickets' fields
	// (board-format.md, Multi-file writes).
	t := s.begin()
	t.put(path.Join(project, "workstreams", slug+".md"), data)
	if err := s.membership(t, project, input.Tickets, ""); err != nil {
		return CreatedWorkstream{}, err
	}
	for _, id := range input.Tickets {
		if _, err := s.joinTicket(t, project, id, Merge, slug, nil); err != nil {
			return CreatedWorkstream{}, err
		}
	}
	if err := t.commit(); err != nil {
		return CreatedWorkstream{}, err
	}
	return CreatedWorkstream{Slug: slug, Hash: Hash(data)}, nil
}
