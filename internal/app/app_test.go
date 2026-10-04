package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/buildinfo"
	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/singleserve"
)

func sampleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "boards", "sample"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func startRuntime(t *testing.T, options Options, extra http.Handler) (*runtime, *singleserve.Launch) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	runtime, err := newRuntime(options, config.Defaults(), extra)
	if err != nil {
		t.Fatalf("newRuntime(): %v", err)
	}
	launch, err := runtime.server.Start(ctx)
	if err != nil {
		t.Fatalf("Start(): %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer shutdownCancel()
		_ = launch.Shutdown(shutdownCtx)
	})
	return runtime, launch
}

func TestServerListensOnLoopbackOnly(t *testing.T) {
	t.Parallel()

	_, launch := startRuntime(t, Options{Root: sampleRoot(t)}, nil)
	host, _, err := net.SplitHostPort(launch.Address())
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", launch.Address(), err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		t.Fatalf("listener address %q is not a loopback IP (SEC-1)", launch.Address())
	}
}

func TestServerComposesAuthenticatedApplication(t *testing.T) {
	t.Parallel()

	root := sampleRoot(t)
	_, launch := startRuntime(t, Options{Root: root, Build: buildinfo.Info{Version: "test", Commit: "c0ffee", BuildDate: "today"}}, nil)

	unauthenticated := &http.Client{Transport: &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, launch.Address())
		},
	}}
	response, err := unauthenticated.Get(launch.BaseURL() + "api/info")
	if err != nil {
		t.Fatalf("unauthenticated GET /api/info: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", response.StatusCode)
	}

	response, err = launch.Client().Get(launch.BaseURL() + "api/info")
	if err != nil {
		t.Fatalf("GET /api/info: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	for header, want := range map[string]string{
		"Content-Security-Policy": "default-src 'self'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
	} {
		if got := response.Header.Get(header); !strings.Contains(got, want) {
			t.Errorf("%s = %q, want it to contain %q", header, got, want)
		}
	}
	var info struct {
		Name            string `json:"name"`
		Version         string `json:"version"`
		Commit          string `json:"commit"`
		ProtocolVersion int    `json:"protocolVersion"`
		Root            string `json:"root"`
		Theme           string `json:"theme"`
	}
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.Name != "Flashheart" || info.Version != "test" || info.Commit != "c0ffee" || info.ProtocolVersion != 1 || info.Root != root || info.Theme != "system" {
		t.Errorf("info = %+v", info)
	}

	index, err := launch.Client().Get(launch.BaseURL())
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body, _ := io.ReadAll(index.Body)
	index.Body.Close()
	if index.StatusCode != http.StatusOK || !strings.Contains(string(body), "<title>Flashheart</title>") {
		t.Errorf("GET / status = %d body = %.200q", index.StatusCode, body)
	}
}

func TestGuardDeniesShutdownWhileAWriteIsInFlight(t *testing.T) {
	t.Parallel()

	entered := make(chan struct{})
	release := make(chan struct{})
	slowWrite := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	_, launch := startRuntime(t, Options{Root: sampleRoot(t)}, slowWrite)

	writeDone := make(chan error, 1)
	go func() {
		response, err := launch.Client().Post(launch.BaseURL()+"api/test-write", "application/json", strings.NewReader("{}"))
		if err == nil {
			response.Body.Close()
		}
		writeDone <- err
	}()
	<-entered

	response, err := launch.Client().Post(launch.BaseURL()+"_singleserve/shutdown", "", nil)
	if err != nil {
		t.Fatalf("POST shutdown: %v", err)
	}
	var denial struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	_ = json.NewDecoder(response.Body).Decode(&denial)
	response.Body.Close()
	if response.StatusCode != http.StatusConflict || denial.Error.Code != "save_in_progress" || denial.Error.Message == "" {
		t.Fatalf("shutdown during write: status %d, error %+v; want 409 save_in_progress (LIFE-2)", response.StatusCode, denial.Error)
	}

	close(release)
	if err := <-writeDone; err != nil {
		t.Fatalf("slow write: %v", err)
	}
	response, err = launch.Client().Post(launch.BaseURL()+"_singleserve/shutdown", "", nil)
	if err != nil {
		t.Fatalf("POST shutdown after write: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("shutdown after write: status %d, want 202", response.StatusCode)
	}
	result, err := launch.Wait()
	if err != nil {
		t.Fatalf("Wait(): %v", err)
	}
	if result.Reason != singleserve.ShutdownBrowserRequest {
		t.Errorf("reason = %q", result.Reason)
	}
}

func TestReadsDoNotBlockShutdown(t *testing.T) {
	t.Parallel()

	tracker := newRequestTracker()
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := tracker.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(entered)
		<-release
	}))
	go handler.ServeHTTP(discardWriter{}, mustRequest(t, http.MethodGet, "/api/info"))
	<-entered
	defer close(release)
	if tracker.Writes() != 0 {
		t.Errorf("Writes() = %d during a GET, want 0", tracker.Writes())
	}
	if tracker.Empty() {
		t.Error("Empty() = true during an in-flight request")
	}
}

func TestRunReportsManualURLWhenTheBrowserCannotOpen(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	launched := make(chan Launched, 2)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			Root:     sampleRoot(t),
			Opener:   singleserve.BrowserOpenerFunc(func(context.Context, string) error { return errors.New("no browser here") }),
			Launched: func(l Launched) { launched <- l },
		})
	}()
	report := waitLaunched(t, launched, done)
	if report.ManualURL == "" || report.BrowserError != "no browser here" || report.Address == "" {
		t.Errorf("launched = %+v", report)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run() = %v after cancellation", err)
	}
	if len(launched) != 0 {
		t.Error("Launched was reported more than once")
	}
}

func TestRunOpensTheBrowserQuietly(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	launched := make(chan Launched, 1)
	done := make(chan error, 1)
	opened := make(chan string, 1)
	var stdout, stderr strings.Builder
	go func() {
		done <- Run(ctx, Options{
			Root:     sampleRoot(t),
			Stdout:   &stdout,
			Stderr:   &stderr,
			Opener:   singleserve.BrowserOpenerFunc(func(_ context.Context, url string) error { opened <- url; return nil }),
			Launched: func(l Launched) { launched <- l },
		})
	}()
	report := waitLaunched(t, launched, done)
	if report.ManualURL != "" || report.BrowserError != "" {
		t.Errorf("launched = %+v, want no manual URL", report)
	}
	if url := <-opened; !strings.HasPrefix(url, "http://") {
		t.Errorf("opened %q", url)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run() = %v", err)
	}
	if stdout.String() != "" || stderr.String() != "" {
		t.Errorf("non-debug run printed stdout=%q stderr=%q (CLI-2)", stdout.String(), stderr.String())
	}
}

func TestRunRejectsAnInvalidConfigBeforeListening(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".flashheart"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.Path(root), []byte("version: 9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), Options{
		Root:     root,
		Opener:   singleserve.BrowserOpenerFunc(func(context.Context, string) error { t.Error("browser opened"); return nil }),
		Launched: func(Launched) { t.Error("Launched reported") },
	})
	if !errors.Is(err, config.ErrNewerVersion) {
		t.Errorf("Run() = %v, want ErrNewerVersion", err)
	}
}

func waitLaunched(t *testing.T, launched <-chan Launched, done <-chan error) Launched {
	t.Helper()
	select {
	case report := <-launched:
		return report
	case err := <-done:
		t.Fatalf("Run() returned before launching: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Run() did not report a launch")
	}
	return Launched{}
}

type discardWriter struct{}

func (discardWriter) Header() http.Header         { return http.Header{} }
func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
func (discardWriter) WriteHeader(int)             {}

func mustRequest(t *testing.T, method, path string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(method, "http://flashheart.test"+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	return request
}
