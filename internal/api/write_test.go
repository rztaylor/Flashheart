package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

func writableAPI(t *testing.T) (http.Handler, string) {
	t.Helper()
	root := copyBoard(t, "sample")
	files := store.New(root)
	t.Cleanup(func() { files.Close() })
	return New(Options{
		Info:   Info{Root: root, Theme: "system"},
		Board:  index.New(files, index.Options{}),
		Files:  files,
		Writer: files,
	}), root
}

func send(t *testing.T, handler http.Handler, method, path string, body any, status int, into any) {
	t.Helper()
	var reader *bytes.Reader
	switch value := body.(type) {
	case string:
		reader = bytes.NewReader([]byte(value))
	default:
		data, _ := json.Marshal(value)
		reader = bytes.NewReader(data)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, reader))
	if recorder.Code != status {
		t.Fatalf("%s %s status = %d, want %d: %s", method, path, recorder.Code, status, recorder.Body.String())
	}
	if into != nil {
		if err := json.Unmarshal(recorder.Body.Bytes(), into); err != nil {
			t.Fatalf("%s %s decode: %v", method, path, err)
		}
	}
}

func ticketDetail(t *testing.T, handler http.Handler, id string) TicketDetail {
	t.Helper()
	var response TicketResponse
	getJSON(t, handler, "/api/tickets/"+id, http.StatusOK, &response)
	return response.Ticket
}

func TestMoveSavesStatusAndRefusesStaleHashes(t *testing.T) {
	t.Parallel()

	handler, _ := writableAPI(t)
	detail := ticketDetail(t, handler, "AL-2")
	if detail.Hash == "" || !strings.Contains(detail.Raw, "id: AL-2") {
		t.Fatalf("detail hash/raw missing: %q", detail.Hash)
	}
	var moved WriteResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", MoveRequest{Base: detail.Hash, To: "done"}, http.StatusOK, &moved)
	if moved.Hash == detail.Hash || moved.Revision == 0 {
		t.Errorf("move = %+v", moved)
	}
	if after := ticketDetail(t, handler, "AL-2"); after.Column != "done" || after.Hash != moved.Hash {
		t.Errorf("after move column=%s hash=%s", after.Column, after.Hash)
	}

	var conflict struct {
		Error   struct{ Code string }
		Current ConflictJSON
	}
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", MoveRequest{Base: detail.Hash, To: "backlog"}, http.StatusConflict, &conflict)
	if conflict.Error.Code != "conflict" || conflict.Current.Hash != moved.Hash || !strings.Contains(conflict.Current.Content, "status: done") {
		t.Errorf("conflict = %+v", conflict)
	}
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", MoveRequest{To: "sideways"}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-99/move", MoveRequest{To: "done"}, http.StatusNotFound, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", `{"to":"done","extra":1}`, http.StatusBadRequest, nil)
}

func TestMovingABlockedTicketIntoProgressNeedsAReason(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	var refused struct {
		Error struct {
			Code      string
			BlockedBy []string
		}
	}
	send(t, handler, http.MethodPost, "/api/tickets/AL-5/move", MoveRequest{To: "in-progress"}, http.StatusConflict, &refused)
	if refused.Error.Code != "confirm_blocked" || len(refused.Error.BlockedBy) == 0 {
		t.Fatalf("refusal = %+v", refused)
	}
	send(t, handler, http.MethodPost, "/api/tickets/AL-5/move", MoveRequest{To: "in-progress", Reason: "Hotfix needed\nnow"}, http.StatusOK, nil)
	data, _ := os.ReadFile(filepath.Join(root, "alpha", "tickets", "AL-5-column-overflow", "AL-5-column-overflow.md"))
	if !strings.Contains(string(data), "status: in-progress") || !strings.Contains(string(data), "Started while blocked: Hotfix needed now") {
		t.Errorf("ticket =\n%s", data)
	}
}

func TestMovingIntoReviewWarnsButMoves(t *testing.T) {
	t.Parallel()

	handler, _ := writableAPI(t)
	var moved WriteResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-3/move", MoveRequest{To: "review"}, http.StatusOK, &moved)
	if len(moved.Warnings) != 2 || !strings.Contains(moved.Warnings[0], "no review file") || !strings.Contains(moved.Warnings[1], "1 acceptance criterion is not ticked") {
		t.Errorf("warnings = %q", moved.Warnings)
	}
}

func TestMovingIntoReviewWarnsWithoutEvidence(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	review := filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "review.md")
	if err := os.WriteFile(review, []byte("# Review: Drag and drop\n\n## Summary\nDone.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var moved WriteResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-4/move", MoveRequest{To: "review"}, http.StatusOK, &moved)
	if !slices.ContainsFunc(moved.Warnings, func(w string) bool { return strings.Contains(w, "no evidence") }) {
		t.Errorf("warnings = %q", moved.Warnings)
	}
	if ticketDetail(t, handler, "AL-4").Column != "review" {
		t.Error("the move must still happen")
	}
	// The sample's AL-2 review shows a screenshot: no evidence warning.
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", MoveRequest{To: "in-progress"}, http.StatusOK, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", MoveRequest{To: "review"}, http.StatusOK, &moved)
	if slices.ContainsFunc(moved.Warnings, func(w string) bool { return strings.Contains(w, "no evidence") }) {
		t.Errorf("AL-2 warnings = %q", moved.Warnings)
	}
}

func TestPatchTicketFields(t *testing.T) {
	t.Parallel()

	handler, _ := writableAPI(t)
	base := ticketDetail(t, handler, "AL-4").Hash
	send(t, handler, http.MethodPatch, "/api/tickets/AL-4", map[string]any{
		"base": base,
		"fields": map[string]any{
			"title": "Drag and drop, keyboard too", "priority": "high", "tags": []string{"ui", " a11y "},
			"depends-on": []string{"AL-3"}, "branch": "feature/dnd",
		},
	}, http.StatusOK, nil)
	after := ticketDetail(t, handler, "AL-4")
	if after.Title != "Drag and drop, keyboard too" || after.Priority != "high" || !slices.Equal(after.Tags, []string{"ui", "a11y"}) ||
		!slices.Equal(after.DependsOn, []string{"AL-3"}) || after.Branch != "feature/dnd" {
		t.Errorf("after patch = %+v", after.Card)
	}
	for _, fields := range []map[string]any{
		{"priority": "urgent"}, {"status": "nowhere"}, {"created": "yesterday"}, {"depends-on": []string{"nope"}},
		{"id": "AL-40"}, {"title": ""}, {"tags": "not a list"},
	} {
		send(t, handler, http.MethodPatch, "/api/tickets/AL-4", map[string]any{"fields": fields}, http.StatusBadRequest, nil)
	}
}

func TestRawSaveAndCriteria(t *testing.T) {
	t.Parallel()

	handler, _ := writableAPI(t)
	detail := ticketDetail(t, handler, "AL-3")
	var saved WriteResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-3/criteria", CriterionRequest{Base: detail.Hash, Index: 1, Checked: true}, http.StatusOK, &saved)
	after := ticketDetail(t, handler, "AL-3")
	if after.Criteria.Done != 2 {
		t.Errorf("criteria = %+v", after.Criteria)
	}
	send(t, handler, http.MethodPost, "/api/tickets/AL-3/criteria", CriterionRequest{Base: saved.Hash, Index: 9, Checked: true}, http.StatusNotFound, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-3/criteria", CriterionRequest{Index: 0, Checked: true}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPut, "/api/tickets/AL-3/raw", RawRequest{Content: "x"}, http.StatusBadRequest, nil)

	raw := strings.Replace(after.Raw, "Fixture ticket.", "Edited in the raw editor.", 1)
	send(t, handler, http.MethodPut, "/api/tickets/AL-3/raw", RawRequest{Base: saved.Hash, Content: raw}, http.StatusOK, &saved)
	if !strings.Contains(ticketDetail(t, handler, "AL-3").Body, "Edited in the raw editor.") {
		t.Error("raw save not applied")
	}
	send(t, handler, http.MethodPut, "/api/tickets/AL-3/raw", RawRequest{Base: saved.Hash, Content: "---\nid: [AL-3\n---\n# Broken\n"}, http.StatusOK, &saved)
	if len(saved.Warnings) != 1 || !strings.Contains(saved.Warnings[0], "needs repair") {
		t.Errorf("broken raw warnings = %q", saved.Warnings)
	}
}

func TestCreateArchiveAndUnarchive(t *testing.T) {
	t.Parallel()

	handler, _ := writableAPI(t)
	var created CreateResponse
	send(t, handler, http.MethodPost, "/api/projects/beta/tickets", CreateRequest{
		Title: "Say goodbye", Type: "feature", Priority: "low", Status: "up-next", Criteria: []string{"Waves"},
	}, http.StatusCreated, &created)
	if created.ID != "BE-2" || created.Revision == 0 {
		t.Fatalf("created = %+v", created)
	}
	if detail := ticketDetail(t, handler, "BE-2"); detail.Title != "Say goodbye" || detail.Column != "up-next" {
		t.Errorf("new ticket = %+v", detail.Card)
	}
	send(t, handler, http.MethodPost, "/api/projects/beta/tickets", CreateRequest{Title: ""}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPost, "/api/projects/nope/tickets", CreateRequest{Title: "x"}, http.StatusNotFound, nil)

	send(t, handler, http.MethodPost, "/api/tickets/BE-2/archive", "", http.StatusOK, nil)
	getJSON(t, handler, "/api/tickets/BE-2", http.StatusNotFound, nil)
	send(t, handler, http.MethodPost, "/api/tickets/BE-2/unarchive", "", http.StatusOK, nil)
	ticketDetail(t, handler, "BE-2")
	send(t, handler, http.MethodPost, "/api/tickets/BE-9/unarchive", "", http.StatusNotFound, nil)
}

func TestReorderWorkstreamAndProjectKey(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	send(t, handler, http.MethodPut, "/api/projects/alpha/workstreams/board-ui/order", OrderRequest{Tickets: []string{"AL-4", "AL-2", "AL-3"}}, http.StatusOK, nil)
	data, _ := os.ReadFile(filepath.Join(root, "alpha", "workstreams", "board-ui.md"))
	if !strings.Contains(string(data), "  - AL-4\n  - AL-2\n  - AL-3\n") {
		t.Errorf("workstream =\n%s", data)
	}
	send(t, handler, http.MethodPut, "/api/projects/alpha/workstreams/board-ui/order", OrderRequest{Tickets: []string{"AL-4", "AL-2"}}, http.StatusConflict, nil)

	writeFile(t, filepath.Join(root, "gamma", "project.yaml"), "name: Gamma\n")
	var taken struct{ Error struct{ InUse []string } }
	send(t, handler, http.MethodPut, "/api/projects/gamma/key", KeyRequest{Key: "AL"}, http.StatusConflict, &taken)
	if !slices.Equal(taken.Error.InUse, []string{"AL", "BE"}) {
		t.Errorf("in use = %v", taken.Error.InUse)
	}
	send(t, handler, http.MethodPut, "/api/projects/gamma/key", KeyRequest{Key: "GA"}, http.StatusOK, nil)
	send(t, handler, http.MethodPut, "/api/projects/alpha/key", KeyRequest{Key: "AX"}, http.StatusConflict, nil)
}

func TestPreferencesRoundTrip(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	var prefs Preferences
	getJSON(t, handler, "/api/preferences", http.StatusOK, &prefs)
	if prefs.Theme != "system" || prefs.ColourBy != "type" || prefs.Scopes == nil {
		t.Errorf("defaults = %+v", prefs)
	}
	prefs.Theme, prefs.Density, prefs.ColourBy = "dark", "compact", "priority"
	prefs.Scopes["alpha"] = config.Scope{View: "table", Type: config.Choice{Include: []string{"bug"}, Exclude: []string{}}, State: "blocked", NeedsYou: "only"}
	prefs.Scopes["beta"] = config.Scope{View: "agents"}
	send(t, handler, http.MethodPut, "/api/preferences", prefs, http.StatusNoContent, nil)
	var again Preferences
	getJSON(t, handler, "/api/preferences", http.StatusOK, &again)
	if again.Theme != "dark" || again.ColourBy != "priority" || again.Scopes["alpha"].View != "table" || !slices.Equal(again.Scopes["alpha"].Type.Include, []string{"bug"}) || again.Scopes["beta"].View != "agents" || again.Scopes["alpha"].NeedsYou != "only" {
		t.Errorf("saved = %+v", again)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".flashheart", "config.yaml"))
	if !strings.Contains(string(data), "quiet_minutes: 10") {
		t.Errorf("other settings lost:\n%s", data)
	}
	prefs.ColourBy = "rainbow"
	send(t, handler, http.MethodPut, "/api/preferences", prefs, http.StatusBadRequest, nil)
	prefs.ColourBy = "type"
	prefs.Scopes["alpha"] = config.Scope{View: "table", NeedsYou: "sometimes"}
	send(t, handler, http.MethodPut, "/api/preferences", prefs, http.StatusBadRequest, nil)
}

// The Backlog can be hidden from the Board (FH-41); a config without the
// setting shows it.
func TestPreferencesHideTheBacklog(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	var prefs Preferences
	getJSON(t, handler, "/api/preferences", http.StatusOK, &prefs)
	if prefs.HiddenColumns == nil || len(prefs.HiddenColumns) != 0 {
		t.Fatalf("default hidden columns = %#v", prefs.HiddenColumns)
	}
	prefs.HiddenColumns = []string{"backlog"}
	send(t, handler, http.MethodPut, "/api/preferences", prefs, http.StatusNoContent, nil)
	var again Preferences
	getJSON(t, handler, "/api/preferences", http.StatusOK, &again)
	if !slices.Equal(again.HiddenColumns, []string{"backlog"}) {
		t.Errorf("saved hidden columns = %#v", again.HiddenColumns)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".flashheart", "config.yaml"))
	if !strings.Contains(string(data), "hidden_columns: [backlog]") {
		t.Errorf("config.yaml lacks hidden_columns:\n%s", data)
	}
	prefs.HiddenColumns = []string{"done"}
	send(t, handler, http.MethodPut, "/api/preferences", prefs, http.StatusBadRequest, nil)
}

func TestReadOnlyServerRefusesWrites(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/move", MoveRequest{To: "done"}, http.StatusNotImplemented, nil)
	if detail := ticketDetail(t, handler, "AL-2"); detail.Hash != "" {
		t.Error("read-only detail has a hash")
	}
}

// The detail an edit is based on matches the hash it sends, even before the
// snapshot catches up with a change on disk.
func TestTicketDetailMatchesItsHash(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	ticketDetail(t, handler, "BE-1")
	writeFile(t, filepath.Join(root, "beta", "tickets", "BE-1-hello", "BE-1-hello.md"),
		"---\nid: BE-1\nstatus: backlog\n---\n# Renamed on disk\n\n## Acceptance Criteria\n\n- [ ] New first\n- [ ] Old\n")
	detail := ticketDetail(t, handler, "BE-1")
	if detail.Title != "Renamed on disk" || len(detail.CriteriaItems) != 2 || detail.CriteriaItems[0].Text != "New first" ||
		!strings.Contains(detail.Raw, "Renamed on disk") {
		t.Errorf("detail = %+v", detail.Card)
	}
}

func TestTitlesAreMeasuredInCharacters(t *testing.T) {
	t.Parallel()

	handler, _ := writableAPI(t)
	title := strings.Repeat("é", 300)
	base := ticketDetail(t, handler, "AL-4").Hash
	send(t, handler, http.MethodPatch, "/api/tickets/AL-4", map[string]any{"base": base, "fields": map[string]any{"title": title}}, http.StatusOK, nil)
	if got := ticketDetail(t, handler, "AL-4").Title; got != title {
		t.Errorf("title = %q", got)
	}
	var created CreateResponse
	send(t, handler, http.MethodPost, "/api/projects/beta/tickets", CreateRequest{Title: "a" + strings.Repeat("é", 400)}, http.StatusCreated, &created)
	if got := ticketDetail(t, handler, created.ID).Title; !utf8.ValidString(got) || utf8.RuneCountInString(got) != 300 {
		t.Errorf("created title has %d runes, valid=%v", utf8.RuneCountInString(got), utf8.ValidString(got))
	}
}

func backlogOrder(t *testing.T, handler http.Handler) []string {
	t.Helper()
	var response BoardResponse
	getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &response)
	var ids []string
	for _, card := range response.Cards {
		if card.Column == "backlog" {
			ids = append(ids, card.ID)
		}
	}
	return ids
}

func TestMoveWithAfterPlacesTheTicket(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	top := ""
	send(t, handler, http.MethodPost, "/api/tickets/AL-4/move", MoveRequest{To: "backlog", After: &top}, http.StatusOK, nil)
	if got := backlogOrder(t, handler); !slices.Equal(got[:3], []string{"AL-4", "AL-5", "AL-6"}) {
		t.Errorf("backlog = %v", got)
	}
	al4 := "AL-4"
	send(t, handler, http.MethodPost, "/api/tickets/AL-6/move", MoveRequest{To: "backlog", After: &al4}, http.StatusOK, nil)
	if got := backlogOrder(t, handler); !slices.Equal(got[:3], []string{"AL-4", "AL-6", "AL-5"}) {
		t.Errorf("backlog = %v", got)
	}

	// The ticket to follow has left the column: the move happens, the
	// placement does not, and the response says so.
	gone := "AL-3"
	var moved WriteResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-5/move", MoveRequest{To: "up-next", After: &gone}, http.StatusOK, &moved)
	if len(moved.Warnings) != 1 || !strings.Contains(moved.Warnings[0], "AL-3 is no longer in Up next") {
		t.Errorf("warnings = %v", moved.Warnings)
	}
	if ticketDetail(t, handler, "AL-5").Column != "up-next" {
		t.Error("AL-5 should have moved")
	}

	// Done keeps most recent first; a position there is ignored.
	send(t, handler, http.MethodPost, "/api/tickets/AL-6/move", MoveRequest{To: "done", After: &top}, http.StatusOK, nil)
	data, _ := os.ReadFile(filepath.Join(root, "alpha", "tickets", "AL-6-offline-mode", "AL-6-offline-mode.md"))
	if !strings.Contains(string(data), "status: done") {
		t.Errorf("AL-6 =\n%s", data)
	}

	path := "../x"
	send(t, handler, http.MethodPost, "/api/tickets/AL-4/move", MoveRequest{To: "backlog", After: &path}, http.StatusBadRequest, nil)
}

func TestReviewStepsTickInTheReviewFile(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	name := filepath.Join(root, "alpha", "tickets", "AL-2-board-columns", "review.md")
	detail := ticketDetail(t, handler, "AL-2")
	if detail.Review == nil || len(detail.Review.Steps) != 2 || detail.Review.Steps[0].Text != "Open the board." || detail.Review.Hash == "" {
		t.Fatalf("review = %+v", detail.Review)
	}
	var saved WriteResponse
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/review/steps", ReviewStepRequest{Base: detail.Review.Hash, Index: 1, Checked: true}, http.StatusOK, &saved)
	data, _ := os.ReadFile(name)
	if !strings.Contains(string(data), "2. [x] See four columns.") {
		t.Fatalf("review.md:\n%s", data)
	}
	if after := ticketDetail(t, handler, "AL-2"); !after.Review.Steps[1].Done || after.Review.Hash != saved.Hash {
		t.Errorf("after = %+v", after.Review)
	}
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/review/steps", ReviewStepRequest{Base: detail.Review.Hash, Index: 0, Checked: true}, http.StatusConflict, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-2/review/steps", ReviewStepRequest{Base: saved.Hash, Index: 9, Checked: true}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPost, "/api/tickets/AL-3/review/steps", ReviewStepRequest{Base: "x", Index: 0, Checked: true}, http.StatusNotFound, nil)
}
