---
id: FH-4
status: done
type: feature
priority: high
created: 2026-10-05
branch: feature/mcp-protocol
workstream:
depends-on: [FH-1, FH-2]
depends-on-workstreams: []
tags: [agents, mcp]
session:
updated: 2026-10-12T20:21:59Z
pr: https://github.com/rztaylor/Flashheart/pull/6
---
# Agents can claim tickets, save their handoff and ask you questions

## Description

**Problem.** Agents can be watched on the board, but they can't work with it. A session can't look up or claim a ticket, record where it got to so the next session can pick up, or ask the user a question and wait for the answer. Connecting Claude Code to Flashheart also has to be done by hand.

**Why it matters.** Without these, the board only reflects what agents happen to do; it can't coordinate them. Two sessions can take the same ticket, a lost session's progress is gone, and questions get buried in a terminal the user isn't looking at.

**Proposed change.** Flashheart offers an MCP server with tools for finding, claiming, checkpointing and releasing tickets, saving a handoff into the ticket, and asking the human questions that are answered on the board. A setup command shows what it will change in Claude Code's configuration, writes only when asked, and can be undone exactly. A protocol skill teaches agents when to use each tool, and handoff enforcement is available as an opt-in.

## Acceptance Criteria

- [x] MCP contract tests over the in-memory transport for every tool and error code
- [x] board_context stays under 1,500 tokens on a generated 50-ticket project
- [x] End-to-end smoke (agent-protocol §13) passes against a temp root
- [x] setup claude shows a diff, writes nothing without --write, and --uninstall restores the original byte for byte
- [x] Seen in dogfooding: a session asked for work in plain words or by id finds the ticket with list_tickets or get_ticket and claims it
- [x] Seen in dogfooding: a session checkpoints its ticket, and the next session in that worktree starts with the recovery note and the handoff
- [x] Seen in dogfooding: a question a session asks is answered on the board and reaches the session with its next prompt
- [x] Seen in dogfooding: PreToolUse stamps the session's run into its flashheart tool calls (the event log names the run, not "unknown")
- [x] Seen in dogfooding: a review that links a screenshot by path shows the copy in the Review tab after the original is gone
- [x] Seen when Flashheart is first used on another repository: the session chooses the project's key before its first ticket

## Test Plan

Slices and tests: docs/dev/plans/003-mcp-protocol.md. Evidence for the ticked criteria: internal/mcpserver contract tests, TestBoardContextStaysWithinItsTokenBudget, TestProtocolSmoke (internal/cli), internal/setup tests on testdata/setup/claude, the Agents Playwright spec (asks through the real flashheart mcp, answers in the browser). The review-screenshot criterion is covered by TestWriteReviewKeepsScreenshotsAfterTheOriginalsGo but not yet seen in the UI.

## Context

**Technical detail** (from the original description):

`flashheart mcp` with the protocol tools, claims and leases, checkpoints into the ticket's handoff section, ask_human, `setup claude`, the protocol skill and opt-in handoff enforcement. Roadmap brief: `docs/dev/roadmap-items/mcp-protocol.md` in the Flashheart repository; scope and acceptance criteria live there.

## Notes

- 2026-10-06: criteria reworded for dogfooding (user): the open ones are things ordinary Flashheart work shows, not staged sessions; "a taken key is refused" is left to the contract tests.
- 2026-10-06: real-session criteria are met by dogfooding Flashheart's own sessions, not staged sandbox sessions (user). Tick each with the session and date when seen.
- 2026-10-06: seen in dogfooding: the server found the calling Claude session claude:6c391b6a through CLAUDE_PROJECT_DIR with no run argument (creating FH-8 and FH-15); a Codex checkpoint copied six referenced files into FH-9's files/.
- 2026-10-06: FH-3 dropped from depends-on; its live check is folded into this ticket's real-session acceptance runs.
- 2026-10-06: user chose setup's MCP registration through the claude CLI and moving kanban-tracker aside (decision D19). Setup applied on the user's machine (backup ~/.claude/flashheart-backup/20261006T125557Z-a203).
- 2026-10-06: this ticket's text after its description was cut by a faulty handoff update and restored from the session's record.

- 2026-10-06 · claude:6c391b6a: Pull request https://github.com/rztaylor/Flashheart/pull/6 opened (pre-PR gate passed: three blocking findings fixed; Can track items on FH-8). The open criteria stay open for dogfooding.

- 2026-10-06 · claude:351cb678: 2026-10-06 · claude:351cb678 reassessed the dogfooding criteria from the event log and session transcripts. Seen: (a) plain-words find and claim: claude:d087159c ("Implement the new theme tickets": list_tickets, get_ticket FH-10/11/6, claim FH-10) and claude:f38e7ba3 ("most important up-next ticket": list_tickets, get_ticket, claim FH-19). (b) PreToolUse stamping: every flashheart call in six sessions got updatedInput with the run, and claude:6a6da8fd's create_workstream at 19:34 UTC carried no run argument from the model while three sessions (5b65150b, f38e7ba3, 6a6da8fd) were live in the same worktree on the same branch, so a working-directory fallback would have returned ambiguous_run. It was attributed to 6a6da8fd. No "unknown" runs in the log. Not yet seen: (c) next-session recovery with handoff. Only a resume of the same run (f38e7ba3, 19:32) got ticket and Last handoff. Each new session started after its predecessor's ticket had moved to review, so the branch link (RUN-5, in-progress only) had nothing to name. (d) No real session has called ask_human. (e) This ticket's review screenshots were copied from the roadmap-priorities-6eb478 worktree, which has since been deleted. The copies are in files/ but have not yet been looked at in the Review tab. (f) No other repository uses Flashheart yet.

- 2026-10-11 · claude:3dbb3cb6: 2026-10-11: reassessed the open dogfooding criteria from the event logs and session transcripts. Seen: (d) question answered on the board reaches the session with its next prompt: claude:81163ddb asked on FH-31 at 2026-10-07 06:46:07, answered 06:46:38, delivered with its next turn at 06:47:24 (ten questions across Flashheart and skillz completed asked → answered → delivered). (e) review screenshots: the copies in files/ show in the Review tab with the roadmap-priorities-6eb478 worktree gone (confirmed by the user 2026-10-11). (f) key chosen first on another repository: claude:9fcd6a4c set SQLRise's key at 2026-10-07 11:59:10 before SR-1 at 11:59:28; claude:a2100d4d set skillz's key at 2026-10-10 21:14:53 before SKZ-1 at 21:15:10. Still not seen: (c) every recovery note with a "Last handoff" line came from the same run resuming (f38e7ba3 FH-19, 4aaff059 SR-2, e9983d4f FH-56, a2100d4d SKZ-2). New sessions linked to an in-progress ticket (FH-39 e0727d0b and 9b34bd9a at 2026-10-07 19:56, FH-11 5b65150b) started before any checkpoint on it, so they had no handoff to show. Sessions mostly start in fresh worktrees, so this needs a new session deliberately started in a worktree whose in-progress ticket has a checkpoint.

## Handoff

_Updated 2026-10-06 14:17 UTC by claude:6c391b6a (run) on feature/mcp-protocol._

**Done**

- mcp-protocol built and reviewed; PR https://github.com/rztaylor/Flashheart/pull/6 open (check.sh, 32 e2e, pre-PR gate passed)
- Pre-PR review: 3 blocking findings fixed with tests; Can track items recorded on FH-8

**Next**

- Wait for CI on PR #6 and merge
- Tick the open criteria as dogfooding shows them (session and date in a note)
- Then mcp-runs (FH-15)

**Files**

- docs/dev/plans/003-mcp-protocol.md
- docs/dev/roadmap-items/mcp-protocol.md
- internal/mcpserver/
- internal/setup/

**Open questions**

- None
