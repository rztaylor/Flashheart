# board-core

Status: **Partial**. Its dependency, `foundation`, is complete.

Done: `mdfile`, `board` (blocking with reasons, workstream status), `store`
read side on `os.Root`, `index` (revisioned, rebuilt when older than 2 s;
5,000 tickets in about 160 ms), and the read API, plus a read-only
attachment endpoint so review screenshots render (SEC-4). Remaining: the
design gate and all UI scope below. Event-log reading stays with
`agent-runs`.

## Goal

A read-only, multi-project board good enough to replace opening the files:
every project under the root, real columns, blocking explained, Workstreams
and Table views, and a card panel that renders the ticket.

## Gate before UI code

Design the shell, Board, card, card panel and Workstreams view with the
`impeccable` skill (the user's choice, 2026-10-04, replacing
`ui-concept-design`) and get approval before any board UI code. Record the
approved direction in `.agents/facts/frontend-ui.md` and `DESIGN.md`.

## Scope

- `internal/mdfile`: parse frontmatter and sections; tolerate errors
  (`STO-4`); no writes yet beyond what tests need.
- `internal/board`: ticket, workstream and project model; blocking rules with
  reasons (`CARD-4`, board-format §Blocking); derived workstream status.
- `internal/store` (read side): discover projects (`PRJ-1`), read columns,
  workstreams, reviews and attachment indexes; path confinement (`SEC-2`).
- `internal/index`: in-memory index with a revision number; full rebuild on
  demand (watching arrives in `board-editing`).
- API: `GET /api/projects`, `GET /api/projects/{p}/board`,
  `GET /api/projects/{p}/tickets/{slug}`, `GET /api/projects/{p}/workstreams`,
  `GET /api/all/board` (All projects).
- UI: project rail with counts (`PRJ-6`); Board (`VIEW-1`) with density
  (`VIEW-6`); Workstreams (`VIEW-4`); Table (`VIEW-5`); filters and search
  (`VIEW-7`, client-side); card panel Ticket and Review tabs with
  GitHub-flavoured markdown, no raw HTML (`CARD-1`, `CARD-2`, `CARD-4`).
- Light and dark themes; keyboard navigation of board and panel (`NFR-3`).

## Acceptance criteria

- The `testdata/boards/` sample renders every ticket in the right column with
  correct blocked reasons, including a ticket with broken frontmatter shown as
  *needs repair*.
- A generated fixture of 5,000 tickets across 10 projects indexes in under
  1 s (`NFR-1`); a realistic 50-ticket project renders without layout
  breakage at 1280 px and 1920 px.
- Ticket links between tickets open the linked ticket; external links open in
  a new tab with `rel="noopener noreferrer"`.
- axe-core passes on Board, Workstreams, Table and the open panel in both
  themes.

## Out of scope

Any writes; drag and drop; live updates; runs.

## Validation

Go unit tests for parsing, blocking and discovery against fixtures;
Vitest for view-models and filters; Playwright screenshots at 1280 and 1920 in
both themes with axe.
