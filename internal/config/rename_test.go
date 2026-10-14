package config

import (
	"slices"
	"testing"
)

// TestUIRenameWorkstream: a workstream's rename rewrites its project's
// filter choices and expanded list, a slug already listed is not doubled,
// another project's and All projects' choices stay, and the UI it was
// given is left as it was (FH-170).
func TestUIRenameWorkstream(t *testing.T) {
	t.Parallel()

	ui := UI{
		Theme: "dark",
		Scopes: map[string]Scope{
			"alpha": {View: "board", Workstream: Choice{Include: []string{"board-ui", "board"}, Exclude: []string{"x"}}},
			"all":   {Workstream: Choice{Exclude: []string{"board-ui"}}},
		},
		ExpandedWorkstreams: map[string][]string{"alpha": {"a", "board-ui"}, "beta": {"board-ui"}},
	}
	next, changed := ui.RenameWorkstream("alpha", "board-ui", "board")
	if !changed {
		t.Fatal("not changed")
	}
	if got := next.Scopes["alpha"]; !slices.Equal(got.Workstream.Include, []string{"board"}) || !slices.Equal(got.Workstream.Exclude, []string{"x"}) || got.View != "board" {
		t.Errorf("alpha = %+v", got)
	}
	if got := next.Scopes["all"].Workstream.Exclude; !slices.Equal(got, []string{"board-ui"}) {
		t.Errorf("all = %v, want it kept", got)
	}
	if got := next.ExpandedWorkstreams["alpha"]; !slices.Equal(got, []string{"a", "board"}) {
		t.Errorf("alpha expanded = %v", got)
	}
	if got := next.ExpandedWorkstreams["beta"]; !slices.Equal(got, []string{"board-ui"}) {
		t.Errorf("beta expanded = %v, want it kept", got)
	}
	if next.Theme != "dark" {
		t.Errorf("theme = %q", next.Theme)
	}
	if got := ui.Scopes["alpha"].Workstream.Include; !slices.Equal(got, []string{"board-ui", "board"}) {
		t.Errorf("the original scope changed: %v", got)
	}
	if got := ui.ExpandedWorkstreams["alpha"]; !slices.Equal(got, []string{"a", "board-ui"}) {
		t.Errorf("the original expanded list changed: %v", got)
	}
	if _, changed := ui.RenameWorkstream("gamma", "board-ui", "board"); changed {
		t.Error("a project with no preferences changed")
	}
	if _, changed := ui.RenameWorkstream("alpha", "offline", "away"); changed {
		t.Error("a slug no preference names changed them")
	}
}
