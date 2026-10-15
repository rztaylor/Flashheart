package api

import (
	"encoding/json"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// titleField is a workstream patch's title field.
func titleField(value string) map[string]json.RawMessage {
	data, _ := json.Marshal(value)
	return map[string]json.RawMessage{"title": data}
}

// TestPatchWorkstreamNamesItsSlugFromTheTitle: the workstream page's
// name edit (FH-167) sends the new title with slugFromTitle, and the
// server renames the slug to the one New workstream makes from it,
// rewriting every reference; a name that makes the same slug only sets
// the title, and a slug made unique (board-ui-2) is kept for a name that
// makes its base. A name whose slug another workstream has is a 409
// exists in the words of names, and writes nothing.
func TestPatchWorkstreamNamesItsSlugFromTheTitle(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	dir := filepath.Join(root, "alpha", "workstreams")
	base := workstreamDetail(t, handler, "alpha", "board-ui").Hash

	var edited EditedWorkstreamResponse
	send(t, handler, http.MethodPatch, "/api/projects/alpha/workstreams/board-ui", WorkstreamPatchRequest{
		Base: base, Fields: titleField("Board columns & lanes"), SlugFromTitle: true,
	}, http.StatusOK, &edited)
	if edited.Slug != "board-columns-lanes" {
		t.Fatalf("slug = %q, want board-columns-lanes", edited.Slug)
	}
	if detail := workstreamDetail(t, handler, "alpha", "board-columns-lanes"); detail.Title != "Board columns & lanes" {
		t.Errorf("renamed = %+v", detail.WorkstreamJSON)
	}
	if detail := ticketDetail(t, handler, "AL-2"); detail.Workstream != "board-columns-lanes" {
		t.Errorf("AL-2 workstream = %q", detail.Workstream)
	}

	// The same slug: the title alone changes.
	url := "/api/projects/alpha/workstreams/board-columns-lanes"
	send(t, handler, http.MethodPatch, url, WorkstreamPatchRequest{
		Base: edited.Hash, Fields: titleField("Board Columns, lanes"), SlugFromTitle: true,
	}, http.StatusOK, &edited)
	if edited.Slug != "board-columns-lanes" {
		t.Errorf("slug = %q, want it kept", edited.Slug)
	}

	// A name whose slug another workstream has is refused in words of names.
	send(t, handler, http.MethodPost, "/api/projects/alpha/workstreams", NewWorkstreamRequest{Title: "Offline"}, http.StatusCreated, nil)
	before := dirFiles(t, dir)
	var refused struct {
		Error struct{ Code, Message string }
	}
	send(t, handler, http.MethodPatch, url, WorkstreamPatchRequest{
		Base: edited.Hash, Fields: titleField("OFFLINE!"), SlugFromTitle: true,
	}, http.StatusConflict, &refused)
	if refused.Error.Code != "exists" || !strings.Contains(refused.Error.Message, `"OFFLINE!"`) ||
		!strings.Contains(refused.Error.Message, "offline") || !strings.Contains(refused.Error.Message, "choose another name") {
		t.Errorf("taken name refusal = %+v", refused.Error)
	}
	if after := dirFiles(t, dir); !maps.Equal(before, after) {
		t.Error("a refused name changed the workstreams")
	}

	// slugFromTitle needs a title, and is not sent with a slug.
	send(t, handler, http.MethodPatch, url, WorkstreamPatchRequest{Base: edited.Hash, SlugFromTitle: true}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPatch, url, WorkstreamPatchRequest{Base: edited.Hash, Fields: titleField("Lanes"), Slug: "lanes", SlugFromTitle: true}, http.StatusBadRequest, nil)

	// A slug made unique keeps its number for a name that makes its base.
	var created CreatedWorkstreamResponse
	send(t, handler, http.MethodPost, "/api/projects/alpha/workstreams", NewWorkstreamRequest{Title: "Offline!"}, http.StatusCreated, &created)
	if created.Slug != "offline-2" {
		t.Fatalf("created = %+v", created)
	}
	send(t, handler, http.MethodPatch, "/api/projects/alpha/workstreams/offline-2", WorkstreamPatchRequest{
		Base: created.Hash, Fields: titleField("Offline?"), SlugFromTitle: true,
	}, http.StatusOK, &edited)
	if edited.Slug != "offline-2" {
		t.Errorf("slug = %q, want offline-2 kept", edited.Slug)
	}
}

// TestPatchWorkstreamRenameKeepsSavedPreferences: after a rename on the
// board, the saved filter scope and expanded finished workstreams name
// the new slug (FH-170).
func TestPatchWorkstreamRenameKeepsSavedPreferences(t *testing.T) {
	t.Parallel()

	handler, _ := writableAPI(t)
	var preferences Preferences
	getJSON(t, handler, "/api/preferences", http.StatusOK, &preferences)
	preferences.Scopes = map[string]ScopeJSON{
		"alpha": {View: "board", Workstream: ChoiceJSON{Include: []string{}, Exclude: []string{"board-ui"}}},
	}
	preferences.ExpandedWorkstreams = map[string][]string{"alpha": {"board-ui"}}
	send(t, handler, http.MethodPatch, "/api/preferences", preferences, http.StatusNoContent, nil)

	base := workstreamDetail(t, handler, "alpha", "board-ui").Hash
	send(t, handler, http.MethodPatch, "/api/projects/alpha/workstreams/board-ui", WorkstreamPatchRequest{Base: base, Slug: "board"}, http.StatusOK, nil)

	getJSON(t, handler, "/api/preferences", http.StatusOK, &preferences)
	if got := preferences.Scopes["alpha"].Workstream.Exclude; !slices.Equal(got, []string{"board"}) {
		t.Errorf("alpha excludes %v, want [board]", got)
	}
	if got := preferences.ExpandedWorkstreams["alpha"]; !slices.Equal(got, []string{"board"}) {
		t.Errorf("alpha expanded = %v, want [board]", got)
	}

	// A tab that loaded before the rename saves only what it changed, so
	// its theme does not bring the old slug back (FH-173).
	send(t, handler, http.MethodPatch, "/api/preferences", `{"theme":"dark"}`, http.StatusNoContent, nil)
	getJSON(t, handler, "/api/preferences", http.StatusOK, &preferences)
	if got := preferences.Scopes["alpha"].Workstream.Exclude; preferences.Theme != "dark" || !slices.Equal(got, []string{"board"}) {
		t.Errorf("after a stale tab's theme: theme %q, alpha excludes %v", preferences.Theme, got)
	}
}

// dirFiles reads every file in dir by name.
func dirFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, entry := range entries {
		content, _ := os.ReadFile(filepath.Join(dir, entry.Name()))
		out[entry.Name()] = string(content)
	}
	return out
}
