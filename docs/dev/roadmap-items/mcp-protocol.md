# mcp-protocol

Status: **Partial**. Plan: `docs/dev/plans/003-mcp-protocol.md`. Tracked on
the dogfood board as FH-4 (and FH-3 for `agent-runs`' remaining real-session
evidence).

Built (2026-10-06): `internal/mcpserver` with every tool below and contract
tests over the in-memory transport; `flashheart mcp` (initialize in about
6 ms); claim leases and questions in the run fold; checkpoints into
`## Handoff`; referenced-file copies (`REV-5`); `PreToolUse` run stamping;
answers delivered through the session's inbox (D20); handoff enforcement;
`flashheart setup claude` (D19); the protocol skill; answering questions in
the card panel and the Agents view; `TestProtocolSmoke`; a Playwright test
that asks through the real MCP server and answers in the browser.

Remaining (needs a person at the keyboard, with `setup claude --write`
applied on their machine):

- "Tackle the top two bugs" in a real session lists, claims and works the
  two highest-priority open bugs of its project.
- A real session in a new project sets a key it chose before its first
  ticket; a taken key is refused with the keys in use.
- A real session claims, checkpoints, asks a question, receives the answer
  on its next prompt, and a fresh session gets the recovery note; this also
  confirms `updatedInput` stamping and `CLAUDE_PROJECT_DIR` in a live
  Claude Code.
- A review linking a screenshot by absolute path still renders after the
  original is deleted (covered by tests; to be seen once in the UI).

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
- Replace the user's kanban-tracker skill: the protocol skill carries the
  ticket conventions and routes all ticket work through the tools.
- Ticket ids everywhere (`KEY-3`); `list_tickets` and plain-words requests
  such as "tackle the three top-priority bugs" (`KEY-4`, `MCP-7`,
  agent-protocol §7.3); `create_ticket` assigns the next id (`KEY-2`).
- Agents choose their project's key: `board_context` asks for one while it is
  unset, `set_project_key` and `create_ticket`'s `project_key` record it
  (`KEY-5`), and the protocol text explains how to pick 2–5 letters.
- Copy files referenced by local path in `write_review` and `checkpoint`
  into the ticket's `files/` before recording them (`REV-5`).

## Acceptance criteria

- MCP contract tests over the in-memory transport for every tool and error
  code.
- `board_context` output stays under 1,500 tokens on a generated 50-ticket project
  (token estimate test).
- End-to-end smoke (agent-protocol §13) passes against a temp root.
- `setup claude` on a fixture settings file shows a diff, writes nothing
  without `--write`, and `--uninstall` restores the original byte for byte.
- Asked "tackle the top two bugs", a real session lists, claims and works the
  two highest-priority open bug tickets of its own project.
- In a new project, a real session sets a key it chose before creating its
  first ticket; a taken key is refused with the keys in use.
- A review that links a screenshot by absolute path still renders after the
  original file is deleted.
- A real Claude Code session, against a scratch root, claims a ticket, checkpoints, asks a question,
  receives the answer on its next prompt, and a fresh session gets the
  recovery note.

## Out of scope

Codex; attachments and the review panel.
