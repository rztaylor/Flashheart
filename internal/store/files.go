package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
)

// The functions below are confined file primitives for whole-board
// operations such as migration. Paths are slash-separated and relative to the
// root; an os.Root keeps every access inside it (SEC-2). Ticket-level writes
// with locks and hash preconditions are in write.go.

// WithRootLock runs fn holding the root lock and the locks of the named
// projects, taken in name order as every multi-project writer takes them,
// so a whole-board operation such as migration writes through the
// primitives below while no other writer changes the root. The locks are
// not reentrant: fn must not call a store method that takes them itself.
func (s *Store) WithRootLock(projects []string, fn func() error) error {
	releaseRoot, err := s.lock(".")
	if err != nil {
		return err
	}
	defer releaseRoot()
	names := slices.Clone(projects)
	slices.Sort(names)
	for _, name := range slices.Compact(names) {
		release, err := s.lock(name)
		if err != nil {
			return err
		}
		defer release()
	}
	return fn()
}

// ReadFile reads a file of at most MaxFileBytes.
func (s *Store) ReadFile(name string) ([]byte, error) {
	root, _, err := s.handle()
	if err != nil {
		return nil, err
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileBytes {
		return nil, fmt.Errorf("%s is larger than %d KiB", name, MaxFileBytes>>10)
	}
	return data, nil
}

// ReadDir lists a directory.
func (s *Store) ReadDir(name string) ([]fs.DirEntry, error) {
	_, fsys, err := s.handle()
	if err != nil {
		return nil, err
	}
	return fs.ReadDir(fsys, name)
}

// Exists reports whether name exists (without following a final symlink).
func (s *Store) Exists(name string) bool {
	root, _, err := s.handle()
	if err != nil {
		return false
	}
	_, err = root.Lstat(name)
	return err == nil
}

// IsDir reports whether name is a directory.
func (s *Store) IsDir(name string) bool {
	_, fsys, err := s.handle()
	return err == nil && isDir(fsys, name)
}

// WriteFileAtomic writes data to name through a temporary file in the same
// directory, synced and renamed into place (STO-3), creating parent
// directories as needed.
func (s *Store) WriteFileAtomic(name string, data []byte) error {
	root, _, err := s.handle()
	if err != nil {
		return err
	}
	if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", path.Dir(name), err)
	}
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	temporary := path.Join(path.Dir(name), "."+path.Base(name)+".tmp-"+hex.EncodeToString(suffix))
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		_ = root.Remove(temporary)
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := root.Rename(temporary, name); err != nil {
		_ = root.Remove(temporary)
		return fmt.Errorf("write %s: %w", name, err)
	}
	s.writtenMu.Lock()
	defer s.writtenMu.Unlock()
	if s.written == nil {
		s.written = map[string]string{}
	}
	s.written[path.Clean(name)] = Hash(data)
	return nil
}

// Doc names one of a live ticket's documents.
type Doc int

const (
	// TicketDoc is the ticket's own file.
	TicketDoc Doc = iota
	// ReviewDoc is the ticket's review.md (REV-4).
	ReviewDoc
)

// WroteLast reports whether data is what this store last wrote to a live
// ticket's document, so a reader can tell its own writes from edits made
// elsewhere (REV-5). A ticket that cannot be found never matches.
func (s *Store) WroteLast(project, id string, doc Doc, data []byte) bool {
	name, err := s.TicketFile(project, id)
	if err != nil {
		return false
	}
	if doc == ReviewDoc {
		name = path.Join(path.Dir(name), "review.md")
	}
	s.writtenMu.Lock()
	defer s.writtenMu.Unlock()
	hash, ok := s.written[path.Clean(name)]
	return ok && hash == Hash(data)
}

// Move renames oldname to newname, creating newname's parent directories. It
// refuses to replace anything that already exists at newname.
func (s *Store) Move(oldname, newname string) error {
	root, _, err := s.handle()
	if err != nil {
		return err
	}
	if _, err := root.Lstat(newname); err == nil {
		return fmt.Errorf("move %s: %s already exists", oldname, newname)
	}
	if err := root.MkdirAll(path.Dir(newname), 0o755); err != nil {
		return fmt.Errorf("move %s: %w", oldname, err)
	}
	if err := root.Rename(oldname, newname); err != nil {
		return fmt.Errorf("move %s: %w", oldname, err)
	}
	return nil
}
