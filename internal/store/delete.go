package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Archived tickets and permanent deletion (EDIT-8). A ticket is deleted only
// from <project>/.archive/tickets/, by id, never through a symbolic link.
// The delete removes the id from every ticket's depends-on and the
// project's workstream lists, records it as retired in project.yaml so it is
// never reused (KEY-2), and removes the folder.

// ArchivedTicket is an archived ticket and what its folder holds.
type ArchivedTicket struct {
	Ticket      board.Ticket
	Review      bool
	Attachments int
}

// ReadArchived parses a project's archived tickets, in id order. Folders
// that are symbolic links or hold no ticket file are skipped.
func (s *Store) ReadArchived(project string) ([]ArchivedTicket, error) {
	if err := s.checkProject(project); err != nil {
		return nil, err
	}
	dir := path.Join(project, ".archive", "tickets")
	entries, err := s.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []ArchivedTicket
	for _, entry := range entries {
		if !entry.IsDir() || !ValidName(entry.Name()) {
			continue
		}
		folder := path.Join(dir, entry.Name())
		files, err := s.ReadDir(folder)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(files))
		for _, file := range files {
			names = append(names, file.Name())
		}
		name, _, ok := board.TicketFile(entry.Name(), names)
		if !ok {
			continue
		}
		data, err := s.ReadFile(path.Join(folder, name))
		if err != nil {
			continue
		}
		item := ArchivedTicket{Ticket: board.ParseTicket(entry.Name(), data), Review: slices.Contains(names, "review.md")}
		if attachments, err := s.ReadDir(path.Join(folder, "files")); err == nil {
			for _, file := range attachments {
				if !file.IsDir() && file.Name() != "index.yaml" && !strings.HasPrefix(file.Name(), ".") {
					item.Attachments++
				}
			}
		}
		out = append(out, item)
	}
	slices.SortStableFunc(out, func(a, b ArchivedTicket) int { return board.CompareIDs(a.Ticket.ID, b.Ticket.ID) })
	return out, nil
}

// Reference is a ticket whose depends-on names the ticket being deleted.
type Reference struct {
	Project, ID, Title string
	file, hash         string
}

// DeletePlan is everything a permanent delete touches. Token fingerprints
// all of it: a delete is refused when any of it has changed since.
type DeletePlan struct {
	Project, ID string
	// Folder is the archived folder, relative to the root; Files are its
	// files, relative to it.
	Folder      string
	Files       []string
	Tickets     []Reference
	Workstreams []string
	Token       string
}

// archivedFolder finds an archived ticket's folder by id.
func (s *Store) archivedFolder(project, id string) (string, error) {
	if err := s.checkProject(project); err != nil {
		return "", err
	}
	if _, _, ok := board.ParseID(id); !ok {
		return "", fmt.Errorf("ticket %q: %w", id, ErrInvalidName)
	}
	dir := path.Join(project, ".archive", "tickets")
	entries, err := s.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	for _, entry := range entries {
		if folderID, _, ok := board.ParseFolder(entry.Name()); !ok || folderID != id {
			continue
		}
		folder := path.Join(dir, entry.Name())
		if err := s.refuseLinks(folder); err != nil {
			return "", err
		}
		if entry.IsDir() {
			return folder, nil
		}
	}
	return "", fmt.Errorf("archived ticket %s in %s: %w", id, project, ErrNotFound)
}

// PlanDelete lists what deleting an archived ticket would touch.
func (s *Store) PlanDelete(project, id string) (DeletePlan, error) {
	return s.planDelete(project, id)
}

func (s *Store) planDelete(project, id string) (DeletePlan, error) {
	folder, err := s.archivedFolder(project, id)
	if err != nil {
		return DeletePlan{}, err
	}
	_, fsys, err := s.handle()
	if err != nil {
		return DeletePlan{}, err
	}
	plan := DeletePlan{Project: project, ID: id, Folder: folder}
	token := sha256.New()
	fmt.Fprintf(token, "delete %s %s\n", project, id)
	err = fs.WalkDir(fsys, folder, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		size := int64(0)
		if info, err := entry.Info(); err == nil {
			size = info.Size()
		}
		relative := strings.TrimPrefix(name, folder+"/")
		plan.Files = append(plan.Files, relative)
		fmt.Fprintf(token, "file %s %d\n", relative, size)
		return nil
	})
	if err != nil {
		return DeletePlan{}, err
	}

	projects, err := s.Projects()
	if err != nil {
		return DeletePlan{}, err
	}
	for _, name := range projects {
		info, err := s.ReadProject(name)
		if err != nil {
			continue
		}
		tickets := slices.Clone(info.Tickets)
		dirs := slices.Repeat([]string{path.Join(name, "tickets")}, len(tickets))
		if archived, err := s.ReadArchived(name); err == nil {
			for _, item := range archived {
				tickets = append(tickets, item.Ticket)
				dirs = append(dirs, path.Join(name, ".archive", "tickets"))
			}
		}
		for index, ticket := range tickets {
			if ticket.ID == id || !slices.Contains(ticket.DependsOn, id) {
				continue
			}
			file, err := s.ticketFile(name, dirs[index], ticket.ID)
			if err != nil {
				return DeletePlan{}, err
			}
			data, err := s.ReadFile(file)
			if err != nil {
				return DeletePlan{}, err
			}
			reference := Reference{Project: name, ID: ticket.ID, Title: ticket.Title, file: file, hash: Hash(data)}
			plan.Tickets = append(plan.Tickets, reference)
			fmt.Fprintf(token, "ticket %s %s\n", file, reference.hash)
		}
		if name != project {
			continue
		}
		for _, workstream := range info.Workstreams {
			if !slices.Contains(workstream.Tickets, id) {
				continue
			}
			file := path.Join(project, "workstreams", workstream.Slug+".md")
			data, err := s.ReadFile(file)
			if err != nil {
				return DeletePlan{}, err
			}
			plan.Workstreams = append(plan.Workstreams, workstream.Slug)
			fmt.Fprintf(token, "workstream %s %s\n", file, Hash(data))
		}
	}
	plan.Token = hex.EncodeToString(token.Sum(nil))
	return plan, nil
}

// DeleteArchived permanently deletes an archived ticket. token must be the
// Token of a plan made since nothing it lists changed, or the result is a
// *ConflictError and nothing is written. It holds the root lock, so deletes
// do not overlap, and the lock of every project it rewrites.
func (s *Store) DeleteArchived(project, id, token string) (DeletePlan, error) {
	if _, err := s.archivedFolder(project, id); err != nil {
		return DeletePlan{}, err
	}
	releaseRoot, err := s.lock(".")
	if err != nil {
		return DeletePlan{}, err
	}
	defer releaseRoot()
	first, err := s.planDelete(project, id)
	if err != nil {
		return DeletePlan{}, err
	}
	// Projects are locked in name order; no other writer holds two.
	locked := []string{project}
	for _, reference := range first.Tickets {
		locked = append(locked, reference.Project)
	}
	slices.Sort(locked)
	for _, name := range slices.Compact(locked) {
		release, err := s.lock(name)
		if err != nil {
			return DeletePlan{}, err
		}
		defer release()
	}
	plan, err := s.planDelete(project, id)
	if err != nil {
		return DeletePlan{}, err
	}
	if plan.Token != token || plan.Token != first.Token {
		return DeletePlan{}, &ConflictError{Name: "delete", Hash: plan.Token}
	}

	for _, reference := range plan.Tickets {
		_, err := s.updateLocked(reference.file, reference.hash, func(data []byte) ([]byte, error) {
			return mdfile.SetList(data, "depends-on", without(mdfile.Parse(data).List("depends-on"), id))
		}, true)
		if err != nil {
			return DeletePlan{}, err
		}
	}
	for _, slug := range plan.Workstreams {
		_, err := s.updateLocked(path.Join(project, "workstreams", slug+".md"), "", func(data []byte) ([]byte, error) {
			return mdfile.SetList(data, "tickets", without(mdfile.Parse(data).List("tickets"), id))
		}, false)
		if err != nil {
			return DeletePlan{}, err
		}
	}
	if err := s.retire(project, id); err != nil {
		return DeletePlan{}, err
	}
	root, _, err := s.handle()
	if err != nil {
		return DeletePlan{}, err
	}
	if err := root.RemoveAll(plan.Folder); err != nil {
		return DeletePlan{}, fmt.Errorf("delete %s: %w", plan.Folder, err)
	}
	return plan, nil
}

func without(items []string, id string) []string {
	return slices.DeleteFunc(slices.Clone(items), func(item string) bool { return item == id })
}

// retire adds id to project.yaml's retired list, keeping its other lines.
func (s *Store) retire(project, id string) error {
	name := path.Join(project, "project.yaml")
	data, err := s.ReadFile(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	crlf := bytes.Contains(data, []byte("\r\n"))
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	// project.yaml is plain YAML; edit it as frontmatter of an empty body.
	doc := []byte("---\n" + text + "---\n")
	retired := mdfile.Parse(doc).List("retired")
	if slices.Contains(retired, id) {
		return nil
	}
	doc, err = mdfile.SetList(doc, "retired", append(retired, id))
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	out := strings.TrimSuffix(strings.TrimPrefix(string(doc), "---\n"), "---\n")
	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return s.WriteFileAtomic(name, []byte(out))
}
