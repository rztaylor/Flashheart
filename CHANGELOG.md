# Changelog

All notable changes to this project are documented here. The project follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Changed

- The protocol skill and `ask_human` tell agents to ask any question that
  ends their turn with `ask_human`, not only in chat, so the session shows
  in Needs you instead of Waiting (FH-31). Guidance only; protocol 1.
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

### Added

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
