# board-format-v2

Status: **Pending**. Decisions: D15 (storage), D16 (ticket ids).

## Goal

Move the board to format v2 (`docs/dev/specs/board-format.md`): a folder per
ticket with the column in frontmatter, JIRA-style ids, five columns including
Up next, and a one-time migration of existing v1 boards.

## Scope

- `internal/board`: five statuses (Backlog, Up next, In progress, Ready to
  review, Done); ids and project keys (`KEY-1`, `KEY-2`); blocking by id
  across projects; duplicate ids need repair.
- `internal/config`: format version 2; detection of v1 roots.
- `internal/store`: read tickets from `tickets/<id>-<slug>/ticket.md`,
  `review.md` and `files/index.yaml`; project keys from `project.yaml`
  (derived default with a warning); archived ids.
- `internal/migrate` + `flashheart migrate [--write] [--key project=KEY]`:
  plan, number once in creation order, rewrite references to ids, move the v1
  tree into `.flashheart/backup/` (`MIG-1`). Atomic writes inside the root.
- API and UI: ids on cards, panel, table, search and URLs; lookup by id;
  the Up next column; bare ids in markdown link to tickets (`KEY-3`,
  `CARD-2`); a v1 root shows the migrate command (`MIG-1`).
- Fixtures: the sample board in v2, the old sample kept as the v1 migration
  fixture; the demo board in v2.

## Acceptance criteria

- The v2 sample renders every ticket in the right column, with ids, blocked
  reasons (including a cross-project dependency) and the broken ticket as
  needs repair.
- Migrating the v1 sample produces the documented v2 tree: ids in creation
  order, references rewritten, v1 files in the backup, nothing deleted; a
  second run is a no-op; a dry run writes nothing.
- `serve` on a v1 root shows the migrate command and writes nothing.
- Lookup by id works in the API and the UI; a bare `FH-12` in markdown opens
  that ticket.
- Indexing 5,000 v2 tickets stays under 1 s (`NFR-1`).

## Out of scope

Creating tickets and assigning new ids from the UI or MCP (board-editing,
mcp-protocol); copying referenced files (REV-5, mcp-protocol and
review-and-orchestration).
