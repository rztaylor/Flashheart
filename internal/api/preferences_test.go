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
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/preferences", bytes.NewReader(body)))
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
