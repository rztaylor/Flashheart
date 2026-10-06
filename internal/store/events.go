package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Event log files and agent-created projects. The event log's format belongs
// to events; these primitives only confine, lock and name its files
// (<project>/.flashheart/events/YYYY-MM-DD.jsonl, STO-5).

var eventFileName = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.jsonl$`)

func eventDir(project string) string { return path.Join(project, ".flashheart", "events") }

func (s *Store) checkEventFile(project, file string) error {
	if !eventFileName.MatchString(file) {
		return fmt.Errorf("event file %q: %w", file, ErrInvalidName)
	}
	return s.checkProject(project)
}

// AppendEventLines appends complete JSONL lines to a project's event file
// under the project lock, creating the file as needed.
func (s *Store) AppendEventLines(project, file string, lines []byte) error {
	if err := s.checkEventFile(project, file); err != nil {
		return err
	}
	release, err := s.lock(project)
	if err != nil {
		return err
	}
	defer release()
	root, _, err := s.handle()
	if err != nil {
		return err
	}
	if err := root.MkdirAll(eventDir(project), 0o755); err != nil {
		return fmt.Errorf("append events: %w", err)
	}
	name := path.Join(eventDir(project), file)
	handle, err := root.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("append events: %w", err)
	}
	_, writeErr := handle.Write(lines)
	return errors.Join(writeErr, handle.Close())
}

// EventFiles lists a project's event files in date order.
func (s *Store) EventFiles(project string) ([]string, error) {
	if err := s.checkProject(project); err != nil {
		return nil, err
	}
	entries, err := s.ReadDir(eventDir(project))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && eventFileName.MatchString(entry.Name()) {
			files = append(files, entry.Name())
		}
	}
	slices.Sort(files)
	return files, nil
}

// OpenEventFile opens a project's event file for reading.
func (s *Store) OpenEventFile(project, file string) (*os.File, error) {
	if err := s.checkEventFile(project, file); err != nil {
		return nil, err
	}
	root, _, err := s.handle()
	if err != nil {
		return nil, err
	}
	return root.Open(path.Join(eventDir(project), file))
}

// RemoveEventFile deletes one event file. Expiring old event files is the
// only deletion Flashheart performs (board-format.md, Event log).
func (s *Store) RemoveEventFile(project, file string) error {
	if err := s.checkEventFile(project, file); err != nil {
		return err
	}
	root, _, err := s.handle()
	if err != nil {
		return err
	}
	return root.Remove(path.Join(eventDir(project), file))
}

// IsProject reports whether name is an existing project.
func (s *Store) IsProject(name string) bool {
	_, fsys, err := s.handle()
	return err == nil && ValidProject(name) && isProject(fsys, name)
}

var unsafeProjectRun = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// ErrNeedsMigration reports a v1 project, which only `flashheart migrate`
// may change (MIG-1).
var ErrNeedsMigration = errors.New("the project uses board format v1; run flashheart migrate")

// ProjectName turns a repository directory name into a safe project name,
// or "" when nothing usable is left.
func ProjectName(name string) string {
	name = strings.Trim(unsafeProjectRun.ReplaceAllString(name, "-"), "-._")
	for strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", ".")
	}
	if len(name) > 200 {
		name = name[:200]
	}
	if name == "" || !safeName.MatchString(name) {
		return ""
	}
	return name
}

func repoSuffix(repo string) string {
	sum := sha256.Sum256([]byte(repo))
	return hex.EncodeToString(sum[:])[:6]
}

// ProjectFor returns the project for a repository's main checkout (PRJ-2):
// the project named after it, created with its name and repository but no
// key when create is set (PRJ-5, KEY-5). A project that records no
// repositories adopts this one; a project that records others leaves this
// repository to <name>-<6 hex of sha256(repo)> (PRJ-3). An empty repo means
// activity outside git, which goes to _scratch (PRJ-4); a name with nothing
// usable becomes repo-<6 hex>, never _scratch. A v1 project is left for
// migrate (ErrNeedsMigration).
func (s *Store) ProjectFor(name, repo string, create bool) (string, error) {
	candidates := []string{ScratchProject}
	if repo != "" {
		base := ProjectName(name)
		if base == "" {
			base = "repo-" + repoSuffix(repo)
		}
		candidates = []string{base, base + "-" + repoSuffix(repo)}
	}
	// Fast path without a lock: the project already records this repository.
	for _, candidate := range candidates {
		if repos, ok := s.projectRepos(candidate); ok && (repo == "" || slices.Contains(repos, repo)) {
			return candidate, nil
		}
	}
	release, err := s.lock(".")
	if err != nil {
		return "", err
	}
	defer release()
	for _, candidate := range candidates {
		if s.IsV1Project(candidate) {
			return "", fmt.Errorf("project %q: %w", candidate, ErrNeedsMigration)
		}
		repos, exists := s.projectRepos(candidate)
		switch {
		case exists && (repo == "" || slices.Contains(repos, repo)):
			return candidate, nil
		case exists && len(repos) == 0:
			return candidate, s.addRepo(candidate, repo)
		case exists:
			continue
		case !create:
			return "", fmt.Errorf("project %q: %w", candidate, ErrNotFound)
		}
		return candidate, s.createProject(candidate, repo)
	}
	return "", fmt.Errorf("project for %s: %w: both %s are taken by other repositories", repo, ErrInvalidInput, strings.Join(candidates, " and "))
}

// projectRepos reads a project's recorded repositories; ok is false when the
// project does not exist.
func (s *Store) projectRepos(project string) ([]string, bool) {
	if !s.IsProject(project) {
		return nil, false
	}
	data, err := s.ReadFile(path.Join(project, "project.yaml"))
	if err != nil {
		return nil, true
	}
	var parsed projectFile
	_ = yaml.Unmarshal(data, &parsed)
	return parsed.Repos, true
}

func (s *Store) addRepo(project, repo string) error {
	release, err := s.lock(project)
	if err != nil {
		return err
	}
	defer release()
	name := path.Join(project, "project.yaml")
	data, err := s.ReadFile(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	text := string(data)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	// project.yaml is plain YAML; edit it as frontmatter of an empty body.
	doc, err := mdfile.SetList([]byte("---\n"+text+"---\n"), "repos", []string{repo})
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return s.WriteFileAtomic(name, []byte(strings.TrimSuffix(strings.TrimPrefix(string(doc), "---\n"), "---\n")))
}

func (s *Store) createProject(project, repo string) error {
	if !ValidProject(project) {
		return fmt.Errorf("project %q: %w", project, ErrInvalidName)
	}
	content := "name: " + project + "\n"
	if repo != "" {
		quoted, err := yaml.Marshal([]string{repo})
		if err != nil {
			return err
		}
		content += "repos:\n" + indent(string(quoted))
	}
	return s.WriteFileAtomic(path.Join(project, "project.yaml"), []byte(content))
}

func indent(block string) string {
	lines := strings.SplitAfter(block, "\n")
	var b strings.Builder
	for _, line := range lines {
		if line != "" {
			b.WriteString("  " + line)
		}
	}
	return b.String()
}

// IsV1Project reports a project directory still in the v1 column-folder
// layout, which only `flashheart migrate` may change (MIG-1).
func (s *Store) IsV1Project(name string) bool {
	_, fsys, err := s.handle()
	if err != nil || !ValidProject(name) || isDir(fsys, path.Join(name, "tickets")) {
		return false
	}
	return slices.ContainsFunc(V1Columns, func(column string) bool { return isDir(fsys, path.Join(name, column)) })
}
