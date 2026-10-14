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

// WorkstreamTitleTakenError refuses a workstream title that another of the
// project's workstreams already has, compared by board.SameWorkstreamTitle
// (FH-163, D73). Slug and Title name that workstream as it is.
type WorkstreamTitleTakenError struct {
	Project, Slug, Title string
}

func (e *WorkstreamTitleTakenError) Error() string {
	return fmt.Sprintf("a workstream called %q is already in %s (%s); choose another title", e.Title, e.Project, e.Slug)
}

// Unwrap lets errors.Is match ErrInvalidInput.
func (e *WorkstreamTitleTakenError) Unwrap() error { return ErrInvalidInput }

// checkWorkstreamTitle refuses the title workstream slug has in t with a
// *WorkstreamTitleTakenError when another of the project's workstreams
// has the same one (board.SameWorkstreamTitle). A title is the file's H1,
// or its slug without one (board.ParseWorkstream). Every write that makes
// a workstream or may change its title calls it once the file is staged,
// before the transaction commits. A title the workstream already had
// before t, in any case or spacing, is not checked: workstreams that share
// one from before titles were unique, or from a hand edit, still take
// every other edit, and either can be retitled apart (D73). Callers hold
// the project lock.
func (s *Store) checkWorkstreamTitle(t *tx, project, slug string) error {
	dir := path.Join(project, "workstreams")
	name := path.Join(dir, slug+".md")
	data, err := t.read(name)
	if err != nil {
		return err
	}
	title := board.ParseWorkstream(slug, data).Title
	if before, err := s.ReadFile(name); err == nil && board.SameWorkstreamTitle(board.ParseWorkstream(slug, before).Title, title) {
		return nil
	}
	slugs, err := s.workstreamSlugs(project)
	if err != nil {
		return err
	}
	for _, other := range slugs {
		if other == slug {
			continue
		}
		data, err := t.read(path.Join(dir, other+".md"))
		if err != nil {
			return err
		}
		if taken := board.ParseWorkstream(other, data).Title; board.SameWorkstreamTitle(taken, title) {
			return &WorkstreamTitleTakenError{Project: project, Slug: other, Title: taken}
		}
	}
	return nil
}

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
	return s.setTicketWorkstream(project, id, pre, workstream, edit, nil)
}

// setTicketWorkstream is SetTicketWorkstream, with also (nil for none)
// staging more of the ticket's files in the same transaction.
func (s *Store) setTicketWorkstream(project, id string, pre Precondition, workstream string, edit Edit, also func(t *tx, ticket string) error) (string, error) {
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
	if also != nil {
		if err := also(t, name); err != nil {
			return "", err
		}
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
// tickets into it, all under the project lock. A title another of the
// project's workstreams has is a *WorkstreamTitleTakenError (D73).
// Nothing is written when any input is refused.
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
	t := s.begin()
	// A workstream edited by hand may already wait for this slug.
	if err := board.WorkstreamCycle(slug, input.DependsOnWorkstreams, s.dependencies(t, project)); err != nil {
		return CreatedWorkstream{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	// The new file, then the other lists, which give up the initial tickets
	// (the new file already lists them), then the tickets' fields
	// (board-format.md, Multi-file writes).
	t.put(path.Join(project, "workstreams", slug+".md"), data)
	if err := s.checkWorkstreamTitle(t, project, slug); err != nil {
		return CreatedWorkstream{}, err
	}
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

// dependencies gives each workstream's depends-on-workstreams as the
// transaction has them; a missing or broken file has none. Callers hold
// the project lock.
func (s *Store) dependencies(t *tx, project string) func(string) []string {
	return func(slug string) []string {
		if !board.ValidSlug(slug) {
			return nil
		}
		data, err := t.read(path.Join(project, "workstreams", slug+".md"))
		if err != nil {
			return nil
		}
		return mdfile.Parse(data).List("depends-on-workstreams")
	}
}

// WorkstreamEdit is one edit of a workstream (FH-46): its fields
// (board.NewWorkstreamChanges), the project's tickets that join it (Add,
// appended to its list in order, each leaving any other workstream) and
// its tickets that leave it (Remove, then in no workstream).
type WorkstreamEdit struct {
	Changes     board.WorkstreamChanges
	Add, Remove []string
}

// EditWorkstream applies edit to workstreams/<slug>.md and the tickets
// joining and leaving it in one write under the project lock, once pre
// holds for the workstream file: its fields, then the lists, then the
// tickets' workstream fields (board-format.md, Multi-file writes), so the
// field and the lists agree after. Every file is prepared before any is
// written: a workstream, ticket or dependency the project does not have
// (ErrNotFound; a *WorkstreamNotFoundError for a workstream), a removed
// ticket that is not a member, a ticket added and removed, or a
// dependency that leads back to the workstream (a *board.CycleError; both
// ErrInvalidInput), or a new title another workstream has (a
// *WorkstreamTitleTakenError, D73) writes nothing. The slug never changes. It returns
// the workstream file's hash after the call.
func (s *Store) EditWorkstream(project, slug string, pre Precondition, edit WorkstreamEdit) (string, error) {
	if err := s.checkProject(project); err != nil {
		return "", err
	}
	if !board.ValidSlug(slug) {
		return "", fmt.Errorf("workstream %q: %w", slug, ErrInvalidName)
	}
	moved := slices.Concat(edit.Add, edit.Remove)
	for index, id := range moved {
		if _, _, ok := board.ParseID(id); !ok {
			return "", fmt.Errorf("%w: %q is not a ticket id", ErrInvalidInput, id)
		}
		if slices.Contains(moved[:index], id) {
			return "", fmt.Errorf("%w: %s is listed twice", ErrInvalidInput, id)
		}
	}
	release, err := s.lock(project)
	if err != nil {
		return "", err
	}
	defer release()

	slugs, err := s.workstreamSlugs(project)
	if err != nil {
		return "", err
	}
	if !slices.Contains(slugs, slug) {
		return "", &WorkstreamNotFoundError{Project: project, Slug: slug, Available: slugs}
	}
	name := path.Join(project, "workstreams", slug+".md")
	t := s.begin()
	if dependencies, ok := edit.Changes.List("depends-on-workstreams"); ok {
		for _, dependency := range dependencies {
			if !slices.Contains(slugs, dependency) {
				return "", &WorkstreamNotFoundError{Project: project, Slug: dependency, Available: slugs}
			}
		}
		if err := board.WorkstreamCycle(slug, dependencies, s.dependencies(t, project)); err != nil {
			return "", fmt.Errorf("%w: %w", ErrInvalidInput, err)
		}
	}
	if _, err := t.edit(name, pre, edit.Changes.Apply, false); err != nil {
		return "", err
	}
	if err := s.checkWorkstreamTitle(t, project, slug); err != nil {
		return "", err
	}
	if err := s.checkMembers(t, project, slug, edit.Remove); err != nil {
		return "", err
	}
	if err := s.membership(t, project, edit.Add, slug); err != nil {
		return "", err
	}
	if err := s.membership(t, project, edit.Remove, ""); err != nil {
		return "", err
	}
	for _, id := range edit.Add {
		if _, err := s.joinTicket(t, project, id, Merge, slug, nil); err != nil {
			return "", err
		}
	}
	for _, id := range edit.Remove {
		if _, err := s.joinTicket(t, project, id, Merge, "", nil); err != nil {
			return "", err
		}
	}
	data, err := t.read(name)
	if err != nil {
		return "", err
	}
	return Hash(data), t.commit()
}

// checkMembers refuses ids that are not members of slug: neither in its
// list nor naming it in their workstream field. Callers hold the project
// lock.
func (s *Store) checkMembers(t *tx, project, slug string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	data, err := t.read(path.Join(project, "workstreams", slug+".md"))
	if err != nil {
		return err
	}
	listed := mdfile.Parse(data).List("tickets")
	for _, id := range ids {
		if slices.Contains(listed, id) {
			continue
		}
		name, err := s.TicketFile(project, id)
		if err != nil {
			return err
		}
		ticket, err := t.read(name)
		if err != nil {
			return err
		}
		if field, _ := mdfile.Parse(ticket).String("workstream"); field != slug {
			return fmt.Errorf("%w: %s is not in workstream %s", ErrInvalidInput, id, slug)
		}
	}
	return nil
}
