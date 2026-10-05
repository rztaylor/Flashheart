# agent-runs

Status: **Partial**. Plan: `docs/dev/plans/002-agent-runs.md`.

## Goal

See every Claude Code session and subagent across all projects, live, with no
effort from the model: what it is doing, its plan, and whether it needs you.
New sessions get a recovery note.

## Scope

- Record golden hook payloads from real Claude Code sessions first
  (`testdata/hooks/claude/`), scrubbed (agent-protocol §13).
- `internal/events`: envelope, kinds, append under lock, daily files,
  retention, tolerant reader (`STO-5`).
- `internal/scrub`: secret scrubber and length limits (`SEC-3`, `HOOK-2`).
- `internal/gitinfo`: project, branch and worktree from `cwd` without
  subprocesses, cached (`PRJ-2`, `PRJ-3`, `PRJ-4`).
- `internal/runs`: state derivation table and flags (agent-protocol §4,
  `RUN-1`–`RUN-5`).
- `internal/hooks` + `internal/hooks/claude`: every event in agent-protocol
  §5.2 except MCP stamping; recovery note (§8, `HOOK-3`); fail-open
  (`HOOK-1`); auto-create projects with a
  `project.yaml` but no key (`PRJ-5`, `KEY-5`).
- `flashheart hook claude <Event>` wired in the CLI.
- UI: Agents view with lanes and subagent nesting (`VIEW-3`); live badges
  (`VIEW-8`); virtual columns with toggles (`VIEW-2`); Runs tab timeline
  (`CARD-1`); Needs you badges in rail and top bar.
- A documented manual hook configuration (setup automation arrives in
  `mcp-protocol`).

## Acceptance criteria

- Golden-payload tests produce the expected events for every mapped event.
- State-table tests cover every transition, including Quiet and stale
  endings, with a fake clock.
- Hook latency: p95 under 50 ms over 1,000 invocations on a warm cache
  (benchmark in CI-free script).
- A hook with a corrupt root, missing permissions or malformed input exits 0
  and logs to `hook-errors.log`.
- A real Claude Code session with the hooks installed against a scratch
  root (`--root`) shows Working →
  Needs you (permission) → Waiting → Ended in the Agents view, with its plan.

## Out of scope

MCP tools, claims, questions, Codex.
