package store

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// prefsConfig is config.yaml with saved filters and expanded workstreams
// that name board-ui in alpha, choices in beta and in All projects that
// name the same slug, and a comment the rewrite keeps.
const prefsConfig = `version: 2
# kept
ui:
  theme: dark
  density: normal
  colour_by: type
  hidden_columns: []
  editor: formatted
  scopes:
    alpha:
      view: board
      workstream: {include: [], exclude: [board-ui, offline]}
    all:
      workstream: {include: [board-ui], exclude: []}
    beta:
      workstream: {include: [board-ui], exclude: []}
  expanded_workstreams:
    alpha: [board-ui]
    beta: [board-ui]
`

// TestEditWorkstreamRenameRewritesSavedPreferences: a rename rewrites
// the project's saved filter scope (workstream include and exclude) and
// its expanded finished workstreams in config.yaml to the new slug;
// other projects and All projects keep theirs, as their bare slugs may
// name other projects' workstreams (FH-170).
func TestEditWorkstreamRenameRewritesSavedPreferences(t *testing.T) {
	t.Parallel()

	s, root := renameFixture(t)
	configPath := filepath.Join(root, filepath.FromSlash(ConfigFile))
	if err := os.WriteFile(configPath, []byte(prefsConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{Rename: "board"}); err != nil {
		t.Fatal(err)
	}
	settings, err := s.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	ui := settings.UI
	if got := ui.Scopes["alpha"].Workstream; !slices.Equal(got.Exclude, []string{"board", "offline"}) || len(got.Include) != 0 {
		t.Errorf("alpha's workstream filter = %+v, want board and offline excluded", got)
	}
	if got := ui.Scopes["alpha"].View; got != "board" {
		t.Errorf("alpha's view = %q, want it kept", got)
	}
	for _, name := range []string{"beta", "all"} {
		if got := ui.Scopes[name].Workstream.Include; !slices.Equal(got, []string{"board-ui"}) {
			t.Errorf("%s's workstream filter = %v, want it kept", name, got)
		}
	}
	if got := ui.ExpandedWorkstreams["alpha"]; !slices.Equal(got, []string{"board"}) {
		t.Errorf("alpha expanded = %v, want [board]", got)
	}
	if got := ui.ExpandedWorkstreams["beta"]; !slices.Equal(got, []string{"board-ui"}) {
		t.Errorf("beta expanded = %v, want it kept", got)
	}
	if ui.Theme != "dark" {
		t.Errorf("theme = %q, want dark kept", ui.Theme)
	}
	if data := string(mustRead(t, configPath)); !strings.Contains(data, "\n# kept\n") {
		t.Errorf("the comment was lost:\n%s", data)
	}
}

// TestEditWorkstreamRenameLeavesOtherPreferences: a rename whose
// config.yaml names no preference with the old slug leaves the file
// byte for byte, and an edit with no rename never touches it.
func TestEditWorkstreamRenameLeavesOtherPreferences(t *testing.T) {
	t.Parallel()

	s, root := renameFixture(t)
	configPath := filepath.Join(root, filepath.FromSlash(ConfigFile))
	before := mustRead(t, configPath)
	if _, err := s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{Rename: "board"}); err != nil {
		t.Fatal(err)
	}
	if after := mustRead(t, configPath); string(after) != string(before) {
		t.Errorf("config.yaml changed:\n%s", after)
	}
}

// TestEditWorkstreamRenameSurvivesAPreferenceFailure: config.yaml that
// cannot be read as settings neither refuses nor undoes the rename; the
// preferences are best effort and the file is left as it was (FH-170).
func TestEditWorkstreamRenameSurvivesAPreferenceFailure(t *testing.T) {
	t.Parallel()

	s, root := renameFixture(t)
	configPath := filepath.Join(root, filepath.FromSlash(ConfigFile))
	broken := "ui: [not, a, mapping\nscopes: board-ui\n"
	if err := os.WriteFile(configPath, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	edited, err := s.EditWorkstream("alpha", "board-ui", Merge, WorkstreamEdit{Rename: "board"})
	if err != nil {
		t.Fatalf("rename refused over a broken config.yaml: %v", err)
	}
	if edited.Slug != "board" {
		t.Fatalf("slug = %q", edited.Slug)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "workstreams", "board.md")); err != nil {
		t.Fatalf("not renamed: %v", err)
	}
	if got := string(mustRead(t, configPath)); got != broken {
		t.Errorf("config.yaml changed:\n%s", got)
	}
}

// TestWorkstreamSlugFor: a name makes the slug New workstream makes from
// it, unless the current slug is that slug made unique (FH-167).
func TestWorkstreamSlugFor(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ current, title, want string }{
		{"board-ui", "Board UI", "board-ui"},
		{"board-ui", "Board columns & lanes", "board-columns-lanes"},
		{"offline-2", "Offline!", "offline-2"},
		{"offline-2", "Offline mode", "offline-mode"},
		{"offline-x", "Offline", "offline"},
		{"offline-", "Offline", "offline"},
		{"ws", "日本", "workstream"},
	} {
		if got := WorkstreamSlugFor(tc.current, tc.title); got != tc.want {
			t.Errorf("WorkstreamSlugFor(%q, %q) = %q, want %q", tc.current, tc.title, got, tc.want)
		}
	}
}
