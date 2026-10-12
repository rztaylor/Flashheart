package store

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// projectFile is a project's project.yaml (PRJ-3) as parseProjectFile reads
// it, the one reader every read of that file goes through; its edits go
// through mdfile's plain-YAML editor (SetYAMLScalar, SetYAMLList), which
// keeps the file's other lines, line endings and byte-order mark (FH-77).
type projectFile struct {
	Name string
	// Key is a valid key (board.ValidKey), or "" when there is none.
	Key    string
	NextID int
	Repos  []string
	// Retired lists permanently deleted ticket ids (EDIT-8).
	Retired []string
	// Settings are per-project overrides of config.yaml (CFG-1).
	Settings projectSettings
	// Warnings are the problems found, worded for the board. A field with
	// a problem keeps its zero value; a file that does not parse has none.
	Warnings []string
	// unread holds the fields that could not be read; "" is the whole file.
	unread []string
}

type projectSettings struct {
	EnforceHandoff *bool `yaml:"enforce_handoff"`
}

// parseProjectFile reads project.yaml's content for the named project. A
// list may be written as a single value.
func parseProjectFile(project string, data []byte) projectFile {
	var f projectFile
	doc := mdfile.ParseYAML(data)
	if doc.FrontmatterError != nil {
		f.Warnings = []string{"project.yaml does not parse: " + doc.FrontmatterError.Error()}
		f.unread = []string{""}
		return f
	}
	problem := func(field string, err error) {
		f.Warnings = append(f.Warnings, fmt.Sprintf("project.yaml %s: %v", field, err))
		f.unread = append(f.unread, field)
	}
	decode := func(field string, v any) bool {
		if err := doc.Decode(field, v); err != nil {
			problem(field, err)
			return false
		}
		return true
	}
	list := func(field string) []string {
		var items []string
		if err := doc.Decode(field, &items); err != nil {
			var one string
			if doc.Decode(field, &one) != nil {
				problem(field, err)
				return nil
			}
			items = []string{one}
		}
		var out []string
		for _, item := range items {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
		return out
	}
	var name, key string
	var nextID int
	var settings projectSettings
	if decode("name", &name) {
		f.Name = strings.TrimSpace(name)
	}
	if decode("next_id", &nextID) {
		f.NextID = max(nextID, 0)
	}
	f.Repos = list("repos")
	f.Retired = list("retired")
	if decode("settings", &settings) {
		f.Settings = settings
	}
	if decode("key", &key) {
		switch key = strings.TrimSpace(key); {
		case key == "":
		case board.ValidKey(key):
			f.Key = key
		default:
			f.Warnings = append(f.Warnings, fmt.Sprintf("key %q in project.yaml is not 2–10 uppercase letters or digits starting with a letter; using %s", key, board.DeriveKey(project)))
		}
	}
	return f
}

// readable reports whether field, and the file, could be read.
func (f projectFile) readable(field string) bool {
	return !slices.Contains(f.unread, "") && !slices.Contains(f.unread, field)
}

// addToProjectList adds item to a list field of project.yaml's content,
// keeping the file's other lines. A field that cannot be read is never
// replaced (mdfile.ErrBrokenFrontmatter).
func addToProjectList(project string, data []byte, field, item string) ([]byte, error) {
	f := parseProjectFile(project, data)
	if !f.readable(field) {
		return nil, fmt.Errorf("%s/project.yaml: %w: %s", project, mdfile.ErrBrokenFrontmatter, strings.Join(f.Warnings, "; "))
	}
	items := map[string][]string{"repos": f.Repos, "retired": f.Retired}[field]
	if slices.Contains(items, item) {
		return data, nil
	}
	out, err := mdfile.SetYAMLList(data, field, append(slices.Clone(items), item))
	if err != nil {
		return nil, fmt.Errorf("%s/project.yaml: %w", project, err)
	}
	return out, nil
}

// ProjectFileKey returns the valid key a project's project.yaml records, or
// "" when it records none or cannot be read.
func (s *Store) ProjectFileKey(project string) string {
	data, err := s.ReadFile(path.Join(project, "project.yaml"))
	if err != nil {
		return ""
	}
	return parseProjectFile(project, data).Key
}
