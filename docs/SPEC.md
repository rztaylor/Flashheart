# Flashheart specification

Status: **draft 1** (2026-10-04). Authoritative for product direction and
requirements. Detailed contracts live in:

- [`docs/dev/specs/board-format.md`](dev/specs/board-format.md): the on-disk
  board format (projects, tickets, workstreams, reviews, attachments, events).
- [`docs/dev/specs/agent-protocol.md`](dev/specs/agent-protocol.md): runs,
  hooks, MCP tools, claims, handoffs, questions and recovery.

Where another document disagrees with this one, fix the other document.
Requirement ids (`PRJ-1`, `RUN-4` …) are stable; cite them in briefs, tests and
pull requests. Rationale for the major choices is in
[`docs/dev/decisions.md`](dev/decisions.md).

---

## 1. Summary

Flashheart is a **single pane of glass for AI-assisted software projects**. It
is one Go binary that:

1. serves a local **kanban board** in the user's browser, across every project
   the user works on; and
2. acts as an **MCP server** and **hook handler** for AI coding agents (Claude
   Code and Codex first), so agents, orchestrators and subagents record what
   they are doing on the same tickets humans read.

Tickets outlive sessions. When a session is lost, compacted or replaced, the
next one (in either agent) reads the ticket's handoff and continues. Meanwhile
the user sees, for all projects at once, which agents are working, which are
stuck, and which are waiting for them.

Flashheart **observes and coordinates** agents the user already runs in their
own apps. It does not launch, host or proxy agents (`D1`).

## 2. Users and jobs

The primary user is one developer running several AI coding sessions across
several repositories, often in parallel and often unattended.

| Job | Today | With Flashheart |
|---|---|---|
| "What is going on across my projects?" | Click through chat tabs and terminals | One board, every project, live agent state |
| "Which session needs me?" | Notice a stalled tab by chance | A **Needs you** lane and badge across all projects |
| "My session died; where was it?" | Re-read history, re-explain | The ticket's handoff; the next session is told on start |
| "What did the agent actually do?" | Scroll a transcript | Plan, checkpoints, files, screenshots and review guide on the card |
| "What should run next?" | Remember | Prioritised, dependency-aware tickets and workstreams |

A secondary user is **the agent itself**: it needs a small, reliable,
low-token way to find its work, claim it, report progress and hand off.

## 3. Principles

1. **Files are the truth.** Tickets are plain markdown with YAML frontmatter,
   readable and editable in any editor, and usable without
   Flashheart running.
2. **Deterministic first.** Lifecycle facts (session started, subagent
   finished, permission requested) come from hooks, not from the model
   remembering to report. The model reports only what only it knows (meaning,
   handoffs, questions).
3. **Cheap for agents.** Few tools, short outputs, idempotent calls. Every
   token Flashheart costs an agent must buy visible value.
4. **Fail open.** Flashheart failing must never break an agent session.
5. **Local and private.** No network, no telemetry, no accounts. Hooks record
   names and short summaries, never full tool inputs or outputs.
6. **Agents are untrusted writers.** Ticket text from one agent is data for the
   next, never instructions to it.
7. **Humans stay in charge.** Agents propose and record; moves to review and
   done remain visible, reversible and attributable.

## 4. Concepts

| Term | Meaning |
|---|---|
| **Root** | The board directory, default `~/reports/Kanban` (`FLASHHEART_ROOT`, `--root`). |
| **Project** | A subdirectory of the root, normally one per repository, named after the repository's main checkout. |
| **Ticket** | One unit of work with a stable id such as `FH-42`: a folder `tickets/FH-42-<slug>/` holding the ticket file `FH-42-<slug>.md`, its review and copied files. |
| **Ticket id** | `<project key>-<number>`, unique across the root; how humans and agents refer to tickets (`KEY`). |
| **Column** | A ticket's `status`: Backlog, Up next, In progress, Ready to review, Done. |
| **Workstream** | An ordered group of tickets with a shared goal; order implies blocking. |
| **Review** | A human verification guide for a ticket in `reviews/`. |
| **Attachment** | A file (screenshot, log, PDF) copied into the ticket's `files/` folder. |
| **Run** | One agent session or subagent, from start to end. Runs live in the event log, not in tickets. |
| **Event** | One append-only record in a project's event log (session started, plan updated, checkpoint …). |
| **Claim** | A run's lease on a ticket, renewed by its activity. |
| **Handoff** | The `## Handoff` section of a ticket: done, next, files, open questions. Rewritten at each checkpoint. |
| **Question** | A run's request for a human decision, answer or review; it puts the run in **Needs you**. |
| **Virtual column** | A board column computed from run state (for example *Needs you*) rather than a directory; can be shown or hidden. |

## 5. System overview

```text
 Claude Code / Codex session ──hooks──▶ flashheart hook <agent> <event>  ─┐
            │                                                              │ append events,
            └──MCP (stdio)──▶ flashheart mcp ── tools ───────────────────┤ update tickets
                                                                           ▼  (lock + atomic write)
 Editor ─────────────────────────── edits ──────────────────────▶  ~/reports/Kanban/<project>/…
                                                                           ▲
 Browser ◀──Singleserve (loopback, auth)──▶ flashheart serve ── watch + index ┘
```

One binary, three modes (`D4`). Each mode works directly on the files through
one shared store package; none depends on another mode running. `serve`
watches the root and keeps an in-memory index; the UI is optional.

## 6. Requirements

### 6.1 Projects (`PRJ`)

- `PRJ-1` Every immediate subdirectory of the root that contains a `tickets/`
  directory or a `project.yaml` is a project. Directories starting with
  `.` or `_` are ignored, except the reserved `_scratch` project (`PRJ-4`).
- `PRJ-2` A project's name is its directory name. For a repository, hooks and
  setup derive it from the **main checkout** directory name, so worktrees map
  to the same project:
  `basename(dirname(git rev-parse --path-format=absolute --git-common-dir))`.
- `PRJ-3` `project.yaml` (optional) records a display name, the repository
  paths seen, and per-project settings. When a second, different repository
  path would map to an existing name, the new one becomes
  `<name>-<first 6 hex of sha256(path)>` and both are shown with their paths.
- `PRJ-4` Agent activity outside any git repository goes to `_scratch`.
- `PRJ-5` A project directory is created on first agent contact when
  `auto_create_projects` is on (default on), with a `project.yaml` holding its
  name and repository but no key yet (`KEY-5`). A project can be archived (moved to
  `<root>/.archive/`) from the UI; Flashheart never deletes a project.
- `PRJ-6` The UI lists projects with counts per column, active runs and a
  **Needs you** count, sorted by most recent activity; **All projects** shows
  every project's tickets and runs together.

### 6.2 Storage (`STO`)

Full format: `docs/dev/specs/board-format.md`.

- `STO-1` Boards use format v2 (`board-format.md`, D15): one folder per
  ticket with the column in frontmatter. Flashheart owns the format; it stays
  plain markdown readable in any editor. Format v1 (kanban-tracker column
  folders) is read only to migrate it (`MIG-1`).
- `STO-2` Ticket and workstream frontmatter edits preserve key order, comments
  and unknown keys, and produce minimal diffs.
- `STO-3` All writes are atomic (write temp, fsync, rename) and made under the
  project lock. Edits from the UI and MCP carry the content hash they were
  based on and fail with a conflict if the file changed since.
- `STO-4` A ticket whose frontmatter cannot be parsed is shown as a
  *needs repair* card with the parse error, never hidden or rewritten
  automatically.
- `STO-5` Runs, events, claims and questions are stored in the project's
  append-only event log (`.flashheart/events/YYYY-MM-DD.jsonl`), rotated daily,
  retained for 90 days by default.
- `STO-6` Durable meaning lands in the ticket, not only the log: handoffs are
  written into the ticket body (`## Handoff`), answered questions into
  `## Notes`. A ticket is understandable with the log deleted.
- `STO-7` External edits (editors, agents editing files directly) are
  detected by file watching and reflected in the UI within one second.
- `MIG-1` `flashheart migrate` converts a v1 root to v2: it shows the plan
  and writes only with `--write`, numbers tickets once in creation order,
  rewrites references to ids, and moves (never deletes) the v1 tree into
  `.flashheart/backup/`. `serve` on a v1 root shows the command and writes
  nothing.

### 6.3 Ticket ids (`KEY`)

- `KEY-1` Every project has a key (2–10 characters, uppercase letter then
  uppercase letters or digits) recorded in `project.yaml` and unique across
  the root. A project without one shows a key derived from its name.
- `KEY-2` Every ticket has an id `<key>-<number>`, assigned under the project
  lock at creation from `next_id`, never reused or renumbered.
- `KEY-3` Ids are how tickets are named everywhere: cards, the card panel,
  search, URLs, events, recovery notes, MCP tool arguments and outputs, and
  `depends-on` and workstream lists. A bare id in ticket or review markdown
  links to that ticket.
- `KEY-4` Agents can be told "tackle the three top-priority bugs": the MCP
  server knows the caller's project (PRJ-2) and its key, and `list_tickets`
  returns open tickets filtered by type, status, priority or text, highest
  priority first (`MCP-7`).
- `KEY-5` A key is chosen before the project's first ticket and then fixed.
  Agents choose it, because they know what the project is called in
  conversation: `board_context` says when a project has no key and suggests
  one, and the agent sets 2–5 letters with `set_project_key` or with
  `project_key` on its first `create_ticket`. The UI can set it too. A
  ticket created with no key chosen records the derived key, adding a digit
  when it is taken. Keys are assigned under the root lock, so two new
  projects never get the same key; a taken or invalid key is refused with
  the keys in use.

### 6.4 Views (`VIEW`)

- `VIEW-1` **Board**: one column per status in workflow order (Backlog, Up
  next, In progress, Ready to review, Done).
  *done* shows the most recent 20 by default with **Show all**.
- `VIEW-2` **Virtual columns**: *Needs you* (tickets with a run in Needs you or
  an open question) and *Agent working* (tickets claimed by a live run). Each
  can be shown or hidden; a ticket in a virtual column also stays in its real
  column, marked as mirrored.
- `VIEW-3` **Agents**: one row per run, grouped into lanes by run state
  (`RUN-3`): *Working*, *Needs you*, *Waiting*, *Quiet*, *Ended*. Subagents
  nest under their parent. Ended runs older than 24 hours are hidden by
  default.
- `VIEW-4` **Workstreams**: swimlanes in workstream order, each showing its
  tickets in ticket order with status, blocked state and progress.
- `VIEW-5` **Table**: sortable, filterable list of tickets with chosen fields.
- `VIEW-6` Card density: *Compact* (title, type, priority), *Normal* (+
  workstream, blocked, live badge, criteria progress), *Detailed* (+
  description excerpt, handoff "next", attachments count). Every card shows
  its id and its workstream's line as a stripe and bullet. **Colour by**
  (type by default, priority, age or none) tints the card header and names
  the value in a tag, with a colour key on the board; the colour is never the
  only carrier of meaning and never uses the workstream line colours.
- `VIEW-7` Filters and search across title, slug, body, tags, type, priority,
  workstream, blocked/unblocked, has-run, has-question, `later-possibility`.
  Filters and view choices are remembered per project (`CFG-2`).
- `VIEW-8` Live badge on a card with a live run: agent (Claude/Codex),
  current plan step ("3/7 · Fix header"), state, and time since last activity.

### 6.5 Card detail (`CARD`)

- `CARD-1` Opening a card shows a panel beside the board (covering the view
  only on narrow screens); the board narrows and scrolls so the card's column
  stays in view. Its tabs: **Ticket** (rendered
  markdown), **Edit** (frontmatter form and raw markdown), **Runs** (timeline
  and subagent tree), **Attachments**, **Review** (when a review file exists).
- `CARD-2` Markdown renders GitHub-flavoured markdown (tables, task lists,
  code). Raw HTML is not rendered. Ticket ids in text (`FH-42`) and relative
  links into another ticket's folder open that ticket.
- `CARD-3` Acceptance-criteria checkboxes can be ticked in the rendered view
  and are saved to the file.
- `CARD-4` A **Blocked by** section explains each blocking reason: ticket
  dependency, workstream dependency, or earlier ticket in the workstream.
- `CARD-5` The **Handoff** section is shown prominently with its author run
  and time, and a warning when the claimed run has edited files since.
- `CARD-6` Open questions show with an answer box or option buttons
  (`RUN-8`).

### 6.6 Editing (`EDIT`)

- `EDIT-1` Drag and drop moves a ticket between columns (edits its `status`).
  Every drag action has a keyboard and menu alternative (**Move to …**).
- `EDIT-2` Moving a blocked ticket into *in-progress* asks for confirmation
  and records the override reason in `## Notes`.
- `EDIT-3` Moving into *Ready to review* warns when the review file is
  missing or criteria are unticked; it does not prevent the move.
- `EDIT-4` Reordering tickets inside a workstream swimlane rewrites the
  workstream's `tickets:` list.
- `EDIT-5` **New ticket** creates a ticket folder from the template with the
  next id (`KEY-2`), a slug from the title, `created`, a chosen column
  (default Backlog) and optional workstream.
- `EDIT-6` Frontmatter edits use typed controls for known fields (enums,
  dates, lists) and a raw editor for the whole file.
- `EDIT-7` A save conflict (`STO-3`) shows both versions and lets the user
  reload or overwrite deliberately.
- `EDIT-8` Flashheart never deletes a ticket. **Archive** moves its folder
  to `<project>/.archive/tickets/` and can be undone.

### 6.7 Runs (`RUN`)

Full model: `docs/dev/specs/agent-protocol.md` §2–§4.

- `RUN-1` A run is created from the first hook event of a session or
  subagent, keyed `<agent>:<session-id>` or `<agent>:<session-id>/<agent-id>`.
- `RUN-2` A run records agent, kind (session/subagent), parent, project, cwd,
  branch, worktree, linked ticket and how it was linked (claim or branch
  match), plan, start, last activity, end and end reason.
- `RUN-3` Run state is derived from events: *Working* (turn in progress),
  *Needs you* (permission request, open question or review requested),
  *Waiting* (turn finished, session open), *Quiet* (working but no event for
  the quiet threshold, default 10 minutes), *Ended*. *Ended* runs whose linked
  ticket has edits since its last checkpoint are flagged **no handoff**.
- `RUN-4` A run's **plan** mirrors the agent's own task list (Claude Code's
  task/todo tools, Codex's plan tool) from hook events, without model effort.
- `RUN-5` A run links to a ticket by an explicit claim, or provisionally when
  exactly one *in-progress* ticket has `branch:` equal to the run's branch.
  Subagent runs inherit their parent's link. Unlinked runs show under
  *Unassigned* in the Agents view.
- `RUN-6` A claim is a lease renewed by any event from the claiming run. A
  ticket held by a live claim cannot be claimed by another run without
  `force` and a reason, which both runs' tickets record.
- `RUN-7` A checkpoint rewrites the ticket's `## Handoff` and is the unit of
  recovery.
- `RUN-8` A question (`ask_human`) has a kind (question, decision, review,
  blocked), text, and optional options. Answering it in the UI records the
  answer in the ticket and delivers it to the run at its next prompt or start
  (`HOOK-5`).

### 6.8 Hooks (`HOOK`)

Mappings per agent: `docs/dev/specs/agent-protocol.md` §5.

- `HOOK-1` `flashheart hook <agent> <event>` reads the agent's hook JSON on
  stdin, appends normalised events, and exits 0 within 100 ms (p95 50 ms) on
  a warm cache. Errors go to `<root>/.flashheart/hook-errors.log`, never to the
  agent, and never change the exit status (except `HOOK-6`).
- `HOOK-2` Hooks record tool names, edited file paths relative to the
  repository, success or failure, and summaries of at most 120 characters,
  scrubbed of likely secrets. They never record prompts, command lines, tool
  inputs or tool outputs.
- `HOOK-3` On session start and resume (including after compaction), the hook
  returns a **recovery note** as additional context: the run id, project,
  linked ticket, its handoff, and answered questions, in at most about 400
  tokens.
- `HOOK-4` Permission requests and notifications that the agent is waiting
  for permission put the run in *Needs you*; the next tool result or prompt
  clears it.
- `HOOK-5` On prompt submit, the hook adds answers to the run's questions that
  arrived since its last turn.
- `HOOK-6` Optional, per project (`enforce_handoff`): at turn end, if the run
  edited files since its last checkpoint and is linked to a ticket, the hook
  blocks the stop once with a reason asking for a checkpoint. It never blocks
  twice in a row.
- `HOOK-7` Claude Code (CLI, IDE and desktop Code tab) and Codex (CLI, IDE and
  app) are first-class. Each agent has an isolated adapter so changes in one
  agent's hook schema touch one package.

### 6.9 MCP server (`MCP`)

Tool contracts: `docs/dev/specs/agent-protocol.md` §7.

- `MCP-1` `flashheart mcp` is a stdio MCP server built on the official
  `modelcontextprotocol/go-sdk`.
- `MCP-2` Tools: `board_context`, `list_tickets`, `get_ticket`, `claim`,
  `release`, `checkpoint`, `update_ticket`, `move`, `create_ticket`,
  `set_project_key`,
  `write_review`, `attach`, `ask_human`. Tickets are named by id. No delete
  tool exists.
- `MCP-3` Tool outputs are compact text designed for model context;
  `board_context` stays under about 1,500 tokens.
- `MCP-4` Each call is attributed to a run: from a run id stamped into the
  call by a hook where the agent supports it, else the `run` argument given in
  the recovery note, else the unique live run in the server's working
  directory and branch. Ambiguity is an error that names the fix.
- `MCP-5` Calls are idempotent where it makes sense (claiming a ticket you
  hold, writing the same checkpoint) and validate inputs with clear,
  actionable errors.
- `MCP-6` Remote MCP (for example ordinary ChatGPT chat connectors) is out of
  scope; it would require exposing Flashheart beyond the machine.
- `MCP-7` `list_tickets` filters by project (default: the caller's), type,
  status, priority, tag, blocked or not, and text; returns open tickets
  (Backlog, Up next, In progress) by default, ordered by status (In progress,
  Up next, Backlog), then priority, then age, with a limit; each row is id,
  title, type, priority, status and blocked reason.

### 6.10 Setup and the protocol skill (`SET`)

- `SET-1` `flashheart setup claude|codex` shows the exact configuration
  changes (hooks, MCP server registration, protocol instructions) as a diff
  and writes nothing. `--write` applies them with timestamped backups.
  `--uninstall` reverses them.
- `SET-2` For Claude, setup installs the **Flashheart protocol skill**
  (`~/.claude/skills/flashheart/SKILL.md`), which replaces the kanban-tracker
  skill: tickets are created, read and moved through Flashheart, never by
  writing board files directly. For Codex it
  adds the same protocol text to the user's global `AGENTS.md` between
  Flashheart markers.
- `SET-3` Configuration uses the absolute path of the running binary and
  passes `--root` only when it differs from the default.
- `SET-4` `flashheart doctor` checks the root, permissions, the installed hook
  and MCP configuration for each agent, and recent hook errors.

### 6.11 Review and attachments (`REV`)

- `REV-1` `attach` copies a file into the ticket's `files/` with a caption
  and kind (screenshot, log, other); it never links or moves the original.
- `REV-2` Allowed types: PNG, JPEG, GIF, WebP, PDF, plain text, markdown,
  JSON and log files, at most 20 MB each. SVG and HTML are refused.
- `REV-3` The Review tab shows the review file beside the ticket's
  screenshots and the *How to Verify* steps as a checklist.
- `REV-4` `write_review` creates or replaces the ticket's `review.md` in the
  review template.
- `REV-5` Files an agent refers to are copied into the ticket before they
  are recorded, because agents and their tools clean up their own files:
  local paths in `write_review`, `checkpoint` and `attach` input, and in
  markdown links or images written into a ticket or review, are copied into
  `files/` (allow-listed types, size limit, `REV-2`) and the references are
  rewritten to the copies. A path that cannot be copied stays as text with a
  warning.

### 6.12 Settings (`CFG`)

- `CFG-1` Settings live in `<root>/.flashheart/config.yaml` (global) and
  `project.yaml` (per project): quiet threshold, event retention, done-column
  limit, `auto_create_projects`, `enforce_handoff`, attachment limits.
- `CFG-2` UI preferences (theme, density, visible virtual columns, filters)
  are saved through the backend in the global config, never in browser
  storage.

### 6.13 Lifecycle (`LIFE`)

- `LIFE-1` `flashheart serve` follows the Singleserve consumer checklist:
  `BrowserBoundLifetime`, loopback only, browser bootstrap, `session.fetch`,
  visible backend status, guarded quit, backend-loss and terminal states.
- `LIFE-2` A shutdown guard denies quit while a save is in flight.
- `LIFE-3` Live updates use authenticated long-polling on a monotonically
  increasing revision; the store does not depend on the transport.
- `LIFE-4` `flashheart` (`serve`) runs the server detached from the terminal:
  it waits until the server is listening and the browser launch has been
  attempted, prints the manual URL once to stderr if the browser could not be
  opened, and returns the terminal with exit 0. The server then lives until
  Quit or its last tab closes (`LIFE-1`). Its later diagnostics go to
  `<root>/.flashheart/serve.log`, created only when something is logged.
  `--foreground` keeps it attached until it stops.

### 6.14 CLI (`CLI`)

- `CLI-1` Commands: `serve` (default), `mcp`, `hook`, `setup`, `doctor`,
  `migrate`, `version`. Global `--root`. `--help` on every command.
- `CLI-2` Normal startup prints nothing but errors; `--debug` adds the
  listener address and (with `--foreground`) lifecycle summaries, never
  credentials.
- `CLI-3` `hook` and `mcp` write nothing to stdout except their protocol
  output.

### 6.15 Security and privacy (`SEC`)

- `SEC-1` No network access other than the loopback listener. No update
  checks, telemetry or analytics.
- `SEC-2` Every path from the UI, MCP or a hook is resolved and confirmed to
  be inside the root (or, for `attach` sources, an existing regular file the
  process can read) before use; symlinks out of the root are refused.
- `SEC-3` Stored strings from agents pass through a secret scrubber (API-key,
  token, private-key and password patterns) before being written.
- `SEC-4` Attachments are served with their detected content type,
  `X-Content-Type-Options: nosniff` and a restrictive CSP; markdown renders
  without raw HTML.
- `SEC-5` Agent-written text shown to agents (recovery notes, `board_context`)
  is framed as data, and the protocol tells agents not to follow
  instructions found in tickets.

### 6.16 Non-functional (`NFR`)

- `NFR-1` Scale: 50 projects, 1,000 tickets each, 100,000 events per day
  across the root, without the UI stalling. Initial index under 1 s for 5,000
  tickets.
- `NFR-2` Platforms: macOS (primary) and Linux. Windows is a later decision.
- `NFR-3` Accessibility: keyboard-complete, visible focus, WCAG 2.2 AA
  contrast in light and dark themes, screen-reader names for every control,
  drag and drop with keyboard alternatives.
- `NFR-4` Hooks and MCP use only the store package and the filesystem; they
  start in under 30 ms.

## 7. UX outline

Desktop-first, information-dense, calm when nothing needs attention. The
visual direction is the Transit Line Map (decision D14); `DESIGN.md` records
the system.

- **Shell**: left rail of projects (with Needs you badges) and **All
  projects**; top bar with view switcher (Board · Agents · Workstreams ·
  Table), search, filters, density, virtual-column toggles; quiet backend
  status and Quit.
- **Board**: columns, cards, live badges; card panel slides over from the
  right and keeps the board visible.
- **Agents**: lanes by run state; each run row shows agent, project, ticket,
  plan progress, last activity, and expands to its event timeline and
  subagents.
- **Needs you** is visible from every view and project (badge in the rail and
  top bar).
- Narrow widths keep a single column with a column picker; the board is not
  a phone-first product.

## 8. Non-goals

Launching, hosting or proxying agents; remote access or multi-user sharing;
accounts; a hosted service; telemetry; issue-tracker sync (GitHub Issues,
Jira, Linear); time tracking; replacing git or PRs; editing source code.

## 9. Later possibilities (not committed)

Passive reading of agent session logs for activity without hooks; desktop
notifications for *Needs you*; Windows support; launching an agent on a
ticket; more agents (Gemini CLI, Cursor, others with hooks); GitHub PR status
on cards; read-only sharing of a snapshot; board history from git if the root
is a repository.

## 10. Delivery

Milestones, order and status: [`docs/dev/roadmap.md`](dev/roadmap.md).
Implementation starts from
[`docs/dev/prompts/kickoff.md`](dev/prompts/kickoff.md).

## 11. Decisions needed

- Whether the root should be its own git repository by default (history for
  the board) or left to the user.
- Windows support and timing.
- Default quiet threshold and event retention after real use.
