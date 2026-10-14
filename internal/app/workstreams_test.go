package app

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/store"
)

// A workstream is created from the board only through Singleserve's
// authentication (FH-17): without it no file is written.
func TestCreatingAWorkstreamNeedsTheBoardsAuthentication(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(sampleRoot(t))); err != nil {
		t.Fatal(err)
	}
	_, launch := startRuntimeWith(t, Options{Root: root}, config.Defaults(), nil)
	file := filepath.Join(root, "alpha", "workstreams", "offline.md")
	const path = "api/projects/alpha/workstreams"
	const body = `{"title":"Offline","goal":"Work without a network.","priority":"low","dependsOnWorkstreams":[]}`

	unauthenticated := &http.Client{Transport: &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, launch.Address())
		},
	}}
	response, err := unauthenticated.Post(launch.BaseURL()+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated create status = %d, want 401", response.StatusCode)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("an unauthenticated create wrote the workstream: %v", err)
	}

	response, err = launch.Client().Post(launch.BaseURL()+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", response.StatusCode)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("the workstream was not written: %v", err)
	}
}

// A workstream is renamed from the board only through Singleserve's
// authentication (FH-158): without it the file stays where it is.
func TestRenamingAWorkstreamNeedsTheBoardsAuthentication(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(sampleRoot(t))); err != nil {
		t.Fatal(err)
	}
	_, launch := startRuntimeWith(t, Options{Root: root}, config.Defaults(), nil)
	old := filepath.Join(root, "alpha", "workstreams", "board-ui.md")
	data, err := os.ReadFile(old)
	if err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(root, "alpha", "workstreams", "board.md")
	url := launch.BaseURL() + "api/projects/alpha/workstreams/board-ui"
	body := `{"base":"` + store.Hash(data) + `","slug":"board"}`
	patch := func(client *http.Client) int {
		t.Helper()
		request, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}

	unauthenticated := &http.Client{Transport: &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, launch.Address())
		},
	}}
	if status := patch(unauthenticated); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated rename status = %d, want 401", status)
	}
	if _, err := os.Stat(renamed); !os.IsNotExist(err) {
		t.Fatalf("an unauthenticated rename moved the workstream: %v", err)
	}

	if status := patch(launch.Client()); status != http.StatusOK {
		t.Fatalf("rename status = %d", status)
	}
	if _, err := os.Stat(renamed); err != nil {
		t.Fatalf("the workstream was not renamed: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("the old file is still there: %v", err)
	}
}
