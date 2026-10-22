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
own apps. It does not host, proxy or supervise agents (`D1`); it may start an
agent's own app or command-line tool on a ticket, which then owns the session
(`D48`, not yet built).

## 2. Users and jobs

The primary user is one developer running several AI coding sessions across
several repositories, often in parallel and often unattended.

| Job | Today | With Flashheart |
|---|---|---|
| "What is going on across my projects?" | Click through chat tabs and terminals | One board, every project, live agent state |
| "Which session needs me?" | Notice a stalled tab by chance | A **Needs you** lane and badge across all projects |
| "My session died; where was it?" | Re-read history, re-explain | The ticket's handoff; the next session is told on start |
| "What did the agent actually do?" | Scroll a transcript | Ticked criteria, checkpoints, files, screenshots and review guide on the card |
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
| **Column** | A ticket's `status`: Backlog, Up next, In progress, Ready to review, Done. A `rejected` ticket (closed without being accepted) is in Done and says Rejected (FH-58). |
| **Workstream** | A group of tickets with a shared goal, like an epic, worked on in sequence or in parallel; only `depends-on` orders its tickets (D26). |
| **Review** | A human verification guide for a ticket in `reviews/`. |
| **Attachment** | A file (screenshot, log, PDF) copied into the ticket's `files/` folder. |
| **Run** | One agent session or subagent, from start to end. Runs live in the event log, not in tickets. |
| **Event** | One append-only record in a project's event log (session started, permission requested, checkpoint …). |
| **Claim** | A run's lease on a ticket, renewed by its activity. |
| **Handoff** | The `## Handoff` section of a ticket: done, next, files, open questions. Rewritten at each checkpoint. |
| **Question** | A run's request for a human decision, answer or review; it puts the run in **Needs you**. |

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
  `.` or `_` are ignored.
- `PRJ-2` A project's name is its directory name. For a repository, hooks and
  setup derive it from the **main checkout** directory name, so worktrees map
  to the same project:
  `basename(dirname(git rev-parse --path-format=absolute --git-common-dir))`.
- `PRJ-3` `project.yaml` (optional) records a display name, the repository
  paths seen, per-project settings and, once the user sets one from the
  board, the project's image: a JPEG or PNG the user uploads, judged by
  its bytes (`SEC-7`), cropped to a centred square, resized to 128×128 and
  stored as a PNG in the project folder, shown on the project's rail badge
  and in the page header in place of its key (the key stays in the badge's
  name and beside the page title). A missing or unreadable image is a
  project warning and the key shows. When a second, different repository
  path would map to an existing name, the new one becomes
  `<name>-<first 6 hex of sha256(path)>` and both are shown with their paths.
- `PRJ-4` Agent activity outside any git repository belongs to no project
  and is not recorded: hooks write nothing and print nothing, MCP read tools
  report that there is no current project, and MCP tools that need the
  caller's project answer `no_project` (D33). To track a folder, run
  `git init` there.
- `PRJ-5` A project directory is created on first agent contact when
  `auto_create_projects` is on (default on), with a `project.yaml` holding its
  name and repository but no key yet (`KEY-5`). A project can be archived
  from the UI (moved to `<root>/.archive/<project>/`, after a confirmation
  that warns about live runs, tickets being worked on and open questions)
  and restored, unless a live project has its name. While archived its
  tickets count as done for other projects, its key stays taken, and agent
  activity in its repositories is not recorded and does not recreate it:
  hooks stay quiet and MCP tools answer `project_archived`. A project is
  deleted permanently only once archived, with its name typed: the delete
  removes other projects' `depends-on` references to its tickets, retires
  its key and removes the directory. After a delete its repositories are
  unknown again, so new activity there starts a new project with a new key.
  Agents have no archive or delete tool.
- `PRJ-6` The UI lists projects with counts per column, active runs and a
  **Needs you** count, sorted by most recent activity; **All projects** shows
  every project's tickets and runs together.

### 6.2 Storage (`STO`)

Full format: `docs/dev/specs/board-format.md`.

- `STO-1` Boards use format v2 (`board-format.md`, D15): one folder per
  ticket with the column in frontmatter. Flashheart owns the format; it stays
  plain markdown readable in any editor. Format v1 (column folders) is
  not read; its one-off conversion is retired (D55).
- `STO-2` Ticket and workstream frontmatter edits, and edits to
  `project.yaml`, preserve key order, comments, unknown keys, line endings
  and a byte-order mark, and produce minimal diffs.
- `STO-3` All writes are atomic (write temp, fsync, rename) and made under the
  project lock. The one exception is the hooks' per-session activity state
  (agent-protocol §3), a disposable cache written in place under the lock
  on every tool result. Every write names its precondition: an edit built
  from content the writer read (a form, the raw file, the agent's field
  changes) carries that content's hash and fails with a conflict if the
  file changed since; a write without a hash is refused. A merge (a move,
  which sets only `status`, a note appended, the Handoff replaced, a
  workstream list reordered, a comment or history entry appended) is
  computed from the file as it is under the lock, so it needs no hash and
  cannot lose another writer's change; a comment's edit carries the hash
  of the activity file it was based on.
- `STO-4` A ticket whose frontmatter cannot be parsed is shown as a
  *needs repair* card with the parse error, never hidden or rewritten
  automatically.
- `STO-5` Runs, events, claims and questions are stored in the project's
  append-only event log (`.flashheart/events/YYYY-MM-DD.jsonl`), rotated daily,
  retained for 90 days by default. A session's inbox (answers, comments
  and change requests waiting for it, D54) expires with its day's events and its hook activity state after `stale_hours`, so a
  session that never reports its end leaves nothing behind (FH-82).
- `STO-6` Durable meaning lands in the ticket, not only the log: handoffs are
  written into the ticket body (`## Handoff`), answered questions into
  `## Notes`. A ticket is understandable with the log deleted.
- `STO-7` External edits (editors, agents editing files directly) are
  detected by file watching and reflected in the UI within one second.
- `STO-8` Each ticket's comments and history live in its folder's
  `activity.md` (board-format §Activity, D47), one dated, typed entry
  each, kept as long as the ticket. People add and edit comments on the
  board (edits shown as edited; each has a state for change requests:
  open, change requested, addressed, resolved, `REV-6`); agents never
  write comments and change only a request's state, to addressed. A
  comment sent to a session (`CARD-7`) names its run and, once handed
  over, when; it can no longer be edited after that. A person deletes a
  comment posted by mistake, on the hash of the activity they read,
  until a session has been handed it, and a change request only once it
  is resolved (FH-140); its entry leaves the file whole. Every
  ticket write by the board or an agent (create, move, field and raw
  edits, ticked criteria and review steps, archive, restore, comment
  states, a deleted comment) appends one history entry in the same
  locked write: who, what changed and when, never the ticket's text. A
  ticket without the file has no activity.

### 6.3 Ticket ids (`KEY`)

- `KEY-1` Every project has a key (2–10 characters, uppercase letter then
  uppercase letters or digits) recorded in `project.yaml` and unique across
  the root. A project without one shows a key derived from its name.
- `KEY-2` Every ticket has an id `<key>-<number>`, assigned under the project
  lock at creation from `next_id`, never reused or renumbered. A permanently
  deleted ticket's id is recorded as retired in `project.yaml` (`EDIT-8`) so
  it is not reused even if `next_id` is lost.
- `KEY-3` Ids are how tickets are named everywhere: cards, the card panel,
  search, URLs, events, recovery notes, MCP tool arguments and outputs, and
  `depends-on` and workstream lists. Wherever the board shows a ticket's id
  (cards, table rows, workstream stations, the Overview, the archive, the
  card panel), the id is a link to the ticket's full page (`CARD-7`) that
  opens in a new tab. A bare id of a known project key in ticket or review
  markdown, and in criteria, handoff next steps and blocking reasons, links
  there too (D30).
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
  the keys in use. Archived projects keep their keys, and a permanently
  deleted project's key is retired (`<root>/.flashheart/retired.yaml`), so
  no other project takes it and its ids are never reused.

### 6.4 Views (`VIEW`)

- `VIEW-1` **Board**: one column per status in workflow order (Backlog, Up
  next, In progress, Ready to review, Done), each in its manual order
  (`EDIT-9`). *done* shows the most recent 20 by default with **Show all**.
  The Backlog can be hidden from the Board (shown by default; FH-41).
- `VIEW-2` **Needs you filter**: the band's *Needs you* plate is a toggle.
  Pressed, it shows *All projects* (the Board, or the Table when open) with
  only tickets that need you: a linked run in Needs you whose reason is
  about the ticket (its permission prompt, or a question about the ticket
  or about no ticket), or an open question about the ticket from a live
  session, whoever asked it. A run that asks about another ticket needs
  you on that ticket's card, which words the question's reason; its own
  card shows it as live, waiting for the answer, so the cards shown agree
  with the plate's count of runs (FH-185). Pressed again, it returns to
  the project it was pressed in. It
  combines with the other filters, carries from project to project, and is
  not remembered. While it is on, runs in Needs you that no card on the
  board shows (no linked ticket, and no open question about a ticket on the
  board) are named, with why no card can show them (they asked about a
  project or workstream, not a ticket), and **Answer it in the Overview**
  opens the Overview at *Needs your decision* (FH-157, D74): a notice above
  the cards, or in place of the empty state when no card is left, which
  never advises clearing a filter that cannot show them. Tickets show only in their real
  columns (D31, FH-44). Tickets linked to a Working or Quiet run are found
  with the State filter's *Agent working* option (D27, FH-42).
- `VIEW-3` **Overview** (D32, FH-51): what a project manager asks of the
  board, for *All projects* or one project, about tickets with agent
  sessions as evidence. Under the title, the pipeline (FH-63): every board
  column in order (Backlog, Up next, In progress, Ready to review, Done)
  with its whole count in the scope, each leading to the Board. Done counts
  accepted work: Rejected tickets (`FH-58`) sit in the Done column but are
  left out of it, and archived tickets are not counted.
  The other stages are what sits there now, so they agree with the
  sections below. One column of section cards, in order: *Needs your
  decision* (open questions, answered in place as on the card, `CARD-6`,
  and kept until the session takes the answer; permission prompts, which
  name the project and branch of the session to answer them in, since
  Flashheart never grants permissions; runs with no ticket on the board
  included), *Ready for your review* (tickets in Ready to review, longest
  waiting first, with criteria met and **Review results** opening the
  ticket's Review tab), *At risk* (in-progress tickets no live session
  works on or whose session has gone quiet, tickets whose session ended
  without a handoff, and the top of a project's Up next when blocked, each
  reason in words), *In progress* (the session's state and its subagents
  in words, "subagents 3 done · 2 running", criteria and last activity),
  then two collapsed tiles: *Work with no ticket* (live sessions linked to
  no ticket, with project, branch, state and **Create ticket**) and *Up
  next* (the top of each project's Up next, blocked or ready). At risk and
  In progress start open when they have something; every section has a
  one-line empty state. Run detail (tools, edited files, subagents)
  stays on the card's Runs tab. The Agents view it replaced is gone; its
  old routes and a remembered Agents view open the Overview.
- `VIEW-4` **Workstreams**: swimlanes in workstream order, each showing its
  tickets as a railway graph of their `depends-on` links, with status,
  blocked state and progress. A finished workstream (every ticket Done,
  Rejected or Archived) shows collapsed to its head until the user expands
  it; any other workstream always shows in full. A workstream's name opens
  its page (`EDIT-13`). **New workstream** in its header makes one
  (`EDIT-14`).
- `VIEW-5` **Table**: sortable, filterable list of tickets with chosen fields.
- `VIEW-6` Card density, one set of markers in three layouts (FH-147):
  *Compact* (the title led by its id, with only Needs you, blocked, change
  requests and priority beside it), *Normal* (the title, one meta line of
  type, id, blocked, change requests, live run, age, review, attachments
  and priority, and a Needs you reason on its own line), *Detailed* (+
  blocker reason, run state and agent, handoff "next", description excerpt,
  workstream name). Every card shows its id and its workstream's line as a
  stripe, and, at every density, its change requests still open or
  addressed (`REV-6`) as a count; its accessible name says every marker.
  Priority is a three-bar marker and type an icon in its colour. **Colour
  by** (type by default, priority, age or none) tints the whole card, with
  colour chips on the board that also filter (type chips carry the type
  icons); the colour is never the only carrier of meaning and never uses
  the workstream line colours.
- `VIEW-7` Filters and search across title, slug, body, tags, type, priority,
  workstream, state (blocked, not blocked, needs repair, agent working or
  rejected, FH-59), has-run, has-question. Type, priority,
  workstream and the Colour by value can each show only some values or hide
  some (FH-39). Filters and view choices are remembered per project
  (`CFG-2`).
- `VIEW-8` Live run on a card: its state mark on Normal and Detailed cards,
  with the agent (Claude/Codex) and state in its tooltip and the card's
  name; a Detailed card also says the state, agent and time since last
  activity. The card's Needs you reason is derived once on the server
  with the run's (D39): the linked run's own when it is about the ticket,
  else the oldest open question about the ticket (`VIEW-2`, FH-185). A
  card never shows a plan: a ticket's progress is its ticked criteria and
  its handoff's next steps (D51).

### 6.5 Card detail (`CARD`)

- `CARD-1` Opening a card shows a panel beside the board (covering the view
  only on narrow screens); the board narrows and scrolls so the card's column
  stays in view. Its tabs: **Ticket** (rendered
  markdown), **Comments** (people's comments and the full page's comment
  box, `CARD-7`; when the ticket has comments or can be edited), **Edit**
  (frontmatter form and raw markdown), **Runs** (timeline and subagent
  tree), **Attachments** (when the ticket has files or can be edited,
  where files are dropped or attached, `REV-7`), **Review** (when a
  review file exists).
- `CARD-2` Markdown renders GitHub-flavoured markdown (tables, task lists,
  code). Raw HTML is not rendered. Ticket ids in text (`FH-42`) and relative
  links into another ticket's folder open that ticket's full page
  (`CARD-7`); links to a ticket's files (`files/…`) open the file in a new
  tab (`SEC-4`).
- `CARD-3` Acceptance-criteria checkboxes can be ticked in the rendered view
  and in the editor (`EDIT-11`, `EDIT-12`) and are saved to the file.
- `CARD-4` A **Blocked by** section explains each blocking reason: ticket
  dependency or workstream dependency.
- `CARD-5` The **Handoff** section is shown prominently with its author run
  and time, and a warning when the claimed run has edited files since.
- `CARD-6` Open questions show with an answer box or option buttons
  (`RUN-8`).
- `CARD-7` Every ticket has a full page, `#/ticket/<id>`, reachable by URL,
  reload or a new tab, under the band, the browser tab named
  `<id> · <title>`, with a link to the ticket beside its project's board.
  It shows the whole ticket as one continuous page, with no tabs: needs
  repair, change requests still open or addressed (`REV-6`, with Resolve
  and Reopen), questions waiting for the user, Handoff, Blocked by, Acceptance
  criteria, Description, Test plan (or Reproduction), Context and the
  ticket's other sections; then, when `review.md` exists, the review in its
  own sections, its *Evidence* screenshots as one captioned gallery that
  opens the lightbox; then **Files**, the attachments neither the review nor
  the ticket links, so each screenshot shows once (shown, even empty,
  where the ticket can be edited: files are dropped or attached there,
  `REV-7`); then **Activity**, a
  timeline, oldest first, of people's comments, agents' notes, the
  ticket's history (`STO-8`; moves the runs recorded fill in only before
  the history begins), agents' questions with their answers, and handoff
  updates, filtered **All**, **Comments** or **History** (the outline's
  count follows the filter), ending with a comment box: a person posts a
  comment with **Comment** or Ctrl or Cmd and Enter (never an empty one;
  a failed post keeps the text with the reason beside it), optionally
  quoting a section, criterion or How to verify step chosen from the
  page, shown as a quote in the comment. Per comment, **Tell a session
  now** also sends it, as the user's own words, to a live session that
  has claimed the ticket, chosen from those sessions with their subagents
  under them; with none the option is off and says why, and nothing is
  ever queued for a later session (D47, FH-127). The session gets it
  through its inbox with its next prompt, Flashheart tool result,
  `board_context` or `flashheart await` (`HOOK-5`); the comment shows
  *Delivered to <run> at <time>* and is then locked, or says it was not
  delivered when the session ended first, staying on the ticket. A
  person's comment edits in
  place (`EDIT-11`) until it is delivered and then shows as edited, and shows its state when it
  is a change request. Beside its pencil, **Delete comment** asks first
  (**Keep it** has focus) and deletes it, on the page and in the panel's
  Comments tab (FH-140); a comment still waiting for its session is
  withdrawn, so the session never reads it. A comment its session has
  read, or a change request open or addressed, keeps the control, marked
  unavailable, which says why when pressed and as its description. On a ticket in review the properties offer
  **Request changes** (`REV-6`). From 1280px an outline
  of the sections (with counts, empty template sections marked empty, the
  section in view highlighted) sits on the left and the properties (status
  with **Move to …**, priority, type, workstream, branch, pull request,
  depends on, tags, agent, created, changed and **Archive**) on the right,
  both staying in view; narrower, the outline is a **Jump to** menu and,
  on narrow screens, the properties stack above the page. Choosing a
  section moves focus to it. The full page has no Edit tab: its title,
  sections and properties are edited in place (`EDIT-11`). Clicking a
  card, row or station opens the panel; clicking a ticket id opens the
  full page in a new tab (`KEY-3`). Ticket links on a full page stay in
  its tab.

### 6.6 Editing (`EDIT`)

- `EDIT-1` Drag and drop moves a ticket between columns (edits its `status`).
  Every drag action has a keyboard and menu alternative: Shift with an arrow
  moves a focused card to the next column (and a station along its line),
  and the card panel has **Move to …**, which also offers **Rejected**: a
  Done state for a ticket closed without being accepted (FH-58). Each move
  can be undone.
- `EDIT-2` Moving a blocked ticket into *in-progress* asks for confirmation
  and records the override reason in `## Notes` as a dated bullet, the same
  one an agent's forced claim writes (FH-81).
- `EDIT-3` Moving into *Ready to review* warns when the review file is
  missing, when the review shows no evidence (no screenshot and no stated
  reason in its *Evidence* section, `REV-4`), when criteria are unticked,
  or while change requests are open (`REV-6`; the warning names each, by
  its id, its first words and who asked); it does not prevent the move.
- `EDIT-4` Reordering a workstream's tickets that have no dependencies on
  the line rewrites the workstream's `tickets:` list; tickets on the graph
  are placed by their dependencies.
- `EDIT-5` **New ticket** creates a ticket folder from the template with the
  next id (`KEY-2`), a slug from the title, `created`, a chosen column
  (default Backlog) and optional workstream, then opens the new ticket in
  the card panel ready to edit: on its Edit tab, focus in its title
  (FH-17).
- `EDIT-6` Frontmatter edits use typed controls for known fields (enums,
  dates, lists) and a raw editor for the whole file (the card panel's
  Edit tab); the full page edits them in place (`EDIT-11`). Choosing a
  ticket's workstream (or None) adds it to that workstream and takes it
  out of any other: the ticket's `workstream:` and every workstream's
  `tickets:` list change together, in one write under the project lock
  and the content-hash precondition, so a stale ticket changes no file
  (board-format §Multi-file writes).
- `EDIT-7` A save conflict (`STO-3`) shows both versions and lets the user
  reload or overwrite deliberately.
- `EDIT-8` Deleting takes two deliberate steps. **Archive**, the normal
  action, moves a ticket's folder to `<project>/.archive/tickets/` and can be
  undone; a project's **Archive** view lists its archived tickets, searchable,
  and **Restore** returns one to the column it left. Only an archived ticket
  can be **deleted permanently**, from that view: a confirmation lists the
  folder's files and every ticket (`depends-on`) and workstream (`tickets:`)
  that refers to it, and the user types the ticket's id. The delete removes
  the folder and those references in one locked operation, so nothing waits
  on the id, and retires the id (`KEY-2`). It is refused when anything listed
  changed since the confirmation opened. Prose mentions stay as text. Board
  files have no history (D12), so a delete cannot be undone. Agents have no
  delete or archive tool.
- `EDIT-9` **Manual order**: dragging a card within a column, or dropping it
  at a place in another column, sets its place, saved in the ticket's `rank`
  (board-format §Order within a column) so it survives reloads and
  restarts. Ranked tickets come first; unranked ones (new tickets, until
  placed) follow by priority, created date and id. *Done* lists the most
  recently finished first and is not reordered. Places are taken among the
  visible cards of the ticket's own project, so filtered-out tickets and
  other projects in *All projects* keep their order. Shift with Up or Down
  and the panel's **Position** buttons are the keyboard and menu alternatives;
  Undo puts the ticket back after the card it followed; it is offered only
  for the latest move, so two quick steps never leave the first one's Undo
  showing. A temporary display sort (the Table's column sort, or a future
  board sort) rearranges only what is shown and never writes ranks; while
  one is on, cards cannot be reordered within a column and a move keeps the
  ticket's rank. Board order
  is separate from workstream order (`EDIT-4`) and never changes it.
- `EDIT-10` Every board write that changes a ticket (new ticket, move or
  place, field, section, raw and criterion edits, a ticked review step,
  archive, restore, permanent delete, a file added, `REV-7`, recorded as
  the field `files`) or comments on it (STO-8: added, edited, deleted) appends
  one event to the ticket's project log saying that the human changed it,
  and when: the ticket's id and at most the columns or names of the fields
  changed, or the comment's id, never their values or any ticket or
  comment text (agent-protocol §3). With answers to agents' questions
  (`RUN-8`), these are the human's board activity, which the server reads
  per project and across projects. Edits made to ticket files outside the
  board are not recorded and do not count.
- `EDIT-11` **In-place editing** on a ticket's full page (`CARD-7`,
  FH-66): hovering or focusing the title, a section or a property shows a
  pencil named for it ("Edit description"); the pencil, a double-click on
  the content, or Enter on the focused pencil opens that one item, and
  Save, Cancel, Ctrl or Cmd with Enter, and Escape close it, returning
  focus to it. Only one item is open at a time: opening another, or
  leaving the page, while it has unsaved changes asks whether to keep or
  discard them (closing or reloading the tab gets the browser's own
  prompt). A save still waiting for its answer is not unsaved (D70,
  FH-156): opening another item or leaving then asks nothing (closing or
  reloading the tab still gets the browser's prompt until the answer
  arrives). If that save then fails or meets a conflict with its item no
  longer open, a toast says what was not saved and why, and its **Back
  to the edit** returns to where it was made and reopens the item with
  the person's text and the reason, or the conflict dialog; an item
  opened again takes back its text too. The same holds for the card
  panel's Edit tab and its comments, and for a comment being posted.
  The title is one line; properties use typed controls (`EDIT-6`:
  priority and type selects, a workstream picker of the project's
  workstreams, branch and pull request as text, tags as chips, depends on
  as a picker of ticket ids), the status keeps **Move to** and its rules
  (`EDIT-2`); agent, created and changed are read-only. Sections
  (Description, Test plan or Reproduction, Context, the ticket's own) edit
  in the formatted editor (`EDIT-12`); an empty template section shows
  **Add <section>** in its place; the acceptance criteria edit as a list
  (add, reword, tick, remove, reorder; a section with other text edits in
  the formatted editor), and
  ticking one stays one click (`CARD-3`). In Activity a person's comment
  edits as markdown, saved on the hash of the activity file it was read
  from, so an edit over a newer version opens the conflict dialog with
  that comment as it is now; a comment being written is unsaved work until it is posted, so leaving the page asks
  about it too. Handoff, the review, notes and history are not edited. A
  save writes only that field or section, leaving every
  other byte of the file as it was (board-format §Section edits), and an
  untouched save writes nothing; it carries the content hash its edit
  opened on, moved on to a newer version only while that version left the
  item itself unchanged (a section's save that meets such a version it
  had not yet seen is made again on it, once), so a change to the item on
  disk opens the conflict dialog (`EDIT-7`). A failed save keeps the item open with the
  reason beside it. The card panel keeps its Edit tab.
- `EDIT-12` **Formatted editor** (FH-16, D61, D62, D68): a section edits as
  markdown shown formatted, as the view renders it (`CARD-2`), except on
  the lines the cursor or selection is on, where its markdown shows to be
  edited. Headings, emphasis, strikethrough, code, quotes and lists are
  styled with their marks hidden; a task's box is a checkbox that ticks
  it (`CARD-3`); the ticket's own images (`files/`) show, any other image
  as its words; links, relative `files/` links and ticket ids are styled
  and open in a new tab with Ctrl or Cmd and a click, or with Alt and
  Enter on them (on a task's line, Alt and Enter ticks it); tables and
  fenced code are edited as text. Typing `/` at the start of a line or
  after a space opens a menu of blocks (heading and subheading, at levels
  3 and 4 under the section's own, bulleted, numbered and task lists,
  quote, code block, table, divider, link), chosen with the arrow keys and
  Enter; Escape closes only the menu. Selected text shows a toolbar (bold,
  italic, code, link, bulleted list, task list), reached with Tab or Alt
  and F10 and moved along with the arrow keys, Escape returning to the
  text; Ctrl or Cmd with B, I, E or K formats too. A **Formatted** /
  **Markdown** switch shows the markdown as written; switching never
  changes the text, and the choice is remembered (`CFG-2`). The text saved
  is the text edited, never re-written, so an untouched section saves
  nothing and an edit changes only what was typed. A pasted screenshot
  and dropped or chosen files are attached (`REV-7`). Pasted HTML, such
  as a copy from a web page, is inserted as equivalent markdown (FH-142,
  D68): headings (levels 1 and 2 become 3, so none starts a section),
  emphasis, strikethrough, inline and fenced code, links, nested lists,
  task boxes, quotes, tables and rules, blocks on lines of their own.
  It is parsed without loading or running anything; only `http`,
  `https`, `mailto` and relative `files/` links survive, any other
  keeping only its words; a remote image becomes a link to it, any other
  image its alt text; any other element gives its text and no raw HTML
  is written. A copy with nothing markdown keeps, and any paste after
  Ctrl or Cmd with Shift and V, inserts plain text. Raw HTML in the text
  shows as text and is never rendered, and the editor fetches nothing
  but the ticket's own images. Tab leaves the editor. It loads on
  the first edit, so the board's first load does not carry it.
- `EDIT-13` **Workstream page** (FH-139): a workstream's page, opened from
  its name on its route card (`VIEW-4`; beside a finished line's head,
  which is its disclosure), shows its head and line as the route card
  does, then its sections: Goal, Scope, Success Criteria, Notes and any of
  its own, in file order, a missing template section in the template's
  place. Each section edits in place as a ticket's does (`EDIT-11`: the
  pencil, a double-click or Enter; Save, Cancel and their keys; one item
  open at a time; the unsaved-changes prompt) in the formatted editor
  (`EDIT-12`, without attachments, since a workstream has no files), and
  an empty one shows **Add <section>**. Every section is a person's to
  edit, Notes too. A save writes only that section (board-format §Section
  edits), an untouched save writes nothing, a change elsewhere in the
  file saves as on a ticket's page, and a change to the section on disk
  opens the conflict dialog (`EDIT-7`). Its name, the h1, edits in place
  the same way (FH-167, D78): saving a new name renames its slug to the
  one the name makes, as **New workstream** makes it, so members,
  workstream dependencies, the file name and saved filters follow, and
  the page moves to the new address in place of the old one; a name that
  makes the same slug changes only the name, and one whose slug or name
  another workstream has is refused beside the field, which stays open.
  Its properties and tickets list are not edited there.
- `EDIT-14` **New workstream** (FH-17), in the Workstreams view's header,
  creates `workstreams/<slug>.md` from the board-format template: a title
  (unique in the project, ignoring case and spacing, D76; its slug made
  from it, unique in the project), a goal, a priority
  (default medium) and, optionally, the project's workstreams it depends
  on (`depends-on-workstreams`). It starts with no tickets; tickets join
  it from their workstream field (`EDIT-6`). A title another of the
  project's workstreams has is named at the Title field and not sent. A
  refusal (an empty or taken title, a
  dependency that no longer exists) shows in words and writes nothing.
  The new workstream's page (`EDIT-13`) then opens. Like other workstream
  writes it records no event (`EDIT-10` is about tickets).

### 6.7 Runs (`RUN`)

Full model: `docs/dev/specs/agent-protocol.md` §2–§4.

- `RUN-1` A run is created from the first hook event of a session or
  subagent, keyed `<agent>:<session-id>` or `<agent>:<session-id>/<agent-id>`,
  or, for a session that reaches the MCP server without hooks, from its first
  MCP write, keyed `<agent>:mcp-<random>`, or `<agent>:<session-id>` when
  the call names its session (Codex, D45); such a run lasts as long as its
  MCP connection and reports claims, checkpoints, questions and tool calls,
  not turns or edits (D34), until hooks of that session report it.
- `RUN-2` A run records agent, kind (session/subagent), parent, project, cwd,
  branch, worktree, linked ticket and how it was linked (claim or branch
  match), start, last activity, end and end reason.
- `RUN-3` Run state is derived from events: *Working* (turn in progress),
  *Needs you* (permission request, open question or review requested),
  *Waiting* (turn finished, session open), *Quiet* (working but no event for
  the quiet threshold, default 10 minutes), *Ended*. A run in *Needs you*
  has one **reason**, derived with its state and worded the same on its
  card, its Runs tab and the Overview (D39); first match wins: its own
  permission prompt, its own unanswered question, a live subagent's prompt,
  a subagent's unanswered question, then an answer waiting for delivery
  (its own, then a subagent's). *Ended* runs whose linked
  ticket has edits since its last checkpoint (through the edit tools, or
  files the turn-end hook finds changed in the worktree during the run's
  own shell commands, not while it waited) are flagged **no handoff**.
- `RUN-4` Retired (D51, FH-133): runs record no plan. A ticket's plan is
  its acceptance criteria and its handoff's next steps, which agents
  already write; hooks do not read agents' own task lists, and older logs
  that hold plans still read.
- `RUN-5` A run links to a ticket by an explicit claim, or provisionally when
  exactly one *in-progress* ticket in the run's home project (the one it
  works in, never one where it only claimed a ticket) has `branch:` equal
  to the run's branch.
  Subagent runs inherit their parent's link. Unlinked runs show under
  *Work with no ticket* on the Overview while they are live; an unlinked
  session that changes files is asked once to claim a ticket (`HOOK-8`).
- `RUN-6` A claim is a lease renewed by any event from the claiming run. A
  ticket held by a live claim cannot be claimed by another run without
  `force` and a reason, which both runs' tickets record.
- `RUN-7` A checkpoint rewrites the ticket's `## Handoff` and is the unit of
  recovery.
- `RUN-8` A question (`ask_human`) has a kind (question, decision, review,
  blocked), text, and optional options. Answering it in the UI records the
  answer, under the configured `user_name` (else the computer account's
  name), in the ticket and delivers it to the run at its next prompt or start
  (`HOOK-5`). A prompt the user types in the asking session before
  answering on the board answers it there: the question stops waiting and
  stays in the run's history marked as answered in the session (D28).
  A prompt the user did not type (a subagent's report, another agent's
  message or a background task's notice) answers nothing (D41). A
  question whose session has ended stays open and
  answerable, marked as waiting for the session to resume; it does not
  need you (`VIEW-2`). A subagent's question waits on its session, so
  after the subagent finishes it still needs you while the session is
  live: the session's reason, the card's question count and the
  Overview agree (FH-112). A board that cannot record answers shows questions
  without an answer form.
- `RUN-9` Each ticket's board data summarises the runs working for it,
  derived from the same events (FH-50): the sessions linked to it, their
  subagents that inherit the link (no claim of their own, or a claim on
  the session's ticket), and subagents that claimed it themselves, each
  counted once. A subagent that claimed another ticket counts only on
  that ticket. The summary names the run that speaks for the ticket (the live one
  that most needs attention, else the most recently active; a subagent
  only when its session is not linked to the ticket), with its agent,
  state and **dirty** / **no handoff** flags (`RUN-3`); the latest
  activity of any of them; and the subagents as counts: done (Ended),
  running and needs you (subagents never wait). An in-progress ticket that
  no live run works on is marked **no live session**, with its last
  activity when it had runs.

### 6.8 Hooks (`HOOK`)

Mappings per agent: `docs/dev/specs/agent-protocol.md` §5.

- `HOOK-1` `flashheart hook <agent> <event>` reads the agent's hook JSON on
  stdin, appends normalised events, and exits 0 within 100 ms (p95 50 ms) on
  a warm cache. Errors go to `<root>/.flashheart/hook-errors.log`, never to the
  agent, and never change the exit status (except `HOOK-6` and `HOOK-8`).
- `HOOK-2` Hooks record how many tools a run used and how many failed, in
  at most one activity record per run per `activity_seconds` (default 60;
  agent-protocol §3), edited file paths relative to the repository (each
  once per run per turn, and again after a checkpoint), the names of tools awaiting
  permission, and summaries of at most 120 characters, scrubbed of likely
  secrets. They never record prompts, command lines, tool inputs or tool
  outputs.
- `HOOK-3` On session start and resume (including after compaction), the hook
  returns a **recovery note** as additional context: the run id, project,
  linked ticket, its handoff, and answered questions, in at most about 400
  tokens.
- `HOOK-4` Permission requests and notifications that the agent is waiting
  for permission put the run in *Needs you*; the next tool result or prompt
  clears it.
- `HOOK-5` On prompt submit, the hook adds what waits in the session's
  inbox since its last turn: answers to its questions, and comments and
  change requests sent to it from the board, each framed as what it is
  (agent-protocol §4, D54). With nothing waiting it costs one file check.
- `HOOK-6` Optional, per project (`enforce_handoff`): at turn end, if the run
  (with its subagents) edited files since its last checkpoint (by it or any
  of them) and is linked to a ticket, the hook
  blocks the stop once with a reason asking for a checkpoint. It never blocks
  twice in a row.
- `HOOK-7` Claude Code (CLI, IDE and desktop Code tab) and Codex (CLI, IDE and
  app) are first-class. Each agent has an isolated adapter so changes in one
  agent's hook schema touch one package.
- `HOOK-8` A session that changed files since its last checkpoint (with
  its subagents, through the edit tools or, as the worktree check finds,
  shell commands) while it holds no ticket, by claim or by branch, is
  asked once to claim one or create one, naming the tools (D50,
  agent-protocol §9). Where the project enforces handoffs (`HOOK-6`) the
  request holds that turn's stop; otherwise it goes as additional context
  with the session's next prompt (and, in Claude Code, its next tool
  result). It is asked at most once per session, never of a subagent, and
  recorded only in the session's activity state.

### 6.9 MCP server (`MCP`)

Tool contracts: `docs/dev/specs/agent-protocol.md` §7.

- `MCP-1` `flashheart mcp` is a stdio MCP server built on the official
  `modelcontextprotocol/go-sdk`.
- `MCP-2` Tools: `board_context`, `list_tickets`, `get_ticket`, `claim`,
  `release`, `checkpoint`, `update_ticket`, `move`, `create_ticket`,
  `set_project_key`, `create_workstream`, `update_workstream`,
  `write_review`, `attach`, `ask_human`. Tickets are named by id. No delete
  tool exists. Setting a ticket's workstream through a tool keeps the
  ticket's field and the workstreams' `tickets:` lists in step.
  `update_workstream` changes a workstream's title (its slug stays), goal,
  priority, tags, workstream dependencies and members in one write, and
  refuses a dependency cycle between workstreams; its `rename_to` renames
  the slug in that write, moving the file and rewriting every ticket and
  workstream that names it (FH-158); `create_workstream` and
  `update_workstream` refuse a title another of the project's workstreams
  has (D76); `create_ticket` files a
  ticket in another project when given its name or key (FH-46).
  `update_ticket` rewrites named sections and edits acceptance criteria
  by operation, keeping ticks, but never Handoff or Notes; the replaced
  text is kept in the ticket's files and noted (FH-69, D75). The
  ticket tools ask agents to write titles and descriptions for a person,
  with implementation detail under Context, and warn about a title that
  reads like code (agent-protocol §7.2); `checkpoint` asks for a handoff
  the person reviewing the ticket can read, and warns about an item that
  reads like code or runs long (FH-187, D87). `get_ticket` and `claim` end
  with the ticket's comments (STO-8), framed as the user's words apart
  from ticket text, the newest kept within each tool's budget.
- `MCP-3` Tool outputs are compact text designed for model context;
  `board_context` stays under about 1,500 tokens.
- `MCP-4` Each call is attributed to a run: from a run id stamped into the
  call by a hook where the agent supports it, else the `run` argument given in
  the recovery note, else the session the call itself names (Codex's
  `_meta`, D45), else the unique live session of the same agent in the
  worktree and branch the agent works in (`CLAUDE_PROJECT_DIR` or the
  server's working directory), else a run the server starts for its
  connection on the first write (`RUN-1`), which stays the connection's
  caller; answers to that run arrive with its next tool result. Ambiguity is an error that names the fix.
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
  changes (hooks, MCP server registration, protocol instructions) as a diff,
  then asks `Apply these changes? [y/N]` in a terminal and applies them,
  with timestamped backups, only on `y` or `yes` (D59). `--yes` (or its
  older name `--write`) applies without asking; `--dry-run`, or no
  terminal to ask in without `--yes`, only shows them. `--uninstall` does
  the same for the reverse, and for Claude also puts back a kanban-tracker
  skill an earlier setup moved into its backup (D55).
- `SET-2` Setup installs the **Flashheart protocol skill** for every
  agent, the same folder for each (D89): `SKILL.md`, a short guide (what
  Flashheart is for, the rules, the session loop, and which page to read
  before each kind of task), and reference pages in `references/` for
  writing tickets and handoffs, finishing and the review, questions and
  comments, and workstreams. Claude Code reads it from
  `~/.claude/skills/flashheart/`, Codex from `~/.agents/skills/flashheart/`.
  It says tickets are created, read and moved through Flashheart, never by
  writing board files directly. Setup also adds a short section between
  Flashheart markers to each agent's global instructions file (Claude
  Code's `~/.claude/CLAUDE.md`, Codex's `AGENTS.md`), read in every
  session, saying that Flashheart tracks the work and to use the skill;
  text outside the markers is the user's, and a symlinked file is edited
  at its target. Each agent's session-start hook keeps the installed skill
  and section current, rewriting what differs from the binary's with the
  user's text around the section untouched (agent-protocol §5.4, D53);
  neither is ever created there, except that Codex's hook writes the skill
  beside a section from before the skill existed.
- `SET-3` Configuration uses the absolute path of the running binary and
  passes `--root` only when it differs from the default.
- `SET-4` `flashheart doctor` checks the root, permissions, the installed hook
  and MCP configuration for each agent, and recent hook errors. It changes
  nothing, reports each check as ok, warning or problem with the fix (a
  missing hook, a configured binary that no longer exists, a missing or
  out-of-date skill file or Flashheart section of the global `CLAUDE.md` or
  `AGENTS.md`, Codex hooks turned off, never trusted or
  trusted for an earlier command (naming the events),
  hook errors of the last day), and exits 1 when it finds a problem.
  An MCP server registered at local or project scope instead of setup's
  user scope is a warning naming the scope, not "not registered".

### 6.11 Review and attachments (`REV`)

- `REV-1` `attach` copies a file into the ticket's `files/` with a caption
  and kind (screenshot, log, other); it never links or moves the original.
  A person adds one from the board (`REV-7`).
- `REV-2` Allowed types: PNG, JPEG, GIF, WebP, PDF, plain text, markdown,
  JSON and log files, at most 20 MB each. SVG and HTML are refused. A
  file from the browser is judged by its content, not its name (`SEC-7`).
- `REV-3` The Review tab shows the review file beside the ticket's
  screenshots and the *How to Verify* steps as a checklist. A tick is
  written into the step's own line in `review.md` as a task box, under the
  content-hash precondition, so the file stays the record.
- `REV-4` `write_review` creates or replaces the ticket's `review.md` in the
  review template, whose *Evidence* section holds captioned screenshots of
  every state a visible change touched, or "No visible change:" and why.
- `REV-5` Files an agent refers to are copied into the ticket before they
  are recorded, because agents and their tools clean up their own files:
  local paths in `write_review`, `checkpoint` and `attach` input, and in
  markdown links or images written into a ticket or review, are copied into
  `files/` (allow-listed types, size limit, `REV-2`) and the references are
  rewritten to the copies. A path that cannot be copied stays as text with a
  warning. `serve` does the same for a ticket or review edited directly (in
  a text editor) while it runs, but only for files in the ticket's project's
  repository (`SEC-6`): links to allow-listed files there are copied in and
  rewritten; a link it cannot copy is logged once. Content Flashheart wrote
  itself, including raw edits sent from the browser, is not acted on, so
  the browser cannot pull local files in.
- `REV-6` **Request changes** (D47, FH-126): on a ticket in review, the
  person asks for changes from its full page, choosing existing comments,
  writing a new one, or both. In one write the chosen comments become
  *change requested*, the new one is added as one, and the ticket moves
  back to In progress with its branch and pull request kept. A live
  session holding the ticket (its claim, agent-protocol §6) gets them with
  its next prompt or Flashheart tool call; otherwise nothing is queued,
  and the next session's recovery note and `board_context` name them from
  the ticket. The agent marks each one *addressed* through `update_ticket`
  (agent-protocol §7.2). The page lists the requests still open or
  addressed at its top with their counts, and the card shows the counts;
  at the next review the person **Resolves** each addressed request or
  **Reopens** it (a reopened request goes to the live holder too). Every
  state change is the ticket's history (STO-8).
- `REV-7` **Files from the board** (FH-18, D65): in a section's formatted
  editor (`EDIT-12`), a pasted screenshot (a paste with a file and no
  text) is stored in the ticket as kind *screenshot*, named
  `<time>-screenshot.png`, and files dropped on the text, or chosen with
  **Attach files**, are stored one at a time in the order given. A
  placeholder, never text, shows where each goes while it uploads, then
  becomes its reference, `![name](files/<file>)` for an image and
  `[name](files/<file>)` for anything else, typed into the text at the
  cursor or the drop. The upload writes only the file and its
  `files/index.yaml` entry, never the ticket: the reference is saved
  with the section (`EDIT-11`), and a file whose edit is cancelled stays
  in the ticket's files. Files dropped on the card panel's
  **Attachments** tab or the full page's **Files**, or chosen with their
  **Attach files**, are stored without touching the text. A file over
  the limit is refused before it is sent; one the server refuses
  (`REV-2`) or that fails leaves nothing behind: its placeholder goes and
  its name and the reason show beside the editor or the files, until
  dismissed. **Attach files** is the keyboard's way to paste and drop.

### 6.12 Settings (`CFG`)

- `CFG-1` Settings live in `<root>/.flashheart/config.yaml` (global) and
  `project.yaml` (per project): quiet threshold, event retention, done-column
  limit, `auto_create_projects`, `enforce_handoff`, attachment limits.
- `CFG-2` UI preferences (theme, density, colour by, hidden columns, each
  project's view and filters, the finished workstreams expanded, and the
  section editor's Formatted or Markdown view, `EDIT-12`) are
  saved through the backend in the global config, never in browser storage.
  A change is saved shortly after it is made, and at once when the page is
  hidden or closed, so a reload does not lose it. The server answers a
  preference read only after any save it is writing, so a page reloaded
  straight after a change shows it. A save names only the preferences
  changed, and an open page reads the saved ones again as the board
  changes, taking them when nothing of its own is waiting, so a change
  made elsewhere (an agent's workstream rename) is neither undone nor
  missed (FH-173, D80).

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

- `CLI-1` Commands: `serve` (default), `mcp`, `hook`, `await`, `setup`,
  `doctor`, `version`. Global `--root`. `--help` on every command.
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
  token, private-key and password patterns) before being written, and so
  do comments, which agents read (STO-8).
- `SEC-4` Attachments are served with their detected content type,
  `X-Content-Type-Options: nosniff` and a restrictive CSP; markdown renders
  without raw HTML.
- `SEC-5` Agent-written text shown to agents (recovery notes, `board_context`)
  is framed as data, and the protocol tells agents not to follow
  instructions found in tickets.
- `SEC-6` A board may be shared through git or a sync service, and `serve`
  cannot tell a synced edit from the user's own (D29). So `serve` copies a
  linked file (`REV-5`) only when its real path, symlinks resolved, is in a
  git checkout on this machine whose main checkout the ticket's project
  records in `project.yaml` and maps to that project by name (`PRJ-2`,
  `PRJ-3`); is not inside `.git`; and is not ignored by git (`git
  check-ignore`; if git fails or is missing, nothing is copied). The
  checkout is verified on disk, not taken from the synced repos list. Paths
  given to MCP tools come from the local agent and keep the `REV-5` rule.
- `SEC-7` A file sent from the browser (`REV-7`) is taken only through
  Singleserve's authenticated session (`POST /api/tickets/{id}/files`,
  its bytes the body, its name and kind as parameters). It is read no
  further than the limit (a declared length over it is refused before
  reading), then judged by its bytes: PNG, JPEG, GIF, WebP and PDF by
  their signatures, text as UTF-8 without binary control bytes; SVG, HTML
  and XML (by their opening, or a name that says so) and anything else
  are refused. It is stored under the extension of what it is, so it is
  served as that (`SEC-4`), in the ticket's `files/` alone, its name
  reduced to lowercase letters, digits and dashes with a time prefix and
  a number on a clash; text is scrubbed (`SEC-3`). Nothing is written
  unless the whole file is accepted.

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
layout and content of every view, in every theme, follow
`docs/dev/specs/ui-layout.md`; `DESIGN.md` records the visual system: Metro
Pop in light and Night Service, in black and neutral charcoal, in dark
(D18), which replaced the Transit Line Map of D14.

- **Shell**: left rail of projects (with Needs you badges) and **All
  projects**; top bar with view switcher (Overview · Workstreams · Table ·
  Board), search, filters, density, virtual-column toggles; quiet backend
  status, a Settings gear (theme, board root, version) and Quit.
- **Board**: columns, cards, live badges; card panel slides over from the
  right and keeps the board visible.
- **Overview**: the pipeline of the board's columns and their counts, then
  one calm column of section cards about tickets: Needs your decision,
  Ready for your review, At risk, In progress, then collapsed Work with no
  ticket and Up next (`VIEW-3`).
- **Needs you** is visible from every view and project (badge in the rail and
  top bar); the top bar's plate filters the Board to it (`VIEW-2`).
- Narrow widths keep a single column with a column picker; the board is not
  a phone-first product.

## 8. Non-goals

Hosting, proxying or supervising agents (`D48`); remote access, or
multi-user sharing for now (`D49`);
accounts; a hosted service; telemetry; issue-tracker sync (GitHub Issues,
Jira, Linear); time tracking; replacing git or PRs; editing source code.

## 9. Later possibilities (not committed)

Passive reading of agent session logs for activity without hooks; desktop
notifications for *Needs you*; Windows support; starting an agent's own app
on a ticket (`D48`, FH-131); a small team sharing a synced board (`D49`);
more agents (Gemini CLI, Cursor, others with hooks); GitHub PR status
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
