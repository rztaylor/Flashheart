package api

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/store"
)

func workstreamDetail(t *testing.T, handler http.Handler, project, slug string) WorkstreamDetail {
	t.Helper()
	var response WorkstreamResponse
	getJSON(t, handler, "/api/projects/"+project+"/workstreams/"+slug, http.StatusOK, &response)
	return response.Workstream
}

// A workstream's page reads its line and its file: the body it shows and
// the hash a section save sends (FH-139, STO-3).
func TestWorkstreamDetailCarriesItsFile(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	data, err := os.ReadFile(filepath.Join(root, "alpha", "workstreams", "board-ui.md"))
	if err != nil {
		t.Fatal(err)
	}
	detail := workstreamDetail(t, handler, "alpha", "board-ui")
	if detail.Slug != "board-ui" || detail.Title != "Board UI" || len(detail.Tickets) != 3 || detail.Total != 3 {
		t.Errorf("line = %+v", detail.WorkstreamJSON)
	}
	if detail.Hash != store.Hash(data) || detail.Raw != string(data) || !strings.Contains(detail.Body, "## Goal\n\nRender and edit the board.\n") || strings.Contains(detail.Body, "slug:") {
		t.Errorf("file: hash %q, body %q", detail.Hash, detail.Body)
	}
	getJSON(t, handler, "/api/projects/alpha/workstreams/nowhere", http.StatusNotFound, nil)
	getJSON(t, handler, "/api/projects/nobody/workstreams/board-ui", http.StatusNotFound, nil)
}

func TestPutWorkstreamSectionRewritesOnlyThatSection(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	file := filepath.Join(root, "alpha", "workstreams", "board-ui.md")
	url := "/api/projects/alpha/workstreams/board-ui/section"
	before, _ := os.ReadFile(file)
	base := workstreamDetail(t, handler, "alpha", "board-ui").Hash

	var saved WriteResponse
	send(t, handler, http.MethodPut, url, SectionRequest{Base: base, Section: "goal", Content: "Render the board **and** edit it.\n"}, http.StatusOK, &saved)
	after, _ := os.ReadFile(file)
	want := strings.Replace(string(before), "## Goal\n\nRender and edit the board.\n", "## Goal\n\nRender the board **and** edit it.\n", 1)
	if string(after) != want || saved.Hash != store.Hash(after) || saved.Revision == 0 {
		t.Errorf("after section save (%+v):\n%s", saved, after)
	}
	if detail := workstreamDetail(t, handler, "alpha", "board-ui"); detail.Hash != saved.Hash || !strings.Contains(detail.Body, "**and** edit it") {
		t.Errorf("detail after save: %q %q", detail.Hash, detail.Body)
	}

	// The section's own text again is no write.
	var again WriteResponse
	send(t, handler, http.MethodPut, url, SectionRequest{Base: saved.Hash, Section: "Goal", Content: "\nRender the board **and** edit it.  \n\n"}, http.StatusOK, &again)
	if unchanged, _ := os.ReadFile(file); string(unchanged) != string(after) || again.Hash != saved.Hash {
		t.Errorf("untouched save rewrote the file: hash %s -> %s", saved.Hash, again.Hash)
	}

	// A stale base is the conflict, in the workstream's words, with the
	// file as it is now.
	var conflict struct {
		Error   struct{ Code, Message string }
		Current ConflictJSON
	}
	send(t, handler, http.MethodPut, url, SectionRequest{Base: base, Section: "Scope", Content: "Lost."}, http.StatusConflict, &conflict)
	if conflict.Error.Code != "conflict" || conflict.Error.Message != "This workstream changed since you opened it." || conflict.Current.Hash != saved.Hash || !strings.Contains(conflict.Current.Content, "**and** edit it") {
		t.Errorf("conflict = %+v", conflict)
	}

	// Notes are a person's to rewrite; a section of its own goes above
	// them.
	send(t, handler, http.MethodPut, url, SectionRequest{Base: saved.Hash, Section: "Notes", Content: "Rewritten by hand."}, http.StatusOK, &saved)
	send(t, handler, http.MethodPut, url, SectionRequest{Base: saved.Hash, Section: "Background", Content: "Why."}, http.StatusOK, &saved)
	if final, _ := os.ReadFile(file); !strings.HasSuffix(string(final), "## Background\n\nWhy.\n\n## Notes\n\nRewritten by hand.\n") {
		t.Errorf("notes and an own section:\n%s", final)
	}

	for _, request := range []SectionRequest{
		{Base: saved.Hash, Section: "", Content: "x"},
		{Section: "Goal", Content: "No base."},
		{Base: saved.Hash, Section: "Goal", Content: strings.Repeat("x", board.MaxSectionText+1)},
	} {
		send(t, handler, http.MethodPut, url, request, http.StatusBadRequest, nil)
	}
	send(t, handler, http.MethodPut, "/api/projects/alpha/workstreams/nowhere/section", SectionRequest{Base: "x", Section: "Goal"}, http.StatusNotFound, nil)
	send(t, handler, http.MethodPut, "/api/projects/alpha/workstreams/..%2Fx/section", SectionRequest{Base: "x", Section: "Goal"}, http.StatusBadRequest, nil)
}

// A section save over a copy of a real workstream changes that section's
// text alone (FH-139).
func TestPutWorkstreamSectionLeavesTheRestOfARealWorkstream(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	real, err := os.ReadFile(filepath.Join("..", "..", "testdata", "workstreams", "ticket-editing.md"))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "alpha", "workstreams", "ticket-editing.md")
	writeFile(t, file, string(real))
	base := workstreamDetail(t, handler, "alpha", "ticket-editing").Hash
	send(t, handler, http.MethodPut, "/api/projects/alpha/workstreams/ticket-editing/section", SectionRequest{Base: base, Section: "Scope", Content: "Only the page."}, http.StatusOK, nil)
	after, _ := os.ReadFile(file)
	if want := strings.Replace(string(real), "## Scope\n\n## Success Criteria", "## Scope\n\nOnly the page.\n\n## Success Criteria", 1); string(after) != want {
		t.Errorf("after save:\n%s", after)
	}
}

// A workstream created on the board (FH-17) is board-format's template
// with a slug made unique from its title; a refused one writes nothing.
func TestCreateWorkstreamWritesTheTemplate(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	dir := filepath.Join(root, "alpha", "workstreams")
	url := "/api/projects/alpha/workstreams"

	var created CreatedWorkstreamResponse
	send(t, handler, http.MethodPost, url, NewWorkstreamRequest{
		Title: "  Offline   mode ", Goal: "Work without a network.\n", Priority: "high", DependsOnWorkstreams: []string{"board-ui"},
	}, http.StatusCreated, &created)
	data, err := os.ReadFile(filepath.Join(dir, "offline-mode.md"))
	if err != nil {
		t.Fatal(err)
	}
	day, _ := mdfile.Parse(data).String("created")
	want := "---\nslug: offline-mode\nstatus: active\npriority: high\ncreated: " + day +
		"\ntickets: []\ndepends-on-workstreams: [board-ui]\ntags: []\n---\n\n# Offline mode\n\n## Goal\n\nWork without a network.\n\n" +
		"## Scope\n\n## Success Criteria\n\n- [ ] Every ticket in the line reviewed\n\n## Notes\n\nNone.\n"
	if string(data) != want {
		t.Errorf("created file:\n%s\nwant:\n%s", data, want)
	}
	if created.Slug != "offline-mode" || created.Hash != store.Hash(data) || created.Revision == 0 {
		t.Errorf("created = %+v", created)
	}
	if detail := workstreamDetail(t, handler, "alpha", "offline-mode"); detail.Title != "Offline mode" || detail.Total != 0 {
		t.Errorf("new line = %+v", detail.WorkstreamJSON)
	}

	// Another title that makes the same slug takes the next free one;
	// priority defaults.
	send(t, handler, http.MethodPost, url, NewWorkstreamRequest{Title: "Offline mode?"}, http.StatusCreated, &created)
	if again, err := os.ReadFile(filepath.Join(dir, "offline-mode-2.md")); err != nil || created.Slug != "offline-mode-2" || !strings.Contains(string(again), "priority: medium\n") {
		t.Errorf("second create = %+v, %v:\n%s", created, err, again)
	}

	// Refusals are in words, and none of them writes a file.
	listing := func() []string {
		t.Helper()
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		return names
	}
	before := listing()
	var refused struct {
		Error struct{ Code, Message string }
	}
	send(t, handler, http.MethodPost, url, NewWorkstreamRequest{Title: "Lost", DependsOnWorkstreams: []string{"nowhere"}}, http.StatusNotFound, &refused)
	if !strings.Contains(refused.Error.Message, `"nowhere"`) || !strings.Contains(refused.Error.Message, "board-ui") {
		t.Errorf("unknown dependency refusal = %+v", refused.Error)
	}
	// A title the project's workstreams already have, in any case or
	// spacing, is a conflict naming it (FH-163, D73).
	send(t, handler, http.MethodPost, url, NewWorkstreamRequest{Title: "offline  MODE"}, http.StatusConflict, &refused)
	if refused.Error.Code != "title_taken" || !strings.Contains(refused.Error.Message, `"Offline mode"`) || !strings.Contains(refused.Error.Message, "choose another title") {
		t.Errorf("taken title refusal = %+v", refused.Error)
	}
	send(t, handler, http.MethodPost, url, NewWorkstreamRequest{Title: "  "}, http.StatusBadRequest, &refused)
	if !strings.Contains(refused.Error.Message, "title") {
		t.Errorf("empty title refusal = %+v", refused.Error)
	}
	send(t, handler, http.MethodPost, url, NewWorkstreamRequest{Title: "Lost", Priority: "urgent"}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPost, url, NewWorkstreamRequest{Title: "Lost", Goal: strings.Repeat("x", board.MaxSectionText+1)}, http.StatusRequestEntityTooLarge, nil)
	send(t, handler, http.MethodPost, url, `{"title":"Lost","ordered":true}`, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPost, "/api/projects/nobody/workstreams", NewWorkstreamRequest{Title: "Lost"}, http.StatusNotFound, nil)
	if after := listing(); !slices.Equal(after, before) {
		t.Errorf("refused creates changed the workstreams: %v -> %v", before, after)
	}
}
