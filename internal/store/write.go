package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Ticket and project writes (STO-3). Each takes the project's advisory lock
// (<project>/.flashheart/lock), re-reads the file under it, checks the
// caller's content hash, and writes atomically. Choosing a key or creating a
// ticket also takes the root lock (<root>/.flashheart/lock) first, so keys
// stay unique across the root (KEY-5); locks are always taken root first.

// LockWait bounds how long a write waits for a lock held by another writer.
const LockWait = 5 * time.Second

var (
	// ErrConflict reports a file that changed since the caller read it; the
	// error is a *ConflictError carrying the current content.
	ErrConflict = errors.New("changed since it was read")
	// ErrBusy reports a lock still held after LockWait.
	ErrBusy = errors.New("the board is busy; try again")
	// ErrKeyTaken reports a project key already used; see *KeyTakenError.
	ErrKeyTaken = errors.New("project key is taken")
	// ErrKeyFixed reports a key change on a project that already has tickets.
	ErrKeyFixed = errors.New("the project already has tickets, so its key is fixed")
	// ErrInvalidInput reports a value that fails validation.
	ErrInvalidInput = errors.New("invalid input")
)

// ConflictError carries the file as it is now, so the caller can show both
// versions (EDIT-7).
type ConflictError struct {
	Name    string
	Hash    string
	Current []byte
}

func (e *ConflictError) Error() string { return e.Name + " " + ErrConflict.Error() }

// Unwrap lets errors.Is match ErrConflict.
func (e *ConflictError) Unwrap() error { return ErrConflict }

// KeyTakenError lists the keys in use so the caller can choose another.
type KeyTakenError struct {
	Key   string
	InUse []string
}

func (e *KeyTakenError) Error() string {
	return fmt.Sprintf("project key %s is taken; keys in use: %s", e.Key, strings.Join(e.InUse, ", "))
}

// Unwrap lets errors.Is match ErrKeyTaken.
func (e *KeyTakenError) Unwrap() error { return ErrKeyTaken }

// Hash is the content hash used as a write precondition.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SetClock replaces the clock used for created dates and updated stamps.
func (s *Store) SetClock(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clock = now
}

func (s *Store) now() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clock != nil {
		return s.clock().UTC()
	}
	return time.Now().UTC()
}

// lock takes the advisory lock in dir/.flashheart ("." is the root) and
// returns its release.
func (s *Store) lock(dir string) (func(), error) {
	root, _, err := s.handle()
	if err != nil {
		return nil, err
	}
	name := path.Join(dir, ".flashheart", "lock")
	if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
		return nil, fmt.Errorf("lock %s: %w", dir, err)
	}
	file, err := openLockFile(root, name)
	if err != nil {
		return nil, fmt.Errorf("lock %s: %w", dir, err)
	}
	if err := flock(file, LockWait); err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		_ = funlock(file)
		file.Close()
	}, nil
}

// openLockFile opens the lock file, creating it if needed. Concurrent first
// creation through an os.Root can briefly report the file missing, so it
// retries open-then-exclusive-create a few times.
func openLockFile(root *os.Root, name string) (*os.File, error) {
	var err error
	for range 20 {
		var file *os.File
		if file, err = root.OpenFile(name, os.O_RDWR, 0); err == nil {
			return file, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if file, err = root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644); err == nil {
			return file, nil
		}
		time.Sleep(time.Millisecond)
	}
	return nil, err
}

// checkTicket validates a project and ticket id before any lock is taken.
func (s *Store) checkTicket(project, id string) error {
	if err := s.checkProject(project); err != nil {
		return err
	}
	if _, _, ok := board.ParseID(id); !ok {
		return fmt.Errorf("ticket %q: %w", id, ErrInvalidName)
	}
	return nil
}

func (s *Store) checkProject(project string) error {
	if !ValidProject(project) {
		return fmt.Errorf("project %q: %w", project, ErrInvalidName)
	}
	_, fsys, err := s.handle()
	if err != nil {
		return err
	}
	if !isProject(fsys, project) {
		return fmt.Errorf("project %q: %w", project, ErrNotFound)
	}
	return nil
}

// TicketFile returns the path of a live ticket's file, relative to the root.
func (s *Store) TicketFile(project, id string) (string, error) {
	return s.ticketFile(project, path.Join(project, "tickets"), id)
}

func (s *Store) ticketFile(project, dir, id string) (string, error) {
	if err := s.checkProject(project); err != nil {
		return "", err
	}
	if _, _, ok := board.ParseID(id); !ok {
		return "", fmt.Errorf("ticket %q: %w", id, ErrInvalidName)
	}
	entries, err := s.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	for _, entry := range entries {
		folderID, _, ok := board.ParseFolder(entry.Name())
		if !ok || folderID != id || !entry.IsDir() {
			continue
		}
		files, err := s.ReadDir(path.Join(dir, entry.Name()))
		if err != nil {
			return "", err
		}
		names := make([]string, 0, len(files))
		for _, file := range files {
			names = append(names, file.Name())
		}
		if name, _, ok := board.TicketFile(entry.Name(), names); ok {
			return path.Join(dir, entry.Name(), name), nil
		}
	}
	return "", fmt.Errorf("ticket %s in %s: %w", id, project, ErrNotFound)
}

// ReadTicket returns a live ticket's file and its content hash.
func (s *Store) ReadTicket(project, id string) ([]byte, string, error) {
	name, err := s.TicketFile(project, id)
	if err != nil {
		return nil, "", err
	}
	data, err := s.ReadFile(name)
	if err != nil {
		return nil, "", err
	}
	return data, Hash(data), nil
}

// Edit computes a file's new content from its current content.
type Edit func(current []byte) ([]byte, error)

// UpdateTicket applies edit to a ticket under the project lock. A non-empty
// base must match the file's current hash or the result is a *ConflictError.
// A changed ticket gets `updated:` stamped; an edit that changes nothing
// writes nothing. It returns the file's hash after the call.
func (s *Store) UpdateTicket(project, id, base string, edit Edit) (string, error) {
	if err := s.checkTicket(project, id); err != nil {
		return "", err
	}
	release, err := s.lock(project)
	if err != nil {
		return "", err
	}
	defer release()
	name, err := s.TicketFile(project, id)
	if err != nil {
		return "", err
	}
	return s.updateLocked(name, base, edit, true)
}

// UpdateWorkstream applies edit to workstreams/<slug>.md under the project
// lock, with the same precondition as UpdateTicket (EDIT-4).
func (s *Store) UpdateWorkstream(project, slug, base string, edit Edit) (string, error) {
	if err := s.checkProject(project); err != nil {
		return "", err
	}
	if !ValidName(slug) {
		return "", fmt.Errorf("workstream %q: %w", slug, ErrInvalidName)
	}
	release, err := s.lock(project)
	if err != nil {
		return "", err
	}
	defer release()
	name := path.Join(project, "workstreams", slug+".md")
	if !s.Exists(name) {
		return "", fmt.Errorf("workstream %s in %s: %w", slug, project, ErrNotFound)
	}
	return s.updateLocked(name, base, edit, false)
}

func (s *Store) updateLocked(name, base string, edit Edit, stamp bool) (string, error) {
	current, err := s.ReadFile(name)
	if err != nil {
		return "", err
	}
	hash := Hash(current)
	if base != "" && base != hash {
		return "", &ConflictError{Name: name, Hash: hash, Current: current}
	}
	next, err := edit(current)
	if err != nil {
		return "", err
	}
	if bytes.Equal(next, current) {
		return hash, nil
	}
	if stamp && mdfile.Parse(next).FrontmatterError == nil {
		if stamped, err := mdfile.SetScalar(next, "updated", s.now().Format(time.RFC3339)); err == nil {
			next = stamped
		}
	}
	if err := s.WriteFileAtomic(name, next); err != nil {
		return "", err
	}
	return Hash(next), nil
}

// NewTicket is the content of a ticket created from the template (EDIT-5).
type NewTicket struct {
	Title       string
	Type        string
	Priority    string
	Status      board.Column
	Workstream  string
	Description string
	Criteria    []string
	DependsOn   []string
	Tags        []string
	Session     string
	// PlanOrRepro fills the Test Plan section, or Reproduction for a bug.
	PlanOrRepro string
	// Key is used only when the project has no key yet (KEY-5).
	Key string
}

// Created reports a new ticket.
type Created struct {
	ID     string
	Folder string
	Hash   string
	// Key is set when creating the ticket also chose the project's key.
	Key string
}

func (t *NewTicket) validate() error {
	t.Title = strings.Join(strings.Fields(t.Title), " ")
	if t.Title == "" {
		return fmt.Errorf("%w: a ticket needs a title", ErrInvalidInput)
	}
	if t.Type == "" {
		t.Type = "feature"
	}
	if !slices.Contains(board.TicketTypes, t.Type) {
		return fmt.Errorf("%w: type must be one of %s", ErrInvalidInput, strings.Join(board.TicketTypes, ", "))
	}
	if t.Priority == "" {
		t.Priority = "medium"
	}
	if !slices.Contains(board.Priorities, t.Priority) {
		return fmt.Errorf("%w: priority must be one of %s", ErrInvalidInput, strings.Join(board.Priorities, ", "))
	}
	if t.Status == "" {
		t.Status = board.Backlog
	}
	if _, ok := board.ParseColumn(string(t.Status)); !ok {
		return fmt.Errorf("%w: status %q is not a column", ErrInvalidInput, t.Status)
	}
	if t.Workstream != "" && !ValidName(t.Workstream) {
		return fmt.Errorf("%w: workstream %q", ErrInvalidInput, t.Workstream)
	}
	for _, id := range t.DependsOn {
		if _, _, ok := board.ParseID(id); !ok {
			return fmt.Errorf("%w: depends-on %q is not a ticket id", ErrInvalidInput, id)
		}
	}
	return nil
}

// CreateTicket writes a new ticket folder with the project's next id
// (KEY-2), choosing and recording the project's key first when it has none
// (KEY-5), then advances next_id. A ticket with a workstream is appended to
// that workstream's tickets list; a missing one is a
// *WorkstreamNotFoundError and nothing is written.
func (s *Store) CreateTicket(project string, input NewTicket) (Created, error) {
	if err := s.checkProject(project); err != nil {
		return Created{}, err
	}
	if err := input.validate(); err != nil {
		return Created{}, err
	}
	releaseRoot, err := s.lock(".")
	if err != nil {
		return Created{}, err
	}
	defer releaseRoot()
	release, err := s.lock(project)
	if err != nil {
		return Created{}, err
	}
	defer release()

	info, err := s.ReadProject(project)
	if err != nil {
		return Created{}, err
	}
	var created Created
	key, number := info.Key, info.NextID
	// A derived key is only a suggestion until the project owns ids; once it
	// does, the derived key is the project's key and is recorded as is.
	if info.KeyDerived && !info.OwnsIDs() {
		key, err = s.chooseKey(project, input.Key)
		if err != nil {
			return Created{}, err
		}
		created.Key = key
	}
	created.ID = key + "-" + strconv.Itoa(number)
	created.Folder = created.ID + "-" + Slugify(input.Title)
	name := path.Join(project, "tickets", created.Folder, created.Folder+".md")
	if s.Exists(path.Dir(name)) {
		return Created{}, fmt.Errorf("ticket folder %s already exists", created.Folder)
	}
	data, err := s.ticketTemplate(created.ID, input)
	if err != nil {
		return Created{}, err
	}
	// Prepare project.yaml first, so a file that cannot be edited stops the
	// create before anything is written.
	projectFile, err := s.projectFields(project, key, number+1)
	if err != nil {
		return Created{}, err
	}
	// So is a missing workstream: the ticket joins its list (board-format
	// §Blocking) or is not created.
	var lists []pending
	if input.Workstream != "" {
		if lists, err = s.membership(project, []string{created.ID}, input.Workstream); err != nil {
			return Created{}, err
		}
	}
	if err := s.WriteFileAtomic(name, data); err != nil {
		return Created{}, err
	}
	created.Hash = Hash(data)
	if err := projectFile.write(s); err != nil {
		return created, err
	}
	return created, writeAll(s, lists)
}

func (s *Store) ticketTemplate(id string, t NewTicket) ([]byte, error) {
	now := s.now()
	body := []string{"# " + t.Title, "", "## Description", ""}
	if description := strings.TrimSpace(t.Description); description != "" {
		body = append(body, description, "")
	}
	body = append(body, "## Acceptance Criteria", "")
	criteria := 0
	for _, criterion := range t.Criteria {
		if criterion = strings.Join(strings.Fields(criterion), " "); criterion != "" {
			body = append(body, "- [ ] "+criterion)
			criteria++
		}
	}
	if criteria > 0 {
		body = append(body, "")
	}
	if t.Type == "bug" {
		body = append(body, "## Reproduction", "")
	} else {
		body = append(body, "## Test Plan", "")
	}
	if plan := strings.TrimSpace(t.PlanOrRepro); plan != "" {
		body = append(body, plan, "")
	}
	body = append(body, "## Context", "", "## Notes", "", "None.", "")
	data := []byte("---\n---\n" + strings.Join(body, "\n"))

	steps := []struct {
		key    string
		value  string
		list   []string
		isList bool
	}{
		{key: "id", value: id},
		{key: "status", value: string(t.Status)},
		{key: "type", value: t.Type},
		{key: "priority", value: t.Priority},
		{key: "created", value: now.Format("2006-01-02")},
		{key: "branch"},
		{key: "workstream", value: t.Workstream},
		{key: "depends-on", list: t.DependsOn, isList: true},
		{key: "depends-on-workstreams", isList: true},
		{key: "tags", list: t.Tags, isList: true},
		{key: "session", value: t.Session},
		{key: "updated", value: now.Format(time.RFC3339)},
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
	return data, nil
}

// Slugify makes a folder slug from a title: lowercase words joined by
// hyphens, at most 48 characters, cut at a word boundary.
func Slugify(title string) string { return slugify(title, "ticket") }

// slugify is Slugify with the slug used when the title has no ASCII words.
func slugify(title, fallback string) string {
	var words []string
	for _, word := range strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !(r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)))
	}) {
		words = append(words, word)
	}
	slug := ""
	for _, word := range words {
		next := word
		if slug != "" {
			next = slug + "-" + word
		}
		if len(next) > 48 {
			break
		}
		slug = next
	}
	if slug == "" && len(words) > 0 {
		slug = words[0][:min(len(words[0]), 48)]
	}
	if slug == "" {
		return fallback
	}
	return slug
}

// keyOwner describes a project's key for uniqueness checks.
type keyOwner struct {
	project string
	key     string
}

// keysInUse lists the keys other projects hold: explicit keys, and derived
// keys of projects that already own ids.
func (s *Store) keysInUse(except string) ([]keyOwner, error) {
	names, err := s.Projects()
	if err != nil {
		return nil, err
	}
	var owners []keyOwner
	for _, name := range names {
		if name == except {
			continue
		}
		info, err := s.ReadProject(name)
		if err != nil {
			continue
		}
		if !info.KeyDerived || info.OwnsIDs() {
			owners = append(owners, keyOwner{project: name, key: info.Key})
		}
	}
	return owners, nil
}

// chooseKey validates a requested key, or derives a free one, under the root
// lock. Callers hold the root lock.
func (s *Store) chooseKey(project, requested string) (string, error) {
	owners, err := s.keysInUse(project)
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	var inUse []string
	for _, owner := range owners {
		taken[owner.key] = true
		inUse = append(inUse, owner.key)
	}
	slices.Sort(inUse)
	if requested != "" {
		if !board.ValidKey(requested) {
			return "", fmt.Errorf("%w: key %q must be 2–10 uppercase letters or digits starting with a letter", ErrInvalidInput, requested)
		}
		if taken[requested] {
			return "", &KeyTakenError{Key: requested, InUse: inUse}
		}
		return requested, nil
	}
	base := board.DeriveKey(project)
	if !taken[base] {
		return base, nil
	}
	for n := 2; ; n++ {
		candidate := base + strconv.Itoa(n)
		if len(candidate) > 10 {
			candidate = base[:10-len(strconv.Itoa(n))] + strconv.Itoa(n)
		}
		if !taken[candidate] {
			return candidate, nil
		}
	}
}

// SetProjectKey records a project's key under the root lock. It is refused
// once the project has tickets, live or archived (KEY-5).
func (s *Store) SetProjectKey(project, key string) error {
	if err := s.checkProject(project); err != nil {
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
	info, err := s.ReadProject(project)
	if err != nil {
		return err
	}
	if info.OwnsIDs() {
		if !info.KeyDerived && info.Key == key {
			return nil
		}
		return ErrKeyFixed
	}
	if key == "" {
		return fmt.Errorf("%w: a key is required", ErrInvalidInput)
	}
	chosen, err := s.chooseKey(project, key)
	if err != nil {
		return err
	}
	return s.setProjectFields(project, chosen, 0)
}

// projectEdit is project.yaml's new content, written only when it changed.
type projectEdit struct {
	name    string
	data    []byte
	changed bool
}

func (e projectEdit) write(s *Store) error {
	if !e.changed {
		return nil
	}
	return s.WriteFileAtomic(e.name, e.data)
}

// projectFields computes project.yaml with key and, when positive, next_id
// set, keeping its other lines and line endings.
func (s *Store) projectFields(project, key string, nextID int) (projectEdit, error) {
	name := path.Join(project, "project.yaml")
	data, err := s.ReadFile(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return projectEdit{}, err
	}
	crlf := bytes.Contains(data, []byte("\r\n"))
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	// project.yaml is plain YAML; edit it as frontmatter of an empty body.
	doc := []byte("---\n" + text + "---\n")
	if doc, err = mdfile.SetScalar(doc, "key", key); err != nil {
		return projectEdit{}, fmt.Errorf("%s: %w", name, err)
	}
	if nextID > 0 {
		if doc, err = mdfile.SetScalar(doc, "next_id", strconv.Itoa(nextID)); err != nil {
			return projectEdit{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	out := strings.TrimSuffix(strings.TrimPrefix(string(doc), "---\n"), "---\n")
	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return projectEdit{name: name, data: []byte(out), changed: out != string(data)}, nil
}

// setProjectFields writes key and next_id into project.yaml.
func (s *Store) setProjectFields(project, key string, nextID int) error {
	edit, err := s.projectFields(project, key, nextID)
	if err != nil {
		return err
	}
	return edit.write(s)
}

// Archive moves a ticket's folder to <project>/.archive/tickets/ (EDIT-8).
func (s *Store) Archive(project, id string) error {
	return s.moveTicket(project, id, path.Join(project, "tickets"), path.Join(project, ".archive", "tickets"))
}

// Unarchive moves an archived ticket's folder back to tickets/.
func (s *Store) Unarchive(project, id string) error {
	return s.moveTicket(project, id, path.Join(project, ".archive", "tickets"), path.Join(project, "tickets"))
}

func (s *Store) moveTicket(project, id, from, to string) error {
	if err := s.checkTicket(project, id); err != nil {
		return err
	}
	release, err := s.lock(project)
	if err != nil {
		return err
	}
	defer release()
	file, err := s.ticketFile(project, from, id)
	if err != nil {
		return err
	}
	folder := path.Dir(file)
	return s.Move(folder, path.Join(to, path.Base(folder)))
}

// ConfigFile is the global settings file, relative to the root (CFG-1).
const ConfigFile = ".flashheart/config.yaml"

// ReadConfig returns config.yaml, or nil when there is none.
func (s *Store) ReadConfig() ([]byte, error) {
	data, err := s.ReadFile(ConfigFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// UpdateConfig applies edit to config.yaml under the root lock (CFG-2).
func (s *Store) UpdateConfig(edit Edit) error {
	release, err := s.lock(".")
	if err != nil {
		return err
	}
	defer release()
	current, err := s.ReadConfig()
	if err != nil {
		return err
	}
	next, err := edit(current)
	if err != nil {
		return err
	}
	if bytes.Equal(next, current) {
		return nil
	}
	return s.WriteFileAtomic(ConfigFile, next)
}
