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

Remaining: the real-session criteria. They are met by dogfooding, not by
staged sessions in a sandbox project (user, 2026-10-06): Flashheart's own
development sessions, with `setup claude` applied, use the tools, and each
criterion is ticked on FH-4 with a note of the session and date once it has
been seen. Seen so far (2026-10-06): the server found the calling Claude
session through `CLAUDE_PROJECT_DIR` with no `run` argument (FH-8, FH-15),
and a Codex checkpoint copied six referenced files into FH-9's `files/`.
Still to see in ordinary work:

- A session asked for work in plain words or by id finds the ticket
  (`list_tickets`, `get_ticket`) and claims it.
- A session checkpoints its ticket, and the next session in that worktree
  starts with the recovery note and the handoff.
- A question a session asks is answered on the board and reaches it with
  its next prompt.
- `PreToolUse` stamps the run into a session's tool calls.
- A review that links a screenshot by path shows the copy in the Review tab
  after the original is gone.
- When Flashheart is first used on another repository, the session chooses
  the project's key before its first ticket. (A taken key being refused is
  left to the contract tests.)

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
