# Changelog

All notable changes to this project are documented here. The project follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

- Agents can change a workstream after creating it, and file tickets on
  another project's board (FH-46, D69, `MCP-2`). The new
  `update_workstream` tool sets a workstream's title, goal, priority,
  tags and the workstreams it waits for, and adds and removes its tickets,
  in one write: each ticket's workstream and the workstreams' ticket lists
  stay in step, and a refused call changes nothing. Its short name (slug)
  stays when the title changes. A dependency set on a workstream holds
  back every ticket in it, including ones added later, so a whole line of
  work waits for another with one call instead of a dependency on each
  ticket; workstreams that would wait for each other in a circle are
  refused, naming the circle. `create_ticket` takes a project name or key,
  so a session in one repository can file a follow-up on another board.
  Agent protocol stays 2.
- Make workstreams and fill them on the board (FH-17, `EDIT-14`). **New
  workstream** in the Workstreams view asks for a title, a goal, a
  priority and the workstreams it depends on, writes the workstream file
  with a unique name and opens its page. A ticket's Workstream field on
  the side panel's Edit tab adds it to a workstream or takes it out, and
  the ticket and the workstream's list always change together. After
  **New ticket**, the new ticket opens on its Edit tab with the cursor in
  its title, ready for the details the dialog does not ask.

- See how many agents are live in each project on the All projects
  Overview (FH-162, `PRJ-6`). The Projects list has a Live count beside
  Needs you: the project's running agents that do not need you (working,
  waiting or quiet), subagents included; those that need you count under
  Needs you instead. A project with none
  shows a faint dash; on a phone it reads *N live* after the tickets and
  blocked counts.
- Compare projects at a glance on the All projects Overview (FH-153,
  `PRJ-6`). Under the pipeline, a Projects list shows every project, most
  recently active first as in the sidebar, with its key and name, its
  tickets, its count in each board column, and how many tickets are
  blocked or need repair and how many agents need you; a repair count is
  ringed in dashes, a Needs you count sits on the attention colour and a
  zero is a faint dash, so the counts that matter stand out.
  Choose a row to open that project's Overview. On a phone, or beside an
  open ticket, each project's counts stack under its name. One project's
  Overview has no list.

- Edit a workstream's goal and notes on its own page (FH-139,
  `EDIT-13`). A workstream's name on the Workstreams view now opens its
  page (a finished line has an *Open* link beside its head): its line of
  stations, then its Goal, Scope, Success Criteria, Notes and any
  sections of its own. Each edits in place in the same formatted editor
  as a ticket's sections, with the slash menu, the selection toolbar and
  your Formatted / Markdown choice. Saving rewrites only that section and
  an untouched save writes nothing; a change someone made elsewhere in
  the file saves without asking, and a change to the same section shows
  both versions to choose from.

- Delete a comment you posted by mistake (FH-140, D66, `STO-8`). Beside
  a comment's pencil, on the ticket's full page and the side panel's
  Comments tab, **Delete comment** asks first, then removes it from the
  ticket; the history records that you deleted a comment, never what it
  said. A comment sent to a session that has not taken it yet is
  withdrawn, so the session never reads it. Once a session has read a
  comment, or while it is a change request not yet resolved, it stays,
  and the control says why.

- Pasting from a web page keeps its headings, lists and links (FH-142,
  D68, `EDIT-12`). Copy part of a page, a document or an issue and paste
  it into a section's editor: headings, bold and italic text, code, links,
  lists (nested and numbered), task boxes, quotes and tables arrive as
  markdown. Headings start at level 3, so a pasted heading never splits
  the section. Pasting is safe: nothing on the copied page is loaded or
  run, images from the web become links to them, and links other than
  web, email and the ticket's own files keep only their words. To paste
  plain text instead, press Ctrl+Shift+V (Cmd+Shift+V on a Mac).
- Look closely at screenshots (FH-146). The lightbox that opens a
  ticket's screenshots now fills the window, less a small margin, at
  every width, and zooms from 25% to 800% of the image's actual size:
  with **Zoom in**, **Zoom out** and **Fit** under the image beside the
  zoom level, Cmd or Ctrl and + or −, or the mouse wheel or a trackpad
  pinch, which zoom around the pointer. Drag the image, or use the arrow
  keys while zoomed in, to move around it; at Fit the arrows still step
  between screenshots. 0 (or Cmd or Ctrl and 0) fits the whole image
  again, and every screenshot opens fitted, never enlarged past its
  actual size.
- Paste a screenshot or drop files straight into a ticket (FH-18, D65,
  `REV-7`). In a section's editor on the full page, paste a screenshot
  from the clipboard, drop files, or choose them with **Attach files**:
  each file is stored in the ticket and its image or link goes in where
  the cursor (or the drop) was, with a placeholder while it uploads. The
  reference is saved with the section, like anything you type; a file
  you added and then cancelled the edit on stays in the ticket's files.
  Drop files on the side panel's **Attachments** tab or the full page's
  **Files**, or use their **Attach files**, to add them without touching
  the text. PNG, JPEG, GIF, WebP, PDF, text, markdown, JSON and log files
  up to 20 MB are taken, judged by what they contain rather than their
  name; anything else is refused with the reason beside it. Links to a
  ticket's own files in its text now open the file.
- Send a comment straight to the session working on a ticket (FH-127,
  D47, D63, `CARD-7`). In the comment box, **Tell a session now** also
  hands the comment, as your own words, to a live session that has
  claimed the ticket, picked from those sessions with their subagents
  under them; with none, the option is off and says why, and nothing is
  ever queued for a later session. The session gets it with its next
  prompt, Flashheart tool call, `board_context` or `flashheart await`,
  as the comment reads then, so an edit made before that is what it
  sees. The comment then shows *Delivered to <run> at <time>* and can no
  longer be edited; if the session ends first, it stays on the ticket
  and says it was not delivered. Agent protocol: the inbox entry gains
  an optional `project` and comments gain `sent-to` and `delivered` in
  `activity.md` (additive within version 2). Deleting a comment is not
  offered yet.
- Sections on a ticket's full page edit in a formatted editor instead of
  a markdown text area (FH-16, D61, D62, `EDIT-12`). Headings, lists,
  emphasis, code, quotes, links, ticket ids and the ticket's own images
  show formatted, as the page shows them, and only the line you are
  editing shows its markdown. Task boxes tick with a click, or Alt and
  Enter on their line, which also opens a link or ticket id (as Ctrl or
  Cmd and a click does). Type `/` for a menu of blocks; select text for a
  formatting toolbar (Tab or Alt and F10 reach it), or use Ctrl or Cmd
  with B, I, E or K. A **Formatted** / **Markdown** switch shows the text
  as written, and the choice is remembered. Nothing you did not change is
  reformatted: the text saved is exactly the text edited, so agents' text
  keeps every character. Paste is plain text, raw HTML is never rendered,
  and the editor loads only when you first edit, so the board loads no
  more than before. Built on CodeMirror 6 (MIT).
- Comment on a ticket from the board's side panel (FH-128, D47,
  `CARD-1`). The panel has a **Comments** tab, counting the ticket's
  comments, with the same comments and comment box as the full page:
  post with **Comment** or Ctrl or Cmd with Enter, quote part of the
  ticket, edit your comments in place, and get the conflict dialog when
  a comment changed meanwhile. A comment you are writing stays while you
  look at another tab, and closing the panel, pressing Escape or opening
  another ticket with it unposted asks first.

- Ask for changes in review and see which ones the agent has addressed
  (FH-126, D47, `REV-6`). On a ticket in review, **Request changes** (in
  its properties) sends comments you pick, a new one, or both back as
  change requests and moves the ticket to In progress with its branch and
  pull request kept. A live session holding the ticket gets them with its
  next prompt or Flashheart tool call; otherwise the next session's
  recovery note and `board_context` name them from the ticket, so nothing
  waits on a session that has gone. The agent marks each one addressed
  with `update_ticket`'s new `addressed` input (additive within protocol
  2; no setup rerun). The ticket page lists the requests at its top with
  their counts, each with **Resolve** and **Reopen** for the next review,
  comments in Activity show their state, the card counts the requests
  still waiting, and moving the ticket to review warns, naming each open
  request, without refusing. Every request, address, resolve and reopen
  is in the ticket's history.
- Comment on a ticket from its full page (FH-125, D47, `CARD-7`).
  Activity ends with a comment box: write a comment and post it with
  **Comment** or Ctrl or Cmd with Enter, optionally quoting a section, an
  acceptance criterion or a How to verify step picked from the ticket.
  An empty comment cannot be posted, a failed post keeps your text with
  the reason, and leaving the page with a comment not posted asks first.
  Your comments edit in place, show as edited, and an edit over a newer
  version opens the conflict dialog. Activity now also shows the ticket's
  lasting history (moves, edits, ticks, archive and restore, older than
  the run window too) beside notes, handoffs and questions, oldest first,
  and filters **All**, **Comments** or **History**, with the outline's
  count following the filter. Agents get the comments with `get_ticket`
  and `claim`, framed as the user's own words apart from the ticket's
  text and kept within each tool's output budget, and the flashheart skill
  says how to read them (additive within protocol 2; no setup rerun).
- A ticket's full page edits in place (FH-66, `EDIT-11`). Hovering or
  focusing the title, a section or a property shows a pencil; the pencil,
  a double-click or Enter opens just that item, and Save, Cancel,
  Ctrl or Cmd with Enter, and Escape close it, with focus back where it
  was. Properties use typed controls: selects for priority and type, a
  workstream picker, tags as chips, depends on as a ticket picker, branch
  and pull request as text; Move to keeps asking before starting a
  blocked ticket. Sections edit as markdown, an empty one offers
  **Add <section>** in its place, and the acceptance criteria edit as a
  list you can add to, reword, reorder and trim, while ticking stays one
  click. Handoff, the review and Activity stay read-only. One item is
  open at a time: switching items or leaving the page with unsaved
  changes asks whether to keep or discard them. A save changes only that
  field or section and leaves every other byte of the file as it was; an
  untouched save writes nothing; a change to the same item on disk opens
  the conflict dialog, while an agent's change elsewhere in the ticket
  does not; a failed save stays open with the reason. The card panel
  keeps its Edit tab.
- Each ticket keeps its comments and a lasting change history beside it
  (FH-124, D47). A new `activity.md` in the ticket's folder holds one
  dated, typed entry each: comments people write on the board, and the
  history every ticket write by the board or an agent appends in the same
  write (created, moved, which fields and sections were edited, which
  criteria and review steps were ticked, archived, restored; who and when,
  never the ticket's text). History outlives the event log's retention;
  tickets without the file need no migration. The ticket API returns
  `comments` and `history`, and `POST /api/tickets/{id}/comments` and
  `PATCH /api/tickets/{id}/comments/{comment}` add and edit a comment
  (shown as edited, refused over a newer version), recording the human's
  new `comment.added` and `comment.edited` events. Comments are scrubbed
  of secrets; agents never write them. The page to comment from comes
  with FH-125 (protocol version unchanged).

- Codex's Flashheart instructions stay current after an upgrade (FH-121,
  D53). At the start of each Codex session, Flashheart's hook rewrites the
  Flashheart section of your global `AGENTS.md` when it is out of date, as
  Claude Code's skill already was, and tells the session so it reads the
  section again. Only the text between the markers changes, atomically and
  through a symlinked file to its target; your text around it stays byte
  for byte. A file without the section never gets one, and a file whose
  markers are broken is left alone for `doctor` to report. A failure is
  logged and the session goes on (protocol version unchanged).

- One command connects Codex to Flashheart, and `doctor` checks it (FH-75).
  `flashheart setup codex` shows, as a diff, the `[mcp_servers.flashheart]`
  table it adds to `config.toml`, the hooks it merges into `hooks.json` and
  the protocol text it puts between markers in the global `AGENTS.md`, all
  in `$CODEX_HOME` (default `~/.codex`); `--write` applies them with
  backups and says to trust the hooks with `/hooks` in `codex`, and
  `--uninstall --write` takes out only Flashheart's entries and text.
  `config.toml` is edited as text, so comments, project trust and other
  servers stay byte for byte. `doctor` gains a Codex section: a missing or
  stale MCP entry, hooks or protocol text, hooks turned off or never
  trusted, each with its fix; without a Codex home it reports nothing to
  check. The shared protocol text now says plans are mirrored only where
  the hooks are installed (protocol version unchanged).

- A Codex session's claims, checkpoints and questions land on that
  session's own run (FH-74, D45). Codex names its session in every MCP
  call, so the server credits the call to the run its hooks record, never
  to a second `codex:mcp-…` run, and two Codex sessions in one worktree no
  longer get `ambiguous_run`. A Codex session without hooks gets a run
  under the same session id, so when its hooks start mid-session the board
  keeps one claim holder, one Needs you entry and one Overview row, and
  stops calling it "MCP only". When `ambiguous_run` lists runs whose short
  ids look the same, it names them in full (protocol version unchanged).

- Codex sessions with Flashheart's hooks are recorded on the board like
  Claude Code ones (FH-73). `flashheart hook codex <Event>` turns Codex's
  hook events, mapped from payloads recorded from the Codex CLI and the
  ChatGPT app (Codex 0.160.0), into run starts, turns, tool activity with
  shell windows, every file an `apply_patch` edits, permission requests,
  subagents, compaction and run ends; an interrupted turn ends as a stop
  does. A new session gets the recovery note and a prompt the waiting
  answers, and a stop is blocked for a handoff where the project enforces
  it (an interrupt never is). `flashheart setup codex` installs the hooks
  (FH-75); Codex's own `update_plan` is mapped to the plan but
  was not offered in any recorded session (protocol version unchanged).

- A ticket's pull request shows on its card panel and full page (FH-114).
  Tickets gain a `pr:` frontmatter field beside `branch:`; agents set it
  with `update_ticket` when they open a pull request, as the flashheart
  skill now asks, and you can edit it on the Edit tab. An `http(s)` address
  shows as a link (*PR #7* for a numbered pull request) that opens in a new
  tab; anything else shows as plain text (protocol version unchanged).

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

- Files sent from the browser into a ticket (FH-18, `SEC-7`) go through
  the board's authenticated session only, are read no further than the
  20 MB limit, and are judged by their bytes: SVG, HTML and XML are
  refused whatever they are called, and a file is stored under the
  extension of what it is, so it is served as that, with the attachment
  headers that stop a browser running it. Names are reduced to letters,
  digits and dashes inside the ticket's `files/` folder, and nothing is
  written unless the whole file is accepted.
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

- Dialogs open with the cursor where you type first (FH-17): New
  ticket's title, the reason for starting a blocked ticket and a new
  change request lost focus to the dialog's close button as it opened.
- Saving a section right after an edit keeps that edit (FH-142). Pressing
  Ctrl+Enter (Cmd+Enter on a Mac) a moment after pasting or typing in a
  section's editor could close it without saving the change, as the page
  had not yet heard of it; the editor now reports each change at once.
- Codex sessions started close together no longer look identical on the
  board (FH-122, D67, agent-protocol §2). Codex's session ids start with
  the time, so two sessions started in the same minute both showed as,
  say, `codex:019a7c00` on the Overview, in the Runs tab, on cards and in
  a ticket's history, and passing that run id from the recovery note
  answered `ambiguous_run`. A time-ordered id (UUIDv7) is now shortened to
  its last 8 characters, its random part (`codex:00000001` and
  `codex:000000ff`), and that short id names its one session when passed
  as `run`; the start of a full id is still accepted. Claude Code's short
  ids are unchanged. The server now sends the short form of history and
  comment authors (`byShort`) and of the session a change request went to
  (`sentToShort`), so the frontend no longer shortens run ids itself.

- Changes on the side panel's **Edit** tab are no longer lost without
  warning (FH-138). They now stay while you look at another tab, and
  closing the panel, pressing Escape or opening another ticket with them
  unsaved asks first (*Unsaved changes to the ticket*), as it does for a
  comment being written: **Discard changes** drops them, **Keep editing**
  returns you to the Edit tab with focus where you were.
- Building after a pull that adds or upgrades frontend packages no longer
  fails on missing modules (FH-119). `scripts/build.sh` (and so
  `scripts/e2e.sh`) and `scripts/e2e-visual.sh` installed the packages only
  when none were installed, so an install older than the lockfile was
  reused. They now share `scripts/frontend-deps.sh`, which reruns `npm ci`
  whenever the lockfile's checksum or the Node major version differs from
  the last successful install; an up-to-date install is reused.
  `scripts/check.sh` still always runs `npm ci`.

- Undo after two quick moves of a card now undoes the latest one (FH-115).
  While the second move was saving, the first move's toast still offered
  Undo with the same words, and choosing it put the card back where it was
  before both moves. A move's toast now goes as the next move starts, and
  a move saved after a later one started is announced without Undo.
- Keyboard focus no longer jumps to the top of the page when the board
  changes under it (FH-84). A focused card that a live reload or a filter
  moves to another column, an Overview row that moves to another section
  and a Runs-tab subagent row whose session moves down the list keep
  focus; one that disappears hands focus to the next item in its list,
  else the previous one, else the column or section heading. Focus in a
  dialog, the panel or the search box stays put. The board, the Overview,
  the Runs tab and independent stations share one way of doing this.
- Codex sessions are no longer told how Claude Code waits for an answer
  after `ask_human`; only Claude runs see the `run_in_background` hint.
  `update_ticket`'s description now lists `depends-on-workstreams`, which
  it already accepted, and a test keeps the skill and the protocol spec's
  tool table in step with the tools the server offers (FH-94).

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

- A Detailed card ends on its row of markers (type, id, blocked, change
  requests, live run, age, review, attachments, priority) instead of
  carrying it under the title (FH-168): the title leads straight into the
  blocker's reason, the run, the next step, the excerpt and the
  workstream, and every card ends on the same row. The Needs you line
  stays under the title. Compact and Normal cards do not change.

- Moving on while a save is still on its way no longer asks about
  unsaved changes (FH-156, D70, `EDIT-11`). Once you press Save (or
  Comment), leaving the page, closing the side panel or opening another
  item goes ahead at once. If that save then fails or meets a newer
  version after you have moved on, a message at the foot of the screen
  says what was not saved and why, and **Back to the edit** takes you
  back to it with your text, or to the side-by-side conflict view.
  Closing or reloading the browser tab still asks until the save's
  answer arrives.
- A slim project sidebar gives the board room for another column
  (FH-150). The sidebar is now a narrow column of project key badges at
  every width, whether or not a ticket is open, so at 1440px the Done
  column fits beside the others. A badge keeps what needs you: its Needs
  you count on the corner and a dashed, hatched mark while a ticket in the
  project needs repair. Hover a badge, or listen to it, for the project's
  name and its tickets, in progress, blocked, needs-repair and needs-you
  counts. On a phone the Project picker is unchanged.
- Board cards are less cluttered, and each density shows clearly
  different detail (FH-147, `VIEW-6`). **Compact** is just the title, led
  by its id, with what needs attention (Needs you, blocked, change
  requests) and the priority beside it. **Normal** adds one line of small
  markers: the type's icon, the id, blocked, change requests, a working
  session with its plan (2/5), age, review, attachments and priority; when
  a session or question needs you, why says so on its own line.
  **Detailed** adds the words: the blocker's reason, the agent and its
  plan step, the next step, a short excerpt and the workstream. Priority
  is now three small bars (high three warm, medium two grey, low one
  blue), and each type has its own icon in its colour, shown on the type
  chips at the top of the board too. The workstream initials and the
  acceptance criteria count are gone from cards (the coloured stripe
  stays), and Colour by tints the whole card. On a narrow column the
  marker line drops attachments, then review, then age rather than
  wrapping; every marker's words are in its tooltip and in what a screen
  reader hears for the card.
- Type and priority look the same wherever a ticket appears (FH-154). The
  Table, the archive, the side panel's header and a ticket's full page
  show the card's type icon and priority bars beside the words, and with
  Colour by Priority the chips at the top of the board carry the bars, so
  the marks you learn on cards mean the same everywhere.
- Colour by Priority tints cards in the priority bars' own colours (FH-166):
  high warm orange, medium neutral grey and low blue, in place of red,
  amber and green, so a priority has one colour code on the board. The
  priority chips show the bars and the word, without a separate colour
  dot. Wherever a priority is chosen from a dropdown (the Priority
  filter's menu, New ticket, New workstream, the Edit tab and a ticket
  page's Priority), the bars sit before the word.
- Blocked tickets are marked with a no-entry sign (FH-148). The small
  hollow diamond that marked a blocked ticket read as decoration at card
  size; a filled no-entry sign now marks blocked everywhere: on cards,
  blocker pills and notes, the side panel and full page (Blocked by), the
  blocked-move dialog, the Table, the Workstreams view and the State
  filter's Blocked. It always sits beside the
  word or the reason.
- Theme, board root and version are behind a gear in the title bar
  (FH-149, `CFG-2`). The Settings button sits between the connection dot
  and Quit on every view and on a ticket's full page, at every width. Its
  menu holds the theme choice (System, Light, Dark), still saved with your
  other preferences, the board root and the version line, which no longer
  take the foot of the project sidebar. The Board's View options keep the
  per-view settings.
- `flashheart doctor` tells you which Codex hooks need trusting again
  (FH-120, D64, `SET-4`). Codex skips a hook unless you trusted its exact
  command with `/hooks`, so after setup moves the binary or changes
  `--root` the ChatGPT app silently stops recording sessions. Doctor now
  compares the trust Codex saved in `config.toml` with Flashheart's hooks
  as they are and names the events whose hook was never trusted or was
  trusted for an earlier command, instead of only warning when nothing
  was trusted at all. It still only reads files and never runs Codex.
- `flashheart setup` asks before it writes instead of needing a second
  run with `--write` (FH-129, D59). It shows the diff, then asks
  `Apply these changes? [y/N]` and applies them only on `y`; `--yes`
  applies without asking, for scripts, and `--dry-run` only shows the
  changes. With no terminal to ask in and no `--yes`, it only shows them,
  as before. `--write` still works as an older name for `--yes`, and
  doctor's fixes now say `run flashheart setup <agent>`.

- `setup codex` no longer installs a `PreToolUse` hook on Flashheart's own
  tools (FH-123). Codex's calls are attributed by the session they name
  (D45), so the hook did nothing but start Flashheart on every board call
  and ask to be trusted with `/hooks`. Running `setup codex` again
  removes the hook an earlier setup wrote, and `doctor` warns about a
  Flashheart hook on an event setup no longer registers.

- Each session has one inbox for what the board sends it (FH-135, D54).
  Answers to its questions, and from FH-126 and FH-127 comments and change
  requests, wait in `<project>/.flashheart/inbox/<agent>--<session>.jsonl`
  as typed entries, and every way they reach the agent (the next prompt, a
  hookless session's next tool result, `board_context`, the recovery note
  and `flashheart await`) puts each type under its own heading: answers as
  before, as information, comments and change requests as the user's own
  words. A delivered comment records the new `comment.delivered` event.
  The prompt hook still checks one file when nothing waits. Inboxes in the
  old `answers/` folder are still delivered, and `serve` moves them into
  `inbox/` at startup; retention expires both. Additive within protocol 2:
  nothing to re-run.
- A ticket's full page uses the width (FH-65, CARD-7). Instead of the card
  panel's tabs stretched to one column, it shows the whole ticket as one
  page: questions waiting for you, the handoff, blockers, criteria and the
  ticket's sections, then its review in the review's own sections, then
  Files and a read-only Activity timeline of notes, agents' questions with
  your answers and handoff updates, oldest first. Each screenshot appears
  once: the review's Evidence is a captioned gallery that opens the
  lightbox, and attachments the review does not show are under Files. From
  1280px an outline on the left lists the sections with their counts,
  marks the one in view and moves focus to the one you choose, and the
  properties (Move to, priority, type, workstream, branch, depends on, tags,
  agent, dates and Archive) stay in view on the right; narrower, a Jump to
  menu replaces the outline and on a phone the properties come first. The
  board's card panel keeps its tabs.

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

- The one-off upgrades from before Flashheart's own board format (FH-136,
  D55, D71). `flashheart migrate`, which converted boards in the old
  column-folder format, is gone, and so is the notice the board showed
  for such projects; a `config.yaml` that still says `version: 1` is now
  refused as invalid settings. `flashheart setup claude` no longer moves
  the kanban-tracker skill aside, and `doctor` no longer warns about it;
  `setup claude --uninstall` still puts back a kanban-tracker skill an
  earlier setup moved into its backup, so nothing you had is lost.
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
