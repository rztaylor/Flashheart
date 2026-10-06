package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
}

// server holds what lives as long as the agent's session.
type server struct {
	options Options
	store   *store.Store
	log     *events.Log
}

// New returns the MCP server with every protocol tool registered.
func New(options Options) (*mcp.Server, error) {
	if options.Root == "" {
		return nil, errors.New("no board root")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	s := store.New(options.Root)
	s.SetClock(options.Now)
	srv := &server{options: options, store: s, log: events.New(s)}
	server := mcp.NewServer(&mcp.Implementation{Name: Name, Version: buildinfo.Version}, &mcp.ServerOptions{
		Instructions: protocol.Instructions(),
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
	})
	srv.registerReads(server)
	srv.registerWrites(server)
	return server, nil
}

// Run serves the protocol on stdin and stdout until the client goes away.
func Run(ctx context.Context, options Options) error {
	server, err := New(options)
	if err != nil {
		return err
	}
	return server.Run(ctx, &mcp.StdioTransport{})
}

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
	switch {
	case errors.As(err, &te):
		return te
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
// errors with a fix.
func tool[In any](server *mcp.Server, name, description string, handle func(In) (string, error)) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description}, func(_ context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, any, error) {
		text, err := handle(input)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: asToolError(err).Error()}}}, nil, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}
