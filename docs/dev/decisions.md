# Decisions

Durable product and architecture decisions, newest last. Each records what was
decided, the options considered and why.

## 2026-10-04 — D0: Name and licence

Flashheart (after the Blackadder character), binary `flashheart`, repository
`~/src/Flashheart`. MIT licence, Robert Taylor as copyright holder. The working
name AIshape was dropped before any code.

## 2026-10-04 — D1: Observe and coordinate; do not launch agents

Options: watch agents the user runs in their own apps; launch agents from the
board (as Vibe Kanban, Kagan and Kanbots do); both.

Decision: observe and coordinate. Launchers are an established category;
nothing yet gives one view of the sessions people already run in Claude Code
and Codex, with durable handoffs between them. The agent apps keep owning
sessions, credentials and permissions. Launching is a later possibility.

## 2026-10-04 — D2: Markdown tickets plus an append-only event log

Options: markdown only; markdown tickets and a JSONL event log; SQLite with
markdown export.

Decision: tickets stay markdown (readable in Obsidian, diffable, usable without
Flashheart); high-volume run activity goes to per-project daily JSONL files.
`serve` builds an in-memory index at start and keeps no database. Durable
meaning (handoffs, answers) is written into tickets so the log is disposable.

## 2026-10-04 — D3: A ticket's column is its directory

Options: directory (kanban-tracker format); a `status:` field.

Decision: directory. It keeps compatibility with the existing skill and with
Obsidian, and a move is a rename. Agent state lives on runs, so tickets need no
extra AI states. Adds `done/`. Virtual columns (Needs you, Agent working) are
computed and never stored.

## 2026-10-04 — D4: One binary, three modes, no required daemon

Options: every mode writes files through a shared store with locks; a daemon
that all writers go through.

Decision: `serve`, `mcp` and `hook` each use the same store package directly,
with a per-project advisory lock, atomic writes and content-hash
preconditions. Agents must work when the UI is closed, and hooks must not
depend on Singleserve authentication. `serve` watches the files.

## 2026-10-04 — D5: Hooks first, MCP for meaning, instructions last

Options considered for keeping the board current: model instructions only;
MCP tools; hooks; passive reading of agent session logs.

Decision: hooks carry every lifecycle fact deterministically; MCP tools carry
only what the model knows; the protocol skill explains the tools; an opt-in
Stop hook enforces a handoff after edits. Passive session-log reading is
deferred: formats change without notice and logs contain secrets.

## 2026-10-04 — D6: Project from the main checkout; ticket by claim

Project name is the main checkout's directory name (worktree-safe). Runs link
to tickets by explicit claim, provisionally by branch match, and subagents
inherit. Unlinked runs show as Unassigned rather than being guessed.

## 2026-10-04 — D7: A small MCP tool set

About ten tools with compact outputs and no delete. Remote MCP (ordinary
ChatGPT chat connectors) is rejected to stay local-only; Codex covers the
ChatGPT side as a coding agent.

## 2026-10-04 — D8: A separate Agents view plus virtual columns

Options: extra workflow columns for AI states; a separate run-state view.

Decision: run state is a different axis from workflow status. The Agents view
shows runs in lanes (Working, Needs you, Waiting, Quiet, Ended); the board gets
live badges and toggleable virtual columns (Needs you, Agent working).

## 2026-10-04 — D9: Setup shows a diff and writes only with --write

`flashheart setup <agent>` prints the exact configuration changes and writes
them only with `--write`, with backups, owning only its own entries. Editing a
user's agent configuration silently is not acceptable.

## 2026-10-04 — D10: Libraries

Go: `github.com/rztaylor/singleserve` v0.2.x; `github.com/modelcontextprotocol/go-sdk`
(official, v1.7+); `github.com/fsnotify/fsnotify`; a maintained YAML v3
implementation with node-level round-tripping (`go.yaml.in/yaml/v3`, the
maintained successor of `gopkg.in/yaml.v3`; confirm at `foundation`); a small
cross-platform file-lock library or `golang.org/x/sys`. Frontend: React,
strict TypeScript, Vite, Tailwind CSS v4, `@dnd-kit` (accessible drag and
drop), `react-markdown` + `remark-gfm` without raw HTML; Biome, Vitest,
Playwright. Commodity parsing, drag and drop and markdown are not
hand-written.

## 2026-10-04 — D11: Safety model

Writes confined to the root; `attach` copies allow-listed types under a size
limit; hooks store names and scrubbed short summaries only; agent-written text
is data for other agents; no network beyond loopback.

## 2026-10-04 — D12: Board root outside repositories

Default root `~/reports/Kanban`, one subdirectory per project, so the board is
the user's own and is not committed into the repositories it tracks.
Overridable with `--root` or `FLASHHEART_ROOT`. Whether the root is itself a
git repository is left to the user for now.
