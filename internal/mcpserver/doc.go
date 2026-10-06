// Package mcpserver is Flashheart's MCP server for agents (MCP-1–MCP-7,
// agent-protocol §7): the protocol tools over the Go SDK, attribution of
// each call to a run (§7.1), claims and leases (§6), checkpoints into
// ## Handoff, questions, and {code, message, fix} errors. Every call reads
// the board and the last two days of event logs fresh, so it sees other
// writers; files referenced by local path are copied into the ticket
// (REV-5).
//
// Files, locks and copies belong to store; parsing and blocking rules to
// board; text edits to mdfile; run state to runs; the protocol wording to
// protocol; stdio wiring to cli. It never uses index, api, app or webui.
package mcpserver
