# ui-features

Status: **Partial**. Tracked on the dogfood board as the `ui-features`
workstream: FH-24, FH-23, FH-20, FH-21, FH-22, in that order. No roadmap
dependencies; FH-22 follows the Metro Pop rollout (FH-14).

## Goal

Board UI improvements the user asked for that no other item covers: columns
that scroll together, a saved manual card order, archive views with a
deliberate two-step permanent delete for tickets and projects, and
restrained humour in secondary copy.

## Scope

- FH-24 — the board is one vertical scrolling surface; column heads stay in
  view (ui-layout.md §2). **Built.**
- FH-23 — manual card order saved in each ticket's `rank` (`EDIT-9`, D23).
  **Built.**
- FH-20 — a project's Archive view: search, Restore, and Delete permanently
  for archived tickets only, removing references and retiring the id
  (`EDIT-8`, `KEY-2`, D24). **Built.**
- FH-21 — archive, restore and permanently delete projects (`PRJ-5`,
  `KEY-5`, D25); agents in an archived project's repository are not
  recorded (`project_archived`). **Built.**
- FH-22 — discreet Blackadder remarks from the reviewed collection in
  secondary copy.

Each ticket's acceptance criteria and test plan are on the board; the
behaviour lands in `docs/SPEC.md`, the board-format and layout specs, the
decision log and the changelog with the ticket.

## Exclusions

- No agent tools for archiving or deleting (agent-protocol §7).
- No board sorting UI; FH-23 leaves the display-sort extension point only.

## Validation

`scripts/check.sh` and `scripts/e2e.sh` for every ticket; axe-core in both
themes for new surfaces.
