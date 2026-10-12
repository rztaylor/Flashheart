package store

import (
	"bytes"
	"fmt"
	"time"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Precondition is what a write requires of the file it changes (STO-3).
// The zero value requires a hash and has none, so it refuses: a caller
// says which kind of write it makes.
type Precondition struct {
	hash  string
	merge bool
}

// Unchanged requires the file to still have hash, the content the caller
// read; an edit built from what the caller saw (a form, the raw file)
// would otherwise undo a change made since. An empty hash is refused.
func Unchanged(hash string) Precondition { return Precondition{hash: hash} }

// Merge requires nothing: the edit computes its change from the file as
// it is under the lock (a status, a tick, a note, a list entry), so
// another writer's change is kept, not lost.
var Merge = Precondition{merge: true}

func (p Precondition) check(name string, current []byte) error {
	if p.merge {
		return nil
	}
	if p.hash == "" {
		return fmt.Errorf("%s: %w: an edit must carry the hash of the content it was based on", name, ErrInvalidInput)
	}
	if hash := Hash(current); hash != p.hash {
		return &ConflictError{Name: name, Hash: hash, Current: current}
	}
	return nil
}

// tx is one write of a project's files under its lock. Each step reads a
// file as the transaction has left it, checks its precondition, applies an
// edit and stamps a changed ticket; nothing is written until commit, which
// writes each changed file atomically in the order it was staged. A
// failure before commit writes nothing; a crash during commit leaves the
// files written so far (board-format.md, Multi-file writes).
type tx struct {
	s     *Store
	files []staged
}

type staged struct {
	name string
	data []byte
}

// begin starts a transaction; the caller holds the project lock.
func (s *Store) begin() *tx { return &tx{s: s} }

// read returns name's content as the transaction would leave it.
func (t *tx) read(name string) ([]byte, error) {
	for index := len(t.files) - 1; index >= 0; index-- {
		if t.files[index].name == name {
			return t.files[index].data, nil
		}
	}
	return t.s.ReadFile(name)
}

// require reads name and checks pre against it.
func (t *tx) require(name string, pre Precondition) ([]byte, error) {
	current, err := t.read(name)
	if err != nil {
		return nil, err
	}
	return current, pre.check(name, current)
}

// put stages data as name's content, created or replaced.
func (t *tx) put(name string, data []byte) {
	for index := range t.files {
		if t.files[index].name == name {
			t.files[index].data = data
			return
		}
	}
	t.files = append(t.files, staged{name: name, data: data})
}

// edit applies edit (nil for none) to name under pre and stages the
// result when it changed, stamping a ticket (stamp). It returns the
// file's hash after the transaction.
func (t *tx) edit(name string, pre Precondition, edit Edit, stamp bool) (string, error) {
	current, err := t.require(name, pre)
	if err != nil {
		return "", err
	}
	next := current
	if edit != nil {
		if next, err = edit(current); err != nil {
			return "", err
		}
	}
	if bytes.Equal(next, current) {
		return Hash(current), nil
	}
	if stamp {
		next = t.s.stamp(next)
	}
	t.put(name, next)
	return Hash(next), nil
}

// commit writes the staged files in order.
func (t *tx) commit() error {
	for _, file := range t.files {
		if err := t.s.WriteFileAtomic(file.name, file.data); err != nil {
			return err
		}
	}
	return nil
}

// stamp sets a changed ticket's `updated:` to now. A ticket whose
// frontmatter does not parse cannot hold it and is written as the edit
// left it; this is the one stamping rule for every ticket write.
func (s *Store) stamp(data []byte) []byte {
	if mdfile.Parse(data).FrontmatterError != nil {
		return data
	}
	stamped, err := mdfile.SetScalar(data, "updated", s.now().Format(time.RFC3339))
	if err != nil {
		return data
	}
	return stamped
}
