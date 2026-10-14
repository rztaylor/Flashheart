// Command flashheart is the Flashheart binary: the browser board, the MCP
// server and the hook handler for AI coding agents, and their setup.
//
// It owns process signal handling and top-level dependency wiring only.
// Argument parsing and modes belong to internal/cli, detaching to
// internal/background and the server lifecycle to internal/app.
package main
