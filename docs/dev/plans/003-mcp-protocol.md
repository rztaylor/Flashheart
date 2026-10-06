# 003 — mcp-protocol

Brief: `docs/dev/roadmap-items/mcp-protocol.md`. Branch:
`feature/mcp-protocol`. Dogfood ticket FH-4; FH-3 (remaining real payloads
and a session with a task list) closes during the real-session checks in
slice 10. Each slice is test-first and committed on its own.

## Slices

1. Events and runs: data types for questions (`question.asked`,
   `question.answered`, `question.delivered`) and attribution (`by`) on
   ticket events; the run fold tracks claims as leases (`lease_minutes`,
   renewed by the run's and its subagents' events, agent-protocol §6) and
   questions (an open or undelivered question of kind
   review/decision/question/blocked puts its run in Needs you, §4);
   per-project settings from `project.yaml` (`enforce_handoff`,
   `quiet_minutes`) in `config`.
2. Shared rules: field validation, review warnings and the blocked-start
   note move from `api` to `board` so the UI and MCP apply one rule set
   (`EDIT-2`, `EDIT-3`, `EDIT-6`); `store` gains copying a local file into a
   ticket's `files/` with its `index.yaml` entry (allow-list and 20 MB limit,
   `REV-2`, `SEC-2`) and writing `review.md`.
3. `internal/protocol`: protocol text (agent-protocol §12) rendered for the
   MCP instructions and the Claude skill; the recovery note names the MCP
   tools and lists answered questions.
4. `internal/mcpserver` read side on the Go SDK: server, run attribution
   (§7.1), `{code, message, fix}` errors, `board_context` (≤1,500 tokens on
   a 50-ticket project), `list_tickets` (`MCP-7`, `KEY-4`), `get_ticket`.
5. Write tools: `claim`, `release`, `checkpoint` (rewrites `## Handoff`,
   copies referenced files, `REV-5`), `update_ticket`, `move`,
   `set_project_key`, `create_ticket` (`KEY-2`, `KEY-5`), `write_review`
   (`REV-4`, `REV-5`), `ask_human` (`RUN-8`). Each appends its event with
   the caller's run. Contract tests over the in-memory transport for every
   tool and error code.
6. `flashheart mcp` in the CLI (stdio, nothing else on stdout, `CLI-3`;
   starts under 30 ms, `NFR-4`).
7. Hooks: `PreToolUse` run stamping for Flashheart's tools where Claude Code
   supports input rewriting; answered questions delivered on
   `UserPromptSubmit` and in the recovery note (`HOOK-5`); handoff
   enforcement at `Stop` (`HOOK-6`, off by default). Hook latency stays
   under budget (`scripts/hook-bench.sh`).
8. Serve and UI: questions in run views and the API; answering a question
   records it in `## Notes` and the log (`RUN-8`, `STO-6`); open questions
   with an answer box or option buttons on the card and the Agents view
   (`CARD-6`); Needs you counts questions (`VIEW-2`).
9. `internal/setup` and `flashheart setup claude`: diff by default, `--write`
   with timestamped backups, `--uninstall` restoring the original, owning
   only its own entries; installs the protocol skill (`SET-1`–`SET-3`).
10. End-to-end smoke (agent-protocol §13), docs (user guide, spec, facts,
    decisions, changelog), then the real-session acceptance checks with the
    user, re-recording the payloads still missing from
    `testdata/hooks/claude/MANIFEST.md`, and the closure audit.

## Progress

- [ ] 1 events and runs
- [ ] 2 shared rules and file copies
- [ ] 3 protocol text
- [ ] 4 MCP read tools
- [ ] 5 MCP write tools
- [ ] 6 `flashheart mcp`
- [ ] 7 hooks
- [ ] 8 serve and UI
- [ ] 9 setup
- [ ] 10 smoke, docs, real sessions, closure
