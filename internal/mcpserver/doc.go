// Package mcpserver is Flashheart's MCP server for agents (MCP-1–MCP-7,
// agent-protocol §7): the protocol tools over the Go SDK, attribution of
// each call to a run (§7.1), the run it starts for a connection that has no
// hook run (its activity, its end on disconnect and its answers in tool
// results, D30), claims and leases (§6), checkpoints into
// ## Handoff, questions, workstream creation and membership (§7.4), and
// {code, message, fix} errors. Every call reads the board fresh, so it sees
// other writers: every project's identity (store.ReadHeads), then in full
// only the projects it touches and those their tickets depend on, and the
// last two days of those projects' event logs and the claim holder's home.
// Read-only tools change nothing on disk; files referenced by local path are copied into the ticket
// (REV-5). A session in an archived project's repository, or a ticket of an
// archived project, is answered with project_archived (PRJ-5); there is no
// archive, restore or delete tool.
//
// Files, locks and copies belong to store; parsing and blocking rules to
// board; text edits to mdfile; run state to runs; the protocol wording to
// protocol; stdio wiring to cli. It never uses index, api, app or webui.
package mcpserver
