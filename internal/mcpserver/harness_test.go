package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/store"
)

var t0 = time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)

const session = "claude:5b0c7e2a-1f3d-4c8e-9a61-2d7f0e4b9c13"

// env is a board root with a project "demo" for a git checkout on branch
// feature/demo, and a client connected to a server working there.
type env struct {
	t      testing.TB
	root   string
	cwd    string
	store  *store.Store
	now    time.Time
	client *mcp.ClientSession
	server *mcp.Server
	srv    *server
}

func newEnv(t testing.TB, key string) *env {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "board")
	cwd := filepath.Join(base, "src", "demo")
	for _, dir := range []string{filepath.Join(cwd, ".git"), filepath.Join(root, "demo", "tickets")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(cwd, ".git", "HEAD"), "ref: refs/heads/feature/demo\n")
	project := "name: demo\nrepos:\n  - " + cwd + "\n"
	if key != "" {
		project = "key: " + key + "\n" + project
	}
	write(t, filepath.Join(root, "demo", "project.yaml"), project)
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	e := &env{t: t, root: root, cwd: cwd, store: s, now: t0}
	s.SetClock(func() time.Time { return e.now })
	return e
}

func write(t testing.TB, name, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// connect starts the server and a client; call it after setting up files.
// The client names itself as Claude Code does.
func (e *env) connect() {
	e.t.Helper()
	e.client = e.dial("claude-code")
}

// dial connects another client, named as an agent names itself at
// initialize, to the env's server, starting the server on first use.
func (e *env) dial(name string) *mcp.ClientSession {
	e.t.Helper()
	if e.server == nil {
		srv, server, err := newServer(Options{Root: e.root, Cwd: e.cwd, Now: func() time.Time { return e.now }})
		if err != nil {
			e.t.Fatal(err)
		}
		e.server, e.srv = server, srv
		// Runs after the connections close: their runs' ends are written
		// before the board's directory is removed.
		e.t.Cleanup(srv.ending.Wait)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := e.server.Connect(ctx, serverTransport, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { serverSession.Close() })
	client, err := mcp.NewClient(&mcp.Implementation{Name: name, Version: "1"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { client.Close() })
	return client
}

// call runs a tool and returns its text and whether it was an error.
func (e *env) call(tool string, args map[string]any) (string, bool) {
	e.t.Helper()
	if e.client == nil {
		e.connect()
	}
	return e.callOn(e.client, tool, args)
}

// callOn runs a tool over one client's connection.
func (e *env) callOn(client *mcp.ClientSession, tool string, args map[string]any) (string, bool) {
	e.t.Helper()
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		e.t.Fatalf("%s: protocol error: %v", tool, err)
	}
	var text strings.Builder
	for _, content := range result.Content {
		if c, ok := content.(*mcp.TextContent); ok {
			text.WriteString(c.Text)
		}
	}
	return text.String(), result.IsError
}

// ok calls a tool that must succeed.
func (e *env) ok(tool string, args map[string]any) string {
	e.t.Helper()
	text, failed := e.call(tool, args)
	if failed {
		e.t.Fatalf("%s(%v) failed:\n%s", tool, args, text)
	}
	return text
}

// fails calls a tool that must fail with code.
func (e *env) fails(tool string, args map[string]any, code string) string {
	e.t.Helper()
	text, failed := e.call(tool, args)
	if !failed || !strings.HasPrefix(text, "error "+code+":") || !strings.Contains(text, "\nfix: ") {
		e.t.Fatalf("%s(%v) = (failed %v)\n%s\nwant error %s with a fix", tool, args, failed, text, code)
	}
	return text
}

func (e *env) ticket(input store.NewTicket) string {
	e.t.Helper()
	created, err := e.store.CreateTicket("demo", input)
	if err != nil {
		e.t.Fatal(err)
	}
	return created.ID
}

func (e *env) setStatus(id string, column board.Column, branch string) {
	e.t.Helper()
	_, err := e.store.UpdateTicket("demo", id, "", func(data []byte) ([]byte, error) {
		data, err := mdfile.SetScalar(data, "status", string(column))
		if err != nil {
			return nil, err
		}
		return mdfile.SetScalar(data, "branch", branch)
	})
	if err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) file(id string) string {
	e.t.Helper()
	data, _, err := e.store.ReadTicket("demo", id)
	if err != nil {
		e.t.Fatal(err)
	}
	return string(data)
}

// events appends events for a run in demo at the current time.
func (e *env) events(run string, list ...events.Event) {
	e.t.Helper()
	for index := range list {
		list[index].Run, list[index].Agent, list[index].Project = run, "claude", "demo"
		if list[index].Time.IsZero() {
			list[index].Time = e.now
		}
	}
	if err := events.New(e.store).Append(list...); err != nil {
		e.t.Fatal(err)
	}
}

// startSession records a live session in the checkout, as hooks would.
func (e *env) startSession(run string) {
	e.t.Helper()
	e.events(run, events.Event{Kind: events.RunStart, Data: events.RunStartData{Kind: events.KindSession, Cwd: e.cwd, Branch: "feature/demo", Worktree: e.cwd, Source: "startup"}},
		events.Event{Kind: events.TurnStart, Data: events.TurnStartData{Cwd: e.cwd, Branch: "feature/demo", Worktree: e.cwd}})
}

func (e *env) log() []events.Event {
	e.t.Helper()
	var list []events.Event
	if err := events.New(e.store).Read("demo", time.Time{}, func(ev events.Event) { list = append(list, ev) }); err != nil {
		e.t.Fatal(err)
	}
	return list
}

func (e *env) kinds(run string) []string {
	var kinds []string
	for _, ev := range e.log() {
		if ev.Run == run {
			kinds = append(kinds, ev.Kind)
		}
	}
	return kinds
}

func contains(t testing.TB, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Fatalf("output missing %q:\n%s", part, text)
		}
	}
}

func contains2(text, part string) bool { return strings.Contains(text, part) }

func (e *env) readFile(name string) string {
	e.t.Helper()
	data, err := os.ReadFile(filepath.Join(e.root, name))
	if err != nil {
		e.t.Fatal(err)
	}
	return string(data)
}

func (e *env) exists(name string) bool {
	_, err := os.Stat(filepath.Join(e.root, name))
	return err == nil
}

func mdfileList(t testing.TB, file, key string) []string {
	t.Helper()
	return mdfile.Parse([]byte(file)).List(key)
}
