package app

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// A comment is posted only through Singleserve's authentication, is the
// person's (config user_name), and records the human's event; the board's
// ticket writes append history by the same person (D47).
func TestCommentsNeedTheBoardsAuthentication(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(sampleRoot(t))); err != nil {
		t.Fatal(err)
	}
	settings := config.Defaults()
	settings.UserName = "Robert"
	_, launch := startRuntimeWith(t, Options{Root: root}, settings, nil)
	activity := filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", board.ActivityFile)

	unauthenticated := &http.Client{Transport: &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, launch.Address())
		},
	}}
	response, err := unauthenticated.Post(launch.BaseURL()+"api/tickets/AL-4/comments", "application/json", strings.NewReader(`{"body":"Sneaky."}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated comment status = %d, want 401", response.StatusCode)
	}
	if _, err := os.Stat(activity); !os.IsNotExist(err) {
		t.Fatalf("an unauthenticated comment wrote the activity file: %v", err)
	}

	response, err = launch.Client().Post(launch.BaseURL()+"api/tickets/AL-4/comments", "application/json", strings.NewReader(`{"body":"Looks right."}`))
	if err != nil {
		t.Fatal(err)
	}
	var posted struct {
		Comment struct{ ID, By, Body string } `json:"comment"`
	}
	_ = json.NewDecoder(response.Body).Decode(&posted)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || posted.Comment.By != "Robert" || posted.Comment.Body != "Looks right." {
		t.Fatalf("comment status = %d, %+v", response.StatusCode, posted)
	}

	request, _ := http.NewRequest(http.MethodPost, launch.BaseURL()+"api/tickets/AL-4/move", strings.NewReader(`{"to":"backlog"}`))
	request.Header.Set("Content-Type", "application/json")
	if response, err = launch.Client().Do(request); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("move status = %d", response.StatusCode)
	}
	data, _ := os.ReadFile(activity)
	parsed := board.ParseActivity(data)
	if len(parsed.Comments) != 1 || len(parsed.History) != 1 || parsed.History[0].By != "Robert" {
		t.Errorf("activity = %+v\n%s", parsed, data)
	}

	files := store.New(root)
	t.Cleanup(func() { files.Close() })
	var kinds []events.Kind
	if err := events.New(files).Read("alpha", time.Time{}, func(e events.Event) {
		if e.Run == events.Human {
			kinds = append(kinds, e.Kind)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if len(kinds) != 2 || kinds[0] != events.CommentAdded || kinds[1] != events.TicketMoved {
		t.Errorf("human events = %v", kinds)
	}

	// Deleting a comment (FH-140) needs the authentication too.
	remove := func(client *http.Client) int {
		t.Helper()
		body := `{"base":"` + store.Hash(data) + `"}`
		request, _ := http.NewRequest(http.MethodDelete, launch.BaseURL()+"api/tickets/AL-4/comments/"+posted.Comment.ID, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}
	if status := remove(unauthenticated); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated delete status = %d, want 401", status)
	}
	if now, _ := os.ReadFile(activity); string(now) != string(data) {
		t.Fatal("an unauthenticated delete changed the activity file")
	}
	if status := remove(launch.Client()); status != http.StatusOK {
		t.Fatalf("delete status = %d", status)
	}
	if now, _ := os.ReadFile(activity); len(board.ParseActivity(now).Comments) != 0 {
		t.Errorf("the comment is still there:\n%s", now)
	}
}
