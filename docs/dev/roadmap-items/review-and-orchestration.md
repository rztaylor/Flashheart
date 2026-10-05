# review-and-orchestration

Status: **Pending**. Depends on `mcp-protocol`.

## Goal

Make finished work easy to verify and orchestrated work easy to follow:
screenshots and logs on the card, a review panel, and a clear tree of an
orchestrator's subagents.

## Scope

- `attach` tool with allow-list, size limit and index (`REV-1`, `REV-2`,
  `SEC-4`); Attachments tab with gallery and lightbox.
- Referenced-file copying beyond the MCP tools: when `serve` sees a ticket or
  review edited directly with links to local files outside the root, it
  copies them into `files/` and rewrites the links (`REV-5`).
- Review tab: review file beside screenshots, *How to Verify* as a checklist
  (`REV-3`); `write_review` already exists from `mcp-protocol` (`REV-4`).
- Subagent tree on the card's Runs tab and in the Agents view, with each
  subagent's plan and checkpoints (agent-protocol §10).
- `flashheart doctor` (`SET-4`).
- Protocol text updated for the screenshot workflow (agent-protocol §11).

## Acceptance criteria

- `attach` refuses SVG, HTML, oversized files, and paths that are not regular
  files; stored files are copies with matching sha256.
- An orchestrated run with three subagents renders as a tree with correct
  states from golden payloads.
- `doctor` reports a missing hook, a stale binary path in agent config, and
  recent hook errors on fixture setups.

## Out of scope

Image annotation; video.
