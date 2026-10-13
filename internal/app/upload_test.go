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

// A file is uploaded into a ticket only through Singleserve's
// authentication (FH-18): without it nothing reaches the ticket's files/.
func TestUploadsNeedTheBoardsAuthentication(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(sampleRoot(t))); err != nil {
		t.Fatal(err)
	}
	_, launch := startRuntimeWith(t, Options{Root: root}, config.Defaults(), nil)
	files := filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "files")
	const png = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"
	path := "api/tickets/AL-4/files?name=shot.png&kind=screenshot"

	unauthenticated := &http.Client{Transport: &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, launch.Address())
		},
	}}
	response, err := unauthenticated.Post(launch.BaseURL()+path, "application/octet-stream", strings.NewReader(png))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated upload status = %d, want 401", response.StatusCode)
	}
	if _, err := os.Stat(files); !os.IsNotExist(err) {
		t.Fatalf("an unauthenticated upload wrote files/: %v", err)
	}

	response, err = launch.Client().Post(launch.BaseURL()+path, "application/octet-stream", strings.NewReader(png))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d", response.StatusCode)
	}
	entries, err := os.ReadDir(files)
	if err != nil || len(entries) != 2 {
		t.Fatalf("files/ = %v, %v", entries, err)
	}
}
