# codex-support

Status: **Pending**. Depends on `mcp-runs` (which gives hookless Codex
sessions a run) and so on `mcp-protocol`.

## Goal

Codex sessions (CLI, IDE extension, app) are as visible and recoverable as
Claude Code sessions, and can pick up each other's tickets.

## Scope

- Golden payloads recorded from real Codex sessions (`testdata/hooks/codex/`).
- `internal/hooks/codex` mapping per agent-protocol §5.3, including
  `update_plan` and `apply_patch` paths; Ended by the stale rule.
- `flashheart setup codex`: `config.toml` MCP entry, `hooks.json`, protocol
  text between markers in the global `AGENTS.md` (`SET-1`, `SET-2`).
- Attribution for Codex hook runs where input stamping is unavailable, and
  merging a session's hook run with any run the MCP server started for it
  before its hooks were installed (`mcp-runs`).
- Where Codex reads MCP servers from: on 2026-10-06 a Codex session used
  the server with no `[mcp_servers.flashheart]` in `~/.codex/config.toml`.

## Acceptance criteria

- Golden-payload tests for every mapped Codex event.
- Cross-agent handoff: a Claude session checkpoints and ends; a Codex session
  in the same worktree receives the recovery note and continues the claim
  (manual check recorded in the review, plus an automated simulation).
- `setup codex` diff/write/uninstall tests against fixture config files.

## Out of scope

Agents other than Claude Code and Codex.
