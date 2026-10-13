# Decisions

Durable product and architecture decisions, newest last. Each records what was
decided, the options considered and why.

## 2026-10-04 — D0: Name and licence

Flashheart (after the Blackadder character), binary `flashheart`, repository
`~/src/Flashheart`. MIT licence, Robert Taylor as copyright holder. The working
name AIshape was dropped before any code.

## 2026-10-04 — D1: Observe and coordinate; do not launch agents (revised by D48)

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

Refined by D47: a ticket's comments and change history are durable too, in
the ticket folder's `activity.md`, so they outlive the log's retention.

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

Revised by D51: hooks no longer record plans; a ticket's criteria and
handoff are its plan. D50 adds a reminder to claim for sessions that edit
without a ticket.

## 2026-10-04 — D6: Project from the main checkout; ticket by claim (revised by D50)

Project name is the main checkout's directory name (worktree-safe). Runs link
to tickets by explicit claim, provisionally by branch match, and subagents
inherit. Unlinked runs show as Unassigned rather than being guessed.

## 2026-10-04 — D7: A small MCP tool set (revised by D52)

About ten tools with compact outputs and no delete. Remote MCP (ordinary
ChatGPT chat connectors) is rejected to stay local-only; Codex covers the
ChatGPT side as a coding agent.

## 2026-10-04 — D8: A separate Agents view plus virtual columns (superseded by D27, D31 and D32)

Options: extra workflow columns for AI states; a separate run-state view.

Decision: run state is a different axis from workflow status. The Agents view
shows runs in lanes (Working, Needs you, Waiting, Quiet, Ended); the board gets
live badges and toggleable virtual columns (Needs you, Agent working).

Superseded: Agent working became a filter (D27), Needs you the band's
filter (D31), and the Agents view gave way to the Overview (D32).

## 2026-10-04 — D9: Setup shows a diff and writes only with --write (revised by D59)

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

Confirmed at `foundation` (2026-10-04): `go.yaml.in/yaml/v3` (v3.0.5, maintained
by the YAML organisation) rather than `gopkg.in/yaml.v3`, whose last release
was v3.0.1 in May 2022 and whose repository is archived. The API is the same.
Singleserve is pinned at v0.2.2, which requires Go 1.26.6.

Later additions: `@fontsource-variable/archivo` and an authored icon set
(D14), and the interaction-test libraries (D42). A rich text editor needs
its own decision before it is added (D56): CodeMirror 6 with a live
preview (D61), its packages in D62.

## 2026-10-04 — D11: Safety model

Writes confined to the root; `attach` copies allow-listed types under a size
limit; hooks store names and scrubbed short summaries only; agent-written text
is data for other agents; no network beyond loopback.

## 2026-10-04 — D12: Board root outside repositories

Default root `~/reports/Kanban`, one subdirectory per project, so the board is
the user's own and is not committed into the repositories it tracks.
Overridable with `--root` or `FLASHHEART_ROOT`. Whether the root is itself a
git repository is left to the user for now.

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

## 2026-10-05 — D15: Board format v2: a folder per ticket, status in frontmatter (migration retired by D55)

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

Refined: every reader now folds the same 48-hour window (D37). The Agents
view and the virtual columns this decision describes were retired (D27,
D31, D32); Needs you leading survives as the Overview's first section.
Dogfooding (user, 2026-10-05) is recorded in `.agents/facts/roadmap.md`.

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

## 2026-10-06 — D19: Setup registers MCP through the claude CLI and retires kanban-tracker (kanban-tracker handling retired by D55; --write revised by D59)

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

## 2026-10-06 — D20: Answers reach a session through an inbox (revised by D54)

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

Decision (user): the theme rollout (FH-11–FH-14) went ahead before FH-6
instead of after it (the order D18 set). FH-6 built its new review surfaces
to the shared contract in `docs/dev/specs/ui-layout.md`, so the themes
needed no second pass. Both have shipped; this was a sequencing decision.

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

## 2026-10-08 — D28: A prompt in the asking session answers its questions (refined by D41)

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
plan displays are always empty (D51 removes them); agents edit mostly through Bash, so edited
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

Built (FH-55): `PROTOCOL_VERSION` 2. Adapters still report every tool
result; hooks count them into an `activity` record at most once a minute
per run (`activity_seconds`), recording at once an edit of a path not
recorded in the turn or since the run's last checkpoint (each path once
per run per turn and again after a checkpoint: a prompt, a subagent's
start and the checkpoint tool's own result reset them; the user chose the
per-turn reset, 2026-10-11, so a missed checkpoint result costs at most
one turn) and the tool
result a permission request waits on, and flushing what is pending before
a turn end or end. FH-53's shell command windows are kept in a small
per-session state file under the project lock and travel in the records,
so the worktree check sees every command. Readers still fold `tool.used`
from older logs. Replaying this board's logs: 6,475 events become 2,132.

Built (FH-52): the headline metrics are Checkpoints, To review, New
tickets and Criteria ticked. The concept's Done tile gave way to
Checkpoints (user, 2026-10-11): only the human moves tickets to Done, and
that move is the human's latest change, so Done since it was always 0.
Criteria ticked counts updates, since the event does not say how many were
ticked.

Superseded (FH-63, user, 2026-10-11): the metrics confused more than they
helped. To review counted tickets that arrived since your last change, so
it read 0 above a Ready for your review section listing four. The headline
is now the pipeline: every board column's count, which can never
disagree with the sections, as interlocking chevrons in the State role.
Done counts accepted work, leaving out Rejected tickets (user).
Throughput figures (done this week, time in review) were left out because
each needs a time window again.

## 2026-10-11 — D33: Activity outside git is not recorded

Decision (user, FH-62): agent activity outside any git repository belongs
to no project and is not recorded (PRJ-4); the reserved `_scratch` project
is removed. Sessions outside git are mostly throwaway, have no repository
or tickets to resume from, and the board's value is per project, so a
`_scratch` board, created by hooks on first contact (even a lone
`run.end`), showed users noise they never asked for. Its one real use, a
home for an `ask_human` question or a ticket from such a session, is better
served by telling the agent there is no project so it talks to the user in
chat.

Outside git, hooks return before writing anything: no project, no events,
no cwd cache entry, not even a missing board root or the skill refresh.
MCP read tools without a `project` argument answer that there is no
current project and list the projects to pass; tools that need the
caller's project or run answer `no_project`; tools given a ticket id or a
`project` still work. A user who wants a folder tracked runs `git init`.

Options: keep `_scratch` but hide it in the UI (still writes files no one
sees); `_scratch` only for MCP writes (a board the user never looks at);
record nothing (chosen). Existing `_scratch` folders need no migration:
without the PRJ-1 exception they are `_`-prefixed directories, which the
board ignores, and the user can delete them.

Versions: board format stays 2; `PROTOCOL_VERSION` stays 2, since
`no_project` is a new coded error with a fix, added as `project_archived`
was, and no event or tool changes shape (agent-protocol §14).

## 2026-10-11 — D34: The MCP server starts a run for a session without hooks

Decision (FH-15, `mcp-runs`): when a write reaches the `flashheart` MCP
server and no run is found (agent-protocol §7.1), the server starts one for
its connection, `<agent>:mcp-<random>`, with `run.start` source `mcp`. One
stdio server process serves one agent session and is closed with it
(documented for Claude Code; Codex spawns its MCP servers per session and
names its client `codex-mcp-client`), so the connection is the session. On
2026-10-06 Codex, with no Flashheart hooks, created and checkpointed
tickets through the tools, but `claim` failed with `ambiguous_run`, its
writes said "unknown", and nothing reached the Agents view.

The run reads as Waiting unless it needs you: it has no turns, so it is
never Working or Quiet, and it has no plan, edits or permission prompts.
Every Flashheart tool call it makes, reads included, counts as a tool
result in a throttled `activity` record, as protocol 2's hooks count them,
which renews its claim's lease; closing the connection records
`run.end` with reason `disconnected`, and a server that dies leaves it to
the stale rule. Answers waiting for it are appended to its next tool result
and marked delivered, standing in for the prompt hook; `board_context`
stands in for the recovery note. Hooks stay the deterministic layer: a run
found from hooks, or named by `run`, always wins: no run starts while a
live hook session of the caller's agent works in the worktree, even on
another branch (an unreported branch switch), and the call asks for `run`
instead. Once started, the connection's run is its caller for the
connection's life, never another connection's, and follows the agent's
branch. The caller's agent comes from the
client's `clientInfo` name, so a Codex session without hooks is not taken
for a Claude Code session working in the same worktree.

Options: a run per server process minted at initialize (every read-only
session would show as a run, even one that never touches the board);
requiring the model to pass a run id it invents (models copy ids
unreliably, and attribution would depend on it); a run started by the first
write (chosen). Reads are activity once the run exists but never start one,
so a session that only reads leaves no trace.

Versions: `PROTOCOL_VERSION` stays 2 (additive: a new `source` and `reason`
value, `activity` written by the server, answers in tool results); board
format stays 2. The protocol skill says that without hooks the answer
arrives with the next Flashheart tool result.

## 2026-10-11 — D35: Only finished workstreams collapse

The Workstreams view collapses a workstream to its route card head only
when every one of its tickets is Done, Rejected or Archived and it needs no
repair (FH-68, user decision). The derived *Completed* status is unchanged
and is reached earlier, once every ticket is at least Ready to review; the
collapse rule is stricter on purpose, so work awaiting the user's review
stays in view. The user's choice to expand a finished workstream is a saved
preference (`ui.expanded_workstreams`, CFG-2) that lasts only while the
workstream stays finished: a reopened workstream shows in full, and it
collapses again once finished.

Options: collapse on the derived Completed status (hides work waiting for
review); collapse nothing and filter finished lines out (loses progress at
a glance); collapse on all tickets finished (chosen).

## 2026-10-12 — D36: Every write refuses an ambiguous caller

Every MCP write tool answers `ambiguous_run` while several hook sessions
of the caller's agent work in its worktree and branch and no `run` names
the caller (agent-protocol §7.1, FH-93). `attach` and `set_project_key`
used to go ahead as `unknown`, the only writes that did. An attachment is
a write to a ticket, recorded as the caller's `attachment.added`, so it
follows the other ticket writes; `set_project_key` sets the key that
`create_ticket`'s `project_key` sets, which was already refused, so the
two ways of choosing a key now agree. Each tool declares this as its
policy at registration (run required, required only when ambiguous, or
none, and whether it guards a held ticket), and §7.1 tables it.

Options: leave both as they were (one more exception for agents and for
each new tool to copy); refuse only `attach` (`set_project_key` records no
event, but then it disagrees with `create_ticket`); refuse both (chosen).
`PROTOCOL_VERSION` stays 2: the code and its fix are already known to
every agent and no event or tool changes shape.

## 2026-10-12 — D37: Hooks fold only the sessions they ask about, over one run window

Every reader folds the same run window, `runs.Lookback` (48 hours of daily
event files): the board's index, the MCP server, `flashheart await` and
the hooks (FH-95). The hooks used to fold one day, so a claim more than
about a day old was on the board but invisible to handoff enforcement
and the recovery note. Two days of a busy log (50,000 events a day) took
a full fold about 180 ms, beyond `HOOK-1`'s 50 ms p95, so the hooks no
longer fold every event. `events` reads each line's envelope header
(time, run, kind) without decoding its data when the line is in the
order Append writes it, and decodes the rest. A stop folds its session's
events and every claim and release (`runs.Keep`: a claim takes its ticket
from any other run, and nothing else another run does changes the
session's state, link, edits or windows). A session start reads the
window once into memory, finds the previous session in its worktree from
the headers (`runs.Latest`, decoding only each session's `run.start`s and
its latest `turn.start` naming a worktree), then folds the two sessions
and the claims. Tests fold random mixed logs both ways and compare
everything a hook reads. At 50,000 events a day a session start takes
about 36 ms and an enforcing stop about 30 ms at p95.

The worktree check's deadline is now counted from the hook's start (75 ms)
rather than from the check's (50 ms): a stop's slowest path, a long log
then a slow git, stays within `HOOK-1`'s 100 ms, and the check usually
gets more time than before. It runs only for a session that ran shell
commands since its last checkpoint and is not already due.

Options: keep one day for the hooks (cheap, but the surfaces disagree);
fold two days in full (correct, about four times `HOOK-1`'s budget); keep
a derived cache of runs per project (fast, but state outside the board
files that every writer must keep current); fold only the sessions asked
about (chosen). For the git check: shrink the timeout (simple, but a slow
fold still adds to it), a deadline from the hook's start (chosen), or
skip the check on a busy log (misses shell edits). Assumption: a
subagent's parent is the session its run id names, as every adapter
records it (agent-protocol §2).

## 2026-10-12 — D38: Serve's watcher is its freshness source; events refold runs only

Serve's index fingerprints the board files and the runs apart (FH-96). A
watched event-log append refolds the runs over the board already read, with
no `ReadBoard`; at hook cadence that used to reparse every ticket per
batch, so NFR-1's 5,000-ticket budget was paid per event. A new event that
records a ticket write (claim, release, checkpoint, ticket, review,
attachment or answer events) still reads the board, because the ticket's
folder may be beyond `FolderBudget`, where an event append used to catch
the edit by accident. While `Watch` runs, `Current` serves the snapshot as
it is instead of rebuilding one older than `MaxAge` (2 s); without a
watcher (tests, a server before its watcher starts, a watcher that
returned) it still does. An API write's own rebuild covers the watcher's
next one: each board change the watcher sees is marked with the count of
board reads started, and it reads again only when no read began after the
mark succeeded, or when it added a watch (FH-36's watch-then-read order).
A change seen after the write's read began is read again: about one write
in thirty under the race detector, none in thirty without it. Event logs
count against `FolderBudget`, before ticket folders; a project's
`.flashheart/` is watched only until its log exists. The revision stays
single: splitting board and runs revisions for the frontend is left to a
later change.

Options: keep `MaxAge` rebuilds alongside the watcher (simple, but every
GET after 2 s reads every ticket); have writes wait for the watcher instead
of rebuilding (one read, but a write to an unwatched folder or a
preference waits for a timeout); compare file modification times to the
read's start (filesystem timestamps are too coarse to be safe); mark
changes by reads started (chosen). For unwatched folders: rely on the 10 s
sweep alone (cheapest, but agents' writes to old tickets show late), or
read the board for events that record ticket writes (chosen).

## 2026-10-12 — D39: The server derives why a run needs you, and its short id

Why a run in Needs you needs you is one value, `runs.Reason`, derived with
its state (FH-98): a run is in Needs you exactly when its reason is not
zero. Before, the live badge (in `api`) and the Runs tab (in the frontend)
each picked the blocker, in different orders: the badge named a
subagent's permission prompt before the session's own question, the tab
the reverse, so a card and its Runs tab could disagree; the Overview held
a third copy of "a session waiting on a subagent is answered through it".
Runs and live badges now carry the reason typed: its kind (permission or
question), the tool, the question's kind and whether it is answered, and
the subagent it is (run id and type), empty when it is the run's own. The
frontend only words it, in the words it used before.

First match wins: the run's own permission prompt, its own unanswered
question, a live subagent's prompt, a subagent's unanswered question, then
an answer waiting for delivery, its own before a subagent's; subagents in
the order they were first seen. `RUN-3` and `HOOK-4` name a permission
request before a question, and a prompt holds the agent mid-tool. The
run's own blocker comes before its subagents' because it is what the
card's session is asking, while each subagent's is also listed on its own
(its Runs tab row, its Overview row). An answer on its way waits only for
the session's next prompt: the human has answered, so anything else they
can act on comes first. The Overview lists a run's own prompt beside its
own questions, where it used to hide it behind them, and a prompt a
session waits on through a subagent with that subagent.

Short ids: agent-protocol §2's is canonical, the agent and the first 8
characters of the session id (`protocol.ShortRun`), which ticket notes,
handoffs and recovery notes already use. The frontend recomputed short ids
and named a subagent by its own id; it now shows the server's `short`, and
questions carry their run's `runShort`, so a subagent shows its session's
short id, as the ticket's notes name it.

Options: send the reason in words (one place, but wording belongs to the
frontend and its tests); derive it in the frontend from a run and its
subagents (a card has no subagents to look at); a typed reason on the
server (chosen). For the order: the badge's (a subagent's prompt first,
which hides what the session itself asks), every prompt in the family
before any question (the same, for the session's own question), or the
run's own blocker first (chosen). For short ids: a subagent's own id
(tells subagents apart, but disagrees with the ticket's notes and names
no session to answer in) or the session's (chosen).

## 2026-10-12 — D40: Tickets are written for a person first

Agent-written tickets tended to describe the code: titles named test
functions or internal mechanisms (FH-111 was titled after a Go test) and
descriptions opened with file paths, so the board could not be read
without decoding each card. The board is how the user sees and steers
every project, so a ticket now leads with what a person would notice or
gain: the title in plain words, under 80 characters; the description
gives the problem, why it matters and the proposed change; implementation
detail, still wanted by whoever builds it, goes in `## Context`
(FH-113, agent-protocol §7.2).

The advice sits in three places because the skill is not always loaded
(another client, a session that never triggers it, or a long session that
compacted it away): the skill holds the full text and an example; the
`create_ticket` and `update_ticket` descriptions and hints repeat it in a
line, where the agent reads it as it writes; and a title that reads like
code or runs long is saved with a `warning:` saying how to improve it,
which agents act on in the same turn. `create_ticket` takes `context` so
the detail has a place other than the description.

Options: the skill only (missed whenever it is not loaded); refuse
code-like titles (a heuristic should not block a write, and hooks and
tools fail open); warn (chosen). Titles a human types in the browser are
not judged. The `find-tech-debt` skill's ticket template, which leads with
code locations, lives outside this repository.

## 2026-10-12 — D41: A turn records who started it; only the user's prompt answers

Decision (FH-57): `turn.start` carries an optional `origin`, classified by
the agent's adapter from the prompt, which is still never stored: `user`
(typed by the user), `task` (a notice the harness submitted: a background
command or agent finished, or the desktop app's pull request monitor) or
`agent` (a subagent's hand-back or another agent's message). Only `user`
answers the session's open questions in the session (D28) and lets a
session record its edited paths afresh; an origin a reader does not know
is not the user's. A turn without `origin` reads as before: the user's
unless `background`.

D28 recognised only Claude Code's `<task-notification>` as not the user,
and runs treated every other turn as the user replying. An orchestrating
session's subagents report back as prompts (`Another Claude session sent a
message:` and an `<agent-message>` in the Claude desktop app), so every
question such a session asked was marked answered in the session, left
Needs you, and `flashheart await` told the agent the user had answered,
although the user never replied. Flashheart's own board log holds dozens
of such hand-backs recorded as prompts.

The adapter still writes `background: true` on every turn whose origin is
not `user`, so a reader that predates `origin` (a board server not yet
restarted after an upgrade) also keeps these turns from answering. A
positive `origin` rather than a wider `background` lets the timeline say
what started the turn ("Message from an agent") and gives the Codex
adapter (FH-73) a value to classify into rather than a negative flag.

The prompt shapes come from Claude Code session transcripts (whose
entries carry the same prompt text and name its origin: `peer` for
agent messages, `task-notification`, an SDK origin for the monitor's
events) and the event log's timing, since recording hooks needs an
interactive session; the fixtures say so (testdata/hooks/claude/MANIFEST.md).

Options: widen `background` to more prefixes (no record of what started
the turn); read Claude Code's transcript for each prompt's origin (the
hook would read a large file on every prompt, against HOOK-1); an
`origin` field (chosen).

Versions: `PROTOCOL_VERSION` stays 2 (additive: an optional field, and
`background` still marks a turn the user did not start); board format
unchanged; the protocol skill does not describe it.

## 2026-10-12 — D42: Interaction tests in happy-dom; screenshots compared in a pinned container

Decision (FH-101): component and hook tests that click, type or check
focus run in **happy-dom** with **Testing Library** (`@testing-library/react`,
`@testing-library/dom`, `@testing-library/user-event`), all dev
dependencies pinned in `package-lock.json`. They are named
`*.dom.test.ts(x)` and run as Vitest's `dom` project; every other unit test
stays in the `node` project, which starts faster. The browser suite's
stable screenshot set (`frontend/e2e/visual.spec.mjs`) is compared with
committed baselines by `toHaveScreenshot`, in the Playwright container whose
version matches the pinned `@playwright/test`, on amd64
(`scripts/e2e-visual.sh`, which CI runs after `scripts/e2e.sh`).

Until now Vitest ran in node and every component test rendered static
markup, so dialogs, popovers, tabs and the data-loading hooks had no tests
of keys, focus or state over time, and `scripts/e2e.sh` saved screenshots
that only a person compared. The ticket page (FH-65) and editing (FH-66)
need focus-return and keep-or-discard tests, and refactors promise
unchanged screenshots.

DOM options: **jsdom** (the most complete DOM, but 20 direct dependencies
and a slower start); **happy-dom** (chosen: seven small dependencies,
released every few weeks, and it covers what these tests check: the
native `<dialog>`'s `showModal` and `cancel`, focus, `activeElement`,
`relatedTarget` on blur, and visibility events); a real browser
through Vitest's browser mode (accurate, but slow to start and a second
Playwright setup for unit tests). Testing Library is the common way to
render, query by role and drive keys with real default actions (Tab order,
focus moves); `user-event` types and tabs as a person does rather than
firing bare events. Pinned versions are at least two weeks old when added.
Layout, CSS and true focus trapping stay with the browser suite.

Screenshot options: one set of baselines for every platform (fails, since
macOS and Linux render fonts differently); baselines per platform (each
needs regenerating on its own machine, and CI only has Linux); baselines
made by a CI job and downloaded (a round trip through GitHub for every
intended change, and they drift with the runner image); the pinned
container everywhere (chosen: the same browser, fonts and libraries on a
Mac, a Linux machine and CI, reproducible offline once the image is
pulled, and on a Mac about 15 s for the set under amd64 emulation).
`scripts/e2e.sh` skips the comparison, so it still runs on any machine
without Docker. The set fixes what moves: a fixed browser clock, file
times, locale and time zone, and the temporary board path is masked. A
deliberate visual change updates the baselines with
`scripts/e2e-visual.sh --update-snapshots` in the same pull request,
where the PNG diff is reviewed.

## 2026-10-12 — D43: A ticket's pull request is one frontmatter field

A ticket's pull request lived only in notes and reviews, so the board could
not link to it. Tickets now carry `pr:` beside `branch:` (FH-114,
board-format §Frontmatter): one line, usually the pull request's URL, set by
the agent that opens it through `update_ticket` or edited on the board. It is
data an agent wrote, so the UI links it only when it is an `http(s)` URL,
opening a new tab, and shows anything else as plain text; Flashheart never
fetches it (local only).

Options: a list of pull requests (rarely needed: one branch per ticket opens
one pull request, and a list is harder to edit in a one-line field and to
show in the meta row); one value (chosen; a later pull request replaces it,
and earlier ones stay in notes); derive it from the forge (a network call,
ruled out). The protocol version stays 2: the field is optional and older
agents never set it.

## 2026-10-12 — D44: setup codex edits config.toml as text, hooks in hooks.json (AGENTS.md refresh revised by D53)

`flashheart setup codex` (FH-75) writes the flashheart MCP server into
Codex's `config.toml`, a file Codex also writes itself (project and hook
trust) and the user keeps comments in. Setup edits only its own
`[mcp_servers.flashheart]` table, through a small statement scanner in
`internal/setup` (headers, keys, strings of every kind, multi-line arrays
and inline tables), and refuses a server defined any other way.

Options: a TOML library (Go's maintained ones decode and re-encode, losing
comments and order, and would add a dependency for one table); running
`codex mcp add` (Codex keeps the file's layout, but the diff could not be
shown before writing, and the CLI is often only inside the ChatGPT app);
editing as text (chosen: the diff shows exactly the lines that change, and
everything else is kept byte for byte). Hooks go into `hooks.json`, not a
`[hooks]` table in `config.toml`: it is JSON, merged with the same code as
Claude Code's `settings.json`, and keeps `config.toml` edits to one table.
The protocol text goes between markers in the global `AGENTS.md` (SET-2);
no hook refreshes it, since that file is the user's, so `doctor` reports
it out of date after an upgrade. Codex skips hooks the user has not
trusted with `/hooks`; setup says so, and `doctor` warns when no trust is
recorded for `hooks.json` at all, but cannot tell a stale trust because
how Codex hashes a hook is not recorded.

## 2026-10-12 — D45: Codex calls are attributed by the session they name

Every MCP `tools/call` Codex sends carries its hook session in `_meta`
(`x-codex-turn-metadata.session_id`, `sessionId`), recorded from the CLI
and the ChatGPT app (FH-72). The MCP server reads it and attributes the
call to `codex:<session_id>`, the run Codex's hooks record (FH-74,
agent-protocol §7.1 step 2), before the connection's started run and the
worktree rule. Two Codex sessions in one worktree each get their own calls,
and a branch switch the hooks have not reported does not matter.

Without hooks the server starts the session's run under that same id
(`source: mcp`) instead of `codex:mcp-<random>`, so hooks that start later
(trusted mid-session) record onto the same run: one claim holder, one
question in Needs you and one Overview row, with no merging of two runs.
Whether a run is *MCP-only* is now derived from its events (`source: mcp`
and no event only hooks record) and sent to the board as `mcpOnly`, rather
than read from an `mcp-` id; once the hooks report the run, the server
records the activity it counted and stops standing in for them (no more
activity, answers in tool results or `run.end` at disconnect).

Options: stamp `run` from Codex's `PreToolUse` (`updatedInput` works, but
only with `permissionDecision: "allow"`, which would skip the user's
approval of Flashheart's tools, and only where the hooks are trusted);
fold a started `mcp-` run into a later hook run (moving claims, questions
and the answers inbox between runs); name the run by the session the call
carries (chosen: it needs no hooks, no stamping and no fold). A client that
names no session keeps `<agent>:mcp-<random>`, which never joins a later
hook run. `PROTOCOL_VERSION` stays 2: run ids keep the §2 grammar, and the
API field and the `_meta` read are additive.

## 2026-10-13 — D46: The ticket page's Activity is read from what the ticket already returns (refined by D47)

The full page's Activity timeline (FH-65, CARD-7) is derived in the
browser (`model/activity`) from the ticket the API already sends: the
`## Notes` bullets (`board.NoteLine`, so authors and days; a question
answered on the board is its `Question from … Answer (…)` note), the
questions the ticket's linked runs asked about it and its open questions,
the runs' `checkpoint` timeline entries on it, and the current handoff's
`_Updated … by …_` line. A question answered on the board appears once:
its run's copy, with times, replaces its note.

Options: a new `activity` list in the ticket response, folded on the
server from the event log beyond the run window (complete, but backend
scope and a second history of what notes already keep); deriving it in
the browser (chosen: no API change, and the durable parts, notes, answers
given on the board and the current handoff, are in the ticket file). Runs
cover the 48-hour run window, so an older question answered in the
session's own chat, or an older checkpoint, drops out of Activity; a note
carries only its day, so a day's notes come before its timed entries. If
Activity needs the full history, the server can add it without changing
the timeline's view.

## 2026-10-13 — D47: People comment on tickets; activity is typed and durable

A person comments on a ticket mostly before work starts (clarifying what
to build) and in review (asking for changes instead of telling a Claude or
Codex session in its chat). Sending a remark to a live session is a
secondary option, not the feature (FH-67, brainstormed with the user;
built by FH-124 to FH-128).

- **Typed activity, like Jira.** A ticket's Activity holds typed entries:
  comments (people), notes (agents' `## Notes`), history (created, moved,
  fields edited, criteria ticked, archived: who, what and when, never the
  ticket's text), handoffs and questions. The page filters it All,
  Comments or History.
- **Storage.** Comments and history live in a new append-only
  `activity.md` in the ticket's folder, beside `review.md`: human-readable
  markdown, one dated, typed entry each. History is kept for the ticket's
  life, so it outlives the event log's retention; board and agent writes
  both append it. `## Notes` stays in the ticket file. Comments never
  contend with agents rewriting `## Handoff` in the ticket file.
- **Comments.** Ticket-level, optionally quoting part of the ticket (a
  failing How to verify step). Editable, shown as edited. Agents read
  them as the user's words, not as agent text: `get_ticket` and `claim`
  carry them, so the session that takes a ticket gets them with it.
- **Request changes.** In review, comments can be marked *change
  requested*; the ticket goes back to In progress on its branch. Open
  requests reach the live holding session at once, and otherwise the next
  session through the recovery note and `board_context`. An agent marks
  each one addressed through the MCP tools; moving to review warns while
  any is open; the person resolves or reopens them at the next review.
- **Tell a session now.** A comment may also be sent to a live session
  that has claimed the ticket, chosen in the comment box; with none, the
  option is off and nothing is queued. It is delivered through the
  answers inbox as answers are, shows *Delivered to … at …*, and is
  locked once delivered.
- **Protocol.** Additive within version 2 (§14): new event kinds, comments
  in `get_ticket` and `claim`, open requests in the recovery note and
  `board_context`, an optional input to mark a request addressed, and
  skill lines. No setup rerun.
- **Where.** The ticket's full page first; the card panel later.

Options: notes delivered to sessions as the main feature (rejected once
the user framed comments as ordinary ticket work); comments in
`## Notes` or a `## Comments` section of the ticket file (one file, but
people's saves would race agents' checkpoints, and history would bloat
the ticket); history from the event log only (lost after retention);
anchoring comments to criteria or steps (anchors go stale as agents
rewrite sections; a quote does the job); requested changes as new
acceptance criteria (cheap, but loses the conversation). Durable history
supersedes the run-window limits D46 accepted for Activity.

## 2026-10-13 — D48: Flashheart may start an agent's own app on a ticket

Decision (user, FH-130 review): revises D1. Flashheart may start an agent
on a ticket by starting the agent's own app or command-line tool (Claude
Code, Codex) with the ticket as its opening prompt. The agent app still
owns the session, its credentials and its permissions, and the started
session reports through hooks and MCP like any other. Flashheart never
holds credentials, grants a permission or runs an agent without a window
the user can see; hosting, proxying or supervising agents stays a non-goal.
D1 kept launching out because launchers were an established category and
nothing gave one view of the sessions people already run. That view now
exists, and the board holds the priorities, reviews and comments, so
starting work from the ticket that describes it is the next short step.

How it works for each agent (which apps can be started locally, the
prompt, the folder or worktree, linking the started session to its
ticket) is worked out in a spike before anything is built (FH-131).
Starting a turn in a session that is already running was a separate
option and is not part of this decision: D47's comments and change
requests still reach a session at its next prompt or Flashheart tool
result.

Options: keep D1 and only deliver to running sessions; start turns in
running sessions; start the agent's own app (chosen); headless runs that
Flashheart supervises, as launchers like Vibe Kanban do (Flashheart would
own sessions, which D1's other reasons still rule out).

## 2026-10-13 — D49: One user on several machines; a small team kept possible

Decision (user, FH-130 review): Flashheart is for one person, who may use
it on several machines with the board root synced between them.
Multi-user sharing stays a non-goal for now, but the user wants to keep
the option of a small team later, so choices should not close it off:
entries name who wrote them (comments, history and notes already do), a
synced root stays a supported case, and D29's protections stay, because a
synced folder can carry edits from anywhere. Nothing is built for teams
yet: no accounts, no identities beyond names, no permissions, and no
merging beyond the lock and hash rules. D47's "people" is the user, or
the user's agents.

Options: one person on several machines, keeping teams possible (chosen);
a small team through a synced folder now (authors, identity and D29 as
core requirements); strictly one person (D29 reduced to basic checks).

## 2026-10-13 — D50: A session that edits without a ticket is asked to claim one

Decision (user, FH-130 review): revises D6. A claim stays the way a
session is linked to its ticket, but the hooks now ask once. When a
session has changed files (shell commands included) and holds no ticket
by claim or branch match, it is reminded once per session to claim a
ticket or create one. D32 found that 21 of 35 sessions never claimed, and
the Overview, Needs you and handoffs all depend on the link; the skill's
instruction alone is too often missed (D5: deterministic first). The
reminder blocks the turn end only where the user has turned on handoff
enforcement (`HOOK-6`); otherwise it reaches the session as a note, and
hooks stay fast and fail open. Subagents are not reminded. Built by
FH-132.

Options: a reminder from the hooks (chosen); stronger wording in the skill
and recovery note only; study the unclaimed sessions first; accept
unlinked sessions and improve the Overview's Work with no ticket.

## 2026-10-13 — D51: A ticket's criteria and handoff are its plan

Decision (user, FH-130 review): revises D5 and D32. Hooks stop recording
agents' plans, and the board stops showing or promising them; a ticket's
progress is its ticked acceptance criteria and its handoff's next steps,
which agents already write. In about 5,750 recorded tool calls on this
board, no session called a plan tool (`TodoWrite`, the task tools or
`update_plan`), and the Claude desktop app's Code session used for this
review offered none, so the plan display was always empty. Older logs that hold plans
still fold. Built by FH-133, which also settles the protocol version
under agent-protocol §14.

Options: criteria and handoff as the plan (chosen); hide the display but
keep the parsing dormant; agents report a plan through `checkpoint` (more
tokens per call, and it repeats the criteria); keep plans as they are.

## 2026-10-13 — D52: New abilities extend existing MCP tools first

Decision (user, FH-130 review): revises D7's "about ten tools", which the
list passed at fourteen. A new ability becomes an optional input on an
existing tool unless it is a genuinely new action; a new tool records why
in its decision. Every tool's name, description and schema is in every
agent's context before it does any work (principle 3), so the list's size
is measured: a test fails when it grows past a recorded budget, and a
change that raises the budget says why (FH-134). "No delete" stands.

Options: extend existing tools first, with a measured size (chosen); a
hard token budget alone; add tools freely.

## 2026-10-13 — D53: Codex's hook keeps its Flashheart section current

Decision (user, FH-130 review): revises D44's "no hook refreshes it".
Codex's SessionStart hook rewrites the Flashheart section between its
markers in the global `AGENTS.md` when it is out of date, as Claude Code's
hook refreshes the skill. It changes only that section, only when the
section exists (it never creates one), writes atomically, and fails open.
Without it, Codex follows outdated protocol rules after every upgrade
until setup runs again. The rest of the file stays the user's. Built by
FH-121.

Options: refresh only between the markers (chosen); move the text into a
file Flashheart owns outright, if Codex offers one; keep D44 and let
`doctor` warn.

## 2026-10-13 — D54: One typed inbox per session (refined by D60)

Decision (user, FH-130 review): revises D20. The answers inbox becomes a
session's inbox, holding typed entries (answer, comment, change request),
because D47's comments and change requests travel the same route. Every
delivery path (the prompt hook, MCP tool results, `board_context`,
`flashheart await`) frames each entry by its type, and the prompt hook
still costs one `stat` when nothing waits (`HOOK-1`). Readers also deliver
from the old `answers/` location while it drains. The inbox stays a
delivery queue, not a store of meaning. Built by FH-135, before FH-126 and
FH-127.

Options: one typed inbox (chosen); add types but keep the `answers/` name;
a second inbox for comments (a second `stat` on every prompt).

## 2026-10-13 — D55: The one-off upgrades are retired

Decision (user, FH-130 review): revises D15 and D19. `flashheart migrate`
(v1 column-folder boards to format 2) and setup's handling of the old
kanban-tracker skill are removed. Both upgrades have been made on the one
board they were for, and code no one runs still needs tests and docs.
`setup --uninstall` still restores a kanban-tracker backup that setup
made earlier, so nothing a user has is lost. Built by FH-136.

Options: retire both (chosen); keep them until a release cut-off; keep
them indefinitely.

## 2026-10-13 — D56: Kept after the October review, with points to revisit

Decision (user, FH-130 review): the 2026-10-13 review of these decisions
kept the following, each with the point at which to look again.

- **A rich text editor gets its own decision first** (D10). FH-16's
  formatted editor would be the largest frontend dependency, and saving it
  can reformat text agents wrote, so Tiptap, Milkdown and CodeMirror with
  live preview are compared, mainly on saving markdown unchanged, before
  FH-16 starts (FH-137).
- **`serve` keeps its browser-bound lifetime** (D13). Hooks and MCP do not
  need it, and neither does a launcher that starts an agent's own app
  (D48). Revisit when desktop notifications are scheduled.
- **Ticket ids keep opening the full page in a new tab** (D30). Revisit
  once in-place editing (FH-66) and comments (FH-124 to FH-128) ship,
  including whether the side panel still earns its place.
- **D9 (setup writes only with `--write`)** is revised by FH-129 in its
  own change, together with D19's `--uninstall`.

## 2026-10-13 — D57: A section edited in place is saved by the server, as one section

The full page's in-place editing (FH-66, `EDIT-11`) saves a section with
`PUT /api/tickets/{id}/section` (`base`, `section`, `content`):
`board.NewSectionChange` decides which sections may be rewritten whole
(not Handoff or Notes), the size limit and where a missing template
section goes, and its `Apply` (`mdfile.PutSection`) replaces only the
lines between the heading and the next one; `store.EditTicket` writes it
under the content-hash precondition. Text equal to the section's own
writes nothing and records nothing; a write records `ticket.updated` with
the template section's key, or `section` for a name that is ticket text.
In the browser, a draft whose own part did not change in a newer version
of the file moves its base hash to that version
(`useTicketDraft`), so an agent's note or checkpoint during an edit is
not a conflict, while a change to the item itself still is (`EDIT-7`).
Leaving the page with unsaved changes goes through one leave guard
(`state/leaveGuard`) that the router asks before every navigation.

Options: rewrite the whole file in the browser and save it as a raw edit
(no API change, but the browser would own section splitting, a raw edit
names no fields in its event, and any change elsewhere in the file would
conflict); a server-side section replace (chosen: the line-anchored
splitter and the Handoff rule already live in `mdfile` and `board`, the
byte-for-byte guarantee is tested against real tickets in Go, and FH-69
can let agents rewrite a section through the same `SectionChange` and
`EditTicket`). Rebasing a draft on any newer version was rejected: a
change to the section being edited must still be shown, never
overwritten.

## 2026-10-13 — D58: A ticket's history is worked out from each write, as the writer

Every ticket write by the board or an agent appends one history entry to
the ticket's `activity.md` (D47, FH-124, board-format §Activity). The
entry is worked out in `store`'s write transaction by comparing the ticket
(and, for a review step, its `review.md`) before and after
(`board.TicketChanges`, `board.ReviewChanges`): moved, the names of the
fields and sections edited, the criteria and steps ticked; only what no
diff can tell (created, archived, restored) is stated by the write. Who
writes is a view of the store, `Store.As(by)`: the API writes as the
person at the board, each MCP call as its caller's run; the plain store
(serve's copies of linked files, migrate) records no history. The entry
is staged in the same transaction as the ticket, so a history that
cannot be written fails the write; an archive or restore moves its folder
back.

Options: each API handler and MCP tool appending its own entry, as they
record events (copies in two adapters, and a new write path such as a
section edit would have to remember); an author argument on every store
write (a wide signature change for one value); recording it after the
write, best effort like events (a ticket's lasting record would silently
miss writes); deriving it from the before and after in the same write
(chosen: every write path, raw edits included, is described alike, and
none names the ticket's text, since keys and sections Flashheart does not
know are named only as `frontmatter` or `body`). A reorder within a
column, notes, handoffs and answers are not history.

## 2026-10-13 — D59: Setup asks before it writes; --yes for scripts

Revises D9, and D19's `--uninstall`, which applied with `--write` (as
D56 noted). `flashheart setup <agent>` showed a diff and wrote only when
run again with `--write`, the way formatters work (`gofmt -w`,
`prettier --write`). Setup commands usually apply their change, so this
took two runs, and someone skimming the diff could think setup had
worked (FH-129, user's choice).

Setup now follows `terraform apply`: it shows the diff, then asks
`Apply these changes? [y/N]` on a terminal and writes only on `y` or
`yes`; any other answer, or none, changes nothing. `--yes` applies
without asking, `--dry-run` only shows. With no terminal on stdin (a
pipe, a script, an agent's shell) and no `--yes`, it only shows the diff
and says to re-run with `--yes`, so nothing hangs waiting for an answer
and nothing is written unseen. D9's guarantee stands: no agent
configuration changes without the user seeing the exact diff and saying
yes. `--write` stays as an older name for `--yes`, so scripts and notes
that use it keep working. Doctor's fixes say `run flashheart setup
<agent>`.

## 2026-10-13 — D60: Old inboxes drain so the prompt hook checks one file

Refines D54 (FH-135). The inbox is
`<project>/.flashheart/inbox/<agent>--<session>.jsonl`, one typed entry
per line (`answer`, `comment`, `change-request`; agent-protocol §4).
D54 asks readers to deliver from the old `answers/` location while it
drains, and HOOK-1 asks the prompt hook for one `stat` when nothing
waits; checking both would cost every prompt a second. So the prompt hook
checks only the new inbox, and old inboxes drain into it: queueing
anything for a session moves its old inbox in first, `serve`'s retention
moves them all at startup and daily, and the recovery note,
`board_context`, tool results and `flashheart await` take both locations.
Only an answer an older `serve` queues after the binary is replaced waits
for one of those. A new session's recovery note takes the previous
session's answers, as before, but leaves its comments and change
requests: they were sent to that session. A delivered comment or change
request records `comment.delivered` on the run it was sent to; the log
never holds its text (D47), so it is delivered from the inbox alone.

Options: the prompt hook checks both locations (a second `stat` on every
prompt, for a short upgrade window); keep the `answers/` name (rejected
by D54); drain into the new inbox and check one (chosen).

## 2026-10-13 — D61: The formatted ticket editor is CodeMirror 6 with a live preview

Decision (user, 2026-10-13, after the FH-137 spike): CodeMirror 6 with a
Flashheart-owned live preview, as recommended below. FH-16 builds it.

Context: FH-16's formatted editor replaces FH-66's markdown text area
behind the same in-place edit, one section at a time (D57). Tickets are
shared with agents, so the main test is whether a section saves back
unchanged when untouched and with only the intended change after a small
edit (D56).

Options: Tiptap v3 with the official `@tiptap/markdown`; Milkdown 7
(ProseMirror with remark); CodeMirror 6 with a Flashheart-owned live
preview, where decorations hide markdown syntax except on the lines being
edited; a hybrid of two editors (rejected: two bundles and two behaviours
for one text box); defer and keep the text area (exact, but not what the
user asked for).

Evidence (throwaway prototypes in Chromium under the app's CSP, versions at
least two weeks old: Tiptap 3.31.3, Milkdown 7.22.2, `@codemirror/view`
6.43.13; 135 board tickets, 524 editable sections; small edits are
replacing a word in a paragraph (389), ticking a criterion (26) and
adding a list item (163)):

| | Tiptap | Tiptap + block preservation | Milkdown (`bullet: "-"`) | CodeMirror live preview |
|---|---|---|---|---|
| Untouched sections unchanged | 75% | 100% | 89% | 100% |
| Small edits with only that change | 71% | 82% | 67% | 100% |
| Added, min / gzip (React external) | 561 / 172 KB | same | 478 / 144 KB | 347 / 111 KB |
| Packages (new to the lockfile) | 62 (56) | same | 108 (38) | 21 (21) |
| Licences | MIT | MIT | MIT | MIT |
| CSP (`style-src 'self'`) | clean with `injectCSS: false` | same | clean | injects a `<style>`; clean in a shadow root (or with a nonce) |
| axe, keyboard | unnamed textbox (one setting); Tab in a list item keeps focus | same | unnamed textbox; task items have no checkbox | axe clean, Tab leaves; lists and headings not exposed to screen readers |

Tiptap rewrites escape `_` and `*` in identifiers (`get_ticket` becomes
`get\_ticket`), turn `_em_` into `*em*`, bare URLs into `[url](url)`, `<id>`
into `&lt;id&gt;`, and tighten loose lists. Block preservation (keeping the
bytes of unchanged top-level blocks) fixes untouched saves but not the
edited block: ticking one criterion re-serialises the whole list. Milkdown
by default rewrites every `-` bullet to `*`, creates a new item after a
ticked criterion already ticked, drops an image without a title (written
as `<br />`), and keeps a pasted `javascript:` link. CodeMirror never
re-serialises: the saved text is the edited text, so its raw-markdown
toggle is exact, and paste is plain text, so pasted HTML is never parsed.
Each can be loaded on the first edit, so the board's first load need not
grow. The full results, diffs and screenshots are attached to FH-137.

Recommendation: CodeMirror 6 with a live preview. It is the only option
that keeps agents' text byte for byte, which makes FH-16's fidelity
machinery unnecessary rather than something to build and test; it adds
the fewest packages and bytes; it meets the paste and network rules by
construction. What it costs: the formatted look is Flashheart's own code
(syntax-hiding decorations, checkbox and image widgets, a slash menu on
`@codemirror/autocomplete`, a selection toolbar, a keyboard command to
tick a criterion), the editor mounts in a shadow root for the CSP, and it
is less like Notion than Tiptap: syntax shows again on the line being
edited and tables are edited as text. If the user prefers Tiptap's look,
the fallback is Tiptap with block preservation, `injectCSS: false`, an
accessible name, Tab released from lists and custom escaping, accepting
that an edit rewrites agent text in the block it touches. Milkdown is not
recommended.

## 2026-10-13 — D62: How the formatted editor is built and used

Decision (FH-16, building D61): the packages are `@codemirror/view`
6.43.13, `state` 6.7.6, `commands` 6.11.1, `language` 6.12.4,
`lang-markdown` 6.5.2 and `autocomplete` 6.20.3, exact and at least two
weeks old, all MIT, 21 packages new to the lockfile. The editor imports the
GFM language alone, not lang-markdown's `markdown()` helper, so the HTML,
CSS and JavaScript parsers it would add stay out of the build. It is its
own chunk, loaded on the first edit (363 KB minified, 119 KB gzip); the
board's first load grows by the lazy entry and the preference (about 1 KB
gzip). It mounts in a shadow root, whose constructed style sheet the page's
CSP allows; a per-response nonce would have the server template
`index.html` and pass the nonce to the page.

Behaviour chosen with it:

- Acceptance criteria that are a plain task list keep FH-66's list editor
  (add, reword, reorder and remove are list operations); a criteria section
  with other text, and every other section, use the formatted editor.
- Keys: Ctrl or Cmd with Enter and Escape stay the page's (save, cancel),
  so CodeMirror's own bindings for them are dropped, and Escape closes the
  slash menu only while it shows. Tab leaves the editor (indent with Ctrl or
  Cmd and `]`), and task checkboxes are not tab stops. Alt+Enter acts on the
  cursor: it opens the link or ticket id there, else ticks the line's task;
  it is the keyboard way to open a link (`CARD-2`) and tick (`CARD-3`). A
  plain click edits a link; Ctrl or Cmd and a click opens it. Links open in
  a new tab, ticket links too, so the open edit is kept. Tab or Alt+F10
  reaches the selection toolbar (the usual toolbar key in editors).
- The slash menu offers headings at levels 3 and 4: a level-2 heading in
  a section would start a new section when saved.
- Saving over an agent's write: the section save (D57) and the draft's
  base moving on while its section is unchanged already merge changes to
  other sections; FH-16 adds the case where the save meets a version the
  page has not heard of yet (the 409 carries the file): when that file
  left the section as it was, the save is made again on it, once, rather
  than asking. A change to the section itself still opens the conflict
  dialog. A three-way merge within a section was not built: the editor
  saves the section's text as edited, and overlapping edits are for the
  person to resolve (`EDIT-7`).
- The Formatted or Markdown view is a UI preference, `ui.editor`
  (`CFG-2`), shared by every section and ticket.
- Screen readers hear headings as headings and task checkboxes by their
  task; hidden marks are hidden from them too, so a list reads as lines,
  and the Markdown view reads every mark.
- Workstream text has no editing surface yet; FH-139 adds one with this
  editor.

Options: Escape that first collapses a selection (CodeMirror's default)
was rejected for one Escape that always cancels; Tab indenting list items
was rejected for Tab leaving (FH-137's keyboard-trap finding); a separate
shortcut per action (one to tick, one to open) was rejected for one
Alt+Enter on whatever the cursor is on; a toolbar always shown above the
text was rejected for one over the selection, which keeps the section's
look.

## 2026-10-13 — D63: A comment told to a session is marked delivered on the comment, by whoever delivers it

Built by FH-127 within D47's choices (the user's: a per-comment choice in
the comment box; only live sessions that have claimed the ticket, with
their subagents under them; nothing queued for a later session; editable
until delivered; *Delivered to … at …*). What FH-127 decided itself:

- **Where delivery is kept.** The comment names its run (`sent-to`) when
  posted and gains `delivered` (a time) in the ticket's `activity.md`
  once handed over, so the board can say so for as long as the ticket
  lasts; the `comment.delivered` event stays the protocol's record but
  expires with the log, and reading delivery from it would also mean
  folding comment ids into every run.
- **Who marks it.** Whoever delivers the session's inbox (prompt hook,
  recovery note, `board_context`, tool results, `flashheart await`)
  marks it, through `events.Deliver` and `store.DeliverComment`, under
  the ticket's project lock. The inbox entry carries the ticket's
  `project`, since the inbox is in the session's home project. The cost
  lands only on a prompt that has a comment to deliver; an empty inbox is
  still one `stat` (`HOOK-1`).
- **What the session reads.** The text as the comment reads when it is
  delivered, not as it was queued: reading it and marking it delivered is
  one locked write, so an edit either lands before (and is what the
  session reads) or is refused (`delivered`). One that was delivered
  already, sent to another run or is gone is left out; one whose ticket
  cannot be read goes as queued, since nothing taken may be lost.
- **Who may be chosen.** The ticket offers its live claimants
  (`index.Snapshot.Claimants`, the rule `Holder` uses), and the server
  checks the chosen run again when the comment is sent and refuses
  (`not_live`) without posting, so the person chooses again. A comment
  that cannot be queued stays posted and is unsent, with a warning.
- **Missed.** A comment whose run has ended (or left the run window)
  undelivered reads *Not delivered*; it stays editable and on the
  ticket, where the next session reads it with `get_ticket` and `claim`.
  Should its own session resume, it is delivered then.
- **Delete** is not offered for comments yet; when it is, a delivered
  comment is locked against it too.

Options: delivery read from the event log only (lost after retention);
`serve` marking comments from the events it folds (a write that depends
on `serve` running, and a second writer); re-queueing an edited comment
(the inbox keeps an entry's first copy, so the session would read the old
text); posting the comment unsent when the chosen run is no longer live
(the person chose to tell a session, so they choose again instead).

## 2026-10-13 — D64: doctor recomputes Codex's hook trust hash

Refines D44's "cannot tell a stale trust". `flashheart doctor` (FH-120)
computes the key and hash Codex saves when you trust a hook
(agent-protocol §5.4) for each Flashheart hook in `hooks.json`, compares
them with `[hooks.state]` in `config.toml`, and names the events whose
hook was never trusted or was trusted with another command. The rule was
recorded from Codex 0.160.0 with `scripts/record-codex-hook-trust.sh`
(Codex's own keys and hashes for Flashheart's hooks, a changed command,
and every hashed field), matches the codex-rs source at that version, and
tests check the implementation against those recordings.

Options: ask Codex (`codex app-server` answers `hooks/list` with each
hook's trust status), rejected because doctor would start Codex, which is
slow, may reach the network, and is often only inside the ChatGPT app;
check only that some trust exists (D44), rejected because a moved binary
or another `--root` silently stops the hooks; recompute the hash (chosen:
read only, local, instant). The cost is coupling to Codex's hashing: a
Codex release that changes it makes doctor report trusted hooks as
changed, so re-record with the script when Codex updates its hooks and
fix the rule if the recordings fail.

## 2026-10-13 — D65: Files from the browser are judged by their bytes; pasted HTML stays text

Decision (FH-18, `REV-7`, `SEC-7`): a file pasted, dropped or chosen on
the board is sent as the body of `POST /api/tickets/{id}/files`, through
Singleserve's session, and stored by `store.UploadAttachment`, which
reads no further than the limit (20 MB, or a lower `attachments.max_bytes`)
and keeps the file only once all of it is accepted. What it is comes from
its bytes (`board.SniffAttachment`: image and PDF signatures, UTF-8 text
without binary control bytes; SVG, HTML and XML by their opening or
name are refused), and it is stored under that type's extension, so the
attachment endpoint, which types files by extension, serves it as what
it is. Agents' `attach` keeps its extension check: its paths come from
the local agent, not the untrusted browser.

The upload writes only the file and its index entry. In the editor the
reference is typed into the text behind a placeholder decoration, so it
is saved with the section (`EDIT-11`, D57) and a save during an upload
carries no placeholder; a cancelled edit leaves the file in the ticket.
The upload records the human's `ticket.updated` with the field `files`
(`EDIT-10`) and no history entry, as agents' attachments record none.
On the full page, Files is the drop target (shown, even empty, where the
ticket can be edited), matching the panel's Attachments tab.

FH-18 asked for pasted web HTML to be converted to markdown with remote
images kept as links and unsafe schemes removed. D61 made paste plain
text, so pasted HTML is never parsed: it arrives as its plain text, which
fetches nothing and carries no link or scheme at all. That meets the
criterion's safety intent; converting HTML to markdown would need an
HTML parser and URL filter in the editor and was not built. A paste that
carries a file and no text is a screenshot; one with text and a picture
of it (a range copied from a sheet) pastes as text.

Options: multipart form data (a parser for one file; a raw body with the
name as a parameter streams as simply); sniffing every attachment,
agents' included (would change a protocol tool's behaviour for files the
local agent already chose); keeping the original extension and only
checking the content (a PNG named `.txt` would be served as text); a
text placeholder in the document (a save during an upload would write
it); converting pasted HTML (see above).
