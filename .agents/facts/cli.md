# CLI facts

- Binary: `flashheart`. Commands: `serve` (default), `mcp`, `hook <agent>
  <event>`, `setup <agent> [--write|--uninstall]`, `doctor`, `version`.
  Global `--root` (else `FLASHHEART_ROOT`, else `~/reports/Kanban`), `--debug`,
  `--help`.
- `hook` and `mcp` print only protocol output to stdout; `hook` always exits 0
  except for the documented handoff-enforcement output.
- Normal `serve` startup is quiet. Browser-open failure prints the short-lived
  manual URL once to stderr. `--debug` never prints credentials.
- `setup` writes nothing without `--write`; it prints a diff.
- Usage errors exit 2; runtime failures exit 1.
- No command deletes tickets or projects.
