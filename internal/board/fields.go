package board

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Typed ticket edits (EDIT-6): the one change-set rule the board's field
// edits and an agent's update_ticket share (FH-85). Each surface decodes
// its own input into FieldChange values; NewFieldChanges checks them;
// store.EditTicket writes them.

// ErrNotEditable is a field typed edits do not change (the raw editor
// can).
var ErrNotEditable = errors.New("not an editable field")

// EditableField reports whether key is an editable field (ScalarFields,
// ListFields) and whether its value is a list.
func EditableField(key string) (list, ok bool) {
	switch {
	case slices.Contains(ListFields, key):
		return true, true
	case slices.Contains(ScalarFields, key):
		return false, true
	}
	return false, false
}

// FieldChange is one typed edit: Value for a scalar field (the title is
// the H1), List for a list field. An empty value clears the field.
type FieldChange struct {
	Key   string
	Value string
	List  []string
}

// FieldChanges is a checked set of field edits, sorted by key. Only
// NewFieldChanges makes a non-empty one, so a FieldChanges always holds
// valid values; the zero value changes nothing.
type FieldChanges struct{ changes []FieldChange }

// NewFieldChanges checks changes for a write: each key editable and set
// once, values and list items trimmed and valid (ValidateField; an item is
// checked before an empty one is dropped). A key that is not editable is
// ErrNotEditable; any other error is ValidateField's.
func NewFieldChanges(changes []FieldChange) (FieldChanges, error) {
	out := make([]FieldChange, 0, len(changes))
	for _, change := range changes {
		list, ok := EditableField(change.Key)
		if !ok {
			return FieldChanges{}, fmt.Errorf("%q: %w", change.Key, ErrNotEditable)
		}
		if slices.ContainsFunc(out, func(c FieldChange) bool { return c.Key == change.Key }) {
			return FieldChanges{}, fmt.Errorf("%s is set twice", change.Key)
		}
		checked := FieldChange{Key: change.Key}
		if list {
			checked.List = make([]string, 0, len(change.List))
			for _, item := range change.List {
				item = strings.TrimSpace(item)
				if err := ValidateField(change.Key, item); err != nil {
					return FieldChanges{}, err
				}
				if item != "" {
					checked.List = append(checked.List, item)
				}
			}
		} else {
			checked.Value = strings.TrimSpace(change.Value)
			if err := ValidateField(change.Key, checked.Value); err != nil {
				return FieldChanges{}, err
			}
		}
		out = append(out, checked)
	}
	slices.SortFunc(out, func(a, b FieldChange) int { return strings.Compare(a.Key, b.Key) })
	return FieldChanges{changes: out}, nil
}

// Len is the number of fields changed.
func (c FieldChanges) Len() int { return len(c.changes) }

// Keys names the fields changed, sorted (what a ticket.updated event
// records).
func (c FieldChanges) Keys() []string {
	keys := make([]string, 0, len(c.changes))
	for _, change := range c.changes {
		keys = append(keys, change.Key)
	}
	return keys
}

// Value is a scalar field's new value and whether it changes.
func (c FieldChanges) Value(key string) (string, bool) {
	for _, change := range c.changes {
		if change.Key == key {
			return change.Value, true
		}
	}
	return "", false
}

// Apply writes every change but the workstream into a ticket's text: the
// title as its H1, the rest as frontmatter. The workstream field changes
// only with the workstreams' tickets lists, which is store's
// (store.EditTicket).
func (c FieldChanges) Apply(data []byte) ([]byte, error) {
	var err error
	for _, change := range c.changes {
		switch list, _ := EditableField(change.Key); {
		case change.Key == "workstream":
			continue
		case change.Key == "title":
			data, err = mdfile.SetTitle(data, change.Value)
		case list:
			data, err = mdfile.SetList(data, change.Key, change.List)
		default:
			data, err = mdfile.SetScalar(data, change.Key, change.Value)
		}
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

// StartsBlocked reports a move that starts a blocked ticket: into In
// progress from another column (EDIT-2). The board then asks for a reason
// and notes it (BlockedStartNote); an agent claims with force instead.
func StartsBlocked(from, to Column, blocked bool) bool {
	return blocked && to == InProgress && from != InProgress
}
