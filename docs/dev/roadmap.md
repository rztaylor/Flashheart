# Roadmap

`docs/SPEC.md` defines the product; this file is the lean execution index.
Briefs: `docs/dev/roadmap-items/<id>.md`. Active plans: `docs/dev/plans/`.

## Direction

Ship a board people use daily first, then make agents first-class writers to
it: hooks before MCP, Claude Code before Codex, recovery before polish.

## Execution order

1. [`agent-runs`](roadmap-items/agent-runs.md) — **Partial**. Built; open:
   recording the remaining real Claude Code payloads and a real session with
   a task list (needs a person; folded into `mcp-protocol`'s real sessions).
2. [`mcp-protocol`](roadmap-items/mcp-protocol.md) — **Partial**. Built (MCP
   server, claims, checkpoints, questions, `setup claude`, protocol skill,
   handoff enforcement); open: the real-session acceptance checks (needs a
   person).
3. [`codex-support`](roadmap-items/codex-support.md) — **Pending**. Codex
   hook adapter, `setup codex`, AGENTS.md protocol text. Depends on
   `mcp-protocol`.
4. [`review-and-orchestration`](roadmap-items/review-and-orchestration.md) —
   **Pending**. Attachments, review panel, subagent tree, `doctor`. Depends on
   `mcp-protocol`; can run in parallel with `codex-support`.
5. [`metro-theme-rollout`](roadmap-items/metro-theme-rollout.md) — **Pending**.
   Metro Pop light and black/charcoal Night Service dark, with one shared
   Metro Pop layout/content contract. FH-10 → FH-11 → FH-12 → FH-13 → FH-14.
   Implementation follows `review-and-orchestration`; the shared contract
   can be prepared beforehand.

## Later possibilities (not scheduled)

See `docs/SPEC.md` §9.

## Policy

- Statuses: `Pending`, `Partial`, `Blocked`. Order follows dependencies.
- Briefs record goal, scope, acceptance criteria by requirement id,
  exclusions, dependencies and validation.
- Remove an item once its outcomes are in the spec, decisions, changelog and
  tests; delete its ExecPlan.
