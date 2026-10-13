---
id: FH-109
status: done
type: bug
priority: medium
created: 2026-10-11
branch: feature/preference-reload-race
workstream:
depends-on: []
depends-on-workstreams: []
tags: [flaky, e2e, preferences]
session: claude:887c783f-1797-4e73-ab4c-c2806a0f123c
updated: 2026-10-12T20:31:52Z
pr: https://github.com/rztaylor/Flashheart/pull/70
---
# A preference changed just before a reload can show its old value afterwards

## Description

**Problem.** A browser test failed once in CI: it changes a board preference (Colour by) and reloads straight away. The new value was saved, but the reloaded page kept showing the old choice. The page most likely read preferences before the save finished, and never reads them again.

**Why it matters.** If this is a real race, a user who changes a setting and quickly reloads sees the wrong value and may think it wasn't saved. If it is only the test, a flaky check turns CI red for unrelated changes and wastes time.

**Proposed change.** Reproduce it by running the test many times in a row. Then either make a page that loads while a save is in progress end up showing the saved value, or have the test wait for the save and document that as the expected behaviour. The test should pass 20 runs in a row.

## Acceptance Criteria

- [x] The FH-48 e2e test passes 20 consecutive runs locally (--repeat-each)
- [x] A page loaded while a flushed preference save is in flight ends up showing the saved value, or the test documents why waiting for the save is the contract

## Reproduction

Run `scripts/e2e.sh` with `--repeat-each 20 -g "FH-48"` (or loop the spec) until the Colour by Age radio is unchecked after the reload. CI log: https://github.com/rztaylor/Flashheart/actions/runs/38173375929

## Context

**Technical detail** (from the original description):

CI run 38173375929 (PR #47, 2026-10-11) failed `e2e/editing.spec.mjs:1062` "a preference changed just before a reload is saved (FH-48)". After the reload, config.yaml eventually held `colour_by: age` (the poll passed), but the reloaded page's Colour by radio stayed unchecked for 10 s. The reloaded page probably fetched preferences before the flushed keepalive save landed, and nothing reloads them afterwards. That is either a product race (preferences read once at load) or a test that needs to wait for the save before reloading. PR #47 only changes Go hooks code, so the change did not cause it.

## Notes

- 2026-10-12 · claude:4572d33f: 2026-10-12: This was a product race, not only a test-ordering problem. Timing logs showed the save flushed on unload starts about 28 ms before the reloaded page's read and takes about 6 ms to write. A save slowed to 60 ms (standing in for a slow disk or a wait on the root lock) failed the FH-48 test 3/3, exactly like CI. Fix: the server answers a preference read only after any preference save in progress has finished (read-write lock in internal/api). Remaining assumption: the flushed save reaches the server before the reloaded page's read. The browser sends it before the new page loads, and the read comes at least one heartbeat round trip later. Evidence: Go test TestPreferencesReadWaitsForASaveInFlight (failed before the fix); FH-48 e2e with the simulated slow save 10/10; FH-48 --repeat-each 20 locally 20/20; full e2e --repeat-each=3 240 passed; check.sh green. PR #70: CI is green on a head that contains origin/main. The browser suite's first run failed on the known FH-115 keyboard-reorder flake and passed on rerun. The merge (gh pr merge) was refused by the session's permission classifier, so it is left for the user.

## Handoff

_Updated 2026-10-12 15:33 UTC by claude:4572d33f (run) on main._

**Done**

- Reproduced: not reproducible alone (20/20 pass), but timing logs show the flushed PUT starts ~28 ms before the reloaded page's GET and takes ~6 ms; a 60 ms slow save (simulated slow disk) fails the FH-48 e2e 3/3 exactly like CI. It is a product race.
- Fix: the server orders preference reads after any preference save in flight (RWMutex in internal/api); Go test TestPreferencesReadWaitsForASaveInFlight failed before, passes after; e2e with the simulated 60 ms slow save passes 10/10.
- Docs: SPEC CFG-2, internal/api/doc.go, CHANGELOG Fixed, e2e comment.

**Next**

- Run scripts/check.sh and e2e FH-48 --repeat-each 20, full suite --repeat-each 3, then PR

**Files**

- internal/api/write.go
- internal/api/board.go
- internal/api/api.go
- internal/api/preferences_test.go
- docs/SPEC.md
- CHANGELOG.md

**Open questions**

- None
