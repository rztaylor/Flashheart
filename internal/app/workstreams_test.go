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
