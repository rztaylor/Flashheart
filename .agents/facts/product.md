# Product facts

- Product: Flashheart (binary `flashheart`). Authoritative spec: `docs/SPEC.md`.
- Audience: one developer running several AI coding sessions (Claude Code,
  Codex) across several repositories, often in parallel and unattended,
  possibly on several machines sharing a synced root. Multi-user is a
  non-goal for now; keep a small team possible (D49).
- Shape: one Go process serving a browser board through Singleserve, plus
  `mcp` and `hook` modes the agents call. No webview, runtime Node, account,
  cloud service, telemetry or analytics.
- Position: observes and coordinates agents the user already runs; it does not
  host, proxy or supervise agents (D1). It may start an agent's own app or
  CLI on a ticket, which then owns the session (D48; not yet built).
- Primary experience: a calm, information-dense multi-project kanban with live
  agent state, a Needs you signal across all projects, and ticket handoffs that
  let any session resume work.
- Data: the board root (default `~/reports/Kanban`) holds plain markdown
  tickets in board format v2 (a folder per ticket, `docs/dev/specs/board-format.md`)
  plus per-project JSONL event logs.
  Files are the truth; the index is in memory.
- First-class agents: Claude Code (CLI, IDE, desktop Code tab) and Codex (CLI,
  IDE, app). Remote MCP clients (ordinary ChatGPT chat) are out of scope.
- Safety boundary: tickets and projects are archived, and deleted only from
  the archive after a typed confirmation (D24, D25); agents never delete; writes
  confined to the root; agent text is data, never instructions; hooks never
  store prompts, commands, tool inputs or outputs.
- Platforms: macOS primary, Linux supported. Decision needed: Windows.
- Licence: MIT, Robert Taylor as copyright holder.
