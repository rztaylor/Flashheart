package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/rztaylor/flashheart/internal/board"
)

// MaxFileBytes bounds every markdown or YAML file read from the root.
const MaxFileBytes = 1 << 20

// ScratchProject is the reserved project for activity outside git (PRJ-4).
const ScratchProject = "_scratch"

var (
	// ErrInvalidName reports a project, ticket or file name that is unsafe or
	// malformed; it is never resolved against the filesystem.
	ErrInvalidName = errors.New("invalid name")
	// ErrNotFound reports a valid name that does not exist.
	ErrNotFound = errors.New("not found")
	// ErrTypeNotAllowed reports an attachment type outside the allow-list.
	ErrTypeNotAllowed = errors.New("attachment type not allowed")
)

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$`)

// Store reads a board root. Every access goes through an os.Root, so paths
// and symlinks cannot escape the root (SEC-2).
type Store struct {
	path string
	mu   sync.Mutex
	root *os.Root
}

// New returns a store that opens the root on first use and keeps trying
// until it exists, so a root created after startup is picked up.
func New(path string) *Store {
	return &Store{path: path}
}

// Open opens the board root at path now. A missing root returns an error
// that wraps os.ErrNotExist.
func Open(path string) (*Store, error) {
	s := New(path)
	if _, _, err := s.handle(); err != nil {
		return nil, err
	}
	return s, nil
}

// handle returns the open root, opening it if needed.
func (s *Store) handle() (*os.Root, fs.FS, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nil {
		root, err := os.OpenRoot(s.path)
		if err != nil {
			return nil, nil, fmt.Errorf("open board root: %w", err)
		}
		s.root = root
	}
	return s.root, s.root.FS(), nil
}

// Close releases the root.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nil {
		return nil
	}
	err := s.root.Close()
	s.root = nil
	return err
}

// validProject reports whether name can be a project directory name.
func validProject(name string) bool {
	if !safeName.MatchString(name) && name != ScratchProject {
		return false
	}
	return !strings.Contains(name, "..")
}

func validSlug(name string) bool {
	return safeName.MatchString(name) && !strings.Contains(name, "..")
}

// Projects lists project directory names in sorted order (PRJ-1).
func (s *Store) Projects() ([]string, error) {
	_, fsys, err := s.handle()
	if err != nil {
		return nil, err
	}
	return projects(fsys)
}

func projects(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("list board root: %w", err)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if !validProject(name) {
			continue
		}
		if isProject(fsys, name) {
			names = append(names, name)
		}
	}
	return names, nil
}

func isProject(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	if err != nil || !info.IsDir() {
		return false
	}
	if info, err := fs.Stat(fsys, path.Join(name, "project.yaml")); err == nil && info.Mode().IsRegular() {
		return true
	}
	for _, column := range board.Columns {
		if info, err := fs.Stat(fsys, path.Join(name, string(column))); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// ReadProject reads one project's tickets, workstreams, reviews, attachment
// indexes and archived slugs.
func (s *Store) ReadProject(name string) (board.Project, error) {
	root, fsys, err := s.handle()
	if err != nil {
		return board.Project{}, err
	}
	project, _, err := readProject(root, fsys, name)
	return project, err
}

// ReadBoard reads every project and returns a fingerprint of the files read
// (paths, sizes and modification times) that changes when any of them does.
func (s *Store) ReadBoard() (board.Board, string, error) {
	root, fsys, err := s.handle()
	if err != nil {
		return board.Board{}, "", err
	}
	names, err := projects(fsys)
	if err != nil {
		return board.Board{}, "", err
	}
	type result struct {
		project board.Project
		parts   []string
		err     error
	}
	results := make([]result, len(names))
	var wait sync.WaitGroup
	limit := make(chan struct{}, 8)
	for index, name := range names {
		wait.Go(func() {
			limit <- struct{}{}
			defer func() { <-limit }()
			project, parts, err := readProject(root, fsys, name)
			results[index] = result{project, parts, err}
		})
	}
	wait.Wait()

	hash := sha256.New()
	b := board.Board{Projects: make([]board.Project, 0, len(names))}
	for _, result := range results {
		if result.err != nil {
			if errors.Is(result.err, ErrNotFound) {
				continue // removed while reading
			}
			return board.Board{}, "", result.err
		}
		b.Projects = append(b.Projects, result.project)
		for _, part := range result.parts {
			io.WriteString(hash, part)
		}
	}
	return b, hex.EncodeToString(hash.Sum(nil)), nil
}

// reader accumulates one project's fingerprint parts and modification time.
type reader struct {
	root     *os.Root
	fsys     fs.FS
	parts    []string
	modified time.Time
}

func (r *reader) note(name string, info fs.FileInfo) {
	r.parts = append(r.parts, fmt.Sprintf("%s\x00%d\x00%d\n", name, info.Size(), info.ModTime().UnixNano()))
	if info.ModTime().After(r.modified) {
		r.modified = info.ModTime()
	}
}

// markdownFiles lists the .md entries of dir, skipping hidden files and
// subdirectories. A missing directory yields nothing.
func (r *reader) markdownFiles(dir string) []string {
	entries, err := fs.ReadDir(r.fsys, dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".md") {
			continue
		}
		if entry.IsDir() {
			continue
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			if info, err := fs.Stat(r.fsys, path.Join(dir, name)); err == nil && info.IsDir() {
				continue
			}
		}
		names = append(names, name)
	}
	return names
}

// read returns a file's contents up to MaxFileBytes, or a repair reason.
func (r *reader) read(name string) ([]byte, fs.FileInfo, string) {
	file, err := r.root.Open(name)
	if err != nil {
		if info, lerr := r.root.Lstat(name); lerr == nil && info.Mode()&fs.ModeSymlink != 0 {
			return nil, nil, "this file is a symlink that points outside the board root or to a missing file"
		}
		return nil, nil, "could not read the file: " + err.Error()
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, "could not read the file: " + err.Error()
	}
	r.note(name, info)
	if !info.Mode().IsRegular() {
		return nil, info, "not a regular file"
	}
	if info.Size() > MaxFileBytes {
		return nil, info, fmt.Sprintf("file is larger than %d KiB and was not read", MaxFileBytes>>10)
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxFileBytes))
	if err != nil {
		return nil, info, "could not read the file: " + err.Error()
	}
	return data, info, ""
}

func readProject(root *os.Root, fsys fs.FS, name string) (board.Project, []string, error) {
	if !validProject(name) {
		return board.Project{}, nil, fmt.Errorf("project %q: %w", name, ErrInvalidName)
	}
	if !isProject(fsys, name) {
		return board.Project{}, nil, fmt.Errorf("project %q: %w", name, ErrNotFound)
	}
	r := &reader{root: root, fsys: fsys}
	project := board.Project{
		Name: name, DisplayName: name,
		Reviews: map[string]bool{}, Attachments: map[string][]board.Attachment{},
	}
	r.readProjectFile(&project)

	for _, column := range board.Columns {
		dir := path.Join(name, string(column))
		for _, file := range r.markdownFiles(dir) {
			slug := strings.TrimSuffix(file, ".md")
			data, info, problem := r.read(path.Join(dir, file))
			var ticket board.Ticket
			if problem != "" {
				ticket = board.ParseTicket(slug, column, nil)
				ticket.Body, ticket.Warnings = "", nil
				ticket.Repair = []string{problem}
			} else {
				ticket = board.ParseTicket(slug, column, data)
			}
			if info != nil {
				ticket.Modified = info.ModTime()
			}
			project.Tickets = append(project.Tickets, ticket)
		}
	}
	board.MarkDuplicates(&project)

	workstreams := path.Join(name, "workstreams")
	for _, file := range r.markdownFiles(workstreams) {
		slug := strings.TrimSuffix(file, ".md")
		data, _, problem := r.read(path.Join(workstreams, file))
		workstream := board.ParseWorkstream(slug, data)
		if problem != "" {
			workstream.Repair = append([]string{problem}, workstream.Repair...)
		}
		project.Workstreams = append(project.Workstreams, workstream)
	}

	reviews := path.Join(name, "reviews")
	for _, file := range r.markdownFiles(reviews) {
		if info, err := fs.Stat(fsys, path.Join(reviews, file)); err == nil {
			r.note(path.Join(reviews, file), info)
			project.Reviews[strings.TrimSuffix(file, ".md")] = true
		}
	}

	r.readAttachmentIndexes(&project)

	for _, column := range board.Columns {
		dir := path.Join(name, ".archive", string(column))
		for _, file := range r.markdownFiles(dir) {
			r.parts = append(r.parts, path.Join(dir, file)+"\n")
			project.Archived = append(project.Archived, strings.TrimSuffix(file, ".md"))
		}
	}
	slices.Sort(project.Archived)
	project.Archived = slices.Compact(project.Archived)
	project.LastModified = r.modified
	return project, r.parts, nil
}

type projectFile struct {
	Name  string   `yaml:"name"`
	Repos []string `yaml:"repos"`
}

func (r *reader) readProjectFile(project *board.Project) {
	name := path.Join(project.Name, "project.yaml")
	if _, err := r.root.Lstat(name); errors.Is(err, fs.ErrNotExist) {
		return
	}
	data, _, problem := r.read(name)
	if problem != "" {
		project.Warnings = append(project.Warnings, "project.yaml: "+problem)
		return
	}
	var parsed projectFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		project.Warnings = append(project.Warnings, "project.yaml does not parse: "+strings.TrimPrefix(err.Error(), "yaml: "))
		return
	}
	if display := strings.TrimSpace(parsed.Name); display != "" {
		project.DisplayName = display
	}
	project.Repos = parsed.Repos
}

type attachmentEntry struct {
	File    string `yaml:"file"`
	Caption string `yaml:"caption"`
	Kind    string `yaml:"kind"`
	Run     string `yaml:"run"`
	Added   string `yaml:"added"`
}

func (r *reader) readAttachmentIndexes(project *board.Project) {
	dir := path.Join(project.Name, "attachments")
	entries, err := fs.ReadDir(r.fsys, dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		ticket := entry.Name()
		if !entry.IsDir() || !validSlug(ticket) {
			continue
		}
		name := path.Join(dir, ticket, "index.yaml")
		if _, err := r.root.Lstat(name); err != nil {
			continue
		}
		data, _, problem := r.read(name)
		if problem != "" {
			project.Warnings = append(project.Warnings, fmt.Sprintf("attachments/%s/index.yaml: %s", ticket, problem))
			continue
		}
		var items []attachmentEntry
		if err := yaml.Unmarshal(data, &items); err != nil {
			project.Warnings = append(project.Warnings, fmt.Sprintf("attachments/%s/index.yaml is not a list of attachments", ticket))
			continue
		}
		for _, item := range items {
			if !validSlug(item.File) {
				continue
			}
			project.Attachments[ticket] = append(project.Attachments[ticket], board.Attachment{
				File: item.File, Caption: item.Caption, Kind: item.Kind, Run: item.Run, Added: item.Added,
			})
		}
	}
}

// ReadReview returns reviews/<slug>.md, or found=false when there is none.
func (s *Store) ReadReview(project, slug string) (string, bool, error) {
	if !validProject(project) || !validSlug(slug) {
		return "", false, fmt.Errorf("review %s/%s: %w", project, slug, ErrInvalidName)
	}
	root, fsys, err := s.handle()
	if err != nil {
		return "", false, err
	}
	name := path.Join(project, "reviews", slug+".md")
	if _, err := root.Lstat(name); errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	r := &reader{root: root, fsys: fsys}
	data, _, problem := r.read(name)
	if problem != "" {
		return "", false, fmt.Errorf("review %s/%s: %s", project, slug, problem)
	}
	return string(data), true, nil
}

// OpenAttachment opens attachments/<ticket>/<file> if its type is allowed
// (REV-2) and returns the content type to serve it with.
func (s *Store) OpenAttachment(project, ticket, file string) (*os.File, string, error) {
	if !validProject(project) || !validSlug(ticket) || !validSlug(file) {
		return nil, "", fmt.Errorf("attachment %s/%s/%s: %w", project, ticket, file, ErrInvalidName)
	}
	contentType, ok := board.AttachmentType(file)
	if !ok {
		return nil, "", fmt.Errorf("attachment %s: %w", file, ErrTypeNotAllowed)
	}
	root, _, err := s.handle()
	if err != nil {
		return nil, "", err
	}
	handle, err := root.Open(path.Join(project, "attachments", ticket, file))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", fmt.Errorf("attachment %s: %w", file, ErrNotFound)
	}
	if err != nil {
		return nil, "", fmt.Errorf("attachment %s: %w", file, err)
	}
	if info, err := handle.Stat(); err != nil || !info.Mode().IsRegular() {
		handle.Close()
		return nil, "", fmt.Errorf("attachment %s: %w", file, ErrNotFound)
	}
	return handle, contentType, nil
}
