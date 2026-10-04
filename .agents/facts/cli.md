# CLI facts

- Binary: `flashheart`. Commands: `serve` (default), `mcp`, `hook <agent>
  <event>`, `setup <agent> [--write|--uninstall]`, `doctor`, `version`.
  Global `--root` (else `FLASHHEART_ROOT`, else `~/reports/Kanban`), `--debug`,
  `--help`.
- `hook` and `mcp` print only protocol output to stdout; `hook` always exits 0
  except for the documented handoff-enforcement output.
- `serve` detaches by default (`LIFE-4`, D13): it re-executes itself in a new
  session, waits for the startup handshake, then returns the terminal.
  `--foreground` keeps it attached; the internal `--background-child` flag is
  never documented in help.
- Normal `serve` startup is quiet. Browser-open failure prints the short-lived
  manual URL once to stderr. `--debug` adds the listener address (and, in the
  foreground, the stop reason) and never prints credentials.
- Commands that are not implemented yet exit 2 with `<command> is not yet
  available` on stderr.
- `setup` writes nothing without `--write`; it prints a diff.
- Usage errors exit 2; runtime failures exit 1.
- No command deletes tickets or projects.
