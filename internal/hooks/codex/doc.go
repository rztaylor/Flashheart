// Package codex owns Codex's hook payload schema (HOOK-7, agent-protocol
// §5.3). It holds no adapter yet: its tests check the recorded, scrubbed
// payloads in testdata/hooks/codex, the Codex counterpart of
// testdata/hooks/claude, and what Codex sends the MCP server with a call.
//
// Turning payloads into events, resolving projects, scrubbing and storing
// belong to hooks, as for hooks/claude.
package codex
