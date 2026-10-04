# mcp-protocol

Status: **Pending**. Depends on `agent-runs`.

## Goal

Agents can claim tickets, checkpoint, ask the human, and finish work through
a small MCP tool set, and setup installs everything for Claude Code safely.

## Scope

- `internal/mcpserver` on `modelcontextprotocol/go-sdk`: tools in
  agent-protocol §7.2 except `attach` (`MCP-1`–`MCP-5`); run attribution
  §7.1 including `PreToolUse` stamping for Claude where supported.
- Claims and leases (§6, `RUN-6`); checkpoints writing `## Handoff`
  (`RUN-7`); questions and answers (`RUN-8`, `CARD-6`, `HOOK-5`).
- Handoff enforcement (`HOOK-6`), off by default.
- `internal/protocol`: protocol text template and `PROTOCOL_VERSION`.
- `internal/setup`: `flashheart setup claude` diff, `--write` with backups,
  `--uninstall`; installs `~/.claude/skills/flashheart/SKILL.md`
  (`SET-1`–`SET-3`).
- Replace the user's kanban-tracker skill usage: the protocol skill keeps its
  ticket conventions and points file handling at Flashheart.

## Acceptance criteria

- MCP contract tests over the in-memory transport for every tool and error
  code.
- `board_context` output stays under 1,500 tokens on a generated 50-ticket project
  (token estimate test).
- End-to-end smoke (agent-protocol §13) passes against a temp root.
- `setup claude` on a fixture settings file shows a diff, writes nothing
  without `--write`, and `--uninstall` restores the original byte for byte.
- A real Claude Code session, against a scratch root, claims a ticket, checkpoints, asks a question,
  receives the answer on its next prompt, and a fresh session gets the
  recovery note.

## Out of scope

Codex; attachments and the review panel.
