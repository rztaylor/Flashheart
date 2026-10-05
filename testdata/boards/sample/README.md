# Sample board root (format v2)

Fixture for tests. It is `testdata/boards/sample-v1` migrated with
`flashheart migrate --key alpha=AL --key beta=BE`, plus the additions marked
below. Copy it to a temp directory before any test that writes.

| Case | Where |
|---|---|
| Every column (backlog, up-next, in-progress, review, done) | `alpha/tickets/*` (AL-4 moved to up-next) |
| Workstream order blocking | `alpha/workstreams/board-ui.md`: AL-3 waits for AL-2 (in review, so not blocking); AL-4 waits for AL-3 |
| Ticket dependency blocking | AL-5 depends on AL-4 |
| Cross-project dependency (added) | BE-1 depends on AL-3 |
| Missing dependency (warning + blocking) | AL-6 depends on `feat--does-not-exist` |
| Unparseable frontmatter (`STO-4`) | AL-7 (id taken from the folder name) |
| Handoff section | AL-3 |
| Review and files index | `alpha/tickets/AL-2-board-columns/` |
| Archived ticket (added) | `alpha/.archive/tickets/AL-8-old-idea` (next id AL-9) |
| Event log | `alpha/.flashheart/events/2026-10-04.jsonl` (ticket ids, a malformed line) |
| Project keys and next ids | `alpha/project.yaml`, `beta/project.yaml` |
