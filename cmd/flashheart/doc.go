// Command flashheart is the Flashheart binary: the browser board, and later
// the MCP server and hook handler for AI coding agents.
//
// It owns process signal handling and top-level dependency wiring only.
// Argument parsing belongs to internal/cli, detaching to internal/background
// and the server lifecycle to internal/app.
package main
