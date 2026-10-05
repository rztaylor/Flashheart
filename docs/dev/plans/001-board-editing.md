# 001 — board-editing

Brief: `docs/dev/roadmap-items/board-editing.md`. Branch:
`feature/board-editing`. Each slice is test-first and committed on its own.

## Slices

1. `internal/mdfile` edits that preserve everything they do not touch: set a
   scalar or list field, set a checkbox, replace or append to a section;
   no-op edits return identical bytes (`STO-2`, `CARD-3`).
2. `internal/store` writes: per-project lock and root lock (flock via
   `golang.org/x/sys`), content-hash preconditions, ticket update, create
   with the next id and key choice (`KEY-2`, `KEY-5`), archive and
   unarchive (`EDIT-8`), workstream order (`EDIT-4`), project key; a
   multi-process conflict test (`STO-3`).
3. `internal/index` watching with fsnotify (debounced revision bump,
   `STO-7`) and `GET /api/changes?since=` long-poll (`LIFE-3`).
4. `internal/config` atomic save; UI preferences through the backend
   (`CFG-1`, `CFG-2`).
5. `internal/api` write endpoints with 409 conflicts carrying the current
   file; shutdown guard counts writes in flight (`LIFE-2`).
6. Frontend: live updates by long-poll; Move to… menu, keyboard moves and
   `@dnd-kit` drag (`EDIT-1`); blocked-move confirmation with reason
   (`EDIT-2`); review warnings (`EDIT-3`); criteria checkboxes (`CARD-3`);
   Edit tab with typed fields and raw editor (`EDIT-6`); conflict dialog
   (`EDIT-7`); New ticket (`EDIT-5`); archive with undo (`EDIT-8`);
   workstream reordering (`EDIT-4`); saved preferences (`CFG-2`); project
   key while a project has no tickets (`KEY-5`).
7. Playwright coverage, screenshots, docs and closure audit.

## Progress

- [x] 1 mdfile edits
- [x] 2 store writes
- [x] 3 watching and long-poll
- [x] 4 config and preferences
- [ ] 5 write API and guard
- [ ] 6 frontend editing
- [ ] 7 e2e, docs, closure
