package api

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// The API contract (FH-97): every endpoint's response, as the handlers
// write it for one scripted session on the sample board, is kept in
// testdata/api/<name>.json, and the frontend's guards are run over the same
// files (frontend/src/api/contract.test.ts). A field dropped or renamed here
// changes a fixture, which fails this test until it is rewritten with
//
//	go test ./internal/api -run TestAPIContract -update
//
// and the rewritten fixture fails the frontend's guards until they agree.

var update = flag.Bool("update", false, "rewrite testdata/api from the handlers' responses")

const contractDir = "../../testdata/api"

// contractTime is the clock of the scripted session: the sample log's
// session is live and waits on a permission prompt.
var contractTime = time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC)

// routesWithoutFixture are the registered routes whose response has no
// fixture of its own, and why.
var routesWithoutFixture = map[string]string{
	"/api/projects/{project}/tickets/{id}":              "the same handler and response as /api/tickets/{id}",
	"/api/projects/{project}/tickets/{id}/files/{file}": "serves a file's bytes, not JSON",
}

// exchange is one request of the scripted session. route is the pattern as
// registered, so every route can be checked to have a fixture; shape is the
// Go type the handler writes (nil for errors and empty bodies).
type exchange struct {
	name   string
	route  string
	method string
	path   string
	body   any
	status int
	shape  any
}

// Fixture is one file in testdata/api.
type Fixture struct {
	Route   string          `json:"route"`
	Request string          `json:"request"`
	Status  int             `json:"status"`
	Body    json.RawMessage `json:"body"`
}

type contract struct {
	t        *testing.T
	handler  http.Handler
	roots    []string
	fixtures map[string][]byte
	routes   map[string]bool
	shapes   []any
	// placeholders stand in for values that differ from run to run:
	// content hashes over activity entries with random ids, and those ids;
	// numbered within each fixture, so a new step leaves the others be.
	placeholders map[string]string
}

var (
	// randomValue matches a JSON string that is a sha256 hex digest (a
	// content hash or delete token) or an activity entry id (eight hex
	// digits, random).
	randomValue = regexp.MustCompile(`"([0-9a-f]{64}|[0-9a-f]{8})"`)
	// entryInPath matches an activity entry id in a request path.
	entryInPath = regexp.MustCompile(`/[0-9a-f]{8}$`)
	// fileTime matches the fields read from file modification times.
	fileTime = regexp.MustCompile(`"(modified|lastModified|archived)":"\d{4}-[^"]*"`)
	// revision matches the board revision, which counts the steps before.
	revision = regexp.MustCompile(`"revision":\d+`)
)

func (c *contract) normalise(raw []byte) []byte {
	text := string(raw)
	for _, root := range c.roots {
		text = strings.ReplaceAll(text, root, "/board")
	}
	text = randomValue.ReplaceAllStringFunc(text, func(match string) string {
		return `"` + c.placeholder(strings.Trim(match, `"`)) + `"`
	})
	text = revision.ReplaceAllString(text, `"revision":1`)
	return []byte(fileTime.ReplaceAllString(text, `"$1":"2026-10-04T12:00:00Z"`))
}

// placeholder stands in for a random value, the same one each time it is
// seen.
func (c *contract) placeholder(value string) string {
	placeholder, ok := c.placeholders[value]
	if !ok {
		kind := "hash"
		if len(value) == 8 {
			kind = "entry"
		}
		placeholder = fmt.Sprintf("%s-%02d", kind, len(c.placeholders)+1)
		c.placeholders[value] = placeholder
	}
	return placeholder
}

// do sends one request, checks its status, records its fixture and returns
// the raw response body for the next steps.
func (c *contract) do(x exchange) []byte {
	c.t.Helper()
	var reader *bytes.Reader
	switch body := x.body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case []byte:
		reader = bytes.NewReader(body)
	default:
		data, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	request := httptest.NewRequest(x.method, x.path, reader)
	if _, ok := x.body.([]byte); ok {
		request.Header.Set("Content-Type", "application/octet-stream")
	}
	recorder := httptest.NewRecorder()
	c.handler.ServeHTTP(recorder, request)
	raw := recorder.Body.Bytes()
	if recorder.Code != x.status {
		c.t.Fatalf("%s %s status = %d, want %d: %s", x.method, x.path, recorder.Code, x.status, raw)
	}
	if x.name == "" {
		return raw
	}
	if _, taken := c.fixtures[x.name]; taken {
		c.t.Fatalf("two fixtures named %s", x.name)
	}
	c.placeholders = map[string]string{}
	body := json.RawMessage("null")
	if len(bytes.TrimSpace(raw)) > 0 {
		body = c.normalise(raw)
	}
	path := entryInPath.ReplaceAllStringFunc(x.path, func(match string) string { return "/" + c.placeholder(match[1:]) })
	var fixture bytes.Buffer
	encoder := json.NewEncoder(&fixture)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(Fixture{Route: x.route, Request: x.method + " " + path, Status: x.status, Body: body}); err != nil {
		c.t.Fatalf("%s: %v", x.name, err)
	}
	c.fixtures[x.name] = fixture.Bytes()
	c.routes[x.route] = true
	if x.shape != nil {
		c.shapes = append(c.shapes, x.shape)
	}
	return raw
}

func decodeAs[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode %T: %v\n%s", value, err, raw)
	}
	return value
}

// runContract plays the scripted session: reads of the sample board with a
// live session that asked a question, every write, then reads of what the
// writes left, the archives and the errors the browser branches on.
func runContract(t *testing.T) *contract {
	t.Helper()
	root := copyBoard(t, "sample")
	// Every list a guard reads has an item: a project with no key yet,
	// for a key that is taken (KEY-5); a ticket and a workstream blocked
	// from outside; references to the archived AL-8 and to beta, whose
	// deletes rewrite them; an archived project with a repository.
	writeFile(t, filepath.Join(root, "gamma", "project.yaml"), "name: Gamma\n")
	writeFile(t, filepath.Join(root, "alpha", "workstreams", "offline.md"), "---\nslug: offline\nstatus: active\npriority: low\ncreated: 2026-10-02\n"+
		"tickets:\n  - AL-6\ndepends-on-workstreams: [board-ui]\ntags: []\n---\n\n# Offline\n\n## Goal\n\nWork without the server.\n")
	edit := func(name, old, new string) {
		t.Helper()
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Contains(data, []byte(old)) {
			t.Fatalf("%s has no %q: %v", name, old, err)
		}
		writeFile(t, path, strings.Replace(string(data), old, new, 1))
	}
	edit("alpha/tickets/AL-2-board-columns/AL-2-board-columns.md", "depends-on: []", "depends-on: [AL-6]")
	edit("alpha/tickets/AL-1-project-skeleton/AL-1-project-skeleton.md", "depends-on: []", "depends-on: [BE-1, AL-8]")
	edit("alpha/workstreams/board-ui.md", "  - AL-4\n", "  - AL-4\n  - AL-8\n")
	edit("beta/project.yaml", "key: BE\n", "key: BE\nrepos:\n  - /Users/example/src/beta\n")
	// Projects are listed newest file first (summaries). The edits above are
	// only as far apart as the filesystem's clock, which ties them on Linux
	// (FH-176), so pin every file, then one per project: beta newest, then
	// alpha, then gamma, on every platform.
	if err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, contractTime, contractTime)
	}); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"gamma/project.yaml", "alpha/project.yaml", "beta/project.yaml"} {
		when := contractTime.Add(time.Duration(i+1) * time.Hour)
		if err := os.Chtimes(filepath.Join(root, name), when, when); err != nil {
			t.Fatal(err)
		}
	}
	files := store.New(root)
	t.Cleanup(func() { files.Close() })
	files.SetClock(func() time.Time { return contractTime })
	log := events.New(files)
	clock := func() time.Time { return contractTime }
	asked := events.Event{Time: contractTime.Add(-2 * time.Minute), Run: sampleSession, Agent: "claude", Kind: events.QuestionAsked, Project: "alpha",
		Data: events.QuestionData{ID: "q-1", Ticket: "AL-3", Kind: "decision", Text: "Keep the old panel?", Options: []string{"Yes", "No"}}}
	if err := log.Append(asked); err != nil {
		t.Fatal(err)
	}
	roots := []string{root}
	if resolved, err := filepath.EvalSymlinks(root); err == nil && resolved != root {
		roots = append(roots, resolved)
	}
	c := &contract{
		t: t,
		handler: New(Options{
			Info:       Info{Version: "1.2.3", Commit: "abc1234", BuildDate: "2026-10-04", ProtocolVersion: 2, Root: root, Theme: "system"},
			Board:      index.New(files, index.Options{Now: clock, Events: log, Runs: runs.DefaultSettings(), MaxAge: time.Nanosecond}),
			Files:      files,
			Writer:     files.As("Robert"),
			Events:     log,
			Now:        clock,
			DoneLimit:  20,
			AnsweredBy: "Robert",
		}),
		roots: roots, fixtures: map[string][]byte{}, routes: map[string]bool{}, placeholders: map[string]string{},
	}
	ticket := func(id string) TicketDetail {
		return decodeAs[TicketResponse](t, c.do(exchange{method: http.MethodGet, path: "/api/tickets/" + id, status: http.StatusOK})).Ticket
	}
	get := func(name, route, path string, shape any) []byte {
		return c.do(exchange{name: name, route: route, method: http.MethodGet, path: path, status: http.StatusOK, shape: shape})
	}

	// Reads.
	get("info", "/api/info", "/api/info", Info{})
	get("projects", "/api/projects", "/api/projects", ProjectsResponse{})
	get("board", "/api/projects/{project}/board", "/api/projects/alpha/board", BoardResponse{})
	get("all-board", "/api/all/board", "/api/all/board", AllBoardResponse{})
	get("runs", "/api/runs", "/api/runs?ended=all", RunsResponse{})
	get("run", "/api/run", "/api/run?id="+sampleSession, RunResponse{})
	get("changes", "/api/changes", "/api/changes?since=0", ChangesResponse{})
	get("workstreams", "/api/projects/{project}/workstreams", "/api/projects/alpha/workstreams", WorkstreamsResponse{})
	workstream := decodeAs[WorkstreamResponse](t, get("workstream", "/api/projects/{project}/workstreams/{slug}", "/api/projects/alpha/workstreams/board-ui", WorkstreamResponse{})).Workstream
	c.do(exchange{name: "preferences-saved", route: "PATCH /api/preferences", method: http.MethodPatch, path: "/api/preferences", status: http.StatusNoContent,
		body: map[string]any{
			"theme": "dark", "density": "compact", "colourBy": "priority", "hiddenColumns": []string{"backlog"}, "editor": "markdown",
			"scopes": map[string]any{"alpha": map[string]any{
				"view": "board", "type": map[string]any{"include": []string{"bug"}, "exclude": []string{}}, "priority": map[string]any{"include": []string{}, "exclude": []string{"low"}},
				"workstream": map[string]any{"include": []string{"board-ui"}, "exclude": []string{}}, "age": map[string]any{"include": []string{"week"}, "exclude": []string{}}, "state": "blocked",
			}},
			"expandedWorkstreams": map[string]any{"alpha": []string{"board-ui"}},
		}})
	get("preferences", "GET /api/preferences", "/api/preferences", Preferences{})

	// Comments and change requests.
	comments := "POST /api/tickets/{id}/comments"
	posted := decodeAs[CommentResponse](t, c.do(exchange{name: "comment-added", route: comments, method: http.MethodPost, path: "/api/tickets/AL-2/comments",
		body: CommentRequest{Body: "The columns need a gap.", Quote: "Columns render", QuoteFrom: "Summary"}, status: http.StatusCreated, shape: CommentResponse{}}))
	c.do(exchange{name: "comment-sent", route: comments, method: http.MethodPost, path: "/api/tickets/AL-3/comments",
		body: CommentRequest{Body: "Use the shared panel.", SendTo: sampleSession}, status: http.StatusCreated, shape: CommentResponse{}})
	edited := decodeAs[CommentResponse](t, c.do(exchange{name: "comment-edited", route: "PATCH /api/tickets/{id}/comments/{comment}", method: http.MethodPatch,
		path: "/api/tickets/AL-2/comments/" + posted.Comment.ID, body: CommentEditRequest{Base: posted.ActivityHash, Body: "The columns need a wider gap."},
		status: http.StatusOK, shape: CommentResponse{}}))
	c.do(exchange{name: "changes-requested", route: "POST /api/tickets/{id}/request-changes", method: http.MethodPost, path: "/api/tickets/AL-2/request-changes",
		body: RequestChangesRequest{Comments: []string{edited.Comment.ID}}, status: http.StatusOK, shape: RequestChangesResponse{}})
	c.do(exchange{name: "request-resolved", route: "PATCH /api/tickets/{id}/comments/{comment}", method: http.MethodPatch, path: "/api/tickets/AL-2/comments/" + edited.Comment.ID,
		body: CommentEditRequest{State: "resolved"}, status: http.StatusOK, shape: CommentResponse{}})
	extra := decodeAs[CommentResponse](t, c.do(exchange{method: http.MethodPost, path: "/api/tickets/AL-3/comments", body: CommentRequest{Body: "Posted by mistake."}, status: http.StatusCreated}))
	c.do(exchange{name: "comment-deleted", route: "DELETE /api/tickets/{id}/comments/{comment}", method: http.MethodDelete,
		path: "/api/tickets/AL-3/comments/" + extra.Comment.ID, body: CommentDeleteRequest{Base: extra.ActivityHash}, status: http.StatusOK, shape: CommentDeletedResponse{}})

	// Ticket writes.
	saved := func(name, route, method, path string, body any) WriteResponse {
		return decodeAs[WriteResponse](t, c.do(exchange{name: name, route: route, method: method, path: path, body: body, status: http.StatusOK, shape: WriteResponse{}}))
	}
	al4 := ticket("AL-4")
	moved := saved("moved", "POST /api/tickets/{id}/move", http.MethodPost, "/api/tickets/AL-4/move", MoveRequest{Base: al4.Hash, To: "backlog"})
	patched := saved("patched", "PATCH /api/tickets/{id}", http.MethodPatch, "/api/tickets/AL-4", map[string]any{"base": moved.Hash, "fields": map[string]any{"priority": "high", "tags": []string{"ui"}}})
	saved("section-saved", "PUT /api/tickets/{id}/section", http.MethodPut, "/api/tickets/AL-4/section", SectionRequest{Base: patched.Hash, Section: board.SectionContext, Content: "Use the pointer events API."})
	al6 := ticket("AL-6")
	saved("raw-saved", "PUT /api/tickets/{id}/raw", http.MethodPut, "/api/tickets/AL-6/raw", RawRequest{Base: al6.Hash, Content: strings.Replace(al6.Raw, "priority: low", "priority: medium", 1)})
	saved("criterion-set", "POST /api/tickets/{id}/criteria", http.MethodPost, "/api/tickets/AL-3/criteria", CriterionRequest{Base: ticket("AL-3").Hash, Index: 1, Checked: true})
	saved("review-step-set", "POST /api/tickets/{id}/review/steps", http.MethodPost, "/api/tickets/AL-2/review/steps", ReviewStepRequest{Base: ticket("AL-2").Review.Hash, Index: 0, Checked: true})
	saved("archived", "POST /api/tickets/{id}/archive", http.MethodPost, "/api/tickets/AL-6/archive", nil)
	saved("unarchived", "POST /api/tickets/{id}/unarchive", http.MethodPost, "/api/tickets/AL-6/unarchive", nil)
	c.do(exchange{name: "uploaded", route: "POST /api/tickets/{id}/files", method: http.MethodPost, path: "/api/tickets/AL-3/files?name=panel.png&kind=screenshot",
		body: []byte(uploadPNG), status: http.StatusCreated, shape: UploadResponse{}})
	c.do(exchange{name: "ticket-created", route: "POST /api/projects/{project}/tickets", method: http.MethodPost, path: "/api/projects/alpha/tickets",
		body: CreateRequest{Title: "Column headers", Type: "feature", Priority: "medium", Status: "backlog", Workstream: "board-ui", Description: "Name each column.",
			Criteria: []string{"Headers show counts"}, DependsOn: []string{"AL-2"}, Tags: []string{"ui"}}, status: http.StatusCreated, shape: CreateResponse{}})
	createWorkstream := "POST /api/projects/{project}/workstreams"
	createdWorkstream := decodeAs[CreatedWorkstreamResponse](t, c.do(exchange{name: "workstream-created", route: createWorkstream, method: http.MethodPost,
		path: "/api/projects/alpha/workstreams", body: NewWorkstreamRequest{Title: "Sync", Goal: "Share the board.", Priority: "low", DependsOnWorkstreams: []string{"board-ui"}},
		status: http.StatusCreated, shape: CreatedWorkstreamResponse{}}))
	editWorkstream := "PATCH /api/projects/{project}/workstreams/{slug}"
	editedWorkstream := decodeAs[EditedWorkstreamResponse](t, c.do(exchange{name: "workstream-edited", route: editWorkstream, method: http.MethodPatch,
		path: "/api/projects/alpha/workstreams/sync", body: map[string]any{"base": createdWorkstream.Hash, "fields": map[string]any{"title": "Sync everywhere", "tags": []string{"sync"}}, "slug": "sync-everywhere"},
		status: http.StatusOK, shape: EditedWorkstreamResponse{}}))
	// A name whose slug another workstream has (FH-167).
	c.do(exchange{name: "error-name-exists", route: editWorkstream, method: http.MethodPatch, path: "/api/projects/alpha/workstreams/sync-everywhere",
		body: map[string]any{"base": editedWorkstream.Hash, "fields": map[string]any{"title": "Offline"}, "slugFromTitle": true}, status: http.StatusConflict})
	c.do(exchange{name: "workstream-named", route: editWorkstream, method: http.MethodPatch, path: "/api/projects/alpha/workstreams/sync-everywhere",
		body:   map[string]any{"base": editedWorkstream.Hash, "fields": map[string]any{"title": "Sync anywhere"}, "slugFromTitle": true},
		status: http.StatusOK, shape: EditedWorkstreamResponse{}})
	c.do(exchange{name: "error-title-taken", route: createWorkstream, method: http.MethodPost, path: "/api/projects/alpha/workstreams",
		body: NewWorkstreamRequest{Title: "Board UI", Goal: "Again.", Priority: "low"}, status: http.StatusConflict})
	saved("workstream-ordered", "PUT /api/projects/{project}/workstreams/{slug}/order", http.MethodPut, "/api/projects/alpha/workstreams/board-ui/order",
		OrderRequest{Tickets: []string{"AL-3", "AL-2", "AL-4", "AL-8", "AL-9"}})
	saved("workstream-section-saved", "PUT /api/projects/{project}/workstreams/{slug}/section", http.MethodPut, "/api/projects/alpha/workstreams/board-ui/section",
		SectionRequest{Base: decodeAs[WorkstreamResponse](t, c.do(exchange{method: http.MethodGet, path: "/api/projects/alpha/workstreams/" + workstream.Slug, status: http.StatusOK})).Workstream.Hash,
			Section: "Scope", Content: "Columns and the card panel."})
	c.do(exchange{name: "error-key-taken", route: "PUT /api/projects/{project}/key", method: http.MethodPut, path: "/api/projects/gamma/key", body: KeyRequest{Key: "AL"}, status: http.StatusConflict})
	saved("key-set", "PUT /api/projects/{project}/key", http.MethodPut, "/api/projects/gamma/key", KeyRequest{Key: "GA"})
	saved("answered", "POST /api/questions/{id}/answer", http.MethodPost, "/api/questions/q-1/answer", AnswerRequest{Answer: "Yes"})

	// What the writes left.
	get("ticket", "/api/tickets/{id}", "/api/tickets/AL-3", TicketResponse{})
	get("ticket-in-review", "/api/tickets/{id}", "/api/tickets/AL-2", TicketResponse{})
	get("ticket-edited", "/api/tickets/{id}", "/api/tickets/AL-4", TicketResponse{})

	// Archives.
	get("archive", "GET /api/projects/{project}/archive", "/api/projects/alpha/archive", ArchiveResponse{})
	plan := decodeAs[DeletePlanJSON](t, get("delete-plan", "GET /api/projects/{project}/archive/{id}", "/api/projects/alpha/archive/AL-8", DeletePlanJSON{}))
	saved("archived-deleted", "POST /api/projects/{project}/archive/{id}/delete", http.MethodPost, "/api/projects/alpha/archive/AL-8/delete", DeleteRequest{Token: plan.Token, Confirm: "AL-8"})
	get("archive-check", "GET /api/projects/{project}/archive-check", "/api/projects/beta/archive-check", ArchiveCheckJSON{})
	saved("project-archived", "POST /api/projects/{project}/archive-project", http.MethodPost, "/api/projects/beta/archive-project", nil)
	get("archived-projects", "GET /api/archived-projects", "/api/archived-projects", ArchivedProjectsResponse{})
	get("project-delete-plan", "GET /api/archived-projects/{project}", "/api/archived-projects/beta", ProjectDeletePlanJSON{})
	saved("project-restored", "POST /api/archived-projects/{project}/restore", http.MethodPost, "/api/archived-projects/beta/restore", nil)
	c.do(exchange{method: http.MethodPost, path: "/api/projects/beta/archive-project", status: http.StatusOK})
	projectPlan := decodeAs[ProjectDeletePlanJSON](t, c.do(exchange{method: http.MethodGet, path: "/api/archived-projects/beta", status: http.StatusOK}))
	saved("project-deleted", "POST /api/archived-projects/{project}/delete", http.MethodPost, "/api/archived-projects/beta/delete", DeleteRequest{Token: projectPlan.Token, Confirm: "beta"})

	// Errors the browser reads: the common body, a conflict's current
	// file and a blocked start's reasons.
	c.do(exchange{name: "error-not-found", route: "/api/", method: http.MethodGet, path: "/api/nothing", status: http.StatusNotFound})
	c.do(exchange{name: "error-conflict", route: "PATCH /api/tickets/{id}", method: http.MethodPatch, path: "/api/tickets/AL-5",
		body: map[string]any{"base": "stale", "fields": map[string]any{"priority": "low"}}, status: http.StatusConflict})
	c.do(exchange{name: "error-confirm-blocked", route: "POST /api/tickets/{id}/move", method: http.MethodPost, path: "/api/tickets/AL-5/move",
		body: MoveRequest{Base: ticket("AL-5").Hash, To: "in-progress"}, status: http.StatusConflict})
	return c
}

// registeredRoutes reads the patterns the API registers from its source.
func registeredRoutes(t *testing.T) []string {
	t.Helper()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`mux\.Handle(?:Func)?\("([^"]+)"`)
	var routes []string
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range pattern.FindAllSubmatch(data, -1) {
			routes = append(routes, string(match[1]))
		}
	}
	slices.Sort(routes)
	return routes
}

// domainPackages are the packages whose types a response never carries,
// since that would make their internals the wire format (FH-97, FH-175).
var domainPackages = []string{"/internal/board", "/internal/config", "/internal/events", "/internal/runs"}

// domainType reports the first domain type a response type carries.
func domainType(kind reflect.Type, seen map[reflect.Type]bool) string {
	if seen[kind] {
		return ""
	}
	seen[kind] = true
	if path := kind.PkgPath(); slices.ContainsFunc(domainPackages, func(pkg string) bool { return strings.HasSuffix(path, pkg) }) {
		return kind.String()
	}
	switch kind.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return domainType(kind.Elem(), seen)
	case reflect.Map:
		if found := domainType(kind.Key(), seen); found != "" {
			return found
		}
		return domainType(kind.Elem(), seen)
	case reflect.Struct:
		for i := range kind.NumField() {
			field := kind.Field(i)
			if found := domainType(field.Type, seen); found != "" {
				return field.Name + ": " + found
			}
		}
	}
	return ""
}

func TestAPIContract(t *testing.T) {
	t.Parallel()

	c := runContract(t)

	t.Run("fixtures are current", func(t *testing.T) {
		existing, err := filepath.Glob(filepath.Join(contractDir, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		if *update {
			if err := os.MkdirAll(contractDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, path := range existing {
				if _, kept := c.fixtures[strings.TrimSuffix(filepath.Base(path), ".json")]; !kept {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
			}
			for name, data := range c.fixtures {
				if err := os.WriteFile(filepath.Join(contractDir, name+".json"), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			return
		}
		const hint = "; rewrite the fixtures with go test ./internal/api -run TestAPIContract -update, then make the frontend's guards agree"
		for _, path := range existing {
			if _, kept := c.fixtures[strings.TrimSuffix(filepath.Base(path), ".json")]; !kept {
				t.Errorf("%s is no longer written%s", path, hint)
			}
		}
		for name, want := range c.fixtures {
			got, err := os.ReadFile(filepath.Join(contractDir, name+".json"))
			if err != nil {
				t.Errorf("%s: %v%s", name, err, hint)
				continue
			}
			if !bytes.Equal(got, want) {
				t.Errorf("testdata/api/%s.json is stale%s\n--- handler wrote ---\n%s", name, hint, want)
			}
		}
	})

	t.Run("every route has a fixture", func(t *testing.T) {
		for _, route := range registeredRoutes(t) {
			_, exempt := routesWithoutFixture[route]
			if !c.routes[route] && !exempt {
				t.Errorf("route %q has no fixture in runContract (or a reason in routesWithoutFixture)", route)
			}
			if c.routes[route] && exempt {
				t.Errorf("route %q has a fixture and an exemption", route)
			}
		}
	})

	t.Run("responses carry no board, config, runs or events types", func(t *testing.T) {
		for _, shape := range c.shapes {
			if found := domainType(reflect.TypeOf(shape), map[reflect.Type]bool{}); found != "" {
				t.Errorf("%T sends %s as it is; map it to an API type", shape, found)
			}
		}
	})
}
