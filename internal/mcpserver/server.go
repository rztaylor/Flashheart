package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rztaylor/flashheart/internal/buildinfo"
	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/store"
)

// Name is the MCP server name agents register (agent-protocol §7).
const Name = "flashheart"

// Options configure one server.
type Options struct {
	// Root is the board root.
	Root string
	// Cwd is where the calling agent works: CLAUDE_PROJECT_DIR when the
	// agent sets it, else the process's working directory (§7.1).
	Cwd string
	// Now is the clock; nil means time.Now.
	Now func() time.Time
	// Binary is this flashheart's absolute path, for the commands it gives
	// agents; empty means flashheart on PATH.
	Binary string
}

// server holds what lives as long as the process: one session over stdio,
// or several connections in tests.
type server struct {
	options Options
	store   *store.Store
	log     *events.Log

	mu    sync.Mutex
	conns map[*mcp.ServerSession]*conn
	// ending counts started runs whose end is not recorded yet.
	ending sync.WaitGroup
}

// New returns the MCP server with every protocol tool registered.
func New(options Options) (*mcp.Server, error) {
	_, server, err := newServer(options)
	return server, err
}

func newServer(options Options) (*server, *mcp.Server, error) {
	if options.Root == "" {
		return nil, nil, errors.New("no board root")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	s := store.New(options.Root)
	s.SetClock(options.Now)
	srv := &server{options: options, store: s, log: events.New(s), conns: map[*mcp.ServerSession]*conn{}}
	server := mcp.NewServer(&mcp.Implementation{Name: Name, Version: buildinfo.Version}, &mcp.ServerOptions{
		Instructions: protocol.Instructions(),
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
	})
	srv.registerReads(server)
	srv.registerWrites(server)
	return srv, server, nil
}

// Serve speaks the protocol over in and out (stdin and stdout) until the
// client closes its side, then records the end of the run it started for
// the session, if any.
func Serve(ctx context.Context, options Options, in io.Reader, out io.Writer) error {
	srv, server, err := newServer(options)
	if err != nil {
		return err
	}
	err = server.Run(ctx, &mcp.IOTransport{Reader: io.NopCloser(in), Writer: nopWriteCloser{out}})
	srv.ending.Wait()
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// toolError is a tool failure the model can act on: {code, message, fix}
// (agent-protocol §7.2).
type toolError struct {
	Code, Message, Fix string
}

func (e *toolError) Error() string {
	return fmt.Sprintf("error %s: %s\nfix: %s", e.Code, e.Message, e.Fix)
}

func fail(code, fix, format string, args ...any) *toolError {
	return &toolError{Code: code, Message: fmt.Sprintf(format, args...), Fix: fix}
}

// asToolError maps store and file errors to protocol error codes.
func asToolError(err error) *toolError {
	var te *toolError
	var conflict *store.ConflictError
	var taken *store.KeyTakenError
	var workstream *store.WorkstreamNotFoundError
	switch {
	case errors.As(err, &te):
		return te
	case errors.As(err, &workstream):
		fix := "create it with create_workstream"
		if len(workstream.Available) > 0 {
			fix = "use one of " + strings.Join(workstream.Available, ", ") + ", or " + fix
		}
		return fail("not_found", fix, "%v", err)
	case errors.As(err, &taken):
		return fail("key_taken", "choose another key; keys in use: "+strings.Join(taken.InUse, ", "), "%s", taken.Error())
	case errors.As(err, &conflict):
		return fail("conflict", "read the ticket again with get_ticket and retry", "the ticket changed while it was being edited")
	case errors.Is(err, store.ErrKeyFixed):
		return fail("key_fixed", "keep the project's key; it is fixed once the project has tickets", "%v", err)
	case errors.Is(err, store.ErrNotFound), errors.Is(err, mdfile.ErrNotFound):
		return fail("not_found", "check the id with list_tickets", "%v", err)
	case errors.Is(err, store.ErrTypeNotAllowed):
		return fail("type_not_allowed", "attach PNG, JPEG, GIF, WebP, PDF, text, markdown, JSON or log files", "%v", err)
	case errors.Is(err, store.ErrTooLarge):
		return fail("too_large", "attach a smaller file", "%v", err)
	case errors.Is(err, store.ErrInvalidName), errors.Is(err, store.ErrInvalidInput), errors.Is(err, store.ErrNotRegular):
		return fail("invalid_input", "correct the argument and retry", "%v", err)
	case errors.Is(err, mdfile.ErrBrokenFrontmatter):
		return fail("needs_repair", "ask the human to repair the ticket's frontmatter in the board", "the ticket's frontmatter does not parse")
	case errors.Is(err, store.ErrBusy):
		return fail("busy", "retry in a moment", "%v", err)
	case errors.Is(err, config.ErrNewerVersion):
		return fail("unsupported", "update flashheart", "%v", err)
	}
	return fail("internal", "retry; if it keeps failing, tell the human", "%v", err)
}

// tool registers a handler that returns text, turning errors into tool
// errors with a fix. A call of a run the server started is that run's
// activity, and its result carries the run's waiting answers.
func tool[In any](srv *server, server *mcp.Server, name, description string, handle func(*invocation, In) (string, error)) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description}, func(_ context.Context, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, any, error) {
		inv := &invocation{conn: srv.connection(req.Session), tool: name}
		text, err := handle(inv, input)
		if err != nil {
			text = asToolError(err).Error()
		}
		text += srv.called(inv, err == nil)
		return &mcp.CallToolResult{IsError: err != nil, Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}
