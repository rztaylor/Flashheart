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

**Status: pre-implementation.** The specification and plan are written;
code starts with the `foundation` roadmap item. Nothing below works yet.

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

- **Files are the truth.** Tickets are markdown with YAML frontmatter in one
  folder per column (`todo/`, `in-progress/`, `ready-to-review/`, `done/`),
  readable in any editor or Obsidian. See
  [`docs/dev/specs/board-format.md`](docs/dev/specs/board-format.md).
- **Hooks do the bookkeeping.** Session starts, plans, permission prompts,
  subagents and stops are recorded automatically; the model only reports what
  only it knows. See
  [`docs/dev/specs/agent-protocol.md`](docs/dev/specs/agent-protocol.md).
- **Local and private.** No network beyond a loopback listener, no
  telemetry, no accounts. Hooks never store your prompts, commands, or tool
  inputs and outputs.

## Planned usage

```sh
flashheart                          # open the board (serve is the default)
flashheart setup claude             # show the hook, MCP and skill changes
flashheart setup claude --write     # apply them, with backups
flashheart setup codex --write
flashheart doctor                   # check the root and agent configuration
```

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

Requires Go 1.26.5+ and Node.js 22+ (build time only). Run all checks with:

```sh
scripts/check.sh
```

## Licence

MIT. See [`LICENSE`](LICENSE).
