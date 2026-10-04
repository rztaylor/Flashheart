package api

import (
	"encoding/json"
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

func sampleAPI(t *testing.T, mutate func(root string)) (http.Handler, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "boards", "sample"))); err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(root)
	}
	files := store.New(root)
	t.Cleanup(func() { files.Close() })
	return New(Options{
		Info:      Info{Root: root, Theme: "system"},
		Board:     index.New(files, index.Options{}),
		Files:     files,
		DoneLimit: 20,
	}), root
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

func cardBySlug(t *testing.T, cards []Card, slug string) Card {
	t.Helper()
	for _, card := range cards {
		if card.Slug == slug {
			return card
		}
	}
	t.Fatalf("no card %s", slug)
	return Card{}
}

func TestProjectsListsCountsAndActivity(t *testing.T) {
	t.Parallel()

	handler, root := sampleAPI(t, nil)
	beta := filepath.Join(root, "beta", "todo", "feat--hello.md")
	newer := time.Now().Add(time.Hour)
	if err := os.Chtimes(beta, newer, newer); err != nil {
		t.Fatal(err)
	}
	var body ProjectsResponse
	getJSON(t, handler, "/api/projects", http.StatusOK, &body)
	if body.Revision != 1 || body.RootMissing || body.Root != root {
		t.Errorf("response = %+v", body)
	}
	if len(body.Projects) != 2 || body.Projects[0].Name != "beta" || body.Projects[1].Name != "alpha" {
		t.Fatalf("projects not sorted by activity: %+v", body.Projects)
	}
	alpha := body.Projects[1]
	if alpha.DisplayName != "Alpha" || alpha.NeedsRepair != 1 || alpha.Blocked != 3 {
		t.Errorf("alpha = %+v", alpha)
	}
	want := map[string]int{"todo": 4, "in-progress": 1, "ready-to-review": 1, "done": 1}
	for column, count := range want {
		if alpha.Counts[column] != count {
			t.Errorf("alpha %s count = %d, want %d", column, alpha.Counts[column], count)
		}
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
		order = append(order, card.Column+"/"+card.Slug)
	}
	// Columns in workflow order; within a column by priority, then creation.
	want := []string{
		"todo/feat--drag-and-drop", "todo/bug--column-overflow", "todo/spike--offline-mode", "todo/docs--broken-frontmatter",
		"in-progress/feat--card-panel", "ready-to-review/feat--board-columns", "done/infra--project-skeleton",
	}
	if !slices.Equal(order, want) {
		t.Errorf("order =\n  %q\nwant\n  %q", order, want)
	}

	drag := cardBySlug(t, body.Cards, "feat--drag-and-drop")
	if !drag.Blocked || len(drag.BlockedBy) != 1 || drag.BlockedBy[0].Text != "Comes after feat--card-panel in workstream board-ui, which is In progress" {
		t.Errorf("drag-and-drop blocked by %+v", drag.BlockedBy)
	}
	overflow := cardBySlug(t, body.Cards, "bug--column-overflow")
	if len(overflow.BlockedBy) != 1 || overflow.BlockedBy[0].Text != "Depends on feat--drag-and-drop, which is in To do" || overflow.BlockedBy[0].Ticket == nil {
		t.Errorf("column-overflow blocked by %+v", overflow.BlockedBy)
	}
	offline := cardBySlug(t, body.Cards, "spike--offline-mode")
	if len(offline.BlockedBy) != 1 || !offline.BlockedBy[0].Missing || !strings.Contains(offline.BlockedBy[0].Text, "does not exist") {
		t.Errorf("offline-mode blocked by %+v", offline.BlockedBy)
	}
	broken := cardBySlug(t, body.Cards, "docs--broken-frontmatter")
	if len(broken.NeedsRepair) != 1 || broken.Title != "Broken frontmatter" || broken.Blocked {
		t.Errorf("broken = %+v", broken)
	}
	panel := cardBySlug(t, body.Cards, "feat--card-panel")
	if panel.Blocked || panel.HandoffNext != "Review tab" || panel.Criteria != (Progress{Done: 1, Total: 2}) || panel.Workstream != "board-ui" || panel.SearchText == "" {
		t.Errorf("card panel = %+v", panel)
	}
	columns := cardBySlug(t, body.Cards, "feat--board-columns")
	if !columns.HasReview || columns.Attachments != 1 {
		t.Errorf("board columns review=%v attachments=%d", columns.HasReview, columns.Attachments)
	}
}

func TestDoneColumnIsLimitedUnlessAllIsRequested(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, func(root string) {
		for n := range 25 {
			name := filepath.Join(root, "beta", "done", "feat--done-"+string(rune('a'+n))+".md")
			if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(name, []byte("# Done\n"), 0o644); err != nil {
				t.Fatal(err)
			}
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
	if first := limited.Cards[1]; first.Slug != "feat--done-y" {
		t.Errorf("done column not newest first: %s", first.Slug)
	}
	var all BoardResponse
	getJSON(t, handler, "/api/projects/beta/board?done=all", http.StatusOK, &all)
	if countDone(all.Cards) != 25 {
		t.Errorf("done=all shows %d", countDone(all.Cards))
	}
}

func TestTicketDetail(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, nil)
	var columns TicketResponse
	getJSON(t, handler, "/api/projects/alpha/tickets/feat--board-columns", http.StatusOK, &columns)
	ticket := columns.Ticket
	if ticket.Review == nil || !strings.HasPrefix(ticket.Review.Markdown, "# Review: Board columns") {
		t.Errorf("review = %+v", ticket.Review)
	}
	if len(ticket.AttachmentFiles) != 1 || ticket.AttachmentFiles[0].URL != "/api/projects/alpha/attachments/feat--board-columns/20261003T1000-board-desktop.png" {
		t.Errorf("attachments = %+v", ticket.AttachmentFiles)
	}
	if !strings.Contains(ticket.Body, "## Acceptance Criteria") || !strings.Contains(ticket.Frontmatter, "type: feature") || len(ticket.CriteriaItems) != 2 {
		t.Errorf("ticket = %+v", ticket)
	}

	var panel TicketResponse
	getJSON(t, handler, "/api/projects/alpha/tickets/feat--card-panel", http.StatusOK, &panel)
	if panel.Ticket.Handoff == nil || panel.Ticket.Handoff.Next[0] != "Review tab" || panel.Ticket.Review != nil {
		t.Errorf("card panel handoff=%+v review=%+v", panel.Ticket.Handoff, panel.Ticket.Review)
	}

	getJSON(t, handler, "/api/projects/alpha/tickets/feat--nope", http.StatusNotFound, nil)
	getJSON(t, handler, "/api/projects/nope/tickets/feat--x", http.StatusNotFound, nil)
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
	if ws.Slug != "board-ui" || ws.Title != "Board UI" || ws.Status != "active" || ws.Done != 1 || ws.Total != 3 || ws.Next != "feat--card-panel" {
		t.Errorf("workstream = %+v", ws)
	}
	var tickets []string
	for _, ticket := range ws.Tickets {
		tickets = append(tickets, ticket.Slug+":"+ticket.Column)
	}
	if want := []string{"feat--board-columns:ready-to-review", "feat--card-panel:in-progress", "feat--drag-and-drop:todo"}; !slices.Equal(tickets, want) {
		t.Errorf("tickets = %q", tickets)
	}
	if !ws.Tickets[2].Blocked || ws.Tickets[1].Blocked {
		t.Errorf("blocked flags = %+v", ws.Tickets)
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
	if hello := cardBySlug(t, body.Cards, "feat--hello"); hello.Project != "beta" || hello.SearchText != "" {
		t.Errorf("hello = %+v (all-projects cards omit body text)", hello)
	}
}

func TestAttachmentsAreServedSafely(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, func(root string) {
		_ = os.WriteFile(filepath.Join(root, "alpha", "attachments", "feat--board-columns", "x.svg"), []byte("<svg/>"), 0o644)
	})
	recorder := getJSON(t, handler, "/api/projects/alpha/attachments/feat--board-columns/20261003T1000-board-desktop.png", http.StatusOK, nil)
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'none'") || !strings.Contains(got, "sandbox") {
		t.Errorf("Content-Security-Policy = %q", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	getJSON(t, handler, "/api/projects/alpha/attachments/feat--board-columns/x.svg", http.StatusUnsupportedMediaType, nil)
	getJSON(t, handler, "/api/projects/alpha/attachments/feat--board-columns/absent.png", http.StatusNotFound, nil)
	getJSON(t, handler, "/api/projects/alpha/attachments/feat--board-columns/..%2F..%2Ftodo%2Fx.png", http.StatusBadRequest, nil)
}

func TestMissingRootAndUnknownProject(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "absent")
	files := store.New(missing)
	defer files.Close()
	handler := New(Options{Info: Info{Root: missing}, Board: index.New(files, index.Options{}), Files: files, DoneLimit: 20})
	var body ProjectsResponse
	getJSON(t, handler, "/api/projects", http.StatusOK, &body)
	if !body.RootMissing || len(body.Projects) != 0 {
		t.Errorf("missing root = %+v", body)
	}
	getJSON(t, handler, "/api/projects/alpha/board", http.StatusNotFound, nil)

	sample, _ := sampleAPI(t, nil)
	recorder := httptest.NewRecorder()
	sample.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/projects", nil))
	if recorder.Code != http.StatusMethodNotAllowed || !strings.Contains(recorder.Body.String(), "method_not_allowed") {
		t.Errorf("POST /api/projects = %d %s", recorder.Code, recorder.Body.String())
	}
}
