---
id: FH-110
status: done
type: bug
priority: medium
created: 2026-10-12
branch: feature/index-runs-refold
workstream:
depends-on: []
depends-on-workstreams: []
tags: [ci, flaky-test]
session: claude:bc851241-a008-4415-8092-6f6840715414
updated: 2026-10-12T20:31:52Z
pr: https://github.com/rztaylor/Flashheart/pull/59
---
# A board-watching test sometimes fails on macOS CI while cleaning up

## Description

**Problem.** A test of the board watcher, which notices when ticket files change on disk, failed on the macOS CI machine. The test stopped the watcher but didn't wait for it to finish, so the watcher could still be reading the test's temporary folder while that folder was being deleted. Other watcher tests followed the same pattern.

**Why it matters.** A test that fails at random turns CI red for unrelated changes, costs time in re-runs and investigation, and teaches people to ignore real failures. This one has only shown up on CI's macOS machines, which makes it hard to reproduce.

**Proposed change.** Every watcher test waits for the watcher to stop completely before its temporary folder is removed. The watcher tests should pass 50 runs in a row on macOS under the race checker. Nothing changes in the product.

## Acceptance Criteria

- [x] Every index test that starts Watch waits for it to return before the test's cleanup runs
- [x] TestWatchSeesExternalEdits passes `go test -race -count=50 -run TestWatch ./internal/index` on macOS
- [x] scripts/check.sh passes

## Reproduction

`go test -race -count=50 -run TestWatchSeesExternalEdits ./internal/index` on macOS; seen in CI run 38237888527 (check macos-latest). Fix: run Watch in a goroutine that closes a done channel, and register cancel + <-done with t.Cleanup (registered after TempDir so it runs first).

## Context

**Technical detail** (from the original description):

CI on main (run 38237888527, merge of PR #56, 2026-10-12) failed check (macos-latest) in the -race pass: `TestWatchSeesExternalEdits: TempDir RemoveAll cleanup: unlinkat .../board/gamma/tickets: bad file descriptor`. The test starts `go index.Watch(ctx, root, ready)` and defers `cancel()`, but nothing waits for Watch to return, so the watcher (fsnotify/kqueue descriptors, a rebuild reading the tree) can still be running while t.TempDir's RemoveAll runs. Other watch tests in internal/index may share the pattern.

## Notes

- 2026-10-12 · claude:bc851241: 2026-10-12 · claude:bc851241: not reproduced locally on an M4 Max with `go test -race -count=40 -run TestWatch ./internal/index` (all passed, 63 s); seen only on the macos-latest CI runner so far.

- 2026-10-12 · claude:bc851241: 2026-10-12: fixed in FH-96's PR https://github.com/rztaylor/Flashheart/pull/59 — every index watcher test starts Watch through a `watch` helper whose t.Cleanup cancels it and waits for it to return before TempDir removal. Watcher tests passed `-race -count=15` there.

- 2026-10-12 · claude:bc851241: 2026-10-12: on feature/index-runs-refold, `go test -race -count=50 -run TestWatch ./internal/index` passed on macOS (M4 Max, 79 s); scripts/check.sh passed there (FH-96).
