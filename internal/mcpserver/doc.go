// Package mcpserver is Flashheart's MCP server for agents (MCP-1–MCP-7,
// agent-protocol §7): the protocol tools over the Go SDK, attribution of
// each call to a run (§7.1, attribution.go alone, including a Codex call's
// _meta session, D45), the run it starts for a connection with no hook run
// and ends on disconnect (connections.go, D34), claims and leases (§6),
// checkpoints, questions, ticket and workstream edits, tickets filed on
// another project, the session inbox in tool results (D54), people's
// comments shown as the user's words framed apart from ticket text, and
// {code, message, fix} errors.
//
// Every call reads the board fresh, only the projects it touches, so it
// sees other writers. Every tool declares a policy at registration
// (policy.go): whether it writes, when it needs the caller's run, and
// whether it guards the ticket against another session's claim. Tools
// write through the store as the caller (store.As), so each write appends
// the ticket's history; they never write comments and change a change
// request's state only to addressed. Archived projects answer
// project_archived (PRJ-5); outside git, tools that need a project answer
// no_project (PRJ-4). There is no archive, restore or delete tool.
//
// Files, locks and copies belong to store; parsing and editing rules to
// board; text edits to mdfile; run state to runs; protocol wording to
// protocol; stdio wiring to cli. It never uses index, api, app or webui.
package mcpserver
