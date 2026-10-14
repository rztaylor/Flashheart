package board

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Workstream edits (FH-46): the one rule for which of a workstream's
// fields an edit sets and how each is checked, as FieldChanges is a
// ticket's. A surface decodes its input into FieldChange values;
// NewWorkstreamChanges checks them; store.EditWorkstream writes them with
// the workstream's membership changes. The slug is not a field: a new
// title renames the workstream as people see it, and every reference to
// it stays good. A slug changes only by an explicit rename
// (store.WorkstreamEdit.Rename, FH-158), which rewrites every reference.

// WorkstreamFields are the fields a workstream edit sets: its title (the
// H1), its goal (the ## Goal section), priority, tags and
// depends-on-workstreams. Membership changes with the tickets that join
// and leave it, never as a field.
var WorkstreamFields = []string{"title", "goal", "priority", "tags", "depends-on-workstreams"}

// WorkstreamField reports whether key is one of WorkstreamFields and
// whether its value is a list.
func WorkstreamField(key string) (list, ok bool) {
	return workstreamList(key), slices.Contains(WorkstreamFields, key)
}

// workstreamList reports whether a workstream field's value is a list.
func workstreamList(key string) bool { return key == "tags" || key == "depends-on-workstreams" }

// WorkstreamChanges is a checked set of workstream field edits, sorted by
// key. Only NewWorkstreamChanges makes a non-empty one; the zero value
// changes nothing.
type WorkstreamChanges struct {
	fields []FieldChange
	goal   *SectionChange
}

// NewWorkstreamChanges checks changes for a write: each key one of
// WorkstreamFields and set once, values and list items trimmed and
// checked as a ticket's same field is (ValidateField), the goal as a
// section (NewWorkstreamSectionChange). Any other key, the slug and
// status included, is ErrNotEditable.
func NewWorkstreamChanges(changes []FieldChange) (WorkstreamChanges, error) {
	var out WorkstreamChanges
	for _, change := range changes {
		if !slices.Contains(WorkstreamFields, change.Key) {
			return WorkstreamChanges{}, fmt.Errorf("%q: %w of a workstream; its fields are %s", change.Key, ErrNotEditable, strings.Join(WorkstreamFields, ", "))
		}
		if slices.ContainsFunc(out.fields, func(c FieldChange) bool { return c.Key == change.Key }) {
			return WorkstreamChanges{}, fmt.Errorf("%s is set twice", change.Key)
		}
		checked := FieldChange{Key: change.Key}
		switch {
		case change.Key == "goal":
			section, err := NewWorkstreamSectionChange(SectionGoal, change.Value)
			if err != nil {
				return WorkstreamChanges{}, fmt.Errorf("goal: %w", err)
			}
			out.goal = &section
		case workstreamList(change.Key):
			checked.List = make([]string, 0, len(change.List))
			for _, item := range change.List {
				item = strings.TrimSpace(item)
				if err := ValidateField(change.Key, item); err != nil {
					return WorkstreamChanges{}, err
				}
				if item != "" {
					checked.List = append(checked.List, item)
				}
			}
		default:
			checked.Value = strings.TrimSpace(change.Value)
			if err := ValidateField(change.Key, checked.Value); err != nil {
				return WorkstreamChanges{}, err
			}
		}
		out.fields = append(out.fields, checked)
	}
	slices.SortFunc(out.fields, func(a, b FieldChange) int { return strings.Compare(a.Key, b.Key) })
	return out, nil
}

// Len is the number of fields changed.
func (c WorkstreamChanges) Len() int { return len(c.fields) }

// Keys names the fields changed, sorted.
func (c WorkstreamChanges) Keys() []string {
	keys := make([]string, 0, len(c.fields))
	for _, change := range c.fields {
		keys = append(keys, change.Key)
	}
	return keys
}

// List is a list field's new items and whether it changes.
func (c WorkstreamChanges) List(key string) ([]string, bool) {
	for _, change := range c.fields {
		if change.Key == key && workstreamList(key) {
			return change.List, true
		}
	}
	return nil, false
}

// Apply writes every change into a workstream's text: the title as its
// H1, the goal as its ## Goal section, the rest as frontmatter, leaving
// every other byte as it was. A value equal to the current one changes
// nothing.
func (c WorkstreamChanges) Apply(data []byte) ([]byte, error) {
	var err error
	for _, change := range c.fields {
		switch {
		case change.Key == "goal":
			data, err = c.goal.Apply(data)
		case change.Key == "title":
			data, err = mdfile.SetTitle(data, change.Value)
		case workstreamList(change.Key):
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

// WorkstreamTitleKey is what a workstream's title is compared by: its
// words in lower case, with every run of spaces as one (FH-163, D76). A
// project's workstreams have distinct keys, so no two read the same in a
// picker.
func WorkstreamTitleKey(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(title), " "))
}

// SameWorkstreamTitle reports whether two workstream titles read the same
// (WorkstreamTitleKey).
func SameWorkstreamTitle(a, b string) bool { return WorkstreamTitleKey(a) == WorkstreamTitleKey(b) }

// CycleError is a workstream dependency that would lead back to the
// workstream: Path runs from it through its dependencies to itself.
type CycleError struct{ Path []string }

func (e *CycleError) Error() string {
	return "workstream dependencies would form a cycle: " + strings.Join(e.Path, " → ")
}

// WorkstreamCycle reports whether giving slug the dependencies dependsOn
// would make a cycle, where dependenciesOf gives every other workstream's
// depends-on-workstreams (nil for a missing one). The cycle found first,
// in the order of dependsOn, is a *CycleError; a cycle that does not
// pass through slug is not this edit's, and is left alone.
func WorkstreamCycle(slug string, dependsOn []string, dependenciesOf func(string) []string) error {
	seen := map[string]bool{}
	var walk func(path []string, next []string) []string
	walk = func(path []string, next []string) []string {
		for _, dependency := range next {
			if dependency == slug {
				return append(slices.Clone(path), slug)
			}
			if seen[dependency] {
				continue
			}
			seen[dependency] = true
			if found := walk(append(path, dependency), dependenciesOf(dependency)); found != nil {
				return found
			}
		}
		return nil
	}
	if path := walk([]string{slug}, dependsOn); path != nil {
		return &CycleError{Path: path}
	}
	return nil
}
