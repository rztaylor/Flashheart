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
   **Partial**. Attachments, review panel, subagent tree, `doctor`. Built:
   all of it (FH-6); open: the user's review. Depends on `mcp-protocol`; can
   run in parallel with `mcp-runs` and `codex-support`.
6. [`metro-theme-rollout`](roadmap-items/metro-theme-rollout.md) — **Partial**.
   Built: Metro Pop light and black/charcoal Night Service dark on one
   layout contract (`docs/dev/specs/ui-layout.md`), FH-10–FH-14. Open: the
   user's visual sign-off of the review screenshots. FH-6's new review
   surfaces follow the contract (D21).
7. [`ui-features`](roadmap-items/ui-features.md) — **Partial**. Board UI
   requests outside the other items: shared column scrolling (FH-24),
   saved manual card order (FH-23), archive views with a two-step
   permanent delete for tickets (FH-20) and projects (FH-21), discreet
   humour in secondary copy (FH-22), and a full page for every ticket,
   opened from its id (FH-45). Built: all six; open: the user's review of
   each ticket.
8. [`workstream-epics`](roadmap-items/workstream-epics.md) — **Partial**.
   Workstreams are epics: order never blocks, only `depends-on` does, and
   the Workstreams view draws dependencies as a railway graph (D26, FH-38).
   Built: all of it; open: the user's review. No roadmap dependencies;
   FH-17's workstream UI follows it.
9. [`project-overview`](roadmap-items/project-overview.md) — **Partial**.
   An Overview tab for a project manager replaces the Agents view: decisions,
   reviews, risks and progress by ticket, with sessions as evidence and
   headline metrics since your last change; hooks record less (D32, FH-47;
   workstream FH-49–FH-55). Built: FH-49, FH-50, FH-51, FH-54; open: FH-52,
   FH-53, FH-55. No roadmap dependencies.

## Later possibilities (not scheduled)

See `docs/SPEC.md` §9.

## Policy

- Statuses: `Pending`, `Partial`, `Blocked`. Order follows dependencies.
- Briefs record goal, scope, acceptance criteria by requirement id,
  exclusions, dependencies and validation.
- Remove an item once its outcomes are in the spec, decisions, changelog and
  tests; delete its ExecPlan.
