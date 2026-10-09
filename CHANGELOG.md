# Changelog

All notable changes to this project are documented here. The project follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Security

- `serve` no longer copies any linked local file into a directly edited
  ticket or review (FH-34, D29). A board synced through git or a sync
  service could receive a link to a private file on this machine, which
  the next sync would publish. Serve now copies only files in a git
  checkout of the ticket's project's repository, verified on disk, that git
  does not ignore. Links to other files stay as they are and are logged
  once. Paths given to the MCP tools are copied as before.

### Fixed

- A question answered in the agent's own chat no longer leaves the session
  and its ticket in Needs you (FH-43, D28). A prompt in the asking session
  answers its open questions there (its subagents' too, never another
  session's); a background task's completion notice does not. The question
  stays in the run's timeline, marked as answered in the session; the board
  refuses a later answer to it, and a waiting `flashheart await` exits
  saying so. Tickets in Review or Done no longer show Needs you for
  questions their sessions already settled in chat.

### Changed

- Needs you is a filter instead of a column (FH-44, D31). The Needs you
  column, which repeated tickets already shown in their own columns, is
  gone. The band's *N need you* plate is now an on/off filter: pressed, it
  shows All projects' Board (or Table) with only the tickets that have a
  run in Needs you or an open question, and pressed again it returns to the
  project you were in (it used to open the Agents view). Agents that need
  you with no ticket on the board are named in a notice that opens the
  Agents view. It combines with the other filters and Clear filters
  switches it off; it carries from project to project and a reload starts
  with it off. The rail badges and the card's Needs you mark are
  unchanged. A saved `ui.virtual_columns` setting is ignored and removed on
  the next save.
- The Board toolbar is decluttered (FH-39). Filters are buttons named for
  what they filter (Type, Priority, Workstream, State) with a count badge
  while applied, each opening a list of values; View options (⋯) holds the
  virtual columns, Colour by and density; New ticket sits at the top right.
  The workstream chips and the colour key are filter chips: a click shows
  only that value, Cmd-click (Ctrl-click off macOS) hides it, and a click on
  a chosen chip restores it; they hide tickets rather than dimming them.
  Workstream chips are ordered by open tickets and finished workstreams are
  left out; the chip row keeps to one line. Several values of one filter can
  be chosen at once. Remembered filters in `config.yaml` become
  `include`/`exclude` lists; earlier single values still load.

### Removed

- The Hide later filter (FH-39); a remembered `hide_later` is ignored.
- The Agent working column (FH-42, D27): a working ticket now shows once,
  in its own column, with the Working mark; *State: Agent working* lists
  them. A saved `agent-working` virtual column is ignored. Needs you stays.

- Workstreams are epics: their tickets may be worked on in sequence or in
  parallel, and a ticket waits only for the tickets its `depends-on` names
  (one or several) and for workstream dependencies. A workstream's ticket
  order no longer blocks; give a chain that relied on it `depends-on`. The
  Workstreams view draws each workstream as a railway graph of those
  dependencies: chains on one track, parallel work on tracks below, joins
  where a ticket needs several, and a *Needs* pill for a dependency outside
  the line. Re-run `flashheart setup claude` for the new skill text;
  protocol 1 (FH-38, D26).
- The Needs you and Agent working columns stand between In progress and
  Ready to review instead of before Backlog (FH-40).

- The protocol skill and `ask_human` tell agents to ask any question that
  ends their turn with `ask_human`, not only in chat, so the session shows
  in Needs you instead of Waiting (FH-31). Guidance only; protocol 1.
- `scripts/check.sh` runs the 5,000-ticket indexing time check (NFR-1) on
  its own after the other Go tests, so a busy CI runner no longer fails it;
  the one-second budget is unchanged (FH-30).
- Reviews need evidence. The agent skill now requires captioned screenshots
  of every state a visible change touched, or a stated "No visible change"
  reason, in a new Evidence section of the review template, and moving a
  ticket to Ready to review warns when the review has neither. Re-run
  `flashheart setup claude` to install the new skill text (FH-29).
- Agent tools stay fast at full scale: an MCP call reads only the projects it
  touches (about 0.2 s instead of 3.5 s on 50 projects of 1,000 tickets),
  and read-only tools no longer create projects or write the cwd cache
  (FH-8).
- A claim on another project's ticket stays held while its session works at
  home, and a subagent claiming its session's ticket no longer takes it from
  the session. A subagent's edits now count toward its session's handoff
  (FH-8).
- Answers record who gave them: the new `user_name` setting, else the
  computer account's name, instead of "human". Questions from a session that
  has ended can still be answered and wait for it to resume, without
  showing as Needs you; a read-only board shows questions without an answer
  form (FH-8).
- `flashheart setup claude` reads the MCP registration through `claude mcp
  get` instead of parsing `~/.claude.json`, and says when it cannot tell
  (FH-8).
- New look for the board: Metro Pop in light (a raspberry bar and accent on
  a clean white background) and Night Service in black and charcoal in dark, on one shared layout (`docs/dev/specs/ui-layout.md`).
  Views open with the project's name as a page header; columns are rounded
  wells; cards show id and time, title, workstream and type tags, blocker
  pills, the live run and a footer with criteria and priority at every
  density; tickets' status shows as coloured pills with an icon and words;
  workstreams are colour-washed route cards whose stations check off only
  finished work; the card panel leads with status, blocker and priority
  pills and sets questions and the handoff as callouts. Both palettes meet
  WCAG 2.2 AA, checked by a palette test.
- Board columns scroll together on one surface, like a single page: a wheel
  or trackpad anywhere over the board moves every column, column heads stay
  in view, and no column has its own scrollbar (FH-24).
- In the Agents view, a subagent names a ticket only when it claimed one
  other than its session's (FH-6).

### Added

- Each card in the board API carries a `sessions` summary of the runs
  working for its ticket (FH-50, RUN-9): the live agent and its state, the
  last activity, whether the handoff is behind the session's work, and its
  subagents counted as done, running and needing you. An in-progress ticket
  that no live session works on is marked `noLiveSession`. It is derived
  from the events already recorded; hooks are unchanged. The Overview
  (FH-47) builds on it.
- The Board can hide the Backlog column (FH-41): View options › Show
  columns has a Backlog checkbox, checked by default. Hidden, the columns
  being worked on get its width, the phone Column picker leaves it out, and
  Shift with an arrow or a drag never moves a ticket into it. The choice is
  saved as `ui.hidden_columns` in `config.yaml`; a config without it shows
  the Backlog.
- Every ticket has a full page with room to read and review screenshots
  (FH-45, D30). A ticket id anywhere on the board (cards, table rows,
  workstream stations, the Agents view, the archive and the card panel)
  opens it in a new tab, and so do ticket ids in ticket text: markdown,
  acceptance criteria, handoff next steps and blocking reasons. Clicking a
  card still opens the side panel, which now also has an *Open full page*
  button. The page shows the panel's tabs at reading width, names the
  browser tab after the ticket, and links back to the ticket on its board;
  ticket links on it stay in the same tab.

- The installed Flashheart skill keeps itself current: after you upgrade
  the binary, the next Claude Code session start rewrites
  `~/.claude/skills/flashheart/SKILL.md` to match and says so in one line,
  so `setup` no longer has to be rerun. A missing skill is never
  reinstalled; edits to the skill are overwritten, as setup overwrites
  them (FH-33).
- `flashheart await <question-id> --project NAME`: waits for the answer to
  an agent's question and prints it. `ask_human` returns the command, and
  Claude Code runs it in the background, so an answer given on the board
  wakes the session instead of waiting for your next prompt (FH-31).
- Agents can attach screenshots and logs to a ticket with the new `attach`
  MCP tool, which copies the file in with a caption and returns the
  markdown to link it from the review. SVG, HTML, oversized files and
  anything that is not a regular file are refused (FH-6).
- An Attachments tab on the card panel: screenshots as thumbnails that open
  large in a lightbox (arrow keys step through them), other files as tiles
  that open in a new tab. The Review tab shows the screenshots above the
  review (FH-6).
- A review's *How to Verify* steps show as a checklist you can tick; each
  tick is saved into `review.md` (FH-6).
- The Runs tab shows an orchestrated session's subagents as a tree: each
  with its state, the ticket it claimed if that is another, and its plan
  step, opening to its own plan, files and activity, including checkpoints
  (FH-6).
- `flashheart doctor` checks the board root and its permissions, Claude
  Code's hooks, MCP server and skill against what `setup` writes (including
  a configured binary that no longer exists), and the last day's hook
  errors, with the fix for each; it changes nothing (FH-6).
- While the board is open, a ticket or review edited in a text editor that
  links to a screenshot or log outside the board gets a copy of that file,
  and the link points at the copy, so the evidence survives the original
  being cleaned up (FH-6).
- Manual card order: drag a card to any place in its column, or to a place
  in another column, and the order is saved in the ticket's new `rank`
  field, so it survives reloads and restarts. Shift with Up or Down and the
  panel's Position buttons move a card within its column; Undo puts it back.
  New tickets join below the ordered ones, Done stays most recent first,
  and workstream order is unaffected (FH-23).
- Archive view for each project (Archive in the page header): search
  archived tickets, restore one to the column it left, or delete it
  permanently. Deleting needs the ticket's id typed, lists every ticket and
  workstream that refers to it, removes those references so nothing waits
  on it, and retires the id so it is never reused (FH-20). Permanent
  deletes are marked in red, like errors.
- Projects can be archived from the archive page (All projects › Archive, or
  Archive project… in a project's archive), with a warning about live
  sessions, tickets being worked on and open questions; restored; and,
  once archived, deleted permanently with the name typed. An archived
  project's tickets count as done for other projects and its key stays
  reserved; a deleted project's key is retired. Agents working in an
  archived project's repository no longer recreate it: hooks stay quiet and
  MCP tools answer `project_archived` (FH-21).
- Dry marginal remarks from the reviewed Blackadder collection in quiet
  moments: empty boards and searches, an empty review column, quiet Agents
  lanes, open runs' plans, handoffs, the toast when a move finishes a
  workstream (and now and then for other work done), and the stopped
  screen after Quit. At most one shows per screen; none appears in errors,
  blockers or anything agents read (FH-22).

- `flashheart` binary: `serve` (default) opens a private, authenticated board
  shell in the browser and returns the terminal, running the server in the
  background until Quit or the last tab closes; `--foreground` keeps it
  attached. The background server records later errors in
  `<root>/.flashheart/serve.log`. `version` prints the build and agent protocol version;
  `doctor` reports that it is not yet available.
- Board root resolution from `--root`, `FLASHHEART_ROOT` or
  `~/reports/Kanban`, and read-only loading of `.flashheart/config.yaml` with
  documented defaults and validation.
- Browser shell with quiet backend status and health check, guarded Quit that
  explains a refusal, and stopped and connection-lost screens that explain how
  to close the tab. Light and dark themes follow the system setting.
- Board UI: every project in a rail with
  route bars, a Board in three densities, a card panel with the ticket,
  blocked-by explanations, handoff, criteria and review, Workstreams drawn as
  routes of stations, a sortable Table, filters and search, light and dark themes
  and full keyboard movement.
- Board refresh: the card panel sits beside the board, which narrows and
  keeps the card's column in view; a project rail with project-key badges;
  cards carry the workstream's line down their left edge; **Colour by** type
  (default), priority, age or none tints card headers and names the value,
  with a colour key. Below 1440px an open
  panel shrinks the rail to its key badges, keeping three whole columns.
- Board editing: move tickets by drag, Shift with an arrow, or the panel's
  Move to menu, with Undo; a reason is required to start a blocked ticket
  and is recorded in Notes; moving into review warns about a missing review
  or unticked criteria. The panel ticks acceptance criteria, archives with
  Undo, and has an Edit tab with typed fields and the raw file. New ticket
  creates the next id and joins its workstream's list. Workstream stations reorder along their line.
- Safe concurrent writes: per-project and root locks, content-hash
  preconditions, atomic replace, and a side-by-side conflict dialog when a
  save loses a race.
- Live updates: file watching turns external edits into board changes
  within a second, delivered to the browser by long-polling.
- Preferences (theme, density, colour by, and each project's view and
  filters) are saved in `config.yaml`.
- Board API over the root (read and write): projects with counts, project and
  all-projects boards with blocked-by explanations, ticket detail with
  review, handoff and attachments, workstreams with derived status, and
  allow-listed attachment files. Unparseable, oversized or escaping files are
  reported as needing repair instead of being hidden.
- Board format v2: a folder per ticket (`tickets/FH-42-slug/FH-42-slug.md`
  with `review.md` and `files/`), status in frontmatter, and five columns:
  Backlog, Up next, In progress, Ready to review and Done.
- Ticket ids like `FH-42` from a per-project key, shown on cards, the panel,
  the table and in URLs; search by id; a bare id in ticket or review text
  links to that ticket.
- `flashheart migrate [--write] [--key project=KEY]` converts a v1 board:
  numbers tickets oldest first, rewrites references to ids and moves the v1
  files into `.flashheart/backup/`, deleting nothing. A board with v1
  projects shows the command to run.
- Agent runs from Claude Code hooks: `flashheart hook claude <Event>`
  records sessions, subagents, prompts, tool use, edited files, plans,
  permission prompts and compaction in each project's event log, creating a
  project for a new repository on first contact. It always exits 0, logs
  problems to `.flashheart/hook-errors.log`, never stores prompts, commands
  or tool inputs and outputs, and scrubs likely secrets. At session start it
  tells the agent about the ticket linked to its branch, the ticket's
  handoff and an earlier session that left edits.
- The Agents view: every session and subagent across projects as a
  departure board, lanes by state with Needs you first, each run's ticket,
  plan, branch and last activity, opening to its plan, edited files and
  activity. Cards show their live run, Needs you and Agent working columns
  mirror tickets at the front of the board, the card panel has a Runs tab
  and warns when a run has edited since the handoff, and Needs you shows in
  the top bar and the project rail.
- `flashheart mcp`: the MCP server agents use (agent protocol 1).
  `board_context`, `list_tickets` and `get_ticket` find work ("the two
  top-priority bugs", "FH-42"); `claim` and `release` hold a ticket for a
  session while it is active; `checkpoint` rewrites the ticket's handoff;
  `update_ticket`, `move`, `create_ticket` and `set_project_key` edit the
  board (agents choose a new project's key and never move tickets to Done);
  `write_review` writes the review; `ask_human` asks you a question.
  `create_workstream` groups dependent tickets with a shared goal, and
  giving a ticket a workstream (when creating or updating it) also adds it
  to the end of that workstream's list and takes it out of any other, so
  its order and blocking apply; an unknown workstream is refused.
  `board_context` lists the project's unfinished workstreams.
  Screenshots and other files a review or checkpoint links by local path are
  copied into the ticket, so they survive the agent cleaning up.
- `flashheart setup claude` shows, as a diff, the hooks, MCP server and
  Flashheart skill that connect Claude Code, and applies them with
  `--write`, backing up what it changes in `~/.claude/flashheart-backup/`
  and moving the kanban-tracker skill aside; `--uninstall` undoes it. Guide:
  [`docs/user/claude-code.md`](docs/user/claude-code.md).
- Questions from agents show as Needs you on the ticket's card and in the
  Agents view; answer them there and the answer is noted in the ticket and
  reaches the session with its next prompt or in its recovery note.
- Optional handoff enforcement (`enforce_handoff`, per project or global):
  a session linked to a ticket that tries to stop with edits since its last
  checkpoint is asked, once, to checkpoint first.
- Recovery notes now point agents at the MCP tools and include answers
  waiting for them; claims are leases (`lease_minutes`, 30 by default).
- Event logs expire after `event_retention_days` (90 by default).
- `scripts/hook-bench.sh` measures hook latency (p95 under 50 ms) and
  `scripts/record-claude-hooks.sh` records real hook payloads for the tests.
- GitHub foundation: CI runs `scripts/check.sh` on Linux and macOS and the
  Playwright suite on Linux for every pull request; Dependabot, a pull
  request template, `CONTRIBUTING.md` and `SECURITY.md`.
- Validation: strict `scripts/check.sh`, `scripts/build.sh`, and
  `scripts/e2e.sh` for the Playwright lifecycle, accessibility and screenshot
  suite.
- Product specification, board format (v2) and agent protocol (v1), roadmap,
  decisions, contributor facts and the implementation kickoff prompt.

### Fixed

- An edit made in a new ticket folder just after the folder appeared could
  take up to ten seconds to reach the board instead of under a second
  (STO-7): the watcher now watches new folders before it publishes their
  revision. This was also the most common cause of intermittent CI failures
  (FH-36).
- Stopping `flashheart serve` now waits for the board watcher and the
  event-log pruner to finish before it closes the board, so they can no
  longer touch a closed board or print a spurious "expire old events"
  error at shutdown. This also removes an intermittent CI data race
  (FH-37).
