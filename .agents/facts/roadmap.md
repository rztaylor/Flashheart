# Roadmap facts

- Index: `docs/dev/roadmap.md`. Briefs: `docs/dev/roadmap-items/<id>.md`.
- IDs are short kebab-case text. Statuses: `Pending`, `Partial`, `Blocked`.
- Order: `agent-runs` → `mcp-protocol` →
  (`codex-support` ∥ `review-and-orchestration`).
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
