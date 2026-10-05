package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rztaylor/flashheart/internal/api"
	"github.com/rztaylor/flashheart/internal/buildinfo"
	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/store"
	"github.com/rztaylor/flashheart/internal/webui"
	"github.com/rztaylor/singleserve"
)

// Options configures one serve process lifetime.
type Options struct {
	Build  buildinfo.Info
	Root   string
	Debug  bool
	Stdout io.Writer
	Stderr io.Writer
	Opener singleserve.BrowserOpener
	// Launched is called once, after the browser open attempt.
	Launched func(Launched)
}

// Launched reports how the browser was reached. ManualURL is set only when
// opening failed; it is a short-lived secret for the owner's terminal.
type Launched struct {
	Address      string
	ManualURL    string
	BrowserError string
}

type runtime struct {
	server   *singleserve.Server
	requests *requestTracker
	files    *store.Store
	board    *index.Index
	stopping *stopSignal
}

// stopSignal is closed once when shutdown begins, releasing long-polls so
// they never hold up the drain.
type stopSignal struct {
	once sync.Once
	ch   chan struct{}
}

func newStopSignal() *stopSignal { return &stopSignal{ch: make(chan struct{})} }

func (s *stopSignal) stop() { s.once.Do(func() { close(s.ch) }) }

// Run serves Flashheart and blocks until the server has drained.
func Run(ctx context.Context, options Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	stderr := writerOrDiscard(options.Stderr)

	settings, err := config.Load(options.Root)
	if err != nil {
		return err
	}
	runtime, err := newRuntime(options, settings, nil)
	if err != nil {
		return err
	}
	defer runtime.files.Close()
	launch, err := runtime.server.Start(ctx)
	if err != nil {
		return fmt.Errorf("start local server: %w", err)
	}
	// External edits become new revisions within a second (STO-7).
	watchCtx, stopWatching := context.WithCancel(context.Background())
	defer stopWatching()
	if runtime.board != nil {
		go runtime.board.Watch(watchCtx, options.Root, nil)
	}
	go func() {
		select {
		case <-ctx.Done():
			runtime.stopping.stop()
		case <-watchCtx.Done():
		}
	}()

	launched := Launched{Address: launch.Address()}
	if err := launch.OpenBrowser(ctx); err != nil {
		if ctx.Err() != nil {
			return waitForStop(launch, runtime.requests, options.Debug, stderr)
		}
		manualURL, renewErr := launch.NewBootstrapURL()
		if renewErr != nil {
			return errors.Join(fmt.Errorf("open browser: %w", err), fmt.Errorf("create manual browser URL: %w", renewErr), shutdownLaunch(launch))
		}
		launched.ManualURL = manualURL
		launched.BrowserError = err.Error()
	}
	if options.Launched != nil {
		options.Launched(launched)
	}
	return waitForStop(launch, runtime.requests, options.Debug, stderr)
}

func waitForStop(launch *singleserve.Launch, requests *requestTracker, debug bool, stderr io.Writer) error {
	result, err := launch.Wait()
	if err != nil && !tolerableBrowserDrain(result, err, requests) {
		return fmt.Errorf("local server shutdown: %w", err)
	}
	if debug {
		fmt.Fprintf(stderr, "flashheart stopped: %s after %s\n", result.Reason, result.StoppedAt.Sub(result.StartedAt).Round(time.Second))
	}
	return nil
}

// tolerableBrowserDrain accepts a drain timeout caused only by the browser
// holding an idle connection open after an accepted Quit.
func tolerableBrowserDrain(result singleserve.Result, err error, requests *requestTracker) bool {
	return result.Reason == singleserve.ShutdownBrowserRequest && errors.Is(err, context.DeadlineExceeded) && requests.Empty()
}

// newRuntime builds the server. apiOverride replaces the /api/ handler in tests.
func newRuntime(options Options, settings config.Config, apiOverride http.Handler) (*runtime, error) {
	frontend, err := webui.New()
	if err != nil {
		return nil, err
	}
	files := store.New(options.Root)
	stopping := newStopSignal()
	var board *index.Index
	apiHandler := apiOverride
	if apiHandler == nil {
		board = index.New(files, index.Options{})
		apiHandler = api.New(api.Options{
			Info: api.Info{
				Version:         options.Build.Version,
				Commit:          options.Build.Commit,
				BuildDate:       options.Build.BuildDate,
				ProtocolVersion: protocol.Version,
				Root:            options.Root,
				Theme:           settings.UI.Theme,
			},
			Board:     board,
			Files:     files,
			Writer:    files,
			DoneLimit: settings.DoneColumnLimit,
			Stopping:  stopping.ch,
		})
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", apiHandler)
	mux.Handle("/", frontend)

	requests := newRequestTracker()
	serverOptions := singleserve.Options{
		Handler:  requests.Wrap(securityHeaders(mux)),
		Lifetime: singleserve.BrowserBoundLifetime(),
		Guard: singleserve.ShutdownGuardFunc(func(context.Context, singleserve.ShutdownRequest) error {
			if requests.Writes() > 0 {
				return singleserve.DenyShutdown("save_in_progress", "A change is still being saved. Quit again when it finishes.")
			}
			stopping.stop()
			return nil
		}),
	}
	if options.Opener != nil {
		serverOptions.Opener = options.Opener
	}
	server, err := singleserve.New(serverOptions)
	if err != nil {
		files.Close()
		return nil, fmt.Errorf("configure local server: %w", err)
	}
	return &runtime{server: server, requests: requests, files: files, board: board, stopping: stopping}, nil
}

// requestTracker counts in-flight application requests; requests with unsafe
// methods are writes, which the shutdown guard waits for.
type requestTracker struct {
	mu     sync.Mutex
	total  int
	writes int
}

func newRequestTracker() *requestTracker { return &requestTracker{} }

func (t *requestTracker) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		write := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
		t.mu.Lock()
		t.total++
		if write {
			t.writes++
		}
		t.mu.Unlock()
		defer func() {
			t.mu.Lock()
			t.total--
			if write {
				t.writes--
			}
			t.mu.Unlock()
		}()
		next.ServeHTTP(w, r)
	})
}

func (t *requestTracker) Writes() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.writes
}

func (t *requestTracker) Empty() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.total == 0
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func shutdownLaunch(launch *singleserve.Launch) error {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := launch.Shutdown(ctx); err != nil {
		return fmt.Errorf("stop local server after launch failure: %w", err)
	}
	return nil
}

func writerOrDiscard(writer io.Writer) io.Writer {
	if writer == nil {
		return io.Discard
	}
	return writer
}
