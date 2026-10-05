package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/app"
	"github.com/rztaylor/flashheart/internal/background"
	"github.com/rztaylor/flashheart/internal/buildinfo"
)

type harness struct {
	env         map[string]string
	home        string
	appCalls    []app.Options
	appErr      error
	appLaunched *app.Launched
	bgCalls     []background.Options
	bgReport    background.Report
	bgErr       error
	handshake   *fakeHandshake
	serveLog    *bytes.Buffer
	logRoots    []string
	stdin       string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return &harness{env: map[string]string{}, home: t.TempDir()}
}

func (h *harness) deps() Dependencies {
	return Dependencies{
		Build:   buildinfo.Info{Version: "v0.0.1", Commit: "abc1234", BuildDate: "2026-10-04"},
		Getenv:  func(key string) string { return h.env[key] },
		HomeDir: func() (string, error) { return h.home, nil },
		RunApp: func(_ context.Context, options app.Options) error {
			h.appCalls = append(h.appCalls, options)
			if h.appLaunched != nil && options.Launched != nil {
				options.Launched(*h.appLaunched)
			}
			return h.appErr
		},
		StartBackground: func(_ context.Context, options background.Options) (background.Report, error) {
			h.bgCalls = append(h.bgCalls, options)
			return h.bgReport, h.bgErr
		},
		Executable: func() (string, error) { return "/opt/bin/flashheart", nil },
		Stdin:      strings.NewReader(h.stdin),
		OpenServeLog: func(root string) io.Writer {
			h.logRoots = append(h.logRoots, root)
			if h.serveLog == nil {
				h.serveLog = &bytes.Buffer{}
			}
			return h.serveLog
		},
		OpenHandshake: func() (Handshake, error) {
			if h.handshake == nil {
				return nil, errors.New("no handshake descriptor")
			}
			return h.handshake, nil
		},
	}
}

func (h *harness) run(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, &stdout, &stderr, h.deps())
	return code, stdout.String(), stderr.String()
}

type fakeHandshake struct {
	ready  []background.Report
	failed []string
}

func (f *fakeHandshake) Ready(report background.Report) error {
	f.ready = append(f.ready, report)
	return nil
}

func (f *fakeHandshake) Fail(err error) error {
	f.failed = append(f.failed, err.Error())
	return nil
}

func TestHelpListsEveryCommand(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"--help"}, {"-h"}} {
		h := newHarness(t)
		code, stdout, stderr := h.run(args...)
		if code != 0 || stderr != "" {
			t.Fatalf("%v: code=%d stderr=%q", args, code, stderr)
		}
		for _, want := range []string{"Usage: flashheart", "serve", "mcp", "hook", "setup", "doctor", "version", "--root", "--debug"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("%v: help does not mention %q:\n%s", args, want, stdout)
			}
		}
	}
}

func TestEveryCommandHasHelp(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"serve", "mcp", "hook", "setup", "doctor", "migrate", "version"} {
		h := newHarness(t)
		code, stdout, stderr := h.run(command, "--help")
		if code != 0 || stderr != "" {
			t.Errorf("%s --help: code=%d stderr=%q", command, code, stderr)
		}
		if !strings.HasPrefix(stdout, "Usage: flashheart "+command) {
			t.Errorf("%s --help: stdout = %q", command, stdout)
		}
		if len(h.appCalls) != 0 || len(h.bgCalls) != 0 {
			t.Errorf("%s --help started the server", command)
		}
	}
}

func TestVersionPrintsBuildAndProtocol(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	code, stdout, stderr := h.run("version")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	want := "flashheart v0.0.1 (commit abc1234, built 2026-10-04)\nagent protocol 1\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestUsageErrorsExitTwoWithUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{"unknown command", []string{"frobnicate"}, `unknown command "frobnicate"`},
		{"unknown flag", []string{"--frobnicate"}, "flag provided but not defined"},
		{"unknown command flag", []string{"version", "--frobnicate"}, "flag provided but not defined"},
		{"extra argument", []string{"version", "extra"}, `version takes no arguments`},
		{"empty root", []string{"--root", ""}, "--root must not be empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			code, stdout, stderr := h.run(test.args...)
			if code != 2 {
				t.Errorf("code = %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.message) || !strings.Contains(stderr, "Usage: flashheart") {
				t.Errorf("stderr = %q, want %q and usage", stderr, test.message)
			}
			if len(h.appCalls) != 0 || len(h.bgCalls) != 0 {
				t.Error("usage error started the server")
			}
		})
	}
}

func TestUnimplementedCommandsExitTwoQuietly(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"mcp"}, {"setup", "claude"}, {"doctor"}} {
		h := newHarness(t)
		code, stdout, stderr := h.run(args...)
		if code != 2 {
			t.Errorf("%v: code = %d, want 2", args, code)
		}
		// CLI-3: hook and mcp never write anything but protocol output to stdout.
		if stdout != "" {
			t.Errorf("%v: stdout = %q, want empty", args, stdout)
		}
		if want := "flashheart: " + args[0] + " is not yet available\n"; stderr != want {
			t.Errorf("%v: stderr = %q, want %q", args, stderr, want)
		}
	}
}

func TestRootResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		env  string
		want func(home string) string
	}{
		{"default", nil, "", func(home string) string { return filepath.Join(home, "reports", "Kanban") }},
		{"environment", nil, "~/boards", func(home string) string { return filepath.Join(home, "boards") }},
		{"flag beats environment", []string{"--root", "/srv/board/"}, "/elsewhere", func(string) string { return "/srv/board" }},
		{"flag after command", []string{"serve", "--root", "~"}, "", func(home string) string { return home }},
		{"tilde only at start", []string{"--root", "/a/~/b"}, "", func(string) string { return "/a/~/b" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.env["FLASHHEART_ROOT"] = test.env
			code, _, stderr := h.run(append(test.args, "--foreground")...)
			if code != 0 {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if got, want := h.appCalls[0].Root, test.want(h.home); got != want {
				t.Errorf("root = %q, want %q", got, want)
			}
		})
	}
}

func TestRelativeRootBecomesAbsolute(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if code, _, stderr := h.run("--root", "board", "--foreground"); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if root := h.appCalls[0].Root; !filepath.IsAbs(root) || filepath.Base(root) != "board" {
		t.Errorf("root = %q, want an absolute path ending in board", root)
	}
}

func TestServeIsTheDefaultAndDetaches(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.bgReport = background.Report{Address: "127.0.0.1:4321"}
	code, stdout, stderr := h.run("--root", "/srv/board")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q; normal startup must be quiet (CLI-2)", code, stdout, stderr)
	}
	if len(h.appCalls) != 0 {
		t.Fatal("background serve ran the app in the launching process")
	}
	if len(h.bgCalls) != 1 {
		t.Fatalf("background starts = %d, want 1", len(h.bgCalls))
	}
	call := h.bgCalls[0]
	if call.Executable != "/opt/bin/flashheart" {
		t.Errorf("executable = %q", call.Executable)
	}
	if want := []string{"serve", "--background-child", "--root", "/srv/board"}; !slices.Equal(call.Args, want) {
		t.Errorf("args = %q, want %q", call.Args, want)
	}
}

func TestBackgroundServeReportsManualURLOnce(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.bgReport = background.Report{Address: "127.0.0.1:4321", ManualURL: "http://ss-1.localhost:4321/#secret", BrowserError: "no opener"}
	code, stdout, stderr := h.run("serve")
	if code != 0 || stdout != "" {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	want := "Could not open a browser: no opener\nOpen this URL within two minutes: http://ss-1.localhost:4321/#secret\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestBackgroundServeDebugPrintsAddressOnly(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.bgReport = background.Report{Address: "127.0.0.1:4321"}
	code, _, stderr := h.run("--debug")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if want := "flashheart listening on loopback 127.0.0.1:4321 (running in the background)\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if args := h.bgCalls[0].Args; !slices.Contains(args, "--debug") {
		t.Errorf("child args %q do not pass --debug", args)
	}
}

func TestBackgroundStartFailureExitsOne(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.bgErr = errors.New("read config: bad theme")
	code, _, stderr := h.run()
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if want := "flashheart: read config: bad theme\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestForegroundServeRunsAppAndPrintsManualURL(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.appLaunched = &app.Launched{Address: "127.0.0.1:9", ManualURL: "http://ss-2.localhost:9/#secret", BrowserError: "exec: open: not found"}
	code, stdout, stderr := h.run("serve", "--foreground", "--debug", "--root", "/srv/board")
	if code != 0 || stdout != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(h.bgCalls) != 0 {
		t.Fatal("foreground serve started a background process")
	}
	options := h.appCalls[0]
	if options.Root != "/srv/board" || !options.Debug || options.Build.Version != "v0.0.1" {
		t.Errorf("app options = %+v", options)
	}
	want := "flashheart listening on loopback 127.0.0.1:9\nCould not open a browser: exec: open: not found\nOpen this URL within two minutes: http://ss-2.localhost:9/#secret\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestForegroundServeFailureExitsOne(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.appErr = errors.New("start local server: boom")
	code, _, stderr := h.run("--foreground")
	if code != 1 || stderr != "flashheart: start local server: boom\n" {
		t.Errorf("code=%d stderr=%q", code, stderr)
	}
}

func TestBackgroundChildReportsLaunchThroughHandshake(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.handshake = &fakeHandshake{}
	h.appLaunched = &app.Launched{Address: "127.0.0.1:9", ManualURL: "http://ss-3.localhost:9/#secret", BrowserError: "no opener"}
	code, stdout, stderr := h.run("serve", "--background-child", "--root", "/srv/board")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	want := background.Report{Address: "127.0.0.1:9", ManualURL: "http://ss-3.localhost:9/#secret", BrowserError: "no opener"}
	if len(h.handshake.ready) != 1 || h.handshake.ready[0] != want {
		t.Errorf("handshake ready = %+v, want %+v", h.handshake.ready, want)
	}
	if len(h.handshake.failed) != 0 {
		t.Errorf("handshake failed = %q", h.handshake.failed)
	}
}

func TestBackgroundChildReportsStartupFailure(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.handshake = &fakeHandshake{}
	h.appErr = errors.New("read config: bad theme")
	code, _, _ := h.run("serve", "--background-child")
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if len(h.handshake.failed) != 1 || h.handshake.failed[0] != "read config: bad theme" {
		t.Errorf("handshake failed = %q", h.handshake.failed)
	}
}

func TestBackgroundChildWithoutHandshakeIsAnError(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	code, _, stderr := h.run("serve", "--background-child")
	if code != 1 || !strings.Contains(stderr, "no handshake descriptor") {
		t.Errorf("code=%d stderr=%q", code, stderr)
	}
	if len(h.appCalls) != 0 {
		t.Error("app ran without a handshake")
	}
}

func TestServeHelpDoesNotAdvertiseInternalFlag(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	_, stdout, _ := h.run("serve", "--help")
	if strings.Contains(stdout, "background-child") {
		t.Errorf("serve help exposes the internal child flag:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--foreground") {
		t.Errorf("serve help does not document --foreground:\n%s", stdout)
	}
}

func TestBackgroundChildLogsLaterFailuresToServeLog(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.handshake = &fakeHandshake{}
	h.appLaunched = &app.Launched{Address: "127.0.0.1:9"}
	h.appErr = errors.New("local server shutdown: boom")
	code, _, _ := h.run("serve", "--background-child", "--root", "/srv/board")
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if len(h.handshake.failed) != 0 {
		t.Errorf("a failure after ready reached the handshake: %q", h.handshake.failed)
	}
	if len(h.logRoots) != 1 || h.logRoots[0] != "/srv/board" {
		t.Errorf("serve log roots = %q", h.logRoots)
	}
	if got := h.serveLog.String(); got != "flashheart: local server shutdown: boom\n" {
		t.Errorf("serve log = %q", got)
	}
}

func TestBackgroundChildSendsAppOutputToServeLog(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.handshake = &fakeHandshake{}
	if code, _, stderr := h.run("serve", "--background-child", "--debug"); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	options := h.appCalls[0]
	if options.Stderr != io.Writer(h.serveLog) || options.Stdout != io.Writer(h.serveLog) {
		t.Error("background child app output is not routed to serve.log")
	}
}

func TestOnlyTheBackgroundChildOpensServeLog(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.run()
	h.run("--foreground")
	if len(h.logRoots) != 0 {
		t.Errorf("serve log opened outside the background child: %q", h.logRoots)
	}
}

func v1Root(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "boards", "sample-v1"))); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestMigrateDryRunShowsThePlanAndWritesNothing(t *testing.T) {
	t.Parallel()

	root := v1Root(t)
	h := newHarness(t)
	code, stdout, stderr := h.run("migrate", "--root", root, "--key", "alpha=AL", "--key", "beta=BE")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"alpha → key AL (--key)", "AL-3  in-progress/feat--card-panel.md", "Run again with --write"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "tickets")); !os.IsNotExist(err) {
		t.Error("dry run created tickets/")
	}
}

func TestMigrateWriteConvertsTheRoot(t *testing.T) {
	t.Parallel()

	root := v1Root(t)
	h := newHarness(t)
	code, stdout, stderr := h.run("migrate", "--write", "--root", root, "--key", "alpha=AL", "--key", "beta=BE")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "Migrated "+root+" to board format v2: 2 projects, 8 tickets.") {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "tickets", "AL-3-card-panel", "AL-3-card-panel.md")); err != nil {
		t.Errorf("migrated ticket missing: %v", err)
	}
	code, stdout, _ = h.run("migrate", "--write", "--root", root)
	if code != 0 || !strings.Contains(stdout, "Nothing to migrate") {
		t.Errorf("second run: code=%d stdout=%q", code, stdout)
	}
}

func TestMigrateUsageAndFailures(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if code, _, stderr := h.run("migrate", "--key", "alpha"); code != 2 || !strings.Contains(stderr, "--key expects project=KEY") {
		t.Errorf("bad --key: code=%d stderr=%q", code, stderr)
	}
	if code, _, stderr := h.run("migrate", "--root", filepath.Join(t.TempDir(), "absent")); code != 1 || !strings.Contains(stderr, "does not exist") {
		t.Errorf("missing root: code=%d stderr=%q", code, stderr)
	}
	if code, _, stderr := h.run("migrate", "--root", v1Root(t), "--key", "alpha=al"); code != 1 || !strings.Contains(stderr, "uppercase") {
		t.Errorf("invalid key: code=%d stderr=%q", code, stderr)
	}
}

func TestHookRecordsEventsAndPrintsNothing(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	root := t.TempDir()
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	h.stdin = `{"session_id":"abc123","cwd":"` + outside + `","hook_event_name":"UserPromptSubmit","prompt":"secret plans"}`
	code, stdout, stderr := h.run("hook", "claude", "UserPromptSubmit", "--root", root)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("hook = %d, %q, %q", code, stdout, stderr)
	}
	files, _ := filepath.Glob(filepath.Join(root, "_scratch", ".flashheart", "events", "*.jsonl"))
	if len(files) != 1 {
		t.Fatalf("event files = %v", files)
	}
	data, _ := os.ReadFile(files[0])
	if !strings.Contains(string(data), `"run":"claude:abc123"`) || strings.Contains(string(data), "secret plans") {
		t.Fatalf("event log = %s", data)
	}
}

func TestHookAlwaysExitsZero(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"hook"}, {"hook", "claude"}, {"hook", "gemini", "SessionStart"}, {"hook", "claude", "SessionStart", "extra"}} {
		h := newHarness(t)
		root := t.TempDir()
		h.stdin = "{"
		code, stdout, stderr := h.run(append(args, "--root", root)...)
		if code != 0 || stdout != "" {
			t.Errorf("%v: code %d, stdout %q", args, code, stdout)
		}
		if !strings.Contains(stderr, "flashheart hook:") {
			t.Errorf("%v: stderr = %q", args, stderr)
		}
		if data, _ := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log")); !strings.Contains(string(data), "hook") {
			t.Errorf("%v: hook-errors.log = %q", args, data)
		}
	}
	// A malformed payload for a known agent is logged, not reported.
	h := newHarness(t)
	root := t.TempDir()
	h.stdin = "{"
	if code, stdout, stderr := h.run("hook", "claude", "Stop", "--root", root); code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("malformed payload = %d, %q, %q", code, stdout, stderr)
	}
	if data, _ := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log")); !strings.Contains(string(data), "claude Stop: payload is not JSON") {
		t.Fatalf("hook-errors.log = %q", data)
	}
}
