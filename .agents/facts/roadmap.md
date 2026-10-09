# Roadmap facts

- Index: `docs/dev/roadmap.md`. Briefs: `docs/dev/roadmap-items/<id>.md`.
- IDs are short kebab-case text. Statuses: `Pending`, `Partial`, `Blocked`.
- Order: `agent-runs` → `mcp-protocol` →
  ((`mcp-runs` → `codex-support`) ∥ `review-and-orchestration`).
- `metro-theme-rollout` runs alongside `review-and-orchestration` (D21);
  new UI from either follows `docs/dev/specs/ui-layout.md`. Brief and
  approved image references: `docs/dev/roadmap-items/metro-theme-rollout.md`.
- `ui-features` (board UI requests, the `ui-features` workstream FH-24,
  FH-23, FH-20, FH-21, FH-22, FH-45) has no roadmap dependencies.
- `workstream-epics` (FH-38, D26: workstream order never blocks) has no
  roadmap dependencies; FH-17's New workstream dialog builds on it.
- Visible UI changes follow `DESIGN.md`; new surfaces or a changed visual
  world go through the `impeccable` skill first.
- Remove completed items once outcomes are in the spec, decisions, changelog
  and tests.
- Later possibilities (SPEC §9) are candidates, not commitments.
- Dogfooding: Flashheart's own work is tracked on its board, project
  `Flashheart` (key `FH`) in the default root `~/reports/Kanban`, which is
  not committed (D12). Roadmap briefs stay the scope source; each ticket
  names its brief. Agents working here move their ticket to `in-progress`
  with `branch:` set when work starts and to `review` when the pull request
  opens, and keep `## Handoff` current, through the `flashheart` MCP tools
  where `setup claude` is applied (`claim`, `checkpoint`, `move`), else
  through the board UI or in board format v2. Machines without the board
  (CI, other checkouts) skip this.
- Real-session acceptance criteria are met by dogfooding: Flashheart's own
  development sessions are the real sessions. Tick a criterion on its
  ticket, with the session and date in a note, once it has been seen; do
  not stage sessions in sandbox projects to produce the evidence (user,
  2026-10-06). An item whose only open scope is such evidence stays
  `Partial`, and its pull request may merge.
