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

// V1Columns are the column folders of board format v1, read only to migrate.
var V1Columns = []string{"todo", "in-progress", "ready-to-review", "done"}

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
	path  string
	mu    sync.Mutex
	root  *os.Root
	clock func() time.Time
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

// Path returns the root path the store was created with.
func (s *Store) Path() string { return s.path }

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

// ValidProject reports whether name can be a project directory name.
func ValidProject(name string) bool {
	if !safeName.MatchString(name) && name != ScratchProject {
		return false
	}
	return !strings.Contains(name, "..")
}

// ValidName reports whether name is a safe single path segment (a ticket
// folder or a file name).
func ValidName(name string) bool {
	return safeName.MatchString(name) && !strings.Contains(name, "..")
}

// Projects lists v2 project directory names in sorted order (PRJ-1).
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
		if ValidProject(name) && isProject(fsys, name) {
			names = append(names, name)
		}
	}
	return names, nil
}

func isDir(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.IsDir()
}

func isFile(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}

func isProject(fsys fs.FS, name string) bool {
	return isDir(fsys, name) && (isDir(fsys, path.Join(name, "tickets")) || isFile(fsys, path.Join(name, "project.yaml")))
}

// V1Projects lists directories that still use the v1 column-folder layout and
// have no v2 tickets folder: the board needs `flashheart migrate` (MIG-1).
func (s *Store) V1Projects() ([]string, error) {
	_, fsys, err := s.handle()
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("list board root: %w", err)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if !ValidProject(name) || !isDir(fsys, name) || isDir(fsys, path.Join(name, "tickets")) {
			continue
		}
		if slices.ContainsFunc(V1Columns, func(column string) bool { return isDir(fsys, path.Join(name, column)) }) {
			names = append(names, name)
		}
	}
	return names, nil
}

// ReadProject reads one project's tickets, workstreams, reviews, files
// indexes and archived ids.
func (s *Store) ReadProject(name string) (board.Project, error) {
	root, fsys, err := s.handle()
	if err != nil {
		return board.Project{}, err
	}
	project, _, err := readProject(root, fsys, name)
	if err == nil {
		board.CheckProject(&project)
	}
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
		project := result.project
		board.CheckProject(&project)
		b.Projects = append(b.Projects, project)
		for _, part := range result.parts {
			io.WriteString(hash, part)
		}
	}
	board.CheckKeys(&b)
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

// dirs lists the visible subdirectories of dir. A missing directory yields
// nothing.
func (r *reader) dirs(dir string) []string {
	entries, err := fs.ReadDir(r.fsys, dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !ValidName(name) {
			continue
		}
		if entry.IsDir() || (entry.Type()&fs.ModeSymlink != 0 && isDir(r.fsys, path.Join(dir, name))) {
			names = append(names, name)
		}
	}
	return names
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
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".md") || entry.IsDir() {
			continue
		}
		if entry.Type()&fs.ModeSymlink != 0 && isDir(r.fsys, path.Join(dir, name)) {
			continue
		}
		names = append(names, name)
	}
	return names
}

// exists reports whether name exists without following it out of the root.
func (r *reader) exists(name string) bool {
	_, err := r.root.Lstat(name)
	return err == nil
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
	if !ValidProject(name) {
		return board.Project{}, nil, fmt.Errorf("project %q: %w", name, ErrInvalidName)
	}
	if !isProject(fsys, name) {
		return board.Project{}, nil, fmt.Errorf("project %q: %w", name, ErrNotFound)
	}
	// The project itself is part of the fingerprint, so an empty new
	// project is a change.
	r := &reader{root: root, fsys: fsys, parts: []string{"project " + name + "\n"}}
	project := board.Project{
		Name: name, DisplayName: name, NextID: 1,
		Reviews: map[string]bool{}, Attachments: map[string][]board.Attachment{},
	}
	r.readProjectFile(&project)

	tickets := path.Join(name, "tickets")
	for _, folder := range r.dirs(tickets) {
		// One listing per ticket folder tells us which parts exist.
		entries, err := fs.ReadDir(fsys, path.Join(tickets, folder))
		if err != nil {
			continue
		}
		present := map[string]fs.DirEntry{}
		var files []string
		for _, entry := range entries {
			present[entry.Name()] = entry
			if entry.Type().IsRegular() || entry.Type()&fs.ModeSymlink != 0 {
				files = append(files, entry.Name())
			}
		}
		name, misnamed, ok := board.TicketFile(folder, files)
		if !ok {
			continue
		}
		file := path.Join(tickets, folder, name)
		data, info, problem := r.read(file)
		var ticket board.Ticket
		if problem != "" {
			ticket = board.ParseTicket(folder, nil)
			ticket.Body, ticket.Warnings = "", nil
			ticket.Repair = []string{problem}
		} else {
			ticket = board.ParseTicket(folder, data)
		}
		if info != nil {
			ticket.Modified = info.ModTime()
		}
		if misnamed != "" {
			ticket.Warnings = append(ticket.Warnings, misnamed)
		}
		project.Tickets = append(project.Tickets, ticket)
		if ticket.ID == "" {
			continue
		}
		if entry := present["review.md"]; entry != nil {
			if info, err := entry.Info(); err == nil {
				r.note(path.Join(tickets, folder, "review.md"), info)
				project.Reviews[ticket.ID] = true
			}
		}
		if present["files"] != nil {
			r.readFilesIndex(&project, ticket.ID, path.Join(tickets, folder, "files", "index.yaml"))
		}
	}
	slices.SortStableFunc(project.Tickets, func(a, b board.Ticket) int {
		if c := slices.Index(board.Columns, a.Column) - slices.Index(board.Columns, b.Column); c != 0 {
			return c
		}
		_, an, _ := board.ParseID(a.ID)
		_, bn, _ := board.ParseID(b.ID)
		if an != bn {
			return an - bn
		}
		return strings.Compare(a.Folder, b.Folder)
	})

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

	archive := path.Join(name, ".archive", "tickets")
	for _, folder := range r.dirs(archive) {
		r.parts = append(r.parts, path.Join(archive, folder)+"\n")
		if id, _, ok := board.ParseFolder(folder); ok {
			project.Archived = append(project.Archived, id)
		}
	}
	slices.Sort(project.Archived)
	project.Archived = slices.Compact(project.Archived)

	highest := 0
	for _, id := range append(slices.Clone(project.Archived), ticketIDs(project.Tickets)...) {
		if key, number, ok := board.ParseID(id); ok && key == project.Key && number > highest {
			highest = number
		}
	}
	if project.NextID <= highest {
		project.NextID = highest + 1
	}
	// A project with no key is quiet until it owns ids (KEY-5).
	if project.KeyDerived && project.OwnsIDs() && !slices.ContainsFunc(project.Warnings, func(w string) bool { return strings.HasPrefix(w, "key ") }) {
		project.Warnings = append(project.Warnings, fmt.Sprintf("no key in project.yaml; using %s until one is set", project.Key))
	}
	project.LastModified = r.modified
	return project, r.parts, nil
}

func ticketIDs(tickets []board.Ticket) []string {
	ids := make([]string, 0, len(tickets))
	for _, ticket := range tickets {
		ids = append(ids, ticket.ID)
	}
	return ids
}

type projectFile struct {
	Name   string   `yaml:"name"`
	Key    string   `yaml:"key"`
	NextID int      `yaml:"next_id"`
	Repos  []string `yaml:"repos"`
}

func (r *reader) readProjectFile(project *board.Project) {
	project.Key, project.KeyDerived = board.DeriveKey(project.Name), true
	name := path.Join(project.Name, "project.yaml")
	if !r.exists(name) {
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
	switch key := strings.TrimSpace(parsed.Key); {
	case key == "":
	case board.ValidKey(key):
		project.Key, project.KeyDerived = key, false
	default:
		project.Warnings = append(project.Warnings, fmt.Sprintf("key %q in project.yaml is not 2–10 uppercase letters or digits starting with a letter; using %s", key, project.Key))
	}
	if parsed.NextID > 0 {
		project.NextID = parsed.NextID
	}
}

type attachmentEntry struct {
	File    string `yaml:"file"`
	Caption string `yaml:"caption"`
	Kind    string `yaml:"kind"`
	Source  string `yaml:"source"`
	Run     string `yaml:"run"`
	Added   string `yaml:"added"`
}

func (r *reader) readFilesIndex(project *board.Project, id, name string) {
	if !r.exists(name) {
		return
	}
	data, _, problem := r.read(name)
	if problem != "" {
		project.Warnings = append(project.Warnings, fmt.Sprintf("%s files index: %s", id, problem))
		return
	}
	var items []attachmentEntry
	if err := yaml.Unmarshal(data, &items); err != nil {
		project.Warnings = append(project.Warnings, fmt.Sprintf("%s files index is not a list of files", id))
		return
	}
	for _, item := range items {
		if !ValidName(item.File) {
			continue
		}
		project.Attachments[id] = append(project.Attachments[id], board.Attachment{
			File: item.File, Caption: item.Caption, Kind: item.Kind, Source: item.Source, Run: item.Run, Added: item.Added,
		})
	}
}

// ReadReview returns tickets/<folder>/review.md, or found=false when there is
// none.
func (s *Store) ReadReview(project, folder string) (string, bool, error) {
	if !ValidProject(project) || !ValidName(folder) {
		return "", false, fmt.Errorf("review %s/%s: %w", project, folder, ErrInvalidName)
	}
	root, fsys, err := s.handle()
	if err != nil {
		return "", false, err
	}
	name := path.Join(project, "tickets", folder, "review.md")
	if _, err := root.Lstat(name); errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	r := &reader{root: root, fsys: fsys}
	data, _, problem := r.read(name)
	if problem != "" {
		return "", false, fmt.Errorf("review %s/%s: %s", project, folder, problem)
	}
	return string(data), true, nil
}

// OpenAttachment opens tickets/<folder>/files/<file> if its type is allowed
// (REV-2) and returns the content type to serve it with.
func (s *Store) OpenAttachment(project, folder, file string) (*os.File, string, error) {
	if !ValidProject(project) || !ValidName(folder) || !ValidName(file) {
		return nil, "", fmt.Errorf("file %s/%s/%s: %w", project, folder, file, ErrInvalidName)
	}
	contentType, ok := board.AttachmentType(file)
	if !ok {
		return nil, "", fmt.Errorf("file %s: %w", file, ErrTypeNotAllowed)
	}
	root, _, err := s.handle()
	if err != nil {
		return nil, "", err
	}
	handle, err := root.Open(path.Join(project, "tickets", folder, "files", file))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", fmt.Errorf("file %s: %w", file, ErrNotFound)
	}
	if err != nil {
		return nil, "", fmt.Errorf("file %s: %w", file, err)
	}
	if info, err := handle.Stat(); err != nil || !info.Mode().IsRegular() {
		handle.Close()
		return nil, "", fmt.Errorf("file %s: %w", file, ErrNotFound)
	}
	return handle, contentType, nil
}
