package store

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
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
// writes each changed file atomically in the order it was staged, then
// removes the files it removes (a renamed file's old name). A failure
// before commit writes nothing; a crash during commit leaves the
// files written so far (board-format.md, Multi-file writes).
//
// For a store writing as someone (As), commit first stages one history
// entry in the activity file of each ticket the transaction changed,
// after its other files: what changed in the ticket, its review.md and
// its comments' states (FH-126)
// since the transaction first read them, and what the write states
// besides (created, archived, restored). A history that cannot be staged
// fails the whole write, so a ticket write and its history land together.
type tx struct {
	s     *Store
	files []staged
	// removed are files the transaction removes, after every write
	// (a renamed workstream's old name, FH-158).
	removed []string
	// before holds the ticket, review and activity files the transaction
	// edited (an activity file only for its comments' states), as
	// it first read them; noted holds the changes a write states, by
	// ticket folder; folders orders both by first appearance.
	before  []staged
	noted   map[string][]board.Change
	folders []string
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
	if slices.Contains(t.removed, name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
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
	t.removed = slices.DeleteFunc(t.removed, func(removed string) bool { return removed == name })
	for index := range t.files {
		if t.files[index].name == name {
			t.files[index].data = data
			return
		}
	}
	t.files = append(t.files, staged{name: name, data: data})
}

// rename stages from's content, as the transaction has it, as to's, in
// from's place among the staged files (first, when from is not staged
// yet), and from's removal after every write: a crash part-way leaves the
// new file and the old one, never neither. to must not exist.
func (t *tx) rename(from, to string) error {
	data, err := t.read(from)
	if err != nil {
		return err
	}
	if index := slices.IndexFunc(t.files, func(file staged) bool { return file.name == from }); index >= 0 {
		t.files[index].name = to
	} else {
		t.files = slices.Insert(t.files, 0, staged{name: to, data: data})
	}
	t.removed = append(t.removed, from)
	return nil
}

// remove stages name's removal after every write (a project image given
// up, FH-151); a file staged under name is dropped.
func (t *tx) remove(name string) {
	t.files = slices.DeleteFunc(t.files, func(file staged) bool { return file.name == name })
	if !slices.Contains(t.removed, name) {
		t.removed = append(t.removed, name)
	}
}

// edit applies edit (nil for none) to name under pre and stages the
// result when it changed, stamping a ticket (stamp). It returns the
// file's hash after the transaction.
func (t *tx) edit(name string, pre Precondition, edit Edit, stamp bool) (string, error) {
	current, err := t.require(name, pre)
	if err != nil {
		return "", err
	}
	if stamp || path.Base(name) == reviewFile {
		t.track(name, current)
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

// track keeps a ticket or review file's content before the transaction
// changes it, for its history.
func (t *tx) track(name string, current []byte) {
	if slices.ContainsFunc(t.before, func(file staged) bool { return file.name == name }) {
		return
	}
	t.before = append(t.before, staged{name: name, data: current})
	t.folder(path.Dir(name))
}

// note records a change the write states about the ticket in folder.
func (t *tx) note(folder string, change board.Change) {
	if t.noted == nil {
		t.noted = map[string][]board.Change{}
	}
	t.noted[folder] = append(t.noted[folder], change)
	t.folder(folder)
}

func (t *tx) folder(folder string) {
	if !slices.Contains(t.folders, folder) {
		t.folders = append(t.folders, folder)
	}
}

// commit stages the history of a store writing as someone, writes the
// staged files in order, then removes the files the transaction removes.
func (t *tx) commit() error {
	if err := t.history(); err != nil {
		return err
	}
	for _, file := range t.files {
		if err := t.s.WriteFileAtomic(file.name, file.data); err != nil {
			return err
		}
	}
	if len(t.removed) == 0 {
		return nil
	}
	root, _, err := t.s.handle()
	if err != nil {
		return err
	}
	for _, name := range t.removed {
		if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", name, err)
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

// reviewFile is a ticket's review guide (REV-4).
const reviewFile = "review.md"

// history stages one entry in each changed ticket's activity file.
func (t *tx) history() error {
	if t.s.author == "" {
		return nil
	}
	for _, folder := range t.folders {
		changes := slices.Clone(t.noted[folder])
		for _, file := range t.before {
			if path.Dir(file.name) != folder {
				continue
			}
			after, err := t.read(file.name)
			if err != nil {
				return err
			}
			switch path.Base(file.name) {
			case reviewFile:
				changes = append(changes, board.ReviewChanges(string(file.data), string(after))...)
			case board.ActivityFile:
				changes = append(changes, board.CommentChanges(file.data, after)...)
			default:
				changes = append(changes, board.TicketChanges(file.data, after)...)
			}
		}
		if len(changes) == 0 {
			continue
		}
		if err := t.appendHistory(folder, changes); err != nil {
			return err
		}
	}
	return nil
}

// appendHistory stages a history entry in folder's activity file. A file
// already at the size limit takes no more history: the ticket's writes go
// on without it rather than fail for good.
func (t *tx) appendHistory(folder string, changes []board.Change) error {
	name := path.Join(folder, board.ActivityFile)
	data, err := t.readActivity(name)
	switch {
	case errors.Is(err, ErrTooLarge):
		return nil
	case err != nil:
		return err
	}
	id, err := newEntryID(board.ParseActivity(data))
	if err != nil {
		return err
	}
	ticket, _, _ := board.ParseFolder(path.Base(folder))
	next := board.AppendHistory(data, ticket, board.History{ID: id, Time: t.s.now(), By: t.s.author, Changes: changes})
	if len(next) > MaxFileBytes {
		return nil
	}
	t.put(name, next)
	return nil
}

// editActivity applies edit to an activity file's comments (their
// states) as the transaction has left it, a missing file being empty,
// and stages the result when it changed; its history names each
// comment whose state changed. A result over MaxFileBytes is
// ErrTooLarge.
func (t *tx) editActivity(name string, edit Edit) error {
	current, err := t.readActivity(name)
	if err != nil {
		return err
	}
	t.track(name, current)
	next, err := edit(current)
	if err != nil {
		return err
	}
	if bytes.Equal(next, current) {
		return nil
	}
	if len(next) > MaxFileBytes {
		return fmt.Errorf("the ticket's activity is limited to %d KiB: %w", MaxFileBytes>>10, ErrTooLarge)
	}
	t.put(name, next)
	return nil
}

// readActivity reads an activity file as the transaction has left it; a
// missing one is empty.
func (t *tx) readActivity(name string) ([]byte, error) {
	data, err := t.read(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// newEntryID is an activity entry id the file does not use: eight random
// hex digits, so writers on two copies of a synced board do not collide.
func newEntryID(activity board.Activity) (string, error) {
	used := activity.IDs()
	for {
		random := make([]byte, 4)
		if _, err := rand.Read(random); err != nil {
			return "", err
		}
		if id := hex.EncodeToString(random); !slices.Contains(used, id) {
			return id, nil
		}
	}
}
