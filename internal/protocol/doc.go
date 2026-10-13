// Package protocol owns the agent protocol version and the text Flashheart
// gives agents: the recovery note returned at session start (HOOK-3,
// agent-protocol §8) with its lines for what it took from the inbox, the
// inbox note every other delivery path shows (InboxNote: each entry type
// under its own heading, answers as information, comments and change
// requests as the user's words, a change request with its id and how to
// mark it addressed; HOOK-5, D54, FH-126), the linked ticket's open
// change requests in the recovery note (ChangeRequests), and the
// protocol text (§12) rendered as the MCP server's instructions, the
// Claude Code skill setup installs and the body setup puts in Codex's
// AGENTS.md (Body), and the shell commands given to agents
// and agent configuration (the `flashheart await` command, §7.5, and shell
// quoting).
//
// It performs no I/O and renders only what it is given; gathering the
// ticket, handoff and previous run belongs to hooks, storage and transport
// to other packages.
package protocol
