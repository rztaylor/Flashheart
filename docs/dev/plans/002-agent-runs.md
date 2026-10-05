# 002 — agent-runs

Brief: `docs/dev/roadmap-items/agent-runs.md`. Branch: `feature/agent-runs`.
Each slice is test-first and committed on its own.

## Slices

1. Golden payloads: `testdata/hooks/claude/<Event>/*.json`, scrubbed, with a
   `MANIFEST.md` saying which are recorded from Claude Code and which follow
   the documented schema until recorded; `scripts/record-claude-hooks.sh`
   records a real session into a scratch directory.
2. `internal/scrub`: secret patterns and length limits (`SEC-3`, `HOOK-2`).
3. `internal/events`: envelope, kinds, append under the project lock, daily
   files, retention, tolerant reading (`STO-5`); confined primitives in
   `store`.
4. `internal/gitinfo`: project, branch and worktree from `cwd` without
   subprocesses, cached in `.flashheart/cache/cwd.json` (`PRJ-2`–`PRJ-4`).
5. `internal/runs`: fold of events into runs; state table and flags with a
   fake clock (agent-protocol §4, `RUN-1`–`RUN-5`).
6. `internal/hooks` + `internal/hooks/claude`, recovery note in
   `internal/protocol`, project auto-create (`PRJ-5`, `KEY-5`),
   `flashheart hook claude <Event>`, fail-open logging (`HOOK-1`–`HOOK-4`);
   `scripts/hook-bench.sh` for p95 latency.
7. Serve: runs in the index snapshot (events watched, clock-driven state
   changes move the revision), runs API, retention at startup and daily.
8. Frontend: Agents view (`VIEW-3`), live badges (`VIEW-8`), virtual
   columns (`VIEW-2`), Runs tab (`CARD-1`), Needs you badges in rail and
   band.
9. Manual hook configuration guide, spec and changelog updates, e2e,
   closure audit.

## Progress

- [~] 1 golden payloads (3 recorded; the rest follow the documented schema)
- [x] 2 scrub
- [x] 3 events
- [x] 4 gitinfo
- [x] 5 runs
- [x] 6 hooks and CLI
- [x] 7 serve runs API
- [x] 8 frontend
- [~] 9 docs, e2e, closure (real-session check open)
