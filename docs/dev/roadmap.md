# Roadmap

`docs/SPEC.md` defines the product; this file is the lean execution index.
Briefs: `docs/dev/roadmap-items/<id>.md`. Active plans: `docs/dev/plans/`.

## Direction

Ship a board people use daily first, then make agents first-class writers to
it: hooks before MCP, Claude Code before Codex, recovery before polish.

## Execution order

1. [`agent-runs`](roadmap-items/agent-runs.md) — **Partial**. Built; open:
   the remaining real Claude Code payloads and a session with a task list
   (evidence from dogfooding).
2. [`mcp-protocol`](roadmap-items/mcp-protocol.md) — **Partial**. Built (MCP
   server, claims, checkpoints, questions, `setup claude`, protocol skill,
   handoff enforcement); open: the real-session acceptance criteria, met by
   dogfooding Flashheart's own sessions.
3. [`mcp-runs`](roadmap-items/mcp-runs.md) — **Pending**. The MCP server
   starts a run for an agent without hooks, so claims, questions and
   attribution work and it shows in the Agents view. Depends on
   `mcp-protocol`.
4. [`codex-support`](roadmap-items/codex-support.md) — **Pending**. Codex
   hook adapter, `setup codex`, AGENTS.md protocol text. Depends on
   `mcp-runs`.
5. [`review-and-orchestration`](roadmap-items/review-and-orchestration.md) —
   **Pending**. Attachments, review panel, subagent tree, `doctor`. Depends on
   `mcp-protocol`; can run in parallel with `mcp-runs` and `codex-support`.
6. [`metro-theme-rollout`](roadmap-items/metro-theme-rollout.md) — **Partial**.
   Metro Pop light and black/charcoal Night Service dark on one layout
   contract (`docs/dev/specs/ui-layout.md`, done). FH-11 → FH-12 → FH-13 →
   FH-14 implement it; no longer waits for `review-and-orchestration`
   (D21), whose new review surfaces follow the contract.

## Later possibilities (not scheduled)

See `docs/SPEC.md` §9.

## Policy

- Statuses: `Pending`, `Partial`, `Blocked`. Order follows dependencies.
- Briefs record goal, scope, acceptance criteria by requirement id,
  exclusions, dependencies and validation.
- Remove an item once its outcomes are in the spec, decisions, changelog and
  tests; delete its ExecPlan.
