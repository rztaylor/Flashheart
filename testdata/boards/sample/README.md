# Sample board root

Fixture for tests (`foundation`, `board-core` onwards). Copy it to a temp
directory before any test that writes. What it exercises:

| Case | Where |
|---|---|
| Every column, including `done/` | `alpha/*` |
| Workstream order blocking | `alpha/workstreams/board-ui.md`: `feat--card-panel` waits for `feat--board-columns` (in review → not blocking) and `feat--drag-and-drop` waits for `feat--card-panel` |
| Ticket dependency blocking | `alpha/todo/bug--column-overflow.md` depends on `feat--drag-and-drop` |
| Missing dependency (warning + blocking) | `alpha/todo/spike--offline-mode.md` |
| Unparseable frontmatter (`STO-4`) | `alpha/todo/docs--broken-frontmatter.md` |
| Handoff section | `alpha/in-progress/feat--card-panel.md` |
| Review and attachment index | `alpha/reviews/`, `alpha/attachments/` |
| Event log | `alpha/.flashheart/events/2026-10-04.jsonl` |
| Minimal project, no `project.yaml` | `beta/` |
| Ignored directories | `.flashheart/` here, `_ignored/` would be ignored |
