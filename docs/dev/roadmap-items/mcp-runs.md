# mcp-runs

Status: **Pending**. Tracked on the dogfood board as FH-15. Depends on
`mcp-protocol`; `codex-support` depends on it.

## Goal

An agent that reaches the `flashheart` MCP server without Flashheart's hooks
(Codex while its hooks are experimental, a user who declined hooks, a new
agent) still gets a run: it can claim tickets, ask questions and receive
answers, its writes are attributed to it, and it shows in the Agents view.
Hooks stay the deterministic layer; this is the fallback when they are
missing (agent-protocol §1).

Evidence (2026-10-06): Codex created FH-9–FH-14 and checkpointed FH-9
through the MCP tools with no hooks installed; `claim` failed with
`ambiguous_run`, its writes were attributed to "unknown", and none of its
activity reached the event log or the Agents view.

## Scope

- When a write finds no run (agent-protocol §7.1), the MCP server starts
  one for its own connection: `<agent>:mcp-<random>`, the agent named from
  the client's `clientInfo` at initialize, with the working directory,
  branch and worktree of `CLAUDE_PROJECT_DIR` or the server's directory,
  and `source: mcp` on `run.start`. A run found from hooks always wins.
- Every tool call of that connection is activity, so claim leases renew
  (agent-protocol §6, `RUN-6`); closing the connection ends the run
  (`run.end`, reason `disconnected`); a server that dies leaves it to the
  stale rule.
- Answers waiting for an MCP-started run are appended to that connection's
  next tool result, standing in for the prompt hook (`HOOK-5`), and marked
  delivered; `board_context` stands in for the recovery note (`HOOK-3`).
- The run fold and Agents view handle runs with no turn events: they read
  as Waiting unless they need you (`RUN-3`), and are marked as reported by
  MCP only, without plan, edits or permission prompts.
- Spec and protocol: `RUN-1` (a run is created from the first hook event or
  the first MCP write), agent-protocol §1, §4 and §7.1, a decision record,
  and the protocol skill text. Additive: `PROTOCOL_VERSION` stays 1.
- Before building, confirm with each agent that one MCP server process
  serves one session and is closed when the session ends (documented for
  Claude Code; to be checked for Codex).

## Acceptance criteria

- MCP contract tests: with no hook run, `claim`, `release` and `ask_human`
  succeed and record events under the minted run; with a hook run present,
  no run is minted; two connections in one worktree get two runs.
- Run-fold table tests for MCP-started runs: leases renewed by tool calls,
  ended on disconnect and by the stale rule, Needs you from a question.
- An answer to an MCP-started run's question is delivered once, in the next
  tool result of that connection.
- A real Codex session without Flashheart hooks claims a ticket, asks a
  question, receives the answer and shows in the Agents view; its ticket
  writes name its run, not "unknown".

## Out of scope

Codex hooks and `setup codex` (`codex-support`); Working, Quiet, plans,
edited files and handoff enforcement for hookless runs, which need hooks.
