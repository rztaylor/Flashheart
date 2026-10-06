# CLI facts

- Binary: `flashheart`. Commands: `serve` (default), `mcp`, `hook <agent>
  <event>`, `setup <agent> [--uninstall] [--write]`, `doctor`, `version`.
  Global `--root` (else `FLASHHEART_ROOT`, else `~/reports/Kanban`), `--debug`,
  `--help`.
- `hook` and `mcp` print only protocol output to stdout; `hook` always exits 0
  (usage problems go to stderr and `hook-errors.log`); handoff enforcement
  speaks through its JSON output, not the exit code. `hook claude <Event>`
  and `mcp` are implemented; Codex arrives with `codex-support`.
- `mcp` serves stdio and works on the project of `$CLAUDE_PROJECT_DIR`, else
  the working directory (user-scope MCP servers start in `~/.claude`).
- Flags may come before or after a command's positional arguments
  (`hook claude Stop --root DIR`).
- `serve` detaches by default (`LIFE-4`, D13): it re-executes itself in a new
  session, waits for the startup handshake, then returns the terminal.
  `--foreground` keeps it attached; the internal `--background-child` flag is
  never documented in help. The detached child logs diagnostics to
  `<root>/.flashheart/serve.log` (lazy, rotated at 1 MB).
- Normal `serve` startup is quiet. Browser-open failure prints the short-lived
  manual URL once to stderr. `--debug` adds the listener address (and, in the
  foreground, the stop reason) and never prints credentials.
- Commands that are not implemented yet exit 2 with `<command> is not yet
  available` on stderr.
- `setup` writes nothing without `--write`; it prints a diff. `--uninstall`
  previews the reverse and applies it with `--write`. Backups go to
  `~/.claude/flashheart-backup/<UTC time>/`; the MCP server is registered
  with `claude mcp add-json --scope user` (printed when no claude CLI is
  found); the kanban-tracker skill is moved into the backup (user,
  2026-10-06).
- Usage errors exit 2; runtime failures exit 1.
- No command deletes tickets or projects.
