// Package mcpserver is Flashheart's MCP server for agents (MCP-1–MCP-7,
// agent-protocol §7): the protocol tools over the Go SDK, attribution of
// each call to a run (§7.1), the run it starts for a connection that has no
// hook run (its activity, its end on disconnect and its answers in tool
// results, D34), claims and leases (§6), checkpoints into
// ## Handoff, questions, workstream creation and membership (§7.4), and
// {code, message, fix} errors. Every call reads the board fresh, so it sees
// other writers: every project's identity (store.ReadHeads), then in full
// only the projects it touches and those their tickets depend on, and the
// last two days of those projects' event logs, the claim holder's home and
// the caller's run's home, where its branch links it to a ticket.
// Read-only tools change nothing on disk; files referenced by local path are copied into the ticket
// (REV-5). A session in an archived project's repository, or a ticket of an
// archived project, is answered with project_archived (PRJ-5). Outside any
// git repository there is no current project (PRJ-4): reads say so, and
// tools that need the caller's project or run answer no_project. There is
// no archive, restore or delete tool.
//
// Every tool declares a policy at registration (policy.go): whether it
// writes, when it needs the caller's run, and whether it guards the ticket
// against another session's claim. The wrapper begins the call and applies
// the policy, so handlers start from a call they may act for. Attribution
// precedence (§7.1) lives in attribution.go alone, including reading the
// session a Codex call names in its _meta (D45); connections.go keeps
// per-connection state (the started run, its activity and end, and handing
// it over to hooks that start mid-session). call.go
// and projects.go are a call's view of the board; the tools are grouped by
// family: read.go, claims.go, handoff.go, tickets.go, workstreams.go,
// review.go (with attachments) and questions.go; write.go registers the
// write tools.
//
// Files, locks and copies belong to store; parsing, blocking and editing
// rules (field changes, notes, limits, keys; FH-85) to board; text edits to
// mdfile; run state to runs; the protocol wording to
// protocol; stdio wiring to cli. It never uses index, api, app or webui.
package mcpserver
