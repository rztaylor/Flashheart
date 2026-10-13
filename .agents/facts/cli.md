# CLI facts

- Binary: `flashheart`. Commands: `serve` (default), `mcp`, `hook <agent>
  <event>`, `await <question-id> --project NAME [--timeout DUR]`,
  `setup <agent> [--uninstall] [--yes | --dry-run]`, `doctor`, `migrate`,
  `version`.
  Global `--root` (else `FLASHHEART_ROOT`, else `~/reports/Kanban`), `--debug`,
  `--help`.
- `hook` and `mcp` print only protocol output to stdout; `hook` always exits 0
  (usage problems go to stderr and `hook-errors.log`); handoff enforcement
  speaks through its JSON output, not the exit code.
- `await` is run by agents in the background after `ask_human`; its stdout
  holds only the inbox note (answers, and any comment or change request
  that woke it, with a line saying the answer is still to come; or a
  one-line "already delivered"), since that is what wakes the agent. Exit 0 delivered, 1 timeout or unknown
  question, 2 usage.
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
- `doctor` prints its report to stdout (sections Board root, Claude Code,
  Codex, Hook errors; each line ok, warning or problem with the fix),
  changes nothing, and exits 1 with `doctor found N problems` on stderr when
  it finds one; warnings alone exit 0. Without a Codex home its Codex
  section is one ok line, so people who use only Claude Code see no
  problems.
- `setup` prints a diff, then asks `Apply these changes? [y/N]` (on
  stderr, answer read from stdin) and writes only on `y`/`yes` (D59).
  `--yes` (older name `--write`) applies without asking; `--dry-run` only
  shows; with no terminal on stdin and no `--yes` it only shows and says to
  re-run with `--yes`, so agents and scripts never hang or write by
  surprise. `--uninstall` follows the same rules. `setup claude`: backups
  go to `~/.claude/flashheart-backup/<UTC time>/`; the MCP server is
  registered with `claude mcp add-json --scope user` (printed when no claude
  CLI is found); the kanban-tracker skill is moved into the backup (user,
  2026-10-06). `setup codex` works in `$CODEX_HOME` (default `~/.codex`),
  runs no Codex command, backs up into its `flashheart-backup/`, and ends by
  telling the user to trust the hooks with `/hooks` in codex.
- Usage errors exit 2; runtime failures exit 1.
- No command deletes tickets or projects.
