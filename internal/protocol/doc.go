// Package protocol owns the agent protocol version and the text Flashheart
// gives agents: the recovery note returned at session start (HOOK-3,
// agent-protocol §8) with its lines for what it took from the inbox, the
// inbox note every other delivery path shows (InboxNote: each entry type
// under its own heading, answers as information, comments and change
// requests as the user's words, a change request with its id and how to
// mark it addressed; HOOK-5, D54, FH-126), the linked ticket's open
// change requests in the recovery note (ChangeRequests), the reminder
// to claim a ticket (ClaimReminder, HOOK-8, D50), and the
// protocol text (§12): the MCP server's instructions, the skill setup
// installs for every agent (SkillFiles: SKILL.md, a short guide, and the
// reference pages it names, embedded from skill/, D89) and the short
// section setup puts in the global CLAUDE.md and AGENTS.md (AlwaysRead),
// and the shell commands given to agents
// and agent configuration (the `flashheart await` command, §7.5, and shell
// quoting).
//
// It performs no I/O and renders only what it is given; gathering the
// ticket, handoff and previous run belongs to hooks, storage and transport
// to other packages.
package protocol
