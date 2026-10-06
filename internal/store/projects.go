package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Archived projects (PRJ-5). Archiving moves <root>/<project>/ to
// <root>/.archive/<project>/ under the root lock; restoring moves it back
// unless a live project has that name. A project is deleted permanently only
// once archived: the delete removes other projects' depends-on references to
// its tickets, retires its key in <root>/.flashheart/retired.yaml so no
// project takes it (KEY-5), and removes the directory. Everything is by
// name, inside the root, and never through a symbolic link.

var (
	// ErrExists reports that the target of a move is already taken.
	ErrExists = errors.New("already exists")
	// ErrProjectArchived reports a repository whose project is archived:
	// agents' activity there is not recorded until it is restored.
	ErrProjectArchived = errors.New("the project is archived")
)

const archiveDir = ".archive"

// retiredFile records the keys of permanently deleted projects.
const retiredFile = ".flashheart/retired.yaml"

// ArchiveProject moves a live project into <root>/.archive/.
func (s *Store) ArchiveProject(project string) error {
	if err := s.checkProject(project); err != nil {
		return err
	}
	if err := s.refuseLinks(project); err != nil {
		return err
	}
	releaseRoot, err := s.lock(".")
	if err != nil {
		return err
	}
	defer releaseRoot()
	release, err := s.lock(project)
	if err != nil {
		return err
	}
	defer release()
	target := path.Join(archiveDir, project)
	if s.Exists(target) {
		return fmt.Errorf("%w: an archived project named %s already exists; restore or delete it first", ErrExists, project)
	}
	if err := s.Move(project, target); err != nil {
		return err
	}
	// The directory's time dates the archive.
	if root, _, err := s.handle(); err == nil {
		now := s.now()
		_ = root.Chtimes(target, now, now)
	}
	return nil
}

// archivedProjectDir finds an archived project's directory by name.
func (s *Store) archivedProjectDir(project string) (string, error) {
	if !ValidProject(project) {
		return "", fmt.Errorf("project %q: %w", project, ErrInvalidName)
	}
	root, fsys, err := s.handle()
	if err != nil {
		return "", err
	}
	dir := path.Join(archiveDir, project)
	if _, err := root.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("archived project %q: %w", project, ErrNotFound)
	} else if err != nil {
		return "", err
	}
	if err := s.refuseLinks(dir); err != nil {
		return "", err
	}
	if !isProject(fsys, dir) {
		return "", fmt.Errorf("archived project %q: %w", project, ErrNotFound)
	}
	return dir, nil
}

// RestoreProject moves an archived project back to the root, refusing when
// a live project has its name.
func (s *Store) RestoreProject(project string) error {
	dir, err := s.archivedProjectDir(project)
	if err != nil {
		return err
	}
	release, err := s.lock(".")
	if err != nil {
		return err
	}
	defer release()
	if s.Exists(project) {
		_, fsys, _ := s.handle()
		if fsys != nil && !isProject(fsys, project) {
			return fmt.Errorf("%w: a folder named %s, which is not a project, is in the way in the board root; move it aside first", ErrExists, project)
		}
		return fmt.Errorf("%w: a project named %s is on the board; rename or archive it first", ErrExists, project)
	}
	return s.Move(dir, project)
}

// refuseLinks refuses a path any of whose segments is a symbolic link, so
// moves and deletes never act through one (SEC-2).
func (s *Store) refuseLinks(name string) error {
	root, _, err := s.handle()
	if err != nil {
		return err
	}
	current := ""
	for _, segment := range strings.Split(name, "/") {
		current = path.Join(current, segment)
		info, err := root.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s is a symbolic link; Flashheart does not move or delete through links", ErrInvalidInput, current)
		}
	}
	return nil
}

// ArchivedProjects lists the archived projects in name order.
func (s *Store) ArchivedProjects() ([]board.ArchivedProject, error) {
	root, fsys, err := s.handle()
	if err != nil {
		return nil, err
	}
	return archivedProjects(root, fsys), nil
}

func archivedProjects(root interface {
	Lstat(string) (fs.FileInfo, error)
}, fsys fs.FS) []board.ArchivedProject {
	entries, err := fs.ReadDir(fsys, archiveDir)
	if err != nil {
		return nil
	}
	var out []board.ArchivedProject
	for _, entry := range entries {
		name := entry.Name()
		dir := path.Join(archiveDir, name)
		if !entry.IsDir() || !ValidProject(name) || !isProject(fsys, dir) {
			continue
		}
		project := board.ArchivedProject{Name: name, DisplayName: name}
		project.Key, project.KeyDerived = board.DeriveKey(name), true
		if data, err := fs.ReadFile(fsys, path.Join(dir, "project.yaml")); err == nil {
			var parsed projectFile
			if yaml.Unmarshal(data, &parsed) == nil {
				if display := strings.TrimSpace(parsed.Name); display != "" {
					project.DisplayName = display
				}
				if key := strings.TrimSpace(parsed.Key); board.ValidKey(key) {
					project.Key, project.KeyDerived = key, false
				}
				project.Repos = parsed.Repos
			}
		}
		for _, tickets := range []string{path.Join(dir, "tickets"), path.Join(dir, ".archive", "tickets")} {
			folders, _ := fs.ReadDir(fsys, tickets)
			for _, folder := range folders {
				if id, _, ok := board.ParseFolder(folder.Name()); ok && folder.IsDir() {
					project.IDs = append(project.IDs, id)
				}
			}
		}
		slices.SortFunc(project.IDs, board.CompareIDs)
		project.IDs = slices.Compact(project.IDs)
		if info, err := root.Lstat(dir); err == nil {
			project.Archived = info.ModTime()
		}
		out = append(out, project)
	}
	return out
}

// archivedRepos reads an archived project's repositories; ok is false when
// there is no archived project of that name.
func (s *Store) archivedRepos(project string) ([]string, bool) {
	dir, err := s.archivedProjectDir(project)
	if err != nil {
		return nil, false
	}
	data, err := s.ReadFile(path.Join(dir, "project.yaml"))
	if err != nil {
		return nil, true
	}
	var parsed projectFile
	_ = yaml.Unmarshal(data, &parsed)
	return parsed.Repos, true
}

// ProjectReference is a ticket in a live project that depends on tickets of
// the project being deleted.
type ProjectReference struct {
	Project, ID, Title string
	DependsOn          []string
	file, hash         string
}

// ProjectDeletePlan is what deleting an archived project touches. Token
// fingerprints it; a delete is refused when it has changed.
type ProjectDeletePlan struct {
	Name, DisplayName, Key string
	Tickets                int
	References             []ProjectReference
	Token                  string
}

// PlanProjectDelete lists what deleting an archived project would touch.
func (s *Store) PlanProjectDelete(project string) (ProjectDeletePlan, error) {
	if _, err := s.archivedProjectDir(project); err != nil {
		return ProjectDeletePlan{}, err
	}
	root, fsys, err := s.handle()
	if err != nil {
		return ProjectDeletePlan{}, err
	}
	var archived *board.ArchivedProject
	all := archivedProjects(root, fsys)
	for index := range all {
		if all[index].Name == project {
			archived = &all[index]
		}
	}
	if archived == nil {
		return ProjectDeletePlan{}, fmt.Errorf("archived project %q: %w", project, ErrNotFound)
	}
	plan := ProjectDeletePlan{Name: project, DisplayName: archived.DisplayName, Key: archived.Key, Tickets: len(archived.IDs)}
	token := sha256.New()
	fmt.Fprintf(token, "delete project %s %s %s\n", project, archived.Key, strings.Join(archived.IDs, ","))
	names, err := s.Projects()
	if err != nil {
		return ProjectDeletePlan{}, err
	}
	for _, name := range names {
		info, err := s.ReadProject(name)
		if err != nil {
			continue
		}
		tickets := slices.Clone(info.Tickets)
		dirs := slices.Repeat([]string{path.Join(name, "tickets")}, len(tickets))
		if items, err := s.ReadArchived(name); err == nil {
			for _, item := range items {
				tickets = append(tickets, item.Ticket)
				dirs = append(dirs, path.Join(name, ".archive", "tickets"))
			}
		}
		for index, ticket := range tickets {
			var matched []string
			for _, id := range ticket.DependsOn {
				if slices.Contains(archived.IDs, id) {
					matched = append(matched, id)
				}
			}
			if len(matched) == 0 {
				continue
			}
			file, err := s.ticketFile(name, dirs[index], ticket.ID)
			if err != nil {
				return ProjectDeletePlan{}, err
			}
			data, err := s.ReadFile(file)
			if err != nil {
				return ProjectDeletePlan{}, err
			}
			reference := ProjectReference{Project: name, ID: ticket.ID, Title: ticket.Title, DependsOn: matched, file: file, hash: Hash(data)}
			plan.References = append(plan.References, reference)
			fmt.Fprintf(token, "ticket %s %s\n", file, reference.hash)
		}
	}
	plan.Token = hex.EncodeToString(token.Sum(nil))
	return plan, nil
}

// DeleteArchivedProject permanently deletes an archived project. token must
// be the Token of a plan made since nothing it lists changed, or the result
// is a *ConflictError and nothing is written. It holds the root lock and the
// lock of every project it rewrites, in name order.
func (s *Store) DeleteArchivedProject(project, token string) error {
	if _, err := s.archivedProjectDir(project); err != nil {
		return err
	}
	releaseRoot, err := s.lock(".")
	if err != nil {
		return err
	}
	defer releaseRoot()
	first, err := s.PlanProjectDelete(project)
	if err != nil {
		return err
	}
	var locked []string
	for _, reference := range first.References {
		locked = append(locked, reference.Project)
	}
	slices.Sort(locked)
	for _, name := range slices.Compact(locked) {
		release, err := s.lock(name)
		if err != nil {
			return err
		}
		defer release()
	}
	plan, err := s.PlanProjectDelete(project)
	if err != nil {
		return err
	}
	if plan.Token != token || plan.Token != first.Token {
		return &ConflictError{Name: "delete", Hash: plan.Token}
	}
	for _, reference := range plan.References {
		_, err := s.updateLocked(reference.file, reference.hash, func(data []byte) ([]byte, error) {
			list := mdfile.Parse(data).List("depends-on")
			list = slices.DeleteFunc(list, func(id string) bool { return slices.Contains(reference.DependsOn, id) })
			return mdfile.SetList(data, "depends-on", list)
		}, true)
		if err != nil {
			return err
		}
	}
	if err := s.retireKey(plan.Key); err != nil {
		return err
	}
	root, _, err := s.handle()
	if err != nil {
		return err
	}
	if err := root.RemoveAll(path.Join(archiveDir, project)); err != nil {
		return fmt.Errorf("delete project %s: %w", project, err)
	}
	return nil
}

type retired struct {
	Keys []string `yaml:"keys"`
}

// retiredKeys reads the keys of deleted projects.
func (s *Store) retiredKeys() []string {
	data, err := s.ReadFile(retiredFile)
	if err != nil {
		return nil
	}
	var parsed retired
	_ = yaml.Unmarshal(data, &parsed)
	return parsed.Keys
}

// retireKey adds key to retired.yaml. Callers hold the root lock.
func (s *Store) retireKey(key string) error {
	keys := s.retiredKeys()
	if key == "" || slices.Contains(keys, key) {
		return nil
	}
	data, err := s.ReadFile(retiredFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	text := string(data)
	if text == "" {
		text = "# Keys of permanently deleted projects; no project may take them (KEY-5).\n"
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	doc, err := mdfile.SetList([]byte("---\n"+text+"---\n"), "keys", append(keys, key))
	if err != nil {
		return fmt.Errorf("%s: %w", retiredFile, err)
	}
	out := strings.TrimSuffix(strings.TrimPrefix(string(doc), "---\n"), "---\n")
	return s.WriteFileAtomic(retiredFile, []byte(out))
}
