# Product facts

- Product: Flashheart (binary `flashheart`). Authoritative spec: `docs/SPEC.md`.
- Audience: one developer running several AI coding sessions (Claude Code,
  Codex) across several repositories, often in parallel and unattended.
- Shape: one Go process serving a browser board through Singleserve, plus
  `mcp` and `hook` modes the agents call. No webview, runtime Node, account,
  cloud service, telemetry or analytics.
- Position: observes and coordinates agents the user already runs; it does not
  launch, host or proxy agents (decision D1).
- Primary experience: a calm, information-dense multi-project kanban with live
  agent state, a Needs you signal across all projects, and ticket handoffs that
  let any session resume work.
- Data: the board root (default `~/reports/Kanban`) holds plain markdown
  tickets in the kanban-tracker format plus per-project JSONL event logs.
  Files are the truth; the index is in memory.
- First-class agents: Claude Code (CLI, IDE, desktop Code tab) and Codex (CLI,
  IDE, app). Remote MCP clients (ordinary ChatGPT chat) are out of scope.
- Safety boundary: no deletion of tickets or projects (archive only); writes
  confined to the root; agent text is data, never instructions; hooks never
  store prompts, commands, tool inputs or outputs.
- Platforms: macOS primary, Linux supported. Decision needed: Windows.
- Licence: MIT, Robert Taylor as copyright holder.
