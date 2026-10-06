# Engineering facts

- The board (reading, format v2, editing, live updates) and agent runs from
  Claude Code hooks are built; the MCP server is next. Follow
  `docs/dev/roadmap.md`.
- Prefer small, focused standard-library Go packages and explicit TypeScript
  models; use mature libraries for commodity problems (YAML round-tripping,
  MCP, file watching, drag and drop, markdown).
- Test first for features and bugs: a failing test before implementation.
- Every hook and MCP code path handles malformed input, missing files,
  permission errors, lock timeouts and concurrent writers without panicking.
  Hooks fail open.
- Keep payloads, queues, event reads, rendered nodes and outputs bounded;
  agent-facing outputs have token budgets (`MCP-3`, `HOOK-3`).
- Treat the agent protocol as a public contract: additive changes keep
  `PROTOCOL_VERSION`; anything else bumps it (agent-protocol §14).
