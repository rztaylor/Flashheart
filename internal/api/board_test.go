package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

func copyBoard(t *testing.T, fixture string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "boards", fixture))); err != nil {
		t.Fatal(err)
	}
	return root
}

func handlerFor(t *testing.T, root string) http.Handler {
	t.Helper()
	files := store.New(root)
	t.Cleanup(func() { files.Close() })
	return New(Options{
		Info:      Info{Root: root, Theme: "system"},
		Board:     index.New(files, index.Options{}),
		Files:     files,
		DoneLimit: 20,
	})
}

func sampleAPI(t *testing.T, mutate func(root string)) (http.Handler, string) {
	t.Helper()
	root := copyBoard(t, "sample")
	if mutate != nil {
		mutate(root)
	}
	return handlerFor(t, root), root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func getJSON(t *testing.T, handler http.Handler, path string, status int, into any) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != status {
		t.Fatalf("GET %s status = %d, want %d: %s", path, recorder.Code, status, recorder.Body.String())
	}
	if into != nil {
		if err := json.Unmarshal(recorder.Body.Bytes(), into); err != nil {
			t.Fatalf("GET %s decode: %v\n%s", path, err, recorder.Body.String())
		}
	}
	return recorder
}

func cardByID(t *testing.T, cards []Card, id string) Card {
	t.Helper()
	for _, card := range cards {
		if card.ID == id {
			return card
		}
	}
	t.Fatalf("no card %s", id)
	return Card{}
}

func TestProjectsListsKeysCountsAndActivity(t *testing.T) {
	t.Parallel()

	handler, root := sampleAPI(t, nil)
	newer := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(root, "beta", "tickets", "BE-1-hello", "BE-1-hello.md"), newer, newer); err != nil {
		t.Fatal(err)
	}
	var body ProjectsResponse
	getJSON(t, handler, "/api/projects", http.StatusOK, &body)
	if body.Revision != 1 || body.RootMissing || body.Root != root || len(body.V1Projects) != 0 || body.MigrateCommand != "" {
		t.Errorf("response = %+v", body)
	}
	if len(body.Projects) != 2 || body.Projects[0].Name != "beta" || body.Projects[1].Name != "alpha" {
		t.Fatalf("projects not sorted by activity: %+v", body.Projects)
	}
	alpha, beta := body.Projects[1], body.Projects[0]
	if alpha.DisplayName != "Alpha" || alpha.Key != "AL" || alpha.KeyDerived || alpha.NeedsRepair != 1 || alpha.Blocked != 3 || alpha.Stuck != 2 {
		t.Errorf("alpha = %+v", alpha)
	}
	if beta.Key != "BE" || beta.Blocked != 1 || beta.Stuck != 1 {
		t.Errorf("beta = %+v", beta)
	}
	want := map[string]int{"backlog": 3, "up-next": 1, "in-progress": 1, "review": 1, "done": 1}
	for column, count := range want {
		if alpha.Counts[column] != count {
			t.Errorf("alpha %s count = %d, want %d", column, alpha.Counts[column], count)
		}
	}
	if len(alpha.Workstreams) != 1 || alpha.Workstreams[0] != (WorkstreamBrief{Slug: "board-ui", Title: "Board UI", Created: "2026-10-01", Status: "active", Done: 1, Total: 3}) {
		t.Errorf("alpha workstreams = %+v", alpha.Workstreams)
	}
	if beta.Workstreams == nil || len(beta.Workstreams) != 0 {
		t.Errorf("beta workstreams = %#v, want an empty list", beta.Workstreams)
	}
}

func TestProjectBoardPlacesAndExplainsEveryTicket(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, nil)
	var body BoardResponse
	getJSON(t, handler, "/api/projects/alpha/board", http.StatusOK, &body)
	if body.Project.Name != "alpha" || len(body.Cards) != 7 || body.DoneTotal != 1 {
		t.Fatalf("board = %+v with %d cards", body.Project, len(body.Cards))
	}
	var order []string
	for _, card := range body.Cards {
		order = append(order, card.Column+"/"+card.ID)
	}
	want := []string{"backlog/AL-5", "backlog/AL-6", "backlog/AL-7", "up-next/AL-4", "in-progress/AL-3", "review/AL-2", "done/AL-1"}
	if !slices.Equal(order, want) {
		t.Errorf("order =\n  %q\nwant\n  %q", order, want)
	}

	drag := cardByID(t, body.Cards, "AL-4")
	if !drag.Blocked || len(drag.BlockedBy) != 1 || drag.BlockedBy[0].Text != "Comes after AL-3 in workstream board-ui, which is In progress" || drag.Slug != "drag-and-drop" {
		t.Errorf("AL-4 = %+v", drag)
	}
	overflow := cardByID(t, body.Cards, "AL-5")
	if len(overflow.BlockedBy) != 1 || overflow.BlockedBy[0].Text != "Depends on AL-4, which is Up next" || overflow.BlockedBy[0].Ticket == nil {
		t.Errorf("AL-5 blocked by %+v", overflow.BlockedBy)
	}
	offline := cardByID(t, body.Cards, "AL-6")
	if len(offline.BlockedBy) != 1 || !offline.BlockedBy[0].Missing || !strings.Contains(offline.BlockedBy[0].Text, "does not exist") {
		t.Errorf("AL-6 blocked by %+v", offline.BlockedBy)
	}
	broken := cardByID(t, body.Cards, "AL-7")
	if len(broken.NeedsRepair) != 1 || broken.Title != "Broken frontmatter" || broken.Blocked {
		t.Errorf("AL-7 = %+v", broken)
	}
	panel := cardByID(t, body.Cards, "AL-3")
	if panel.Blocked || panel.HandoffNext != "Review tab" || panel.Criteria != (Progress{Done: 1, Total: 2}) || panel.Workstream != "board-ui" || panel.SearchText == "" {
		t.Errorf("AL-3 = %+v", panel)
	}
	columns := cardByID(t, body.Cards, "AL-2")
	if !columns.HasReview || columns.Attachments != 1 {
		t.Errorf("AL-2 review=%v attachments=%d", columns.HasReview, columns.Attachments)
	}

	var beta BoardResponse
	getJSON(t, handler, "/api/projects/beta/board", http.StatusOK, &beta)
	if hello := cardByID(t, beta.Cards, "BE-1"); len(hello.BlockedBy) != 1 || hello.BlockedBy[0].Text != "Depends on AL-3, which is In progress" || hello.BlockedBy[0].Ticket.Project != "alpha" {
		t.Errorf("cross-project block = %+v", hello.BlockedBy)
	}
}

func TestDoneColumnIsLimitedUnlessAllIsRequested(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, func(root string) {
		for n := range 25 {
			id := fmt.Sprintf("BE-%d", n+10)
			name := filepath.Join(root, "beta", "tickets", id+"-done", id+"-done.md")
			writeFile(t, name, "---\nid: "+id+"\nstatus: done\n---\n# Done\n")
			at := time.Now().Add(time.Duration(n) * time.Minute)
			_ = os.Chtimes(name, at, at)
		}
	})
	countDone := func(cards []Card) int {
		n := 0
		for _, card := range cards {
			if card.Column == "done" {
				n++
			}
		}
		return n
	}
	var limited BoardResponse
	getJSON(t, handler, "/api/projects/beta/board", http.StatusOK, &limited)
	if countDone(limited.Cards) != 20 || limited.DoneTotal != 25 {
		t.Errorf("limited done = %d of %d", countDone(limited.Cards), limited.DoneTotal)
	}
	if first := limited.Cards[1]; first.ID != "BE-34" {
		t.Errorf("done column not newest first: %s", first.ID)
	}
	var all BoardResponse
	getJSON(t, handler, "/api/projects/beta/board?done=all", http.StatusOK, &all)
	if countDone(all.Cards) != 25 {
		t.Errorf("done=all shows %d", countDone(all.Cards))
	}
}

func TestTicketDetailByID(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, nil)
	for _, path := range []string{"/api/tickets/AL-2", "/api/projects/alpha/tickets/AL-2"} {
		var body TicketResponse
		getJSON(t, handler, path, http.StatusOK, &body)
		ticket := body.Ticket
		if ticket.ID != "AL-2" || ticket.Review == nil || !strings.HasPrefix(ticket.Review.Markdown, "# Review: Board columns") {
			t.Errorf("%s review = %+v", path, ticket.Review)
		}
		if len(ticket.AttachmentFiles) != 1 || ticket.AttachmentFiles[0].URL != "/api/projects/alpha/tickets/AL-2/files/20261003T1000-board-desktop.png" {
			t.Errorf("%s files = %+v", path, ticket.AttachmentFiles)
		}
		if !strings.Contains(ticket.Body, "## Acceptance Criteria") || !strings.Contains(ticket.Frontmatter, "id: AL-2") || len(ticket.CriteriaItems) != 2 {
			t.Errorf("%s ticket = %+v", path, ticket)
		}
	}

	var panel TicketResponse
	getJSON(t, handler, "/api/tickets/AL-3", http.StatusOK, &panel)
	if panel.Ticket.Handoff == nil || panel.Ticket.Handoff.Next[0] != "Review tab" || panel.Ticket.Review != nil {
		t.Errorf("AL-3 handoff=%+v review=%+v", panel.Ticket.Handoff, panel.Ticket.Review)
	}

	getJSON(t, handler, "/api/tickets/AL-99", http.StatusNotFound, nil)
	getJSON(t, handler, "/api/projects/beta/tickets/AL-2", http.StatusNotFound, nil)
	getJSON(t, handler, "/api/projects/nope/tickets/AL-2", http.StatusNotFound, nil)
}

func TestWorkstreams(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, nil)
	var body WorkstreamsResponse
	getJSON(t, handler, "/api/projects/alpha/workstreams", http.StatusOK, &body)
	if len(body.Workstreams) != 1 {
		t.Fatalf("workstreams = %+v", body.Workstreams)
	}
	ws := body.Workstreams[0]
	if ws.Slug != "board-ui" || ws.Title != "Board UI" || ws.Status != "active" || ws.Done != 1 || ws.Total != 3 || ws.Next != "AL-3" || ws.Suspended {
		t.Errorf("workstream = %+v", ws)
	}
	var tickets []string
	for _, ticket := range ws.Tickets {
		tickets = append(tickets, ticket.ID+":"+ticket.Column)
	}
	if want := []string{"AL-2:review", "AL-3:in-progress", "AL-4:up-next"}; !slices.Equal(tickets, want) {
		t.Errorf("tickets = %q", tickets)
	}
	if !ws.Tickets[2].Blocked || ws.Tickets[1].Blocked || ws.Tickets[2].Held {
		t.Errorf("blocked flags = %+v", ws.Tickets)
	}
}

func TestHeldStationsAreBlockedFromOutsideTheirLine(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, func(root string) {
		path := filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "AL-4-drag-and-drop.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, strings.Replace(string(data), "depends-on: []", "depends-on: [AL-6]", 1))
	})
	var body WorkstreamsResponse
	getJSON(t, handler, "/api/projects/alpha/workstreams", http.StatusOK, &body)
	if drag := body.Workstreams[0].Tickets[2]; !drag.Held || !drag.Blocked {
		t.Errorf("AL-4 = %+v, want held by its ticket dependency", drag)
	}
}

func TestSuspendedLinesWaitOnTheirOwnWorkstreamDependencies(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, func(root string) {
		writeFile(t, filepath.Join(root, "alpha", "workstreams", "later.md"), "---\ntickets: [AL-6]\ndepends-on-workstreams: [board-ui]\n---\n# Later\n")
		writeFile(t, filepath.Join(root, "alpha", "workstreams", "free.md"), "---\ntickets: [AL-5]\n---\n# Free\n")
	})
	var body WorkstreamsResponse
	getJSON(t, handler, "/api/projects/alpha/workstreams", http.StatusOK, &body)
	suspended := map[string]bool{}
	for _, workstream := range body.Workstreams {
		suspended[workstream.Slug] = workstream.Suspended
	}
	if !suspended["later"] || suspended["free"] || suspended["board-ui"] {
		t.Errorf("suspended = %v, want only later", suspended)
	}
}

func TestAllProjectsBoard(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, nil)
	var body AllBoardResponse
	getJSON(t, handler, "/api/all/board", http.StatusOK, &body)
	if len(body.Projects) != 2 || len(body.Cards) != 8 {
		t.Fatalf("all board = %d projects, %d cards", len(body.Projects), len(body.Cards))
	}
	if hello := cardByID(t, body.Cards, "BE-1"); hello.Project != "beta" || hello.SearchText != "" {
		t.Errorf("BE-1 = %+v (all-projects cards omit body text)", hello)
	}
}

func TestFilesAreServedSafely(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, func(root string) {
		writeFile(t, filepath.Join(root, "alpha", "tickets", "AL-2-board-columns", "files", "x.svg"), "<svg/>")
	})
	recorder := getJSON(t, handler, "/api/projects/alpha/tickets/AL-2/files/20261003T1000-board-desktop.png", http.StatusOK, nil)
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'none'") || !strings.Contains(got, "sandbox") {
		t.Errorf("Content-Security-Policy = %q", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	getJSON(t, handler, "/api/projects/alpha/tickets/AL-2/files/x.svg", http.StatusUnsupportedMediaType, nil)
	getJSON(t, handler, "/api/projects/alpha/tickets/AL-2/files/absent.png", http.StatusNotFound, nil)
	getJSON(t, handler, "/api/projects/alpha/tickets/AL-2/files/..%2F..%2Fx.png", http.StatusBadRequest, nil)
	getJSON(t, handler, "/api/projects/alpha/tickets/AL-99/files/x.png", http.StatusNotFound, nil)
}

func TestMissingRootV1RootAndUnknownProject(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "absent")
	var body ProjectsResponse
	getJSON(t, handlerFor(t, missing), "/api/projects", http.StatusOK, &body)
	if !body.RootMissing || len(body.Projects) != 0 {
		t.Errorf("missing root = %+v", body)
	}
	getJSON(t, handlerFor(t, missing), "/api/projects/alpha/board", http.StatusNotFound, nil)

	v1 := copyBoard(t, "sample-v1")
	var legacy ProjectsResponse
	getJSON(t, handlerFor(t, v1), "/api/projects", http.StatusOK, &legacy)
	if !slices.Equal(legacy.V1Projects, []string{"alpha", "beta"}) || legacy.MigrateCommand != "flashheart migrate --root "+v1 {
		t.Errorf("v1 root = %+v", legacy)
	}

	sample, _ := sampleAPI(t, nil)
	recorder := httptest.NewRecorder()
	sample.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/projects", nil))
	if recorder.Code != http.StatusMethodNotAllowed || !strings.Contains(recorder.Body.String(), "method_not_allowed") {
		t.Errorf("POST /api/projects = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestShellQuote(t *testing.T) {
	t.Parallel()

	if got := shellQuote("/Users/me/reports/Kanban"); got != "/Users/me/reports/Kanban" {
		t.Errorf("plain = %q", got)
	}
	if got := shellQuote("/Users/me/My Board's"); got != `'/Users/me/My Board'\''s'` {
		t.Errorf("quoted = %q", got)
	}
}

// TestListsAreNeverNull keeps the JSON contract: every list is an array, so
// the frontend never has to treat null as empty. Only the optional handoff
// and review objects may be null.
func TestListsAreNeverNull(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, nil)
	for _, path := range []string{
		"/api/projects",
		"/api/projects/alpha/board",
		"/api/all/board",
		"/api/tickets/AL-7",
		"/api/tickets/BE-1",
		"/api/projects/alpha/workstreams",
	} {
		body := getJSON(t, handler, path, http.StatusOK, nil).Body.String()
		body = strings.NewReplacer(`"handoff":null`, "", `"review":null`, "").Replace(body)
		if strings.Contains(body, ":null") {
			t.Errorf("GET %s has a null field:\n%s", path, body)
		}
	}
}
