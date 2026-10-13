# Codex setup fixtures

Each folder is a `$CODEX_HOME` (`~/.codex`) as a user might have it before
running `flashheart setup codex`. Tests copy one into a temporary home; none
touches the real `~/.codex`. A test with no fixture starts from an empty
Codex home.

- `user/`: the user's own configuration and nothing of Flashheart's:
  `config.toml` with comments, project trust, another MCP server with an
  `env` table, a multi-line array, an inline table, a dotted key, hook
  trust, and a multi-line string holding text that looks like a table
  header; `hooks.json` with the user's own hooks; and a global `AGENTS.md`.
  Setup must leave all of it exactly as it is.
- `stale/`: an older Flashheart setup made with a binary that has since
  moved: `[mcp_servers.flashheart]` with the old path, a setting and an
  `env` table of the user's; Flashheart hooks for two events, one beside a
  hook of the user's, and a `PreToolUse` hook on Flashheart's tools, which
  setup no longer registers (FH-123); and an out-of-date protocol section between the
  markers in `AGENTS.md`, with the user's text around it.
- `trusted/`: recorded from Codex by `scripts/record-codex-hook-trust.sh`
  (`testdata/hooks/codex/MANIFEST.md`, "Hook trust"), not a starting home:
  `hooks.json` as setup writes it after a hook of the user's, the
  `config.toml` Codex saved after trusting Flashheart's hooks, the same
  hooks after a change of `--root` (`hooks-moved.json`), hooks using every
  field Codex hashes (`hooks-edge.json`), and Codex's own `hooks/list` for
  each (`hooks-list-*.json`: key, hash and trust status). Doctor's trust
  check is tested against these.
