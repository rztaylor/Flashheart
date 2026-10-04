# codex-support

Status: **Pending**. Depends on `mcp-protocol`.

## Goal

Codex sessions (CLI, IDE extension, app) are as visible and recoverable as
Claude Code sessions, and can pick up each other's tickets.

## Scope

- Golden payloads recorded from real Codex sessions (`testdata/hooks/codex/`).
- `internal/hooks/codex` mapping per agent-protocol §5.3, including
  `update_plan` and `apply_patch` paths; Ended by the stale rule.
- `flashheart setup codex`: `config.toml` MCP entry, `hooks.json`, protocol
  text between markers in the global `AGENTS.md` (`SET-1`, `SET-2`).
- Codex-specific attribution fallback where input stamping is unavailable.

## Acceptance criteria

- Golden-payload tests for every mapped Codex event.
- Cross-agent handoff: a Claude session checkpoints and ends; a Codex session
  in the same worktree receives the recovery note and continues the claim
  (manual check recorded in the review, plus an automated simulation).
- `setup codex` diff/write/uninstall tests against fixture config files.

## Out of scope

Agents other than Claude Code and Codex.
