# Changelog

All notable changes to this project are documented here. The project follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

- Agents write tickets a person can read at a glance (FH-113). The
  flashheart skill gains a *Writing tickets* section: a title says what a
  person would notice or gain, in plain words; the description gives the
  problem, why it matters and the proposed change; implementation detail
  goes in the new `create_ticket` `context` input, saved under the
  ticket's Context section. `create_ticket` and `update_ticket` repeat the
  advice where agents read it, and a title that reads like code or runs
  past 80 characters is saved with a warning saying how to improve it
  (protocol version unchanged).

- Finished workstreams collapse on the Workstreams view (FH-68). A
  workstream whose tickets are all Done, Rejected or Archived shows only
  its route card head (name, *Completed*, `d of t served`) with a chevron;
  click it, or press Enter or Space, to show the full line. Flashheart
  remembers your choice in `config.yaml` (`ui.expanded_workstreams`) while
  the workstream stays finished. A workstream with work still in Ready to
  review or earlier always shows in full.

- A **Rejected** status for tickets closed without being accepted (FH-58).
  `status: rejected` is a Done state: the ticket sits in the Done column
  and counts as done for blocking and workstream progress, but its card,
  panel, table row, archive row and workstream station say *Rejected*
  (a neutral pill and a crossed station, never a check). Reject a ticket
  with the panel's **Move to → Rejected** or the Edit tab's Status; moving
  it anywhere else, Done included, clears it. Agents see `rejected` in
  `list_tickets` and `get_ticket` and can filter by it, but `move` refuses
  it, as it refuses `done` (protocol version unchanged).
- The State filter's **Rejected** option shows only rejected tickets, on
  the Board and the Table, for one project or All projects, and is
  remembered like the other states (FH-59). The Done limit still applies,
  so older rejected tickets need *Show all*.

### Security

- A `.flashheart/config.yaml` that is a symlink to a file outside the board
  root is now refused, like every other board file, instead of being
  followed (FH-108, SEC-2). If yours is, replace the link with the file:
  until then `serve` does not start, hooks record nothing (the error is in
  `hook-errors.log`), the MCP tools fail and `flashheart doctor` reports
  the settings as a problem. A link that stays inside the root still works.
  Only the store now opens files under the root; a test lists the few
  packages allowed to touch files at all, each with its reason.
- `serve` no longer copies any linked local file into a directly edited
  ticket or review (FH-34, D29). A board synced through git or a sync
  service could receive a link to a private file on this machine, which
  the next sync would publish. Serve now copies only files in a git
  checkout of the ticket's project's repository, verified on disk, that git
  does not ignore. Links to other files stay as they are and are logged
  once. Paths given to the MCP tools are copied as before.

### Fixed

- Keyboard focus no longer jumps to the top of the page when the board
  changes under it (FH-84). A focused card that a live reload or a filter
  moves to another column, an Overview row that moves to another section
  and a Runs-tab subagent row whose session moves down the list keep
  focus; one that disappears hands focus to the next item in its list,
  else the previous one, else the column or section heading. Focus in a
  dialog, the panel or the search box stays put. The board, the Overview,
  the Runs tab and independent stations share one way of doing this.

- Sessions that crash, and Codex sessions, which never report their end,
  no longer leave their working files in the board folder forever (FH-82).
  `serve`'s regular cleanup, at startup and daily, now removes a session's
  activity file once no hook has written it for 12 hours (`stale_hours`),
  and its answers waiting list with the event log of the day it was last
  written, after `event_retention_days`, when none of its answers could
  still be delivered. Tool activity a crashed session had not recorded yet
  is recorded when the session resumes, or dropped once the session is 12
  hours stale, never kept.

- A session that claimed a ticket in another project keeps its link to
  its own ticket (FH-32). A session's project is now always its home, the
  project it works in, never the project of a ticket it claimed, moved or
  checkpointed elsewhere; before, a claim elsewhere seen first could file
  the session under that project, and once the claim was released the
  session showed as unlinked instead of linked by branch to its own
  in-progress ticket. `board_context` also finds a session's ticket in its
  home when called from another project. The spec now says plainly that
  handoff enforcement and the recovery note read only the home project's
  log, so they see a claim on another project's ticket only by branch.

- An answer to an agent's question is no longer lost when recording its
  delivery fails (FH-78). A starting session's recovery note showed no
  answers if that record failed, although the answers had already been
  taken from the session's waiting list, and `board_context` failed the
  same way; a waiting `flashheart await` then waited until its 12-hour
  timeout. Every path that hands answers over now shares one routine that
  shows what it took even when the record fails (the failure is logged),
  and `flashheart await` also delivers an answer the log shows given and
  not delivered. Such an answer may now be shown twice, never not at all
  (protocol version unchanged).

- A long branch name keeps its distinguishing end in view in the ticket
  panel's header and the Runs tab (FH-83). It is cut from the start, as
  the board root, run ids, worktrees and archived projects' repositories
  already were, so `…overflows-the-panel-fh83-tail` shows instead of
  `feature/a-very-long-bra…`; the full name stays in its tooltip, and the
  text is whole for screen readers.

- A long path in a run's edited files keeps its file name in view in the
  Runs tab (FH-117). Edited files are cut from the start like the run's
  branch and worktree, so `…of-many-parts/RunDetailEditedFile.tsx` shows
  instead of `frontend/src/features/card/a-dee…`; the full path stays in
  its tooltip.

- A question a subagent asked before it finished is no longer hidden
  while its session waits on it (FH-112). The session stays in Needs you
  saying, for example, *Explore has a question*; the ticket card now
  counts the question and the Overview lists it under *Needs your
  decision*, so you can find and answer it. It stops needing you once the
  session itself ends, as any ended session's question does. The project
  archive check counts it the same way.

- Archived projects added, renamed or removed outside Flashheart (by
  hand, by another Flashheart process, or by restoring a backup) now show
  in the archived projects list and the All projects archive count within
  a second, instead of at the next 10 s sweep (FH-28). The watcher now
  watches `<root>/.archive/` itself, but not the archived projects' own
  folders, and never through a symbolic link.

- `flashheart migrate` keeps tickets with Windows line endings or a
  byte-order mark intact (FH-79). Before, such a ticket gained a second
  header and lost its created date and dependencies, and its dependency
  list and its workstream's ticket list were never rewritten to ids. The
  migration now edits every header with the shared editor, which keeps
  each file's line endings and byte-order mark, and writes under the board
  root's lock (and each converted project's) so no other write lands
  part way through. A `project.yaml` or `config.yaml` that does not parse
  is refused before anything is written; a workstream that does not parse
  is warned about and kept as written.
- A preference changed just before a reload shows its new value after the
  reload (FH-109). The save flushed as the page unloads reaches the server
  first, but on a slow disk or a busy machine it could still be writing
  when the reloaded page read preferences, so the page showed the old
  value until the next reload. The server now answers a preference read
  only once any save in progress has finished.

- A project whose `project.yaml` uses Windows line endings or a byte-order
  mark keeps its name and repositories (FH-77). Before, adopting a
  repository and then setting the project key wrote a second header into
  the file, so the board showed the project under its folder name with no
  repositories, and the next hook adopted the repository again. Every edit
  of `project.yaml` now keeps the file's line endings, and every reader
  treats problems alike: a field of the wrong type is shown as a project
  warning and the other fields still count, and a list that cannot be read
  is never replaced.

- A subagent reporting back no longer answers the question its session
  asked you (FH-57). Only a prompt you type answers a session's open
  question in the chat; a subagent's hand-back, a message from another
  agent, a finished background command or agent, and the desktop app's
  pull request monitor leave it in Needs you, and `flashheart await`
  keeps waiting. Before, a session that orchestrated subagents lost every
  question it asked. The run timeline shows such a turn as *Message from
  an agent* or *Background task finished*. Hooks record who started each
  turn (`turn.start` `origin`), never the prompt (protocol version
  unchanged).
- Changing a ticket's workstream in the card's Edit tab moves it between
  the workstreams' ticket lists too (FH-80). Before, only the ticket
  changed: the old workstream still listed it, the new one did not, and
  the ticket showed a "not listed in workstream" warning and did not count
  toward the new workstream's progress. Clearing the workstream removes
  the ticket from its list. An edit based on a stale copy of the ticket,
  or naming a workstream the project does not have, changes no file.
- Starting a blocked ticket from the board records the reason as a dated
  note, `- 2026-10-12 · Started while blocked: …`, like an agent's forced
  claim (FH-81). Before, the board added a bare, undated line. A new
  ticket's description from the board is now limited to 16 KiB, the
  limit agents already had (it was 64 KiB); a heading typed into it stays
  inside the description (FH-87).
- Text written into a ticket section can no longer split it (FH-87).
  This covers a handoff, a note, a new ticket's description or plan from
  the board or an agent, and a workstream's goal. A level-1 or level-2
  heading in that text becomes level 3, and an unclosed code fence is
  closed. Before, a `## ` line could start a new section, and later
  edits and recovery notes read the text as that section (the FH-4
  truncation). Agents' new tickets were already protected; the board's
  New ticket dialog and checkpoints were not.
- Attaching a file keeps a ticket's `files/index.yaml` as it was written
  (FH-88). Before, keys Flashheart does not know and comments were
  dropped from the whole index.
- A run's Activity in the Runs tab names every ticket event in plain
  words instead of its raw event name (FH-70): "Moved FH-1 to In
  progress", "Updated FH-1", "Attached a file to FH-1", where it showed
  "ticket.moved FH-1" after most claims.
- A run without hooks whose agent switches branches shows "Switched to
  branch …" in its Activity instead of a second "Connected without hooks
  (MCP only)" line, as if it had connected again (FH-71).
- A preference changed just before a reload or tab close is no longer lost
  (FH-48). Preferences are saved 400 ms after the last change; a save still
  waiting when the page is hidden or unloaded is now sent at once, through
  the authenticated session with `keepalive`, instead of being dropped.

- The Overview fills the main area like Board, Workstreams and Table
  instead of sitting in a centred 56rem column (FH-60).
- Hooks in an archived project's repository no longer wait for the root
  lock to learn the project is archived (FH-27). They used to wait up to
  5 s behind a long write such as a permanent project delete; they now
  return at once and still record and print nothing.
- Permanently deleting an archived ticket or project no longer holds up
  new tickets and hooks in new repositories while it reads every ticket
  (FH-26). With 2,000 archived tickets the delete held the root lock for
  about 1 s, reading every ticket twice; it now holds it for about 0.1 s,
  for one quicker check and the writes. The preview opens about six times
  faster. A delete is still refused when anything the preview listed has
  changed, or another ticket has started to refer to what is deleted.

- `flashheart doctor` no longer reports a `flashheart` MCP server
  registered at local or project scope as "not registered" (FH-35). It is
  a warning naming the scope, with `setup claude --write` as the fix to
  register it for every project.

- Files changed through shell commands now count toward the handoff
  (FH-53). A run used to be dirty only after the edit tools, though agents
  edit mostly through shell commands, so "no handoff", the At risk reason
  and handoff enforcement missed most edits. At a session's turn end and
  end, when it ran a shell command since its last checkpoint, the hook asks
  git (within 50 ms, without locks) whether a changed, untracked or newly
  committed file was modified during one of its shell commands (from the
  run's previous event to the command's result, give or take 2 s), and
  records only yes or no on `turn.end` and `run.end` (`worktree_changed`,
  agent-protocol §4; protocol version unchanged). Read-only commands never
  flag a run, and neither do files you change while the session waits for
  you.
- A question answered in the agent's own chat no longer leaves the session
  and its ticket in Needs you (FH-43, D28). A prompt in the asking session
  answers its open questions there (its subagents' too, never another
  session's); a background task's completion notice does not. The question
  stays in the run's timeline, marked as answered in the session; the board
  refuses a later answer to it, and a waiting `flashheart await` exits
  saying so. Tickets in Review or Done no longer show Needs you for
  questions their sessions already settled in chat.

### Changed

- Development: a visual change to the board now fails the build until its
  screenshots are updated on purpose (FH-101, D42). `scripts/e2e-visual.sh`
  compares a stable set of screenshots with committed baselines inside the
  pinned Playwright container, so a Mac and CI render alike; CI runs it
  after the browser suite, and `--update-snapshots` rewrites them. Quick
  tests can now press keys and check focus (happy-dom and Testing Library,
  in `*.dom.test.tsx`), covering popovers, dialogs, tabs and how views load
  and reload their data, and the browser specs share one set of helpers.

- Every view shows load errors, stale data and loading the same way
  (FH-103, ui-layout.md §8). When the project list, Workstreams, the card
  panel or a ticket's full page cannot load, they now say why with
  **Try again**, as the Board, Overview and archives already did; so does
  the Archive project dialog when its check fails. When a reload fails, the
  project list, Workstreams, both archives, the card panel and the ticket
  page keep showing the last good copy with a *Showing the last good copy*
  note, as the Board and Overview did. While loading, the Overview,
  Workstreams, both archives and the Table each show a placeholder in
  their own shape (section cards, route cards, a table) instead of the
  Board's columns or a line of text, and the Board's leaves out a hidden
  Backlog.
- A card's live badge, its Runs tab and the Overview now give the same
  reason a run needs you (FH-98, D39). Flashheart derives it once with the
  run's state: the run's own permission prompt, then its own question, then
  a subagent's prompt, then a subagent's question, then an answer still on
  its way. A session with its own open question while a subagent waits on a
  permission prompt used to show the subagent's prompt on its card and the
  question on its Runs tab; both now name the question, and the
  subagent's prompt shows on the subagent's row and in the Overview. A
  card whose session waits on a subagent's question now names the
  subagent ("Explore has a question"), as the Runs tab did, and the
  Overview lists a session's own permission prompt even when it also has a
  question. Short run ids come from the server: a subagent shows its
  session's id (`claude:3f2a9c1e`), as the ticket's notes and handoffs
  name it, not its own.
- `serve` keeps up with busy agents without rereading the board (FH-96, D38).
  An agent's hook events now only refold the runs over the board already
  read, where each batch used to reparse every ticket; an event recording
  a ticket write still rereads it. While the file watcher runs, page loads
  serve the snapshot it keeps fresh instead of rebuilding one older than
  2 s, and a change saved on the board is read once, not again by the
  watcher. Cards find their live runs, subagents and questions through a
  per-ticket index built once per snapshot. Event logs now count toward
  the watcher's 1,500-folder budget, and each project's log is watched
  instead of its whole `.flashheart` folder. The first event logged in a
  project now shows within a second instead of at the next 10 s sweep.
- The session-start recovery note and handoff enforcement now look back as
  far as the board and the MCP server do: the daily event files from two
  days ago on, instead of one (FH-95, D37). A claim made the day before,
  more than about 24 hours ago, still links the session to its ticket at
  a resume and at a stop, and the previous session in a worktree can be
  up to two days old. The hooks read only the sessions they ask about, so
  on a busy log (50,000 events a day) a session start takes about 36 ms
  and a blocked stop about 30 ms at p95, down from 186 ms and 180 ms. The
  worktree check at a stop now ends at most 75 ms after the hook began,
  however long reading the log took, so a stop stays within 100 ms.
  Protocol version unchanged.
- `scripts/hook-bench.sh` takes `-agent`, `-scenario`
  (`mix`, `tools`, `session-start`, `stop`) and `-seed-events` to measure
  session starts and enforcing stops on a seeded log (FH-95).
- The `attach` and `set_project_key` MCP tools now answer `ambiguous_run`
  when several sessions work in the same worktree and branch and the call
  does not name its `run`, as every other write tool already did (FH-93,
  D36). Before, an attachment could be recorded as `unknown` and a key set
  by a session Flashheart could not name. The agent protocol lists each
  tool's run and claim policy (§7.1); protocol version unchanged.
- The view tabs read Overview, Workstreams, Table, Board (FH-61).

- **Agent protocol 2** (FH-55, D32): hooks record a run's tool results as
  throttled `activity` records instead of one `tool.used` per tool call,
  which was about 80% of all events. A record counts the tool uses and
  failures since the run's previous one, at most one a minute
  (`activity_seconds` in `config.yaml`); an edit of a path the run has not
  recorded in this turn or since its last checkpoint, and the tool result a
  permission request waits on, are recorded at once, and what is pending is
  recorded before a turn end or end. Each edited path is recorded once per
  run per turn, and again after a checkpoint, so a run's edit count is now
  of files. Shell
  commands' windows (FH-53) travel in the records, so shell edits still
  count toward the handoff. On this board's own logs the change leaves
  about a third of the events (6,475 to 2,132). Last activity lags a run's
  last tool result by at most a minute; Quiet, Needs you and dirty behave as
  before, and logs written before the upgrade read as before. The Runs
  tab's activity shows turns, checkpoints, questions, moves and summaries
  such as "Edited src/app.ts, 3 tool uses" or "40 tool uses, 2 failed"
  instead of every tool's name. Agents see no difference; the installed
  skill's protocol number updates at the next session start, or re-run
  `flashheart setup claude --write`. Update the binary and the board
  together: an older `serve` does not read the new records.
- An Overview replaces the Agents view (FH-51, D32). The band's tabs read
  Board, Overview, Workstreams, Table. The Overview is about tickets, for
  All projects or one project, in one calm column: Needs your decision
  (answer an agent's question in place; a permission prompt says which
  session to answer it in), Ready for your review (oldest first, with
  Review results opening the ticket's Review tab), At risk (no live
  session, a quiet session, ended without a handoff, a blocked top of Up
  next, each in words), In progress (the session's state and its subagents
  in words), then collapsed Work with no ticket (live sessions with no
  ticket, each with Create ticket) and Up next. Run detail stays on the
  card's Runs tab. Old Agents links and a remembered Agents view open the
  Overview, and the Needs you filter's notice of agents with no ticket
  opens it too.
- Plan displays show only for a run with a plan (FH-54). Most sessions
  never record one, so the card panel's Runs tab (and an open run in the
  Agents view) no longer shows an empty Plan section with *No plan
  recorded*; the card's live badge already named a plan step only when
  there was one. A run with a plan still shows its route and current step.

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

- The `_scratch` project (FH-62, D33). Agent activity outside a git
  repository is no longer recorded: hooks there write and print nothing,
  `board_context` and `list_tickets` say there is no current project, and
  tools that need one (`create_ticket`, `ask_human`, `claim`…) answer the
  new `no_project` error, so the agent talks to you in chat instead. Run
  `git init` in a folder to track it. An old `_scratch` folder in the board
  root is now ignored and can be deleted (protocol version unchanged).
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

- An agent that reaches Flashheart's MCP server without its hooks (Codex
  today, or a user who declined hooks) gets a run of its own (FH-15, D34).
  Its first write starts the run, so `claim`, `release` and `ask_human`
  work and its tickets name it instead of "unknown". Each of its Flashheart
  tool calls keeps its claim alive; closing the session ends the run.
  Answers to its questions arrive with its next tool call. Its session
  shows on the Overview and in the ticket's Runs tab, marked "MCP only",
  since without hooks it reports no plan, edits or permission prompts. A session with hooks is attributed as
  before, and a Codex session is no longer mistaken for a Claude Code
  session in the same worktree.
- The Overview opens with the pipeline (FH-63): Backlog, Up next, In
  progress, Ready to review and Done as a row of interlocking chevron
  cards, each the column's whole count in its status colour, for All
  projects or the project shown; Done counts accepted work, leaving out
  Rejected tickets (project summaries now carry a `rejected` count). Each
  stage opens the Board. It replaced
  FH-52's *Since your last change* tiles, whose To review counted tickets
  arriving since your last change and so could read 0 above a Ready for
  your review section listing four; `GET /api/metrics` is gone.
- Each card in the board API carries a `sessions` summary of the runs
  working for its ticket (FH-50, RUN-9): the live agent and its state, the
  last activity, whether the handoff is behind the session's work, and its
  subagents counted as done, running and needing you. An in-progress ticket
  that no live session works on is marked `noLiveSession`. It is derived
  from the events already recorded; hooks are unchanged. The Overview
  (FH-47) builds on it.
- The board records when you change a ticket (FH-49, `EDIT-10`). Every
  board write that changes a ticket (new ticket, move or reorder, field,
  raw and criterion edits, a ticked review step, archive, restore, delete)
  appends one `ticket.created`, `ticket.moved` or `ticket.updated` event
  with run, agent and `by` set to `human`, naming the ticket and at most the
  columns or fields changed, never their values or any ticket text
  (agent-protocol §3). With answered questions, this is your latest board
  activity, which the server can read per project and across projects for
  the coming Overview. Edits made in an editor do not count, agents' writes
  are unchanged, and the human is never shown as a run.
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
