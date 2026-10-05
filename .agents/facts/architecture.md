# Architecture facts

Intended layout (packages are created when they get real content):

- `cmd/flashheart`: signals and dependency wiring only.
- `internal/cli`: subcommands (`serve`, `mcp`, `hook`, `setup`, `doctor`,
  `version`), flags, root resolution, exit codes, launch presentation.
- `internal/background`: re-executing `serve` as a detached child and its
  startup handshake (D13).
- `internal/buildinfo`: version, commit and build date set by linker flags.
- `internal/logfile`: lazily created, size-rotated diagnostic logs under
  `<root>/.flashheart/` (`serve.log`; later `hook-errors.log`).
- `internal/config`: global `config.yaml` and per-project `project.yaml`
  defaults, validation and atomic persistence.
- `internal/mdfile`: pure markdown-with-frontmatter parsing and round-trip
  editing (key order, comments, unknown keys), sections, checkboxes. No I/O.
- `internal/board`: pure domain model (projects, tickets, workstreams,
  columns), blocking rules with reasons, derived workstream status. No I/O.
- `internal/store`: the only package that touches the board root: discovery,
  reads, locked atomic writes with hash preconditions, moves, archive,
  attachments, path confinement.
- `internal/events`: event envelope and kinds, locked JSONL append, daily
  rotation, retention, tolerant reading.
- `internal/scrub`: secret scrubbing and length limits for agent strings.
- `internal/gitinfo`: project, branch and worktree from a cwd without git
  subprocesses; cached.
- `internal/runs`: pure run-state derivation, links, claims and flags from
  events and a clock; it reads `events`' envelope types and no files.
- `internal/protocol`: `PROTOCOL_VERSION`, recovery note and protocol text
  rendering shared by hooks, MCP and setup.
- `internal/logfile`: also owns `hook-errors.log` (HOOK-1).
- `internal/hooks`: agent-neutral hook handling and outputs;
  `internal/hooks/claude` and `internal/hooks/codex` are the only places that
  know each agent's payload schema.
- `internal/mcpserver`: MCP tools over store, events and runs.
- `internal/setup`: agent configuration diff, write with backup, uninstall,
  skill/AGENTS.md rendering.
- `internal/migrate`: one-time conversion of a v1 root to board format v2
  (`flashheart migrate`, MIG-1): plan, number, rewrite references, move v1
  files to `.flashheart/backup/`. Writes only through `store` primitives.
- `internal/index`: in-memory, revisioned index of the root for `serve`;
  fsnotify watching.
- `internal/api`: HTTP JSON handlers and request validation; no filesystem
  rules.
- `internal/app`: Singleserve composition and lifecycle, shutdown guard.
- `internal/webui`: embedded compiled frontend assets.
- `frontend/`: React UI; ownership in `.agents/facts/frontend-ui.md`.

Dependency direction: `cli` → (`app` | `background` | `migrate` | `mcpserver` | `hooks` | `setup`) →
(`index`, `runs`, `protocol`) → (`store`, `events`) → (`board`, `mdfile`,
`gitinfo`, `scrub`, `config`). Pure packages (`board`, `mdfile`, `scrub`)
import no I/O packages; `runs` imports only `events`' types. Nothing below `app` imports HTTP or
Singleserve.

- `hook` and `mcp` modes never use `index`, `api`, `app` or `webui`; they
  must start in under 30 ms (`NFR-4`). `hooks` is agent-neutral and takes an
  `Adapter`; `cli` picks the adapter by agent name.
- Developer tools: `scripts/hookbench` (Go command behind
  `scripts/hook-bench.sh`), not part of the binary.
- Concurrency model: many processes write the same root. All ticket and event
  writes go through `store`/`events` under the per-project lock
  (`<project>/.flashheart/lock`) with atomic rename; UI and MCP edits carry a
  content hash precondition.
- Boundary documents: every hand-written Go package has a concise `doc.go`;
  each `frontend/src/features/<feature>/` and shared frontend layer has a
  `BOUNDARY.md` (≤150 words, ≤2 KiB). Generated assets are exempt.
- Generated frontend output: `internal/webui/assets/generated/` (untracked).
- Shared fixtures: `testdata/boards/` (sample roots) and
  `testdata/hooks/<agent>/<event>/` (recorded, scrubbed hook payloads).
- External integrations: `github.com/rztaylor/singleserve` v0.2.x (v0.2.2);
  `github.com/modelcontextprotocol/go-sdk` v1.7+; Claude Code and Codex hook
  and MCP configuration (`docs/dev/specs/agent-protocol.md`).
