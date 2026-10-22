package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/background"
	"github.com/rztaylor/flashheart/internal/buildinfo"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/setup"
)

type harness struct {
	env         map[string]string
	home        string
	appCalls    []ServeOptions
	appErr      error
	appLaunched *Launched
	bgCalls     []background.Options
	bgReport    background.Report
	bgErr       error
	handshake   *fakeHandshake
	serveLog    *bytes.Buffer
	logRoots    []string
	stdin       string
	// terminal makes stdin a terminal setup can ask in.
	terminal bool
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
		RunApp: func(_ context.Context, options ServeOptions) error {
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
		Executable:  func() (string, error) { return "/opt/bin/flashheart", nil },
		Stdin:       strings.NewReader(h.stdin),
		Interactive: func() bool { return h.terminal },
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
		for _, want := range []string{"Usage: flashheart", "serve", "mcp", "hook", "await", "setup", "doctor", "version", "--root", "--debug"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("%v: help does not mention %q:\n%s", args, want, stdout)
			}
		}
	}
}

func TestEveryCommandHasHelp(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"serve", "mcp", "hook", "await", "setup", "doctor", "version"} {
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
	want := "flashheart v0.0.1 (commit abc1234, built 2026-10-04)\nagent protocol 2\n"
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

// doctor reports on stdout and exits 1 when it finds a problem (SET-4).
func TestDoctorReportsAndExitsOneOnProblems(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	root := filepath.Join(h.home, "reports", "Kanban")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := h.run("doctor")
	if code != 1 || stderr != "flashheart: doctor found 3 problems\n" {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	for _, want := range []string{"Board root " + root + "\n  ok       exists and is writable", "Claude Code\n  problem  Hooks: none installed", "Hook errors\n  ok       none"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %q in:\n%s", want, stdout)
		}
	}

	// Set up with a binary that exists, and doctor is satisfied.
	binary := filepath.Join(h.home, "bin", "flashheart")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	deps := h.deps()
	deps.Executable = func() (string, error) { return binary, nil }
	deps.FindCLI = func(setup.Agent, string) string { return "/usr/bin/claude" }
	registered := false
	deps.RunCommand = func(_ string, args ...string) ([]byte, error) {
		// A claude CLI with a user-scope MCP registry: get reads it.
		switch args[1] {
		case "get":
			if !registered {
				return []byte(`No MCP server named "flashheart".`), errors.New("exit status 1")
			}
			return []byte("flashheart:\n  Scope: User config (available in all your projects)\n  Command: " + binary + "\n  Args: mcp\n"), nil
		case "add-json":
			registered = true
		}
		return nil, nil
	}
	run := func(args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), args, &stdout, &stderr, deps)
		return code, stdout.String(), stderr.String()
	}
	if code, _, stderr := run("setup", "claude", "--write"); code != 0 {
		t.Fatalf("setup: %d %s", code, stderr)
	}
	code, stdout, stderr = run("doctor")
	if code != 0 || stderr != "" || strings.Contains(stdout, "problem") {
		t.Fatalf("after setup: code %d, stderr %q, stdout:\n%s", code, stderr, stdout)
	}
	if code, _, stderr = run("doctor", "extra"); code != 2 || !strings.Contains(stderr, "doctor takes no arguments") {
		t.Fatalf("arguments: code %d, stderr %q", code, stderr)
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
	h.appLaunched = &Launched{Address: "127.0.0.1:9", ManualURL: "http://ss-2.localhost:9/#secret", BrowserError: "exec: open: not found"}
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
	h.appLaunched = &Launched{Address: "127.0.0.1:9", ManualURL: "http://ss-3.localhost:9/#secret", BrowserError: "no opener"}
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
	h.appLaunched = &Launched{Address: "127.0.0.1:9"}
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

// The one-off v1 board conversion is retired (D55): migrate is an unknown
// command, and help no longer lists it.
func TestMigrateIsRetired(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if code, _, stderr := h.run("migrate", "--write"); code != 2 || !strings.Contains(stderr, `unknown command "migrate"`) {
		t.Errorf("migrate: code=%d stderr=%q", code, stderr)
	}
	if _, stdout, _ := h.run("--help"); strings.Contains(stdout, "migrate") {
		t.Errorf("help still lists migrate:\n%s", stdout)
	}
}

func TestHookRecordsEventsAndPrintsNothing(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	root := t.TempDir()
	base, _ := filepath.EvalSymlinks(t.TempDir())
	repo := filepath.Join(base, "demo")
	mustWrite(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
	h.stdin = `{"session_id":"abc123","cwd":"` + repo + `","hook_event_name":"UserPromptSubmit","prompt":"secret plans"}`
	code, stdout, stderr := h.run("hook", "claude", "UserPromptSubmit", "--root", root)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("hook = %d, %q, %q", code, stdout, stderr)
	}
	files, _ := filepath.Glob(filepath.Join(root, "demo", ".flashheart", "events", "*.jsonl"))
	if len(files) != 1 {
		t.Fatalf("event files = %v", files)
	}
	data, _ := os.ReadFile(files[0])
	if !strings.Contains(string(data), `"run":"claude:abc123"`) || strings.Contains(string(data), "secret plans") {
		t.Fatalf("event log = %s", data)
	}
}

// Outside git a Claude Code session is not recorded (PRJ-4): its hooks exit
// 0, print nothing and leave the root empty, even for a lone SessionEnd,
// and the installed skill is left for a session in a repository to refresh.
func TestHookOutsideGitRecordsNothing(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	root := t.TempDir()
	skill := filepath.Join(h.home, ".claude", "skills", "flashheart", "SKILL.md")
	old := "---\nname: flashheart\ndescription: old\nmetadata:\n  flashheart-protocol: 1\n---\n\nOld.\n"
	mustWrite(t, skill, old)
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	for _, payload := range []struct{ event, extra string }{
		{"SessionStart", `"source":"startup"`},
		{"UserPromptSubmit", `"prompt":"tidy up"`},
		{"SessionEnd", `"reason":"other"`},
	} {
		h.stdin = `{"session_id":"abc123","cwd":"` + outside + `","hook_event_name":"` + payload.event + `",` + payload.extra + `}`
		if code, stdout, stderr := h.run("hook", "claude", payload.event, "--root", root); code != 0 || stdout != "" || stderr != "" {
			t.Errorf("%s = %d, %q, %q", payload.event, code, stdout, stderr)
		}
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatalf("root after a session outside git = %v", entries)
	}
	if got, _ := os.ReadFile(skill); string(got) != old {
		t.Fatalf("skill changed outside git:\n%s", got)
	}
}

func TestHookAlwaysExitsZero(t *testing.T) {
	t.Parallel()

	// Exit 2 would make Claude Code block the turn, prompt or tool, so even
	// flags this build does not know, or a --root with no value, exit 0.
	for _, args := range [][]string{{"hook"}, {"hook", "claude"}, {"hook", "gemini", "SessionStart"}, {"hook", "claude", "SessionStart", "extra"}, {"hook", "claude", "Stop", "--bogus"}} {
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
	for _, agent := range []string{"claude", "codex"} {
		h := newHarness(t)
		root := t.TempDir()
		h.stdin = "{"
		if code, stdout, stderr := h.run("hook", agent, "Stop", "--root", root); code != 0 || stdout != "" || stderr != "" {
			t.Fatalf("%s malformed payload = %d, %q, %q", agent, code, stdout, stderr)
		}
		if data, _ := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log")); !strings.Contains(string(data), agent+" Stop: payload is not JSON") {
			t.Fatalf("%s hook-errors.log = %q", agent, data)
		}
	}
}

func TestHookWithADanglingRootExitsZero(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.stdin = "{}"
	code, stdout, stderr := h.run("hook", "claude", "Stop", "--root")
	if code != 0 || stdout != "" || !strings.Contains(stderr, "flashheart hook:") {
		t.Fatalf("dangling --root = %d, %q, %q", code, stdout, stderr)
	}
}

func TestMCPServesTheProtocolOnStdio(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	root := t.TempDir()
	h.env["CLAUDE_PROJECT_DIR"] = t.TempDir()
	requests := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
	out := &lockedBuffer{}
	// Like an agent, keep stdin open until both responses are written.
	in := &holdingReader{data: []byte(requests), done: func() bool { return strings.Count(out.String(), "\n") >= 2 }}
	deps := h.deps()
	deps.Stdin = in
	var stderrBuffer bytes.Buffer
	code := Run(context.Background(), []string{"mcp", "--root", root}, out, &stderrBuffer, deps)
	stdout, stderr := out.String(), stderrBuffer.String()
	if code != 0 || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	// CLI-3: stdout carries only protocol messages.
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout =\n%s", stdout)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, `{"jsonrpc":"2.0"`) {
			t.Fatalf("not a protocol message: %q", line)
		}
	}
	for _, part := range []string{`"name":"flashheart"`, `"instructions":"Flashheart (protocol 2)`} {
		if !strings.Contains(lines[0], part) {
			t.Errorf("initialize result missing %s:\n%s", part, lines[0])
		}
	}
	for _, tool := range []string{"board_context", "list_tickets", "get_ticket", "claim", "release", "checkpoint", "update_ticket", "move", "set_project_key", "create_ticket", "write_review", "ask_human"} {
		if !strings.Contains(lines[1], `"name":"`+tool+`"`) {
			t.Errorf("tools/list missing %s", tool)
		}
	}
}

func TestMCPRejectsArguments(t *testing.T) {
	t.Parallel()

	code, _, stderr := newHarness(t).run("mcp", "extra")
	if code != 2 || !strings.Contains(stderr, "mcp takes no arguments") {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// holdingReader returns data, then blocks until done (or 5 s) before EOF.
type holdingReader struct {
	data []byte
	done func() bool
}

func (r *holdingReader) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	for deadline := time.Now().Add(5 * time.Second); !r.done() && time.Now().Before(deadline); {
		time.Sleep(5 * time.Millisecond)
	}
	return 0, io.EOF
}

func TestSetupClaudeShowsThenWritesThenUninstalls(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	settings := filepath.Join(h.home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("{\n  \"model\": \"opus\"\n}\n")
	if err := os.WriteFile(settings, original, 0o644); err != nil {
		t.Fatal(err)
	}
	var ran [][]string
	registered := false
	deps := h.deps()
	deps.FindCLI = func(setup.Agent, string) string { return "/usr/bin/claude" }
	deps.RunCommand = func(name string, args ...string) ([]byte, error) {
		// A claude CLI with a user-scope MCP registry: get reads it.
		switch args[1] {
		case "get":
			if !registered {
				return []byte(`No MCP server named "flashheart".`), errors.New("exit status 1")
			}
			return []byte("flashheart:\n  Scope: User config (available in all your projects)\n  Command: /opt/bin/flashheart\n  Args: mcp\n"), nil
		case "add-json":
			registered = true
		case "remove":
			registered = false
		}
		ran = append(ran, append([]string{name}, args...))
		return nil, nil
	}
	run := func(args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), args, &stdout, &stderr, deps)
		return code, stdout.String(), stderr.String()
	}

	code, stdout, stderr := run("setup", "claude")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"command": "/opt/bin/flashheart hook claude SessionStart",`) ||
		!strings.Contains(stdout, "Nothing was changed: setup asks before it writes, and there is no terminal to ask in. Run flashheart setup claude --yes to apply these changes.") {
		t.Fatalf("dry run: code %d, stderr %q, stdout:\n%s", code, stderr, stdout)
	}
	if data, _ := os.ReadFile(settings); !bytes.Equal(data, original) || len(ran) > 0 {
		t.Fatal("the dry run changed something")
	}
	// A non-default root is passed to every command (SET-3).
	code, stdout, _ = run("setup", "claude", "--root", "/data/board")
	if code != 0 || !strings.Contains(stdout, "hook claude Stop --root /data/board") || !strings.Contains(stdout, `"args":["mcp","--root","/data/board"]`) {
		t.Fatalf("root: code %d, stdout:\n%s", code, stdout)
	}

	if code, stdout, stderr = run("setup", "claude", "--write"); code != 0 || !strings.Contains(stdout, "Wrote ~/.claude/settings.json") {
		t.Fatalf("write: code %d, stderr %q, stdout:\n%s", code, stderr, stdout)
	}
	if len(ran) != 1 || ran[0][0] != "/usr/bin/claude" || ran[0][2] != "add-json" {
		t.Fatalf("ran = %v", ran)
	}
	if code, stdout, _ = run("setup", "claude", "--uninstall"); code != 0 || !strings.Contains(stdout, "Delete ~/.claude/skills/flashheart/SKILL.md.") || !strings.Contains(stdout, "--uninstall --yes") {
		t.Fatalf("uninstall preview: code %d, stdout:\n%s", code, stdout)
	}
	if code, _, stderr = run("setup", "claude", "--uninstall", "--yes"); code != 0 {
		t.Fatalf("uninstall: code %d, stderr %q", code, stderr)
	}
	if data, _ := os.ReadFile(settings); !bytes.Equal(data, original) {
		t.Fatalf("settings after uninstall:\n%s", data)
	}
}

func TestSetupUsage(t *testing.T) {
	t.Parallel()

	for args, want := range map[string]string{
		"setup":                         "setup needs an agent",
		"setup gemini":                  `unknown agent "gemini"`,
		"setup codex --dry-run --yes":   "--dry-run and --yes",
		"setup codex --dry-run --write": "--dry-run and --yes",
	} {
		code, _, stderr := newHarness(t).run(strings.Fields(args)...)
		if code != 2 || !strings.Contains(stderr, want) {
			t.Errorf("%s: code %d, stderr %q", args, code, stderr)
		}
	}
}

// Setup shows the diff, then asks before it writes (FH-129): in a
// terminal it applies on y or yes and on nothing else; --yes (or its older
// name --write) applies without asking; --dry-run and a missing terminal
// only show.
func TestSetupAsksBeforeApplying(t *testing.T) {
	t.Parallel()

	const prompt = "Apply these changes? [y/N] "
	for _, test := range []struct {
		name     string
		terminal bool
		stdin    string
		args     []string
		applied  bool
		asked    bool
		says     string
	}{
		{name: "y", terminal: true, stdin: "y\n", applied: true, asked: true, says: "Wrote ~/codex-home/hooks.json"},
		{name: "yes", terminal: true, stdin: "Yes\n", applied: true, asked: true, says: "Wrote ~/codex-home/hooks.json"},
		{name: "no", terminal: true, stdin: "n\n", asked: true, says: "Nothing was changed."},
		{name: "empty answer", terminal: true, stdin: "\n", asked: true, says: "Nothing was changed."},
		{name: "other answer", terminal: true, stdin: "sure\n", asked: true, says: "Nothing was changed."},
		{name: "end of input", terminal: true, stdin: "", asked: true, says: "Nothing was changed."},
		{name: "no terminal", stdin: "y\n", says: "there is no terminal to ask in. Run flashheart setup codex --yes to apply these changes."},
		{name: "--yes", args: []string{"--yes"}, applied: true, says: "Wrote ~/codex-home/hooks.json"},
		{name: "--write", args: []string{"--write"}, applied: true, says: "Wrote ~/codex-home/hooks.json"},
		{name: "--yes in a terminal", terminal: true, args: []string{"--yes"}, applied: true, says: "Wrote ~/codex-home/hooks.json"},
		{name: "--dry-run", terminal: true, stdin: "y\n", args: []string{"--dry-run"}, says: "Nothing was changed (--dry-run). Run flashheart setup codex to apply these changes."},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			codexHome := filepath.Join(h.home, "codex-home")
			if err := os.MkdirAll(codexHome, 0o755); err != nil {
				t.Fatal(err)
			}
			h.env["CODEX_HOME"] = codexHome
			h.terminal, h.stdin = test.terminal, test.stdin
			code, stdout, stderr := h.run(append([]string{"setup", "codex"}, test.args...)...)
			if code != 0 {
				t.Fatalf("code %d, stderr %q", code, stderr)
			}
			if !strings.Contains(stdout, "Create ~/codex-home/config.toml:") {
				t.Fatalf("no diff:\n%s", stdout)
			}
			if asked := strings.Contains(stderr, prompt); asked != test.asked {
				t.Fatalf("asked = %v, stderr %q", asked, stderr)
			}
			if !strings.Contains(stdout, test.says) {
				t.Fatalf("stdout lacks %q:\n%s", test.says, stdout)
			}
			_, err := os.Stat(filepath.Join(codexHome, "config.toml"))
			if applied := err == nil; applied != test.applied {
				t.Fatalf("applied = %v:\n%s", applied, stdout)
			}
			if test.applied && !strings.Contains(stdout, "/hooks") {
				t.Fatalf("no word on trusting the hooks after writing:\n%s", stdout)
			}
		})
	}
}

// --uninstall asks the same way, and an empty plan asks nothing.
func TestSetupUninstallAsksBeforeApplying(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	codexHome := filepath.Join(h.home, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	h.env["CODEX_HOME"] = codexHome
	if code, _, stderr := h.run("setup", "codex", "--yes"); code != 0 {
		t.Fatalf("install: %q", stderr)
	}

	h.terminal, h.stdin = true, "y\n"
	code, stdout, stderr := h.run("setup", "codex")
	if code != 0 || strings.Contains(stderr, "Apply these changes?") || !strings.Contains(stdout, "already configured") {
		t.Fatalf("an empty plan: code %d, stderr %q, stdout:\n%s", code, stderr, stdout)
	}

	h.terminal, h.stdin = false, ""
	if _, stdout, _ = h.run("setup", "codex", "--uninstall"); !strings.Contains(stdout, "Run flashheart setup codex --uninstall --yes to apply these changes.") {
		t.Fatalf("uninstall without a terminal:\n%s", stdout)
	}
	if _, stdout, _ = h.run("setup", "codex", "--uninstall", "--dry-run"); !strings.Contains(stdout, "Run flashheart setup codex --uninstall to apply these changes.") {
		t.Fatalf("uninstall --dry-run:\n%s", stdout)
	}
	h.terminal, h.stdin = true, "no\n"
	if _, _, stderr = h.run("setup", "codex", "--uninstall"); !strings.Contains(stderr, "Apply these changes? [y/N] ") {
		t.Fatalf("uninstall did not ask: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(codexHome, "config.toml")); err != nil {
		t.Fatal("declining the uninstall removed config.toml")
	}
	h.stdin = "y\n"
	if code, stdout, stderr = h.run("setup", "codex", "--uninstall"); code != 0 || !strings.Contains(stdout, "Deleted ~/codex-home/config.toml") {
		t.Fatalf("uninstall: code %d, stderr %q, stdout:\n%s", code, stderr, stdout)
	}
}

// setup codex works in $CODEX_HOME when it is set, as Codex does (FH-75),
// and doctor checks what it wrote.
func TestSetupCodexShowsThenWritesThenUninstalls(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	codexHome := filepath.Join(h.home, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	h.env["CODEX_HOME"] = codexHome
	root := t.TempDir()
	code, stdout, stderr := h.run("setup", "codex", "--root", root)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Create ~/codex-home/config.toml:") ||
		!strings.Contains(stdout, "hook codex SessionStart --root "+root) ||
		!strings.Contains(stdout, "Run flashheart setup codex --yes to apply these changes.") {
		t.Fatalf("dry run: code %d, stderr %q, stdout:\n%s", code, stderr, stdout)
	}
	if entries, _ := os.ReadDir(codexHome); len(entries) != 0 {
		t.Fatalf("the dry run wrote %v", entries)
	}
	if code, stdout, stderr = h.run("setup", "codex", "--yes", "--root", root); code != 0 || !strings.Contains(stdout, "Wrote ~/codex-home/hooks.json") {
		t.Fatalf("write: code %d, stderr %q, stdout:\n%s", code, stderr, stdout)
	}
	if _, err := os.Stat(filepath.Join(h.home, ".codex")); err == nil {
		t.Fatal("setup wrote ~/.codex although CODEX_HOME is set")
	}
	_, stdout, _ = h.run("doctor", "--root", root)
	for _, want := range []string{"\nCodex\n  ok       MCP server: configured", "  ok       Skill: up to date in ~/.agents/skills/flashheart", "  ok       Global AGENTS.md: Flashheart section up to date"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("doctor lacks %q:\n%s", want, stdout)
		}
	}
	if code, _, stderr = h.run("setup", "codex", "--uninstall", "--yes", "--root", root); code != 0 {
		t.Fatalf("uninstall: code %d, stderr %q", code, stderr)
	}
	for _, name := range []string{"config.toml", "hooks.json", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(codexHome, name)); err == nil {
			t.Errorf("%s remains after uninstall", name)
		}
	}
	if _, err := os.Stat(filepath.Join(h.home, ".agents", "skills", "flashheart")); err == nil {
		t.Error("the skill remains after uninstall")
	}
}

func TestClaudeSessionStartRefreshesTheInstalledSkill(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	root := t.TempDir()
	skill := filepath.Join(h.home, ".claude", "skills", "flashheart", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("---\nname: flashheart\ndescription: old\nmetadata:\n  flashheart-protocol: 1\n---\n\nOld.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base, _ := filepath.EvalSymlinks(t.TempDir())
	repo := filepath.Join(base, "demo")
	mustWrite(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
	h.stdin = `{"session_id":"abc123","cwd":"` + repo + `","hook_event_name":"SessionStart","source":"startup"}`
	code, stdout, stderr := h.run("hook", "claude", "SessionStart", "--root", root)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "flashheart skill was updated") {
		t.Fatalf("hook = %d, %q, %q", code, stdout, stderr)
	}
	for _, file := range protocol.SkillFiles() {
		if got, _ := os.ReadFile(filepath.Join(filepath.Dir(skill), file.Path)); string(got) != file.Text {
			t.Fatalf("%s not refreshed:\n%s", file.Path, got)
		}
	}
}

// Codex's session-start hook refreshes the marked section of the global
// AGENTS.md in $CODEX_HOME and says so in the session's context (FH-121,
// D53); the user's text around it stays as it was.
func TestCodexSessionStartRefreshesTheProtocolText(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	root := t.TempDir()
	codexHome := filepath.Join(h.home, "codex-home")
	h.env["CODEX_HOME"] = codexHome
	stale, err := os.ReadFile(filepath.Join("..", "..", "testdata", "setup", "codex", "stale", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(codexHome, "AGENTS.md")
	mustWrite(t, agents, string(stale))
	base, _ := filepath.EvalSymlinks(t.TempDir())
	repo := filepath.Join(base, "demo")
	mustWrite(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
	h.stdin = `{"session_id":"019a7c00-0000-7000-8000-000000000001","cwd":"` + repo + `","hook_event_name":"SessionStart","source":"startup"}`

	code, stdout, stderr := h.run("hook", "codex", "SessionStart", "--root", root)
	if code != 0 || stderr != "" {
		t.Fatalf("hook = %d, %q, %q", code, stdout, stderr)
	}
	var out struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("output %q: %v", stdout, err)
	}
	// The section held the whole protocol before the skill folder (D89):
	// the skill is installed beside it, then the section shrinks.
	if want := protocol.SkillUpdatedNote + "\n" + protocol.SectionUpdatedNote(agents) + "\n"; !strings.HasPrefix(out.HookSpecificOutput.AdditionalContext, want) {
		t.Fatalf("context = %q, want it to open with %q", out.HookSpecificOutput.AdditionalContext, want)
	}
	got, _ := os.ReadFile(agents)
	if !strings.Contains(string(got), protocol.AlwaysRead()) || strings.Contains(string(got), "An older version of the protocol text.") {
		t.Fatalf("section not refreshed:\n%s", got)
	}
	for _, mine := range []string{"# My global rules\n\n- Answer in British English.\n\n<!-- BEGIN FLASHHEART", "<!-- END FLASHHEART -->\n\n## More of my rules\n\n- Keep commits small.\n"} {
		if !strings.Contains(string(got), mine) {
			t.Fatalf("the user's text changed; lacks %q:\n%s", mine, got)
		}
	}
	if _, err := os.Stat(filepath.Join(h.home, ".codex")); err == nil {
		t.Fatal("the hook wrote ~/.codex although CODEX_HOME is set")
	}
	for _, file := range protocol.SkillFiles() {
		if got, _ := os.ReadFile(filepath.Join(h.home, ".agents", "skills", "flashheart", file.Path)); string(got) != file.Text {
			t.Fatalf("the skill's %s was not installed in ~/.agents/skills", file.Path)
		}
	}

	// The next session start finds both current and says nothing about them.
	if _, stdout, _ = h.run("hook", "codex", "SessionStart", "--root", root); strings.Contains(stdout, "was updated") {
		t.Fatalf("a current section was reported updated: %q", stdout)
	}
}
