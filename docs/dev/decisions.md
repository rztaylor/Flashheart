# Decisions

Durable product and architecture decisions, newest last. Each records what was
decided, the options considered and why.

## 2026-10-04 — D0: Name and licence

Flashheart (after the Blackadder character), binary `flashheart`, repository
`~/src/Flashheart`. MIT licence, Robert Taylor as copyright holder. The working
name AIshape was dropped before any code.

## 2026-10-04 — D1: Observe and coordinate; do not launch agents

Options: watch agents the user runs in their own apps; launch agents from the
board (as Vibe Kanban, Kagan and Kanbots do); both.

Decision: observe and coordinate. Launchers are an established category;
nothing yet gives one view of the sessions people already run in Claude Code
and Codex, with durable handoffs between them. The agent apps keep owning
sessions, credentials and permissions. Launching is a later possibility.

## 2026-10-04 — D2: Markdown tickets plus an append-only event log

Options: markdown only; markdown tickets and a JSONL event log; SQLite with
markdown export.

Decision: tickets stay markdown (readable in Obsidian, diffable, usable without
Flashheart); high-volume run activity goes to per-project daily JSONL files.
`serve` builds an in-memory index at start and keeps no database. Durable
meaning (handoffs, answers) is written into tickets so the log is disposable.

## 2026-10-04 — D3: A ticket's column is its directory (superseded by D15)

Options: directory (kanban-tracker format); a `status:` field.

Decision: directory. It keeps compatibility with the existing skill and with
Obsidian, and a move is a rename. Agent state lives on runs, so tickets need no
extra AI states. Adds `done/`. Virtual columns (Needs you, Agent working) are
computed and never stored.

## 2026-10-04 — D4: One binary, three modes, no required daemon

Options: every mode writes files through a shared store with locks; a daemon
that all writers go through.

Decision: `serve`, `mcp` and `hook` each use the same store package directly,
with a per-project advisory lock, atomic writes and content-hash
preconditions. Agents must work when the UI is closed, and hooks must not
depend on Singleserve authentication. `serve` watches the files.

## 2026-10-04 — D5: Hooks first, MCP for meaning, instructions last

Options considered for keeping the board current: model instructions only;
MCP tools; hooks; passive reading of agent session logs.

Decision: hooks carry every lifecycle fact deterministically; MCP tools carry
only what the model knows; the protocol skill explains the tools; an opt-in
Stop hook enforces a handoff after edits. Passive session-log reading is
deferred: formats change without notice and logs contain secrets.

## 2026-10-04 — D6: Project from the main checkout; ticket by claim

Project name is the main checkout's directory name (worktree-safe). Runs link
to tickets by explicit claim, provisionally by branch match, and subagents
inherit. Unlinked runs show as Unassigned rather than being guessed.

## 2026-10-04 — D7: A small MCP tool set

About ten tools with compact outputs and no delete. Remote MCP (ordinary
ChatGPT chat connectors) is rejected to stay local-only; Codex covers the
ChatGPT side as a coding agent.

## 2026-10-04 — D8: A separate Agents view plus virtual columns

Options: extra workflow columns for AI states; a separate run-state view.

Decision: run state is a different axis from workflow status. The Agents view
shows runs in lanes (Working, Needs you, Waiting, Quiet, Ended); the board gets
live badges and toggleable virtual columns (Needs you, Agent working).

## 2026-10-04 — D9: Setup shows a diff and writes only with --write

`flashheart setup <agent>` prints the exact configuration changes and writes
them only with `--write`, with backups, owning only its own entries. Editing a
user's agent configuration silently is not acceptable.

## 2026-10-04 — D10: Libraries

Go: `github.com/rztaylor/singleserve` v0.2.x; `github.com/modelcontextprotocol/go-sdk`
(official, v1.7+); `github.com/fsnotify/fsnotify`; `go.yaml.in/yaml/v3` for
YAML with node-level round-tripping; a small cross-platform file-lock library
or `golang.org/x/sys`. Frontend: React,
strict TypeScript, Vite, Tailwind CSS v4, `@dnd-kit` (accessible drag and
drop), `react-markdown` + `remark-gfm` without raw HTML; Biome, Vitest,
Playwright. Commodity parsing, drag and drop and markdown are not
hand-written.

## 2026-10-04 — D11: Safety model

Writes confined to the root; `attach` copies allow-listed types under a size
limit; hooks store names and scrubbed short summaries only; agent-written text
is data for other agents; no network beyond loopback.

## 2026-10-04 — D12: Board root outside repositories

Default root `~/reports/Kanban`, one subdirectory per project, so the board is
the user's own and is not committed into the repositories it tracks.
Overridable with `--root` or `FLASHHEART_ROOT`. Whether the root is itself a
git repository is left to the user for now.

Confirmed at `foundation` (2026-10-04): `go.yaml.in/yaml/v3` (v3.0.5, maintained
by the YAML organisation) rather than `gopkg.in/yaml.v3`, whose last release
was v3.0.1 in May 2022 and whose repository is archived. The API is the same.
Singleserve is pinned at v0.2.2, which requires Go 1.26.6.

## 2026-10-04 — D13: `serve` detaches from the terminal

Options: block the terminal until the server stops (Singleserve's example
shape); detach after launching the browser; a separate long-lived daemon.

Decision: detach. `flashheart` re-executes itself as a child in a new session
with discarded standard streams and waits on a one-line startup handshake (an
inherited pipe) until the server is listening and the browser launch was
attempted. The launcher prints the manual URL if the browser could not be
opened, then exits 0. The child keeps the browser-bound lifetime, so Quit or
closing the last tab stops it. `--foreground` keeps the old blocking shape for
debugging and tests. Singleserve leaves detached startup to the consumer's CLI
boundary, so it lives in `internal/background` and `internal/cli`. After the
handshake the child writes diagnostics (later errors, the standard logger used
by `net/http`, `--debug` summaries) to `<root>/.flashheart/serve.log`, created
on first write and rotated at 1 MB; a crash that bypasses Go's logger is still
lost, and `--foreground` shows everything.

## 2026-10-04 — D14: Board visual direction: Transit Line Map (superseded by D18)

Options (impeccable decision page, code-led because no image generation is
available locally): Transit Line Map (assigned by the concept roll), Flight
Progress Strips, Terminal Yellow Wayfinding, the category-standard kanban,
and five declined challengers.

Decision: Transit Line Map, chosen by the user. The structure stays a
standard kanban (the user pinned "standard kanban, but with style"); the
world supplies type, palette, density and one signature move: workstreams as
transit lines with tickets as stations. Line colours are reserved for
workstreams so they always mean the same thing; ticket state uses ink,
shape and words, which also keeps colour off the critical path for
accessibility. Waits on an earlier station are shown quietly so the blocked
diamond answers "what is stuck". Frontend additions under D10: self-hosted
`@fontsource-variable/archivo` (OFL) for the signage grotesk under the
strict CSP, and an authored icon set instead of an icon library.

## 2026-10-05 — D15: Board format v2: a folder per ticket, status in frontmatter

Context: the kanban-tracker skill will be replaced by Flashheart and Obsidian
is not in use, so compatibility with the v1 column-folder layout no longer
constrains the format (user, 2026-10-05). The board also needs a new column
(Up next) and a home for files agents refer to.

Options: keep column folders and add ids; a flat `tickets/` folder with the
status in frontmatter; a folder per ticket with the status in frontmatter.

Decision: a folder per ticket (`tickets/FH-42-<slug>/` holding `FH-42-<slug>.md`,
`review.md` and `files/`) with the column as a frontmatter `status`. Moves
become a field edit under the lock with a hash precondition instead of a
rename that races other writers; columns are a list rather than folders, so
Up next costs nothing; archiving or copying a ticket carries its review and
files with it; and a ticket's files can be copied in before agents clean
theirs up (REV-5). Tickets stay plain markdown. The columns are Backlog, Up
next, In progress, Ready to review and Done; v1 boards convert once with
`flashheart migrate`, which moves the old tree into a backup inside the root.
Supersedes D3 and the kanban-tracker compatibility in STO-1.

## 2026-10-05 — D16: JIRA-style ticket ids

Decision: every project has a short key (`FH`) in `project.yaml` and every
ticket an id `FH-42` in its frontmatter, assigned from `next_id` under the
project lock and never reused. The id lives in the file so it is greppable and
visible to agents reading files directly. Ids name tickets everywhere (UI,
URLs, events, MCP, dependencies), and because keys are unique across the
root, ids need no project prefix. Existing tickets are numbered once, oldest
first, by `flashheart migrate`. With `list_tickets` (MCP-7) and the caller's
project known from its working directory, a user can ask an agent for "the
three top-priority bugs" or "FH-42" in plain words.

Keys are chosen before a project's first ticket and then fixed (KEY-5).
Agents choose them, since they know what the project is called in
conversation: `board_context` asks while the key is unset and
`set_project_key` records it under a root-wide lock, so two new projects
cannot take the same key. The ticket file is named after its folder
(`FH-42-<slug>.md`) so editor tabs and search results identify the ticket
(user, 2026-10-05).

## 2026-10-05 — D17: Agent runs are derived on read; Needs you leads

Decision: runs are never stored. Hooks append facts to the event log and
every reader (serve's index, the session-start recovery note) folds them
into runs with a clock, so state transitions that only need time (Working
to Quiet, stale to Ended) need no writer, and a corrupt or deleted log
costs history, not tickets. Serve folds only the last two days of event
files, incrementally, and moves the board revision when the clock changes a
run's state. A pending permission is resolved in derivation by the run's
next tool result, prompt or turn end rather than by an extra event, so the
hot path never reads the log. `turn.start` repeats the working directory
and branch, so a session first seen mid-way (hooks installed while it ran)
still has them.

In the UI, the Agents view is a departure board with Needs you as its first
lane, not the spec's original order, because the one thing that needs the
human must be first (PRODUCT principle 2). Virtual columns sit before
Backlog and appear only while they hold tickets, so a board with nothing to
flag stays calm. Run state follows the ticket-state rule: ink, shape and
words, with Needs you as the one inverted plate and a slowly beating dot as
the board's only motion (user, 2026-10-05).

Rejected: storing run state in a file (a second source of truth that hooks
would have to rewrite under a lock on every event); a run-state daemon
(D4, no required daemon).

Dogfooding (user, 2026-10-05): Flashheart's own work is tracked on its board
(project `Flashheart`, key `FH`) in the default root, outside the
repository (D12).

## 2026-10-06 — D18: Metro Pop structure with light and charcoal palettes

Decision (user): adopt Metro Pop as the light-theme direction and Night
Service in black/neutral charcoal as the dark-theme direction. Standardise
the actual layout and contents of cards, workstreams and other surfaces
using Metro Pop. Both themes share structure, information and workflows;
Night Service references supply the dark palette, not a second layout.

The user rejected navy/dark-blue background surfaces. Keep colour in route
identities and deliberate accents, with neutral charcoal dark surfaces.
Garden Line and the original navy Night Service were considered but are
not selected. Generated sample content and inconsistent state/colour details
are not requirements; SPEC remains the functional authority.

This supersedes D14's visual direction; the rollout (FH-10–FH-14) shipped
it, and DESIGN.md records the built system. Approved images, provenance,
dependencies and validation are linked from
[`metro-theme-rollout`](roadmap-items/metro-theme-rollout.md); the shared
contract is `docs/dev/specs/ui-layout.md`. No protocol/storage change or
cloud deployment is implied.

## 2026-10-06 — D19: Setup registers MCP through the claude CLI and retires kanban-tracker

Options for registering the user-scope MCP server: edit `~/.claude.json`
directly; run `claude mcp add-json` / `claude mcp remove`; print the command
only. Decision (user): run the claude CLI, and print the command when none is
found. `~/.claude.json` is a large file running sessions rewrite constantly,
so an edit from outside could be lost or lose theirs; the CLI owns it.

The kanban-tracker skill would give agents a second, conflicting set of
ticket rules. Decision (user): `setup --write` moves it into setup's backup
and `--uninstall` puts it back, rather than leaving it in place with a note.

`setup` edits `settings.json` preserving key order and the file's indent,
owns only hooks whose command runs a flashheart binary's `hook`
subcommand, and restores the backed-up file byte for byte on uninstall when
nothing changed since. `--uninstall` previews like setup does and applies
with `--write` (D9).

## 2026-10-06 — D20: Answers reach a session through an inbox

The prompt hook must stay fast (`HOOK-1`) and never reads the event log
(D17), but it must hand a session the answers to its questions (`HOOK-5`).
Decision: answering appends `question.answered` and queues the answer in the
asking session's inbox, `<project>/.flashheart/answers/<agent>--<session>.jsonl`;
the prompt hook checks for that file (one `stat` when nothing waits),
empties it and records `question.delivered`. The recovery note and
`board_context` deliver from the inbox and the log, so nothing is lost if
the inbox is. The inbox is a delivery queue, not a store of meaning: the
answer lives in the log and the ticket's `## Notes` (`STO-6`). A question
stays Needs you until its answer is delivered, because delivery needs the
human to prompt the session.

Run attribution (`MCP-4`): Claude Code's `PreToolUse` hook may rewrite a
tool's input without deciding its permission, so the hook stamps the run
into Flashheart's own tool calls. Claude Code starts user-scope MCP servers
in `~/.claude`, so the fallback reads the project from
`CLAUDE_PROJECT_DIR`.

## 2026-10-06 — D21: Theme rollout no longer waits for review-and-orchestration

Decision (user): implement `metro-theme-rollout` (FH-11–FH-14) now, without
waiting for `review-and-orchestration` (FH-6). The surfaces being styled
exist: the card panel's Review tab shows the review and its attachments.

Options: wait for FH-6 so every review surface exists before styling (the
order D18 set), or theme now and have FH-6 build its new surfaces
(Attachments tab, lightbox, *How to Verify* checklist, subagent tree) to
the shared contract in `docs/dev/specs/ui-layout.md`. The second was
chosen: the contract already specifies those surfaces, so the themes need
no second pass, and FH-6's pieces are not built under the theme tickets.

## 2026-10-06 — D22: Light theme on white with a raspberry bar

Decision (user): the light palette drops the references' Metro-blue band and
cream grounds, which read as old-school web, for a white background with a
strong-coloured bar that is not blue ("everyone uses blue"). From rendered
options (emerald, raspberry, violet) the user chose raspberry (#be185d). It
is light's one accent: the bar, the primary action, the current project,
selection and focus. Grounds are white and cool light greys. Night Service
(dark) is unchanged. Structure is unchanged (D18, `ui-layout.md`); only light
tokens moved, and the palette test still holds every pair to WCAG AA.

## 2026-10-06 — D23: Manual card order lives in each ticket's `rank`

Decision (FH-23): a ticket's place in its column is an optional `rank`
frontmatter field holding a fractional index, not a per-project order file.
Ranked tickets come first; unranked ones keep the old default order
(priority, created, id).

Options: an order file per project listing ids per column, or a key on each
ticket. The order file is one write per reorder but drifts from the tickets
(external moves, archives, new tickets) and is a second source of truth for
a ticket's place. A per-ticket key keeps the ticket file the source of
truth (D12, STO-3), survives archive and restore, and a fractional index
means a placement normally writes one file under the existing lock and
hash rules. Unranked tickets are ranked only when a placement lands among
them, in their current order, so nothing else moves. Done keeps recency
order, which is what people look for there, and is not reordered.

## 2026-10-06 — D24: Tickets can be deleted, in two steps

Decision (user, FH-20): reverses "Flashheart never deletes a ticket". Archive
stays the normal, undoable action; a ticket can be deleted permanently only
once archived, from the project's Archive view, after a confirmation that
lists what the delete touches and asks for the typed id. Board files have no
history (D12), so the two steps keep accidents rare.

The delete removes the id from other tickets' `depends-on` and from
workstream lists in the same locked operation, because a reference to a
missing ticket counts as blocking. Ids stay retired: a `retired:` list in
`project.yaml` joins existing and archived numbers in the next-id
safeguard, so a lost or edited `next_id` cannot reuse a deleted id. The
confirmation's preview carries a token over everything it lists; the
delete is refused when any of it changed. Agents get no delete tool.

Danger red marks the delete (user, 2026-10-06): errors and actions that
cannot be undone are red. The row's Delete permanently is red text and the
confirmation's button a red fill (`DESIGN.md` Danger role).

## 2026-10-06 — D25: Projects archive and delete like tickets

Decision (user, FH-21): a project can be archived from the UI and deleted
permanently only once archived, the same two steps as tickets (D24).

- Keys are retired, not released. Ids are global across the root and may be
  referenced from other projects' text and logs, so a deleted project's key
  goes into `<root>/.flashheart/retired.yaml` and is never given to another
  project. An archived project's key stays taken.
- References after a delete are removed, as for tickets: the confirmation
  lists other projects' tickets that depend on the project's tickets, and
  the delete removes those `depends-on` entries, so nothing is left blocked
  on a missing ticket. Prose mentions stay as text.
- Agents in an archived project's repository: with `auto_create_projects`
  on, their next hook or MCP call would otherwise recreate an empty project.
  Instead hooks no-op quietly (fail-open) and MCP tools answer
  `project_archived` without creating anything. After a permanent delete
  the repository is unknown again, so activity there starts a new project,
  with a new key because the old one is retired.


## 2026-10-07 — D26: Workstreams are epics; only depends-on orders tickets

Decision (FH-38, user): a workstream groups tickets toward a shared goal,
like an epic, and its tickets may be worked on in sequence or in parallel.
Its `tickets:` list sets membership and display order and never blocks
(board-format §Blocking lost the old rule 3, "an earlier ticket in the
workstream"). Sequence is expressed only by `depends-on`, which may name
several tickets, and by `depends-on-workstreams`. A workstream is blocked
only when its own workstream dependencies are unmet or every unfinished
ticket is blocked, and its next ticket is the first unfinished one that is
not blocked. The Workstreams view draws each workstream as a railway graph
of its tickets' depends-on links (ui-layout.md §4).

Options: ordered by default with an opt-out (the 2026-10-06 plan in FH-17);
unordered by default with an `ordered: true` opt-in (built first in FH-38);
no ordering at all. The user chose the last: "an ordered workstream isn't
really a thing - it's just that each ticket has a depends_on for another
ticket." List order made every grouped ticket wait for the one before it,
even when the work was independent, and an opt-in would keep two ways to
say the same thing. Drawing depends-on as track shows real sequence,
parallel work and joins, which a line in list order could not.

Versions: board format stays 2 and `PROTOCOL_VERSION` stays 1. No field is
added or removed; tool inputs and outputs keep their shape, and blocked
reasons lose the "Comes after … in workstream …" text, which no tool
contract names. Existing workstreams change meaning, which the user
accepted: a chain that relied on list order needs `depends-on` on its
tickets.

## 2026-10-07 — D27: Agent working is a filter, not a column

Decision (FH-42, user): the Agent working virtual column is retired. A
ticket mirrored in Agent working while it sat in In progress read as being
in two states at once. The card already carries the Working mark, and the
State filter gains an *Agent working* option (tickets linked to a Working or
Quiet run). Needs you stays the one mirrored column, because it is a to-do
for the user rather than a status (VIEW-2).

Options: keep both mirrored columns; drop both and filter instead; group
working cards at the top of In progress. The user chose to drop Agent
working only.

Versions: board format stays 2 and `PROTOCOL_VERSION` stays 1; the card's
`agentWorking` field is unchanged. A saved `virtual_columns` entry
`agent-working` is dropped on load rather than refused.

## 2026-10-08 — D29: Serve copies linked files only from the project's repository

Decision (FH-34): `serve`'s REV-5 copying of files linked from directly
edited tickets and reviews copies a file only when it is in a git checkout
of the ticket's project's repository and git does not ignore it (SEC-6).
A board shared through git, Dropbox or iCloud gets other people's edits,
and serve cannot tell `git pull` from a text editor: both are an mtime
change. Unrestricted, a collaborator's `[x](/Users/me/Documents/statement.pdf)`
would be copied into the board and published by the next sync.

The repository has to be verified on this machine. The MCP call does not
help (serve acts on file changes, not calls), and `project.yaml`'s `repos`,
`config.yaml` and the cwd cache are all inside the synced root, so a
collaborator could widen any allowlist or switch kept there. Serve
therefore resolves the linked file's own checkout with `gitinfo` and
requires that the project records that main checkout and that its name maps
to the project (PRJ-2, PRJ-3) through `store.FindProject`. A forged repos
entry for a folder that is not a checkout, or for another repository, fails
on disk. Gitignored files (local configs and credentials) and files inside
`.git` are refused, because a repository holds private files git never
shares. Checkouts on other machines have other paths, so a collaborator's
links do not resolve here and stay as text; links the user's own serve and
agents copy are rewritten to `files/` before they sync.

Remaining risk: a collaborator who knows a local repository's path and
creates a board project of the same name, listing that path, can pull its
non-ignored files in. The MCP side (`attach`, `checkpoint`,
`write_review`) is unchanged: its paths come from the user's own agent,
whose screenshots often sit outside the repository (temporary folders), the
case REV-5 exists for. Linking a temporary file from a text editor no
longer copies it.

Options: opt-in `copy_linked_files` (the switch lives in synced config);
temporary or screenshot folders only (machine-specific, and the editor case
rarely needs them); images only (a screenshot can be as private as a PDF,
and any path stays reachable); asking in the UI (a prompt for something
that can be safe by default); the project's repository (chosen).

Versions: board format stays 2 and `PROTOCOL_VERSION` stays 1; the agent
protocol is unchanged. Serve now runs `git check-ignore`, the only place
Flashheart runs git; hooks still run none.

## 2026-10-08 — D28: A prompt in the asking session answers its questions

Decision (FH-43): when the user prompts the session that asked
(`UserPromptSubmit`) before answering on the board, its unanswered
questions, and its subagents', are answered in the session: they stop
waiting, leave Needs you and are no longer shown as questions for you,
while the run's question list and timeline keep them, marked. A question
already answered on the board keeps waiting for delivery, which that same
prompt's hook makes. Another session's questions are untouched. Users often
answer in the chat, and the open question had kept the session, and every
ticket it touched (including ones in Review and Done), in Needs you for up
to twelve hours.

The answer is derived in the run fold from `turn.start`, as a pending
permission's resolution is, rather than recorded as a new event: the prompt
hook stays at one `stat` when no answer is waiting, and older logs fold the
same way. Claude Code also submits a prompt when a background command
finishes (recorded: it starts `<task-notification>`), so its adapter marks
that `turn.start` `background: true`, which answers nothing. A board answer
to a question answered in the session is refused (`answered_in_session`),
and `flashheart await` exits 0 saying the user answered in the chat.

Options: a `question.answered` with `by: session` (an empty answer that
recovery notes and `board_context` would try to deliver); a new
`question.withdrawn` event written by the prompt hook (a log read on every
prompt); deriving it from `turn.start` (chosen). Any later prompt counts,
even one about something else, since the user is talking to the session.

Versions: `PROTOCOL_VERSION` stays 1 (additive: an optional `background`
field and a derived state rule); board format stays 2. The protocol skill
says a reply in the chat answers the question.

## 2026-10-08 — D30: Ticket ids open a full page in a new tab

Decision (FH-45): every ticket has a full page, `#/ticket/<id>` (CARD-7),
and every ticket id the board shows is a link to it that opens a new tab:
on cards, table rows, workstream stations, the Agents view, the archive,
the card panel's header, and ids of known keys in ticket and review text
(markdown, criteria, handoff next steps, blocking reasons). Clicking a
card, row or station still opens the side panel. On a full page, ticket
links stay in its tab. The user asked for a Jira-like page with room to
read, review screenshots and, later, edit (FH-16).

Ids in text used to open the side panel in place. They now follow the one
rule, "an id opens the full page", so a reference in a ticket behaves like
the id on a card; the blocked-by section keeps its Open button for the
panel. Links are real links (`href`), so Cmd or middle click and copying
the address work as anywhere else.

A link inside a button is invalid HTML and an accessibility failure, and
cards and stations are buttons that drag. They are now drawn under one
button that covers them, hidden from assistive technology (the button
carries the name, as before), with the id link above the button and off the
tab order; the panel header's id link is the keyboard route. Table rows and
the Agents view split the title button from the id link.

New tabs authenticate with Singleserve's session cookie, as reloads do; no
credential is in the URL. Each tab heartbeats; the server stops when the
last tab closes, as before.

Options: open the full page in the same tab (loses the board); a modal over
the board (no room gained, no URL to keep); an "open full page" button only
(ids would stay inert, which the user asked to change); ids open in new tabs
(chosen).

Versions: board format stays 2 and `PROTOCOL_VERSION` stays 1; no API
change.

## 2026-10-09 — D31: Needs you is the band's filter, not a column

Decision (FH-44, user): the Needs you virtual column is retired, completing
D27. A ticket mirrored in Needs you while it sat in its own column read as
being in two places at once, as Agent working did. The band's Needs you
plate becomes the filter: it is a toggle, and pressed, it shows All
projects' Board (or Table, when that is open) with only the tickets that
have a run in Needs you or an open question about them. It used to open the
Agents view. A second press returns to the project it was pressed in,
unless another scope was chosen since. It combines with every other filter,
and Clear filters switches it off. It stays in the band while pressed, so
it can be switched off once nothing needs you.

All projects, because the plate counts runs across every project: filtering
only the project in view left a "1 needs you" press on an empty board when
the run was elsewhere, contradicting the label (the user's review). Options
weighed: All projects and back (chosen); stay in the project when something
there needs you, else All projects (what a press does would depend on
something unseen); count only the project in view (the plate would stop
warning about other projects); keep the project with an empty state naming
the others (two steps to the answer).

The count and the board can still differ: a run in Needs you may have no
ticket on the board (an ad hoc session hitting a permission prompt, a
question asked before claiming, a branch matching no single ticket, an
archived ticket; RUN-5 allows unlinked runs). While the filter is on, a
notice names those runs ("1 agent needs you with no ticket on the board")
and opens the Agents view, which lists them. A run counts as shown when its
linked ticket, or the ticket one of its open questions is about, is on the
board. The counts gain `needsYouUnticketed`.

It is a lens, not a remembered filter: like the search text it carries from
project to project, and a reload starts with it off. The rail's badges and
the card's Needs you mark are unchanged.

With both mirrored columns gone, View options › Show columns holds only the
Backlog (FH-41).

Options: a *Needs you* State filter option, as Agent working (State takes
one choice, so it could not combine with Blocked); a chip in the chip row,
like the colour key's (built, then dropped: it repeated the plate); the
band's plate as the toggle (chosen); keep the column.

Versions: board format stays 2 and `PROTOCOL_VERSION` stays 1; the card's
`needsYou` field is unchanged. A saved `ui.virtual_columns` is ignored on
load rather than refused, and removed when preferences are next saved.

## 2026-10-09 — D32: An Overview for a project manager replaces the Agents view

Decision (FH-47): the Agents view (`VIEW-3`) gives way to an **Overview**
tab about tickets, with agent sessions as evidence. The user drew no value
from a list of runs: it answers "what is each agent doing", an operator's
question, where the user asks a project manager's: what needs my decision,
what needs my review, what is at risk, what is in progress and what changed.

The Overview is one calm column, in order: Needs your decision (questions
answered in place; a permission prompt names the session to answer it in,
since Flashheart never grants permissions), Ready for your review, At risk
(no live session, ended without a handoff, quiet, top of Up next blocked),
In progress (agent, state and subagent counts in words, "3 done · 2
running"), then collapsed Work with no ticket and Up next. Headline metrics
under the title count what happened since the user's last change on the
board (editor edits do not count). Session detail (tools, files, plan,
subagent tree) stays on the card's Runs tab.

Evidence from the event logs shaped it: no project has recorded a plan, so
plan displays are always empty; agents edit mostly through Bash, so edited
files and "no handoff" miss most edits; 21 of 35 sessions never claimed a
ticket; one `tool.used` per tool call is 78% of all events. Run information
is kept only where it becomes a fact about a ticket; the hooks move from
per-call events to throttled activity records.

Options: keep the lanes and tidy them; remove the view and spread its parts
over the band, cards and Runs tab; an activity feed of live sessions and
events; a ticket-centric Overview (chosen). For its look, Editorial (a
headline sentence and numbered stories), Calm (an inbox of section cards)
and Precision (a three-column dashboard) were generated, with a synthesis of
Editorial and Precision; the user chose Calm, found the others too busy and
rejected the headline sentence. Approved concept: FH-47
`files/20261009T1923-image.png`. Roadmap item: `project-overview`.

Built (FH-51): a question answered on the board stays under Needs your
decision, saying its answer waits for the session's next prompt, because
its run needs the user until then (`RUN-3`); a question from an ended
session does not. Section heads sit on the neutral column surface with a
round mark in their role's colour rather than the concept's tinted heads,
and rows open the ticket from its title rather than a separate Open
button. Old `#/…/agents` routes and a remembered `view: agents` in
`config.yaml` open the Overview, and the next save writes `overview`.

Versions: board format stays 2. `PROTOCOL_VERSION` changes only when the
throttled activity records land (FH-55).
