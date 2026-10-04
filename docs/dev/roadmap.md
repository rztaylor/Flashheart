# Roadmap

`docs/SPEC.md` defines the product; this file is the lean execution index.
Briefs: `docs/dev/roadmap-items/<id>.md`. Active plans: `docs/dev/plans/`.

## Direction

Ship a board people use daily first, then make agents first-class writers to
it: hooks before MCP, Claude Code before Codex, recovery before polish.

## Execution order

1. [`board-core`](roadmap-items/board-core.md) — **Done**, awaiting review;
   remove this entry and its brief once approved. Store (read
   side), index, blocking, multi-project board, Table and Workstreams views,
   card panel with markdown. UI design gate (impeccable) first.
2. [`board-editing`](roadmap-items/board-editing.md) — **Pending**. Locked
   atomic writes, drag and drop, editing, new ticket, conflicts, live updates.
   Depends on `board-core`.
3. [`agent-runs`](roadmap-items/agent-runs.md) — **Pending**. Event log, run
   state, `hook claude`, Agents view, virtual columns, recovery notes.
   Depends on `board-editing` (store writes).
4. [`mcp-protocol`](roadmap-items/mcp-protocol.md) — **Pending**. MCP server,
   claims, checkpoints, questions, `setup claude`, protocol skill,
   handoff enforcement. Depends on `agent-runs`.
5. [`codex-support`](roadmap-items/codex-support.md) — **Pending**. Codex
   hook adapter, `setup codex`, AGENTS.md protocol text. Depends on
   `mcp-protocol`.
6. [`review-and-orchestration`](roadmap-items/review-and-orchestration.md) —
   **Pending**. Attachments, review panel, subagent tree, `doctor`. Depends on
   `mcp-protocol`; can run in parallel with `codex-support`.

## Later possibilities (not scheduled)

See `docs/SPEC.md` §9.

## Policy

- Statuses: `Pending`, `Partial`, `Blocked`. Order follows dependencies.
- Briefs record goal, scope, acceptance criteria by requirement id,
  exclusions, dependencies and validation.
- Remove an item once its outcomes are in the spec, decisions, changelog and
  tests; delete its ExecPlan.
