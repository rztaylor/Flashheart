# Flashheart

> *"Woof!"*

A single pane of glass for AI-assisted software projects. Flashheart is one
local Go binary that is two things at once:

- a **kanban board** in your browser, across every project you work on, built
  on plain markdown tickets you own; and
- an **MCP server and hook handler** for Claude Code and Codex, so agents,
  orchestrators and subagents record what they are doing on the same tickets:
  their plan, checkpoints, questions for you, screenshots and review guides.

When a session dies or runs out of context, the next one (in either agent) is
told where the work stood and carries on. Meanwhile you can see, for every
project at once, which agents are working, which have gone quiet, and which
are waiting for you.

**Status: early development.** The board (views, editing, live updates),
live agent runs from Claude Code hooks, and the MCP server, protocol skill
and setup for Claude Code work; Codex support and the review panel are
next. See the
[roadmap](docs/dev/roadmap.md).

## How it works

```text
Claude Code / Codex ──hooks──▶ flashheart hook …   ─┐  append events,
        └──────────MCP──────▶ flashheart mcp     ─┤  update tickets
Obsidian / your editor ─────────────────────────────┤
                                                      ▼
                                  ~/reports/Kanban/<project>/  (markdown + event log)
                                                      ▲
Browser ◀── loopback, authenticated ──▶ flashheart serve (watches the files)
```

- **Files are the truth.** Tickets are markdown with YAML frontmatter, one
  folder per ticket (`tickets/FH-42-<slug>/`) with its column in
  frontmatter, readable in any editor or Obsidian. See
  [`docs/dev/specs/board-format.md`](docs/dev/specs/board-format.md).
- **Hooks do the bookkeeping.** Session starts, plans, permission prompts,
  subagents and stops are recorded automatically; the model only reports what
  only it knows. See
  [`docs/dev/specs/agent-protocol.md`](docs/dev/specs/agent-protocol.md).
- **Local and private.** No network beyond a loopback listener, no
  telemetry, no accounts. Hooks never store your prompts, commands, or tool
  inputs and outputs.

## Usage

```sh
flashheart                          # open the board and return the terminal
flashheart --foreground             # keep the server attached to this terminal
flashheart version
```

The server stops when you choose **Quit** in the board or close its last tab.
If no browser can be opened, Flashheart prints a one-time link that works for
two minutes.

Connect Claude Code (hooks, the MCP server and the Flashheart skill):

```sh
flashheart setup claude             # show the changes as a diff
flashheart setup claude --write     # apply them, with backups
```

See [`docs/user/claude-code.md`](docs/user/claude-code.md). Planned:
`flashheart setup codex` and `flashheart doctor`.

The board root defaults to `~/reports/Kanban`; override with `--root` or
`FLASHHEART_ROOT`.

## Documentation

- [`docs/SPEC.md`](docs/SPEC.md): what Flashheart is and must do.
- [`docs/dev/roadmap.md`](docs/dev/roadmap.md): delivery order and status.
- [`docs/dev/decisions.md`](docs/dev/decisions.md): why it is built this way.
- [`docs/dev/prompts/kickoff.md`](docs/dev/prompts/kickoff.md): the prompt
  that starts implementation.
- [`AGENTS.md`](AGENTS.md): rules for agents working on this repository.

## Development

Requires Go 1.26.6+ and Node.js 22+ (build time only).

```sh
scripts/build.sh                    # frontend + binary into build/flashheart
scripts/check.sh                    # lint, unit tests, builds, vet, race tests
scripts/e2e.sh                      # Playwright lifecycle, axe and screenshots
```

Changes land through pull requests; see [`CONTRIBUTING.md`](CONTRIBUTING.md).
Report security issues as described in [`SECURITY.md`](SECURITY.md).

## Licence

MIT. See [`LICENSE`](LICENSE).
