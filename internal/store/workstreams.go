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
	t := s.begin()
	// A workstream edited by hand may already wait for this slug.
	if err := board.WorkstreamCycle(slug, input.DependsOnWorkstreams, s.dependencies(t, project)); err != nil {
		return CreatedWorkstream{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	// The new file, then the other lists, which give up the initial tickets
	// (the new file already lists them), then the tickets' fields
	// (board-format.md, Multi-file writes).
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
// appended to its list in order, each leaving any other workstream), its
// tickets that leave it (Remove, then in no workstream) and a new slug
// (Rename, FH-158; "" or the current slug keeps it).
type WorkstreamEdit struct {
	Changes     board.WorkstreamChanges
	Add, Remove []string
	Rename      string
}

// EditedWorkstream reports an edited workstream: its slug (the new one
// when it was renamed), its file's hash after the edit, and the tickets,
// live and archived, whose files the rename rewrote.
type EditedWorkstream struct {
	Slug, Hash string
	Rewritten  []RewrittenTicket
}

// RewrittenTicket is a ticket a workstream's rename rewrote and the
// fields it changed: workstream, depends-on-workstreams or both.
type RewrittenTicket struct {
	ID     string
	Fields []string
}

// editWorkstream applies edit to workstreams/<slug>.md and the tickets
// joining and leaving it in one write under the project lock, once pre
// holds for the workstream file: its fields, then the lists, then the
// tickets' workstream fields (board-format.md, Multi-file writes), so the
// field and the lists agree after. A rename (edit.Rename) also moves the
// file to its new slug and rewrites every reference to the old one: each
// ticket's workstream field and every depends-on-workstreams, in the
// project's workstreams and its live and archived tickets. Every file is
// prepared before any is written: a workstream, ticket or dependency the
// project does not have (ErrNotFound; a *WorkstreamNotFoundError for a
// workstream), a removed ticket that is not a member, a ticket added and
// removed, a dependency that leads back to the workstream (a
// *board.CycleError; both ErrInvalidInput), an invalid new slug
// (ErrInvalidName) or one a file already has (ErrExists) writes nothing.
// EditWorkstream edits a workstream (editWorkstream), then, after a
// rename has committed and the project lock is released, renames it in
// the saved UI preferences too (renamePreferences, FH-170).
func (s *Store) EditWorkstream(project, slug string, pre Precondition, edit WorkstreamEdit) (EditedWorkstream, error) {
	edited, err := s.editWorkstream(project, slug, pre, edit)
	if err == nil && edited.Slug != slug {
		s.renamePreferences(project, slug, edited.Slug)
	}
	return edited, err
}

func (s *Store) editWorkstream(project, slug string, pre Precondition, edit WorkstreamEdit) (EditedWorkstream, error) {
	if err := s.checkProject(project); err != nil {
		return EditedWorkstream{}, err
	}
	if !board.ValidSlug(slug) {
		return EditedWorkstream{}, fmt.Errorf("workstream %q: %w", slug, ErrInvalidName)
	}
	rename := edit.Rename != "" && edit.Rename != slug
	if rename && !board.ValidSlug(edit.Rename) {
		return EditedWorkstream{}, fmt.Errorf("new slug %q: %w: use lower-case letters, digits, '.', '_' and '-', starting with a letter or digit, at most 100 characters", edit.Rename, ErrInvalidName)
	}
	moved := slices.Concat(edit.Add, edit.Remove)
	for index, id := range moved {
		if _, _, ok := board.ParseID(id); !ok {
			return EditedWorkstream{}, fmt.Errorf("%w: %q is not a ticket id", ErrInvalidInput, id)
		}
		if slices.Contains(moved[:index], id) {
			return EditedWorkstream{}, fmt.Errorf("%w: %s is listed twice", ErrInvalidInput, id)
		}
	}
	release, err := s.lock(project)
	if err != nil {
		return EditedWorkstream{}, err
	}
	defer release()

	slugs, err := s.workstreamSlugs(project)
	if err != nil {
		return EditedWorkstream{}, err
	}
	if !slices.Contains(slugs, slug) {
		return EditedWorkstream{}, &WorkstreamNotFoundError{Project: project, Slug: slug, Available: slugs}
	}
	if rename && (slices.Contains(slugs, edit.Rename) || s.Exists(path.Join(project, "workstreams", edit.Rename+".md"))) {
		return EditedWorkstream{}, fmt.Errorf("workstream %s in %s %w; choose another slug", edit.Rename, project, ErrExists)
	}
	name := path.Join(project, "workstreams", slug+".md")
	t := s.begin()
	if dependencies, ok := edit.Changes.List("depends-on-workstreams"); ok {
		for _, dependency := range dependencies {
			if !slices.Contains(slugs, dependency) {
				return EditedWorkstream{}, &WorkstreamNotFoundError{Project: project, Slug: dependency, Available: slugs}
			}
		}
		if err := board.WorkstreamCycle(slug, dependencies, s.dependencies(t, project)); err != nil {
			return EditedWorkstream{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
		}
	}
	if _, err := t.edit(name, pre, edit.Changes.Apply, false); err != nil {
		return EditedWorkstream{}, err
	}
	if err := s.checkMembers(t, project, slug, edit.Remove); err != nil {
		return EditedWorkstream{}, err
	}
	if err := s.membership(t, project, edit.Add, slug); err != nil {
		return EditedWorkstream{}, err
	}
	if err := s.membership(t, project, edit.Remove, ""); err != nil {
		return EditedWorkstream{}, err
	}
	for _, id := range edit.Add {
		if _, err := s.joinTicket(t, project, id, Merge, slug, nil); err != nil {
			return EditedWorkstream{}, err
		}
	}
	for _, id := range edit.Remove {
		if _, err := s.joinTicket(t, project, id, Merge, "", nil); err != nil {
			return EditedWorkstream{}, err
		}
	}
	out := EditedWorkstream{Slug: slug}
	if rename {
		if out.Rewritten, err = s.renameWorkstream(t, project, slug, edit.Rename); err != nil {
			return EditedWorkstream{}, err
		}
		out.Slug, name = edit.Rename, path.Join(project, "workstreams", edit.Rename+".md")
	}
	data, err := t.read(name)
	if err != nil {
		return EditedWorkstream{}, err
	}
	out.Hash = Hash(data)
	return out, t.commit()
}

// renameWorkstream stages workstream from's move to slug to, in its
// file's place among the staged files, then every reference to from
// rewritten: other workstreams' depends-on-workstreams, then each live
// and archived ticket's workstream and depends-on-workstreams; the old
// file is removed after every write (board-format.md, Multi-file writes).
// A file whose frontmatter does not parse is left alone, as its
// references cannot be read; the renamed workstream's own is refused. It
// returns the tickets it rewrote. Callers hold the project lock and have
// checked that to is free.
func (s *Store) renameWorkstream(t *tx, project, from, to string) ([]RewrittenTicket, error) {
	dir := path.Join(project, "workstreams")
	name := path.Join(dir, to+".md")
	if err := t.rename(path.Join(dir, from+".md"), name); err != nil {
		return nil, err
	}
	if _, err := t.edit(name, Merge, func(data []byte) ([]byte, error) { return mdfile.SetScalar(data, "slug", to) }, false); err != nil {
		return nil, fmt.Errorf("workstream %s: %w", from, err)
	}
	slugs, err := s.workstreamSlugs(project)
	if err != nil {
		return nil, err
	}
	for _, slug := range slugs {
		if slug == from {
			continue
		}
		if _, err := t.edit(path.Join(dir, slug+".md"), Merge, renameReferences(from, to), false); err != nil {
			return nil, err
		}
	}
	var rewritten []RewrittenTicket
	for _, tickets := range ticketDirs {
		files, err := s.ticketFiles(path.Join(project, tickets))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			before, err := t.read(file)
			if err != nil {
				return nil, err
			}
			after, err := t.edit(file, Merge, renameReferences(from, to), true)
			if err != nil {
				return nil, err
			}
			if after == Hash(before) {
				continue
			}
			id, _, _ := board.ParseFolder(path.Base(path.Dir(file)))
			ticket := RewrittenTicket{ID: id}
			doc := mdfile.Parse(before)
			if field, _ := doc.String("workstream"); field == from {
				ticket.Fields = append(ticket.Fields, "workstream")
			}
			if slices.Contains(doc.List("depends-on-workstreams"), from) {
				ticket.Fields = append(ticket.Fields, "depends-on-workstreams")
			}
			rewritten = append(rewritten, ticket)
		}
	}
	return rewritten, nil
}

// renameReferences rewrites a ticket's or workstream's references to
// workstream from as to: its workstream field and its
// depends-on-workstreams. A file whose frontmatter does not parse, or
// that names from in neither, is left as it is.
func renameReferences(from, to string) Edit {
	return func(data []byte) ([]byte, error) {
		doc := mdfile.Parse(data)
		if doc.FrontmatterError != nil {
			return data, nil
		}
		var err error
		if field, ok := doc.String("workstream"); ok && field == from {
			if data, err = mdfile.SetScalar(data, "workstream", to); err != nil {
				return nil, err
			}
		}
		dependencies := doc.List("depends-on-workstreams")
		if slices.Contains(dependencies, from) {
			next := make([]string, 0, len(dependencies))
			for _, dependency := range dependencies {
				if dependency == from {
					dependency = to
				}
				if !slices.Contains(next, dependency) {
					next = append(next, dependency)
				}
			}
			if data, err = mdfile.SetList(data, "depends-on-workstreams", next); err != nil {
				return nil, err
			}
		}
		return data, nil
	}
}

// ticketFiles lists the ticket files under dir (a project's tickets or
// archived tickets), one per ticket folder, in folder order; a folder
// without one, or that is not a ticket folder, is left out. A missing
// dir has none.
func (s *Store) ticketFiles(dir string) ([]string, error) {
	entries, err := s.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, entry := range entries {
		folder := entry.Name()
		if _, _, ok := board.ParseFolder(folder); !ok || !entry.IsDir() {
			continue
		}
		files, err := s.ReadDir(path.Join(dir, folder))
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(files))
		for _, file := range files {
			names = append(names, file.Name())
		}
		if name, _, ok := board.TicketFile(folder, names); ok {
			out = append(out, path.Join(dir, folder, name))
		}
	}
	return out, nil
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
