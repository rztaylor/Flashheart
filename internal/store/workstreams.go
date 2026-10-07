package store

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

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

// pending is one file's prepared content, written only when it changed.
type pending struct {
	name    string
	data    []byte
	changed bool
}

func writeAll(s *Store, files []pending) error {
	for _, file := range files {
		if file.changed {
			if err := s.WriteFileAtomic(file.name, file.data); err != nil {
				return err
			}
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

// membership prepares the workstream files so that ids are listed by target
// alone ("" for none): appended to its list in order when missing, removed
// from every other list. A broken workstream file other than the target is
// left alone, since its list cannot be read. Callers hold the project lock.
func (s *Store) membership(project string, ids []string, target string) ([]pending, error) {
	slugs, err := s.workstreamSlugs(project)
	if err != nil {
		return nil, err
	}
	if target != "" && !slices.Contains(slugs, target) {
		return nil, &WorkstreamNotFoundError{Project: project, Slug: target, Available: slugs}
	}
	var files []pending
	for _, slug := range slugs {
		name := path.Join(project, "workstreams", slug+".md")
		data, err := s.ReadFile(name)
		if err != nil {
			return nil, err
		}
		doc := mdfile.Parse(data)
		if doc.FrontmatterError != nil {
			if slug == target {
				return nil, fmt.Errorf("workstream %s: %w", slug, doc.FrontmatterError)
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
			return nil, fmt.Errorf("workstream %s: %w", slug, err)
		}
		files = append(files, pending{name: name, data: data, changed: true})
	}
	return files, nil
}

// prepareTicket applies edit (which may be nil) to a ticket and sets its
// workstream field, checking base like updateLocked. Callers hold the
// project lock.
func (s *Store) prepareTicket(project, id, base, workstream string, edit Edit) (pending, error) {
	name, err := s.TicketFile(project, id)
	if err != nil {
		return pending{}, err
	}
	current, err := s.ReadFile(name)
	if err != nil {
		return pending{}, err
	}
	if hash := Hash(current); base != "" && base != hash {
		return pending{}, &ConflictError{Name: name, Hash: hash, Current: current}
	}
	next := current
	if edit != nil {
		if next, err = edit(current); err != nil {
			return pending{}, err
		}
	}
	if next, err = mdfile.SetScalar(next, "workstream", workstream); err != nil {
		return pending{}, err
	}
	file := pending{name: name, data: next, changed: string(next) != string(current)}
	if file.changed {
		if file.data, err = mdfile.SetScalar(next, "updated", s.now().Format(time.RFC3339)); err != nil {
			return pending{}, err
		}
	}
	return file, nil
}

// SetTicketWorkstream applies edit (nil for none) to a ticket and makes
// workstream ("" for none) the one workstream that lists it: the ticket's
// field and every workstream's tickets list change together under the
// project lock (board-format §Blocking). A non-empty base must match the
// ticket's hash. A workstream the project does not have is a
// *WorkstreamNotFoundError and nothing is written. It returns the ticket's
// hash after the call.
func (s *Store) SetTicketWorkstream(project, id, base, workstream string, edit Edit) (string, error) {
	if err := s.checkTicket(project, id); err != nil {
		return "", err
	}
	if workstream != "" && !ValidName(workstream) {
		return "", fmt.Errorf("workstream %q: %w", workstream, ErrInvalidName)
	}
	release, err := s.lock(project)
	if err != nil {
		return "", err
	}
	defer release()
	ticket, err := s.prepareTicket(project, id, base, workstream, edit)
	if err != nil {
		return "", err
	}
	files, err := s.membership(project, []string{id}, workstream)
	if err != nil {
		return "", err
	}
	if err := writeAll(s, append(files, ticket)); err != nil {
		return "", err
	}
	return Hash(ticket.data), nil
}

// NewWorkstream is the content of a workstream created from the template.
type NewWorkstream struct {
	Title, Goal, Priority string
	// Ordered writes ordered: true, chaining the tickets; unordered, the
	// default, writes no field.
	Ordered bool
	// Tickets are live ticket ids of the project, in order; each joins the
	// new workstream and leaves any other.
	Tickets              []string
	DependsOnWorkstreams []string
	Tags                 []string
}

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
	input.Title = strings.Join(strings.Fields(input.Title), " ")
	if input.Title == "" {
		return CreatedWorkstream{}, fmt.Errorf("%w: a workstream needs a title", ErrInvalidInput)
	}
	if input.Priority == "" {
		input.Priority = "medium"
	}
	if !slices.Contains(board.Priorities, input.Priority) {
		return CreatedWorkstream{}, fmt.Errorf("%w: priority must be one of %s", ErrInvalidInput, strings.Join(board.Priorities, ", "))
	}
	for index, id := range input.Tickets {
		if _, _, ok := board.ParseID(id); !ok {
			return CreatedWorkstream{}, fmt.Errorf("%w: %q is not a ticket id", ErrInvalidInput, id)
		}
		if slices.Contains(input.Tickets[:index], id) {
			return CreatedWorkstream{}, fmt.Errorf("%w: %s is listed twice", ErrInvalidInput, id)
		}
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
	data, err := s.workstreamTemplate(slug, input)
	if err != nil {
		return CreatedWorkstream{}, err
	}
	var tickets []pending
	for _, id := range input.Tickets {
		ticket, err := s.prepareTicket(project, id, "", slug, nil)
		if err != nil {
			return CreatedWorkstream{}, err
		}
		tickets = append(tickets, ticket)
	}
	// Other workstreams give up the initial tickets; the new file already
	// lists them.
	others, err := s.membership(project, input.Tickets, "")
	if err != nil {
		return CreatedWorkstream{}, err
	}
	name := path.Join(project, "workstreams", slug+".md")
	files := append([]pending{{name: name, data: data, changed: true}}, others...)
	if err := writeAll(s, append(files, tickets...)); err != nil {
		return CreatedWorkstream{}, err
	}
	return CreatedWorkstream{Slug: slug, Hash: Hash(data)}, nil
}

func (s *Store) workstreamTemplate(slug string, w NewWorkstream) ([]byte, error) {
	body := []string{"", "# " + w.Title, "", "## Goal", ""}
	if goal := strings.TrimSpace(w.Goal); goal != "" {
		body = append(body, goal, "")
	}
	body = append(body, "## Scope", "", "## Success Criteria", "", "- [ ] Every ticket in the line reviewed", "", "## Notes", "", "None.", "")
	// The skeleton fixes the field order and writes tickets as a block list,
	// as board-format shows it; SetList keeps that style.
	data := []byte("---\nslug: x\nstatus: x\npriority: x\ncreated: x\ntickets:\n  - x\ndepends-on-workstreams: []\ntags: []\n---\n" + strings.Join(body, "\n"))
	steps := []struct {
		key    string
		value  string
		list   []string
		isList bool
	}{
		{key: "slug", value: slug},
		{key: "status", value: board.StatusActive},
		{key: "priority", value: w.Priority},
		{key: "created", value: s.now().Format("2006-01-02")},
		{key: "tickets", list: w.Tickets, isList: true},
		{key: "depends-on-workstreams", list: w.DependsOnWorkstreams, isList: true},
		{key: "tags", list: w.Tags, isList: true},
	}
	var err error
	for _, step := range steps {
		if step.isList {
			data, err = mdfile.SetList(data, step.key, step.list)
		} else {
			data, err = mdfile.SetScalar(data, step.key, step.value)
		}
		if err != nil {
			return nil, err
		}
	}
	if w.Ordered {
		data = []byte(strings.Replace(string(data), "\ntickets:", "\nordered: true\ntickets:", 1))
	}
	return data, nil
}
