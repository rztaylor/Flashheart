package app

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/rztaylor/flashheart/internal/config"
)

// A project's image is set and cleared only through Singleserve's
// authentication (FH-151): without it nothing reaches the project folder.
func TestProjectImageNeedsTheBoardsAuthentication(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(sampleRoot(t))); err != nil {
		t.Fatal(err)
	}
	_, launch := startRuntimeWith(t, Options{Root: root}, config.Defaults(), nil)
	stored := filepath.Join(root, "alpha", "image.png")
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	const path = "api/projects/alpha/image"

	unauthenticated := &http.Client{Transport: &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, launch.Address())
		},
	}}
	send := func(client *http.Client, method string, body []byte) int {
		t.Helper()
		request, err := http.NewRequest(method, launch.BaseURL()+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/octet-stream")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}
	if status := send(unauthenticated, http.MethodPut, picture.Bytes()); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated PUT status = %d, want 401", status)
	}
	if _, err := os.Stat(stored); !os.IsNotExist(err) {
		t.Fatalf("an unauthenticated upload wrote the image: %v", err)
	}
	if status := send(launch.Client(), http.MethodPut, picture.Bytes()); status != http.StatusOK {
		t.Fatalf("PUT status = %d", status)
	}
	if _, err := os.Stat(stored); err != nil {
		t.Fatalf("the image was not stored: %v", err)
	}
	if status := send(unauthenticated, http.MethodDelete, nil); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated DELETE status = %d, want 401", status)
	}
	if _, err := os.Stat(stored); err != nil {
		t.Fatalf("an unauthenticated delete removed the image: %v", err)
	}
	if status := send(unauthenticated, http.MethodGet, nil); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET status = %d, want 401", status)
	}
	if status := send(launch.Client(), http.MethodDelete, nil); status != http.StatusOK {
		t.Fatalf("DELETE status = %d", status)
	}
	if _, err := os.Stat(stored); !os.IsNotExist(err) {
		t.Fatalf("the image was not removed: %v", err)
	}
}
