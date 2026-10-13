// Package protocol owns the agent protocol version and the text Flashheart
// gives agents: the recovery note returned at session start (HOOK-3,
// agent-protocol §8), the answers note added to a prompt (HOOK-5) and the
// one conversion of delivered answers to what the agent is shown
// (AnswersOf), and the
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
