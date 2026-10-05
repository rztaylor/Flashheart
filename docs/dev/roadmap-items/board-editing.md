# board-editing

Status: **Pending**. Depends on `board-refresh` (and format v2).

## Goal

Make the board a safe editor shared with other writers: humans in the UI, in
editors, and (later) agents.

## Scope

- `internal/store` write side: per-project advisory lock, atomic writes,
  content-hash preconditions, moves as `status` edits, archive and unarchive
  of ticket folders, id assignment from `next_id` (`STO-3`, `EDIT-8`,
  `KEY-2`); choosing a project key under the root lock, refused once the
  project has tickets, with the derived key (plus a digit when taken)
  recorded by the first ticket (`KEY-5`).
- `internal/mdfile` round-trip edits preserving key order, comments and
  unknown keys (`STO-2`); checkbox toggling; section replace (`## Handoff`)
  and append (`## Notes`).
- `internal/index` watching with fsnotify, debounced, revision bump on change
  (`STO-7`); long-poll `GET /api/changes?since=` (`LIFE-3`).
- UI: drag and drop with `@dnd-kit` and keyboard/menu alternatives
  (`EDIT-1`); blocked-move confirmation (`EDIT-2`); review warnings
  (`EDIT-3`); workstream reordering (`EDIT-4`); New ticket (`EDIT-5`); Edit
  tab with typed fields and raw editor (`EDIT-6`); conflict dialog
  (`EDIT-7`); criteria checkboxes (`CARD-3`).
- Shutdown guard while saves are in flight (`LIFE-2`).
- Settings: `config.yaml` read/write and UI preferences through the backend
  (`CFG-1`, `CFG-2`).

## Acceptance criteria

- Concurrent-writer test: two processes editing the same ticket — one wins,
  the other gets a conflict; no partial files ever observed.
- Round-trip test: loading and saving every fixture without changes produces
  byte-identical files.
- An edit made in a text editor appears in an open browser within one second.
- Every drag action is possible with the keyboard alone (Playwright).
- Two processes choosing the same key for different projects: one succeeds,
  the other is refused with the keys in use; a project with tickets refuses a
  key change.

## Out of scope

Runs, hooks, MCP.
