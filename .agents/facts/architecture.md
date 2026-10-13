# Architecture facts

Intended layout (packages are created when they get real content):

- `cmd/flashheart`: signals and dependency wiring only.
- `internal/cli`: subcommands (`serve`, `mcp`, `hook`, `setup`, `doctor`,
  `version`), flags, root resolution, exit codes, launch presentation.
- `internal/background`: re-executing `serve` as a detached child and its
  startup handshake (D13).
- `internal/buildinfo`: version, commit and build date set by linker flags.
- `internal/logfile`: lazily created, size-rotated diagnostic logs under
  `<root>/.flashheart/` (`serve.log`, `hook-errors.log`).
- `internal/config`: global `config.yaml` setting names, defaults,
  validation and the `ui` rewrite (CFG-2); no file I/O. `store` reads and
  writes the file (`store.ReadConfig`, `store.UpdateConfig`) and owns its
  location (`store.ConfigFile`); `project.yaml` is read and written by
  `store` alone.
- `internal/mdfile`: pure markdown-with-frontmatter parsing and round-trip
  editing (key order, comments, unknown keys), sections, checkboxes. No I/O.
- `internal/board`: pure domain model (projects, tickets, workstreams,
  columns), blocking rules with reasons, derived workstream status, and the
  ticket-editing rules the board and agents share, each defined once
  (editable fields and their checked change-set, title and section limits,
  key rules, the blocked-start gate, `NoteLine`; FH-85; which sections
  are rewritten whole and where a missing one goes, `SectionChange`,
  FH-66), and a ticket's `activity.md` format with the history changes a
  write made, worked out from the files before and after (D58). No I/O.
- `internal/store`: the only package that touches the board root: discovery,
  reads, locked atomic writes with hash preconditions, typed ticket edits
  from the board and agents alike (`EditTicket`), moves, archive, each
  ticket's activity file (a store writing as someone, `As`, appends the
  history of every ticket write in the same transaction; people's
  comments are added and edited there, never in the ticket file; D47),
  attachments (a browser upload, `UploadAttachment`, is read no further
  than the limit and stored as what `board.SniffAttachment` says its
  bytes are, REV-7, SEC-7), path confinement, creating the root (`EnsureRoot`), reading
  `config.yaml` (`ReadConfig`, through the `os.Root`, so a symlink out of
  the root is refused) and telling its own writes from others'
  (`WroteLast`, by project, ticket and document).
- Root confinement rule (FH-108), enforced by
  `internal/archtest` (`TestOnlyAllowedPackagesTouchFiles`): only
  allowlisted packages may call the `os`, `io/ioutil` and `path/filepath`
  functions that open, create, list, stat, change or remove files; every
  other package reaches the filesystem through `store`. The allowlist, with
  a reason each: `store` (the root, through an `os.Root`); `setup`
  (agents' configuration under the home directory), `gitinfo` (`.git` in
  repositories), `gitchange` (agent worktrees) and `scripts/hookbench`
  (developer tool, temporary root), none of them under the root; and three
  accepted exceptions under the root: `logfile` (the diagnostic logs
  `<root>/.flashheart/serve.log` and `hook-errors.log`, which must record
  store failures), `index` (stats and lists root directories to add
  fsnotify watches; opens no file) and `doctor` (stats the root folder
  itself; without access(2), a probe file it removes). `events` does all
  its I/O through `store`. A new package that touches files fails the test
  until it is routed through `store` or allowlisted with its reason.
- The default root `~/reports/Kanban` is declared once, `cli.DefaultRoot`;
  `hook-errors.log`'s path is `logfile.HookErrorsPath`.
- `internal/events`: event envelope and kinds, locked JSONL append (alone
  or with a session's hook activity state, under one project lock), daily
  rotation, retention (a session inbox expires with its day's events;
  `app` also expires hook activity state after `stale_hours`, FH-82, and
  moves inboxes left in the old `answers/` location into `inbox/`),
  tolerant reading, and the session inbox (D54): typed entries (answer,
  comment, change request) queued with `Log.Queue`, and `Deliver`, the
  one routine every path (prompt and session-start hooks, `await`,
  `board_context`, the MCP server's tool results) hands them over
  through, returning what it took even when recording delivery fails
  (FH-78), and handing a comment sent from the board over as its ticket
  has it then, marked delivered there through `store.DeliverComment`
  (FH-127, D63); `protocol.InboxNote` is the one rendering of what the model is
  shown (the recovery note has its own lines), `runs.Set.PendingDeliveries`
  the fold's undelivered answers. The prompt hook checks one file; the
  other paths also take the old location.
- `internal/scrub`: secret scrubbing and length limits for agent strings.
- `internal/gitinfo`: project, branch and worktree from a cwd without git
  subprocesses; cached.
- `internal/gitchange`: whether a worktree's work changed after a moment
  (changed, untracked and newly committed files' modification times), for
  the turn-end and session-end hooks only (FH-53); one bounded
  `git status` (and `git log` when HEAD moved), without optional locks.
- `internal/runs`: pure run-state derivation, links, claims and flags from
  events and a clock, and why a run needs you (`Reason`, D39), which the
  API sends typed and the frontend only words; it reads `events`' envelope
  types and no files.
- `internal/protocol`: `PROTOCOL_VERSION`, recovery note and protocol text
  rendering shared by hooks, MCP and setup.
- `internal/logfile`: also owns `hook-errors.log` (HOOK-1).
- `internal/hooks`: agent-neutral hook handling and outputs, including
  throttling tool results into activity records (agent-protocol §3);
  `internal/hooks/claude` and `internal/hooks/codex` are the only places that
  know each agent's payload schema.
- `internal/mcpserver`: MCP tools over store, events and runs; attributes
  a Codex call to the session its `_meta` names (D45); starts a run for a
  connection without hooks and ends it on disconnect (D34).
- `internal/await`: `flashheart await`: wait for one question's answer in
  its session's inbox (any entry there wakes it) (or given and undelivered in the `runs` fold, when a
  failed delivery took the inbox), deliver it, notice delivery elsewhere or an answer
  in the session (from the `runs` fold), time out.
- `internal/setup`: an agent-neutral plan engine (`plan.go`: diff, write
  with backup, uninstall moves, commands through the agent's CLI) and the
  `setup.Agent` interface (install, uninstall, diagnose, find its CLI;
  optional `ProtocolRefresher`), implemented per agent (`claude.go`;
  `codex.go`, which edits Codex's `config.toml` as text through its own
  statement scanner, `toml.go`, with no TOML library; refreshing the
  installed protocol text, Claude Code's skill or Codex's marked section of
  `AGENTS.md`, is called by the session-start hook through `cli`; `hooks`
  never imports `setup`).
- `internal/health`: a check's `Finding` and `Level`, shared by setup's
  diagnoses and doctor.
- `internal/doctor`: `flashheart doctor` (SET-4): root checks, a section per
  agent it is given (each agent's own checks), recent hook errors, the
  report. Reads only (a probe file proves the root writable only where
  there is no access(2)).
- Agents are listed once, in `cli`'s registry (`internal/cli/agents.go`):
  each pairs its hook adapter with its `setup.Agent`, and hook, setup,
  doctor and their usage errors read it.
- `internal/references`: serve's copying of files that directly edited
  tickets and reviews link to (REV-5), after each new snapshot, through
  `store`; link syntax is `board.RewriteLocalLinks`, shared with the MCP
  tools. Copies only from the project's repository, not git-ignored
  (SEC-6); the one place serve runs a git subprocess (`check-ignore`).
  Hooks run git only through `gitchange`.
- `internal/migrate`: one-time conversion of a v1 root to board format v2
  (`flashheart migrate`, MIG-1): plan, number, rewrite references, move v1
  files to `.flashheart/backup/`. Writes only through `store` primitives,
  under `store.WithRootLock`; edits text only through `mdfile`.
- `internal/index`: in-memory, revisioned index of the root for `serve`;
  fsnotify watching. Board and runs are fingerprinted apart: event appends
  refold runs over the board already read (no `ReadBoard`), and while
  `Watch` runs it is the freshness source (`Current` skips the `MaxAge`
  rebuild, a board change a write's own rebuild already read is not read
  again). Without a watcher `Current` rebuilds on `MaxAge`. Event logs count
  against `FolderBudget`. Snapshots index runs by ticket, id and parent
  once; API read paths use those lookups, never a scan of `Runs` per card
  (FH-96, D38). `Snapshot.Claimants` are a ticket's live claimants (claim
  link, not Ended, within the lease; each session before its subagents)
  and `Snapshot.Holder` the first of them: the one rule for which session
  a change request (FH-126) is sent to, and which runs a person may send
  a comment to now (FH-127).
- `internal/archtest`: tests over the module's source (the root
  confinement rule, the one default root); no code outside its tests.
- `internal/api`: HTTP JSON handlers and request validation; no filesystem
  rules.
- `internal/app`: Singleserve composition and lifecycle, shutdown guard.
- `internal/webui`: embedded compiled frontend assets.
- `frontend/`: React UI; ownership in `.agents/facts/frontend-ui.md`.

Dependency direction: `cli` → (`app` | `background` | `migrate` | `mcpserver` | `hooks` | `setup` | `await`) →
(`index`, `runs`, `protocol`, `gitchange`) → (`store`, `events`) → (`board`, `mdfile`,
`gitinfo`, `scrub`, `config`). Pure packages (`board`, `mdfile`, `scrub`)
import no I/O packages; `runs` imports only `events`' types. Nothing below `app` imports HTTP or
Singleserve.

- `hook` and `mcp` modes never use `index`, `api`, `app` or `webui`; they
  must start in under 30 ms (`NFR-4`). `hooks` is agent-neutral and takes an
  `Adapter`; `cli` picks the adapter by agent name.
- Developer tools: `scripts/hookbench` (Go command behind
  `scripts/hook-bench.sh`), not part of the binary.
- One run window (`runs.Lookback`, 48 h of daily event files) for every
  reader: index, MCP server, `await` and hooks. Hooks fold only the
  sessions they ask about (`runs.Keep`, `runs.Latest`) through
  `events.Log.ReadSelected`, which judges each line by its envelope before
  decoding its data (D37).
- Concurrency model: many processes write the same root. All ticket and event
  writes go through `store`/`events` under the per-project lock
  (`<project>/.flashheart/lock`) with atomic rename; UI and MCP edits carry a
  content hash precondition.
- Boundary documents: every hand-written Go package has a concise `doc.go`;
  each `frontend/src/features/<feature>/` and shared frontend layer has a
  `BOUNDARY.md` (≤150 words, ≤2 KiB). Generated assets are exempt.
- Generated frontend output: `internal/webui/assets/generated/` (untracked).
- Shared fixtures: `testdata/boards/` (sample roots),
  `testdata/hooks/<agent>/<event>/` (recorded, scrubbed hook payloads) and
  `testdata/tickets/` (read-only copies of real FH tickets).
- External integrations: `github.com/rztaylor/singleserve` v0.2.x (v0.2.2);
  `github.com/modelcontextprotocol/go-sdk` v1.7+; Claude Code and Codex hook
  and MCP configuration (`docs/dev/specs/agent-protocol.md`).
