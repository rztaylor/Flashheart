package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

// slowConfig is the store with a config save that waits until released, as
// a save does on a slow disk or behind the root lock.
type slowConfig struct {
	*store.Store
	saving  chan struct{}
	release chan struct{}
}

func (s slowConfig) UpdateConfig(edit store.Edit) error {
	close(s.saving)
	<-s.release
	return s.Store.UpdateConfig(edit)
}

// A page that loads while a preference save is still being written, such as
// one reloaded straight after a change flushed on unload, reads the saved
// value rather than the one it replaces (FH-109).
func TestPreferencesReadWaitsForASaveInFlight(t *testing.T) {
	t.Parallel()

	root := copyBoard(t, "sample")
	files := store.New(root)
	t.Cleanup(func() { files.Close() })
	writer := slowConfig{Store: files, saving: make(chan struct{}), release: make(chan struct{})}
	handler := New(Options{
		Info:   Info{Root: root, Theme: "system"},
		Board:  index.New(files, index.Options{}),
		Files:  files,
		Writer: writer,
	})
	var prefs Preferences
	getJSON(t, handler, "/api/preferences", http.StatusOK, &prefs)
	prefs.ColourBy = "age"
	body, _ := json.Marshal(prefs)

	saved := make(chan int)
	go func() {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPatch, "/api/preferences", bytes.NewReader(body)))
		saved <- recorder.Code
	}()
	<-writer.saving

	read := make(chan *httptest.ResponseRecorder)
	go func() {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/preferences", nil))
		read <- recorder
	}()
	select {
	case recorder := <-read:
		close(writer.release)
		t.Fatalf("read answered during the save: %s", recorder.Body.String())
	case <-time.After(50 * time.Millisecond):
	}
	close(writer.release)
	if code := <-saved; code != http.StatusNoContent {
		t.Fatalf("save status = %d", code)
	}
	recorder := <-read
	var after Preferences
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &after) != nil {
		t.Fatalf("read = %d %s", recorder.Code, recorder.Body.String())
	}
	if after.ColourBy != "age" {
		t.Errorf("read colour by = %q, want the saved age", after.ColourBy)
	}
}

// A preference save names only what changed (FH-173): a tab whose copy is
// older than the file, such as one open while an agent renamed a workstream
// its filter names, changes the theme without putting its old filter back.
// A project's view and filters, and its expanded workstreams, are replaced
// one project at a time; null removes them.
func TestPreferencesPatchChangesOnlyWhatItNames(t *testing.T) {
	t.Parallel()

	handler, _ := writableAPI(t)
	send(t, handler, http.MethodPatch, "/api/preferences", map[string]any{
		"scopes": map[string]any{
			"alpha": map[string]any{"view": "board", "type": map[string]any{"include": []string{}, "exclude": []string{}}, "priority": map[string]any{"include": []string{}, "exclude": []string{}}, "workstream": map[string]any{"include": []string{}, "exclude": []string{"board"}}, "age": map[string]any{"include": []string{}, "exclude": []string{}}, "state": ""},
			"beta":  map[string]any{"view": "table", "type": map[string]any{"include": []string{}, "exclude": []string{}}, "priority": map[string]any{"include": []string{}, "exclude": []string{}}, "workstream": map[string]any{"include": []string{}, "exclude": []string{}}, "age": map[string]any{"include": []string{}, "exclude": []string{}}, "state": ""},
		},
		"expandedWorkstreams": map[string]any{"alpha": []string{"lines"}, "beta": []string{"offline"}},
	}, http.StatusNoContent, nil)

	send(t, handler, http.MethodPatch, "/api/preferences", `{"theme":"dark"}`, http.StatusNoContent, nil)
	var prefs Preferences
	getJSON(t, handler, "/api/preferences", http.StatusOK, &prefs)
	if prefs.Theme != "dark" || prefs.ColourBy != "type" || prefs.Editor != "formatted" {
		t.Errorf("after the theme = %+v", prefs)
	}
	if got := prefs.Scopes["alpha"].Workstream.Exclude; len(got) != 1 || got[0] != "board" {
		t.Errorf("alpha's filter = %v, want it kept", got)
	}

	send(t, handler, http.MethodPatch, "/api/preferences", `{"scopes":{"beta":null,"gamma":{"view":"overview","type":{"include":[],"exclude":[]},"priority":{"include":[],"exclude":[]},"workstream":{"include":[],"exclude":[]},"age":{"include":[],"exclude":[]},"state":""}},"expandedWorkstreams":{"beta":null}}`, http.StatusNoContent, nil)
	prefs = Preferences{}
	getJSON(t, handler, "/api/preferences", http.StatusOK, &prefs)
	if _, ok := prefs.Scopes["beta"]; ok {
		t.Errorf("beta's scope kept: %+v", prefs.Scopes)
	}
	if prefs.Scopes["gamma"].View != "overview" || prefs.Scopes["alpha"].View != "board" {
		t.Errorf("scopes = %+v", prefs.Scopes)
	}
	if _, ok := prefs.ExpandedWorkstreams["beta"]; ok || len(prefs.ExpandedWorkstreams["alpha"]) != 1 {
		t.Errorf("expanded = %+v", prefs.ExpandedWorkstreams)
	}

	// An empty patch saves nothing and is not an error; a bad value in one
	// is refused whole.
	send(t, handler, http.MethodPatch, "/api/preferences", `{}`, http.StatusNoContent, nil)
	send(t, handler, http.MethodPatch, "/api/preferences", `{"theme":"light","colourBy":"rainbow"}`, http.StatusBadRequest, nil)
	prefs = Preferences{}
	getJSON(t, handler, "/api/preferences", http.StatusOK, &prefs)
	if prefs.Theme != "dark" {
		t.Errorf("a refused patch saved the theme %q", prefs.Theme)
	}
}
