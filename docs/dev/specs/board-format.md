# Board format (v2)

The on-disk format Flashheart reads and writes. Tickets stay plain markdown
with YAML frontmatter, readable in any editor; Flashheart owns the layout
(decision D15). Version 1, the kanban-tracker column-folder layout, is read
only by `flashheart migrate`. Requirement ids refer to `docs/SPEC.md`.

## Layout

```text
<root>/                                   default ~/reports/Kanban
├── .flashheart/
│   ├── config.yaml                       version: 2, settings, UI preferences (CFG-1, CFG-2)
│   ├── cache/cwd.json                    cwd → project/branch cache, validated by HEAD's mtime (agent-protocol §2)
│   ├── backup/v1-<UTC timestamp>/        the v1 tree moved aside by migrate
│   ├── lock                              advisory root lock for choosing keys (KEY-5)
│   ├── retired.yaml                      keys of permanently deleted projects (KEY-5)
│   ├── hook-errors.log                   hook failures (HOOK-1), rotated at 1 MB
│   └── serve.log                         background serve diagnostics (LIFE-4), rotated at 1 MB
├── .archive/<project>/                   archived projects (PRJ-5), as they were
└── <project>/                            e.g. flashheart
    ├── project.yaml                      key, display name, repos, next id (PRJ-3, KEY-1)
    ├── workstreams/<slug>.md
    ├── tickets/
    │   └── FH-42-card-panel/             one folder per ticket: <id>-<slug>
    │       ├── FH-42-card-panel.md       the ticket, named after its folder
    │       ├── review.md                 review guide (REV-4), optional
    │       ├── activity.md               comments and history (STO-8), optional
    │       └── files/                    copied files (REV-1, REV-5)
    │           ├── index.yaml
    │           └── 20261005T1412-board-desktop.png
    ├── .archive/tickets/FH-7-old-idea/   archived tickets (EDIT-8)
    └── .flashheart/
        ├── lock                          advisory project lock (STO-3)
        ├── events/2026-10-05.jsonl       event log (STO-5)
        ├── inbox/<agent>--<session>.jsonl     answers, comments and change requests waiting for a session's next prompt (HOOK-5, D54)
        ├── answers/<agent>--<session>.jsonl   the inbox before D54, delivered and drained into inbox/
        └── activity/<agent>--<session>.json   tool activity hooks have not recorded yet (agent-protocol §3)
```

Rules:

- A project is a directory with a `tickets/` folder or a `project.yaml`
  (PRJ-1).
- A ticket is a folder `<id>-<slug>` in `tickets/` containing the ticket file
  `<id>-<slug>.md`, named after the folder so editor tabs and search results
  say which ticket they are. The frontmatter `id` is authoritative; a mismatch
  with the folder is a warning. If the folder was renamed by hand, a single
  `<id>-*.md` file in it is read with a warning. A folder with no ticket file,
  or more than one candidate, is ignored.
- Two tickets with the same id are both shown as *needs repair*.
- Directories starting with `.` are Flashheart's or archives; directories
  starting with `_` are not projects (`PRJ-1`), so a `_scratch/` left by an
  older version (before D33) is ignored and can be deleted. Unknown files and
  directories are ignored and preserved.

## Ticket ids (KEY)

- Every project has a **key**: 2 to 10 characters, an uppercase letter
  followed by uppercase letters or digits (`FH`, `NG`, `OPS2`), set as `key`
  in `project.yaml` and unique across the root. Without one, readers derive a
  default (the initials of a multi-word directory name, otherwise its first
  three letters, uppercased), warning only when the project already has
  tickets.
- A key is chosen before the first ticket and then fixed (`KEY-5`): by an
  agent (`set_project_key`, or `project_key` on its first `create_ticket`), in
  the UI, or, failing both, the derived default with a digit added when that
  key is taken (`NG`, `NG2`). Choosing a key takes the root lock
  (`<root>/.flashheart/lock`), checks it against every project's key, and
  writes `project.yaml` atomically; it is refused once the project has a
  ticket, live or archived.
- A ticket id is `<key>-<number>` (`FH-42`). Numbers start at 1, are assigned
  under the project lock from `next_id` in `project.yaml` (never lower than
  one past the highest existing, archived or retired number), and are never reused or
  renumbered.
- Ids are global across the root because keys are unique, so references never
  need a project prefix.

## Ticket

`tickets/<id>-<slug>/<id>-<slug>.md`:

```markdown
---
id: FH-42
status: up-next          # backlog | up-next | in-progress | review | done | rejected
type: feature            # feature | test | bug | refactor | infra | docs | spike
priority: high           # high | medium | low
created: 2026-10-04
branch: feature/x        # empty until work starts
pr: https://github.com/o/r/pull/7   # empty until a pull request is open
workstream: board-ui
depends-on: [FH-12, NG-3]
depends-on-workstreams: []
tags: [ui]
session: claude:3321f965-…   # creating session (free text)
git-ref: cf98240
updated: 2026-10-05T14:12:09Z    # last write by Flashheart
---

# Title

## Description
## Acceptance Criteria
- [ ] criterion
## Test Plan            (feature)   /   ## Reproduction   (bug)
## Context
## Notes
## Handoff              (written by checkpoint)
```

### Frontmatter

| Field | Required | Notes |
|---|---|---|
| `id` | yes | `<key>-<number>`. Taken from the folder name, with a warning, when absent. |
| `status` | yes | The column, or `rejected` (shown in Done). Missing reads as `backlog` with a warning; an unknown value marks the ticket *needs repair* (shown in Backlog). |
| `type` | yes | `feature`, `test`, `bug`, `refactor`, `infra`, `docs`, `spike`. |
| `priority` | yes | `high`, `medium`, `low`. |
| `created` | yes | `YYYY-MM-DD`. Orders numbering at migration and ties in sorting. |
| `branch` | no | Used for provisional run links (`RUN-5`). |
| `pr` | no | The ticket's pull request, one line, usually its URL (FH-114, D43). Set by the agent that opens it, or edited on the board. The board links it only when it is an `http(s)` URL and never fetches it. New tickets carry it empty; a ticket without it reads as having none. |
| `workstream` | no | Slug of a workstream in this project. |
| `depends-on` | no | Ticket ids, in any project. |
| `depends-on-workstreams` | no | Workstream slugs in this project. |
| `tags` | no | Free text; `later-possibility` marks ideas (`VIEW-7`). |
| `rank` | no | Place in its column's manual order (`EDIT-9`); written by Flashheart. See below. |
| `session`, `git-ref`, `updated` | no | Free text / RFC 3339 UTC. |

The title is the first `#` heading. Unknown keys are preserved in place
(`STO-2`).

### Columns

| `status` | Column | Meaning |
|---|---|---|
| `backlog` | Backlog | Captured; not yet chosen. |
| `up-next` | Up next | Picked and ready for implementation, in priority order. |
| `in-progress` | In progress | Being worked on. |
| `review` | Ready to review | Implemented; waiting for human review. |
| `done` | Done | Accepted. |
| `rejected` | Done | Closed without being accepted (FH-58). |

`rejected` is a Done state: the ticket sits in the Done column and counts as
done everywhere (blocking, workstream progress, the done limit, ordering),
but says *Rejected* wherever its status is shown. Like Done, only humans set
it: the MCP `move` refuses it. Moving a rejected ticket anywhere, Done
included, writes that status and so clears the rejection.

A move is a frontmatter edit of `status` under the lock, applied to the file
as it is then (`STO-3`); other edits from the UI and agents carry the content
hash they read. Every ticket write by Flashheart stamps `updated`, except
one that leaves the frontmatter unparseable, which is written as it is;
archiving stamps it too, which dates the archive. The folder never moves except to
`.archive/` and back, and is removed only by a permanent delete of an
archived ticket (`EDIT-8`): the delete removes the id from every
ticket's `depends-on` and the project's workstream `tickets:` lists, adds
it to `retired` in `project.yaml`, and removes the folder. It takes an id,
never a path, and refuses any symbolic link on the folder's path.

Its preview and its check share one reference scan: every live and
archived ticket file in every live project (not through a symbolic link),
read once and matched on its frontmatter `depends-on`, plus the project's
workstream files. The preview takes no lock. Its token covers the folder's
files and sizes and the path and content hash of every referring ticket and
listing workstream, so it changes when any of them changes and when a
ticket or workstream starts or stops referring to the id. The delete scans
without a lock first, refusing a stale token at once; then takes the root
lock and the locks of the project and of every project it rewrites (in name
order), scans once more and writes only if the token is unchanged. Under
the root lock it reads each ticket file but parses only its frontmatter
(the whole file only for a referring ticket), and rewrites only what the
preview showed. A project it does not lock can gain a reference after that
scan, as after the delete; it names a retired id, which counts as done.

### Multi-file writes

A write that changes several files (creating a ticket, joining or leaving
a workstream, creating a workstream, placing a ticket that ranks its
neighbours) reads and checks every file under the project lock first and
writes nothing if any check or edit fails. It then replaces each file
atomically, one at a time, in this order:

- creating a ticket: the ticket, then `project.yaml` (`next_id`, and the
  key when it is chosen), then its workstream's `tickets:` list;
- a ticket joining or leaving a workstream: the workstreams' lists, then
  the ticket's `workstream:` field;
- creating a workstream: its file, then the other lists giving up its
  initial tickets, then those tickets' fields;
- placing a ticket: the neighbours it ranks, then the ticket;
- any ticket write from the board or an agent: the files above, then the
  history entry in each changed ticket's `activity.md` (§Activity).

A crash between two of these leaves the files written so far. Each such
state is readable: a ticket number above `next_id` is never reused
(`next_id` is never lower than one past the highest existing number), a
field and lists that disagree show a warning until the next write puts
them back in step, a ranked neighbour keeps its place, and a ticket write
whose history was not written is a change history does not mention.

### Order within a column

`rank` is a fractional index: base-62 digits (`0-9A-Za-z`), compared as
plain strings, never ending in `0`, at most 64 characters, so a new key
always fits between two others. Within a column, ranked tickets come first
in rank order; unranked tickets follow by priority (high, medium, low), then
`created`, then id. Ties fall back to the same order. An invalid `rank` is
ignored with a warning. *Done* ignores ranks and lists the most recently
modified first.

Placing a ticket (`POST /api/tickets/{id}/move` with `after`: the id it
should follow, or `""` for the top) takes the project lock, reads the column
as it is then and normally writes only that ticket's `rank` (and `status`).
Placed inside the unranked tail, the unranked tickets above it are ranked
first, in their current order, and tied ranks above it are spread out, so no
other ticket changes place. A ticket to follow that has left the column is
reported and the ticket keeps its rank. A move without `after` (the MCP
`move` tool, Shift with Left or Right, the panel's **Move to**) keeps the
rank. Archived tickets keep their `rank` and return to their place when
restored. Workstream `tickets:` order is separate and unaffected.

### References in text

Ticket ids written in markdown (`FH-42`) render as links to that ticket.
Relative links to a ticket file (`FH-42-card-panel.md` in the same folder,
`../FH-12-x/FH-12-x.md` for another) also open that ticket. Files are referenced relative to the ticket folder
(`files/20261005T1412-board.png`).

### Handoff section

Written only by `checkpoint` (or a human). Replaced as a whole each time; the
log keeps history. `Done` and `Next` are left out when empty; empty `Files`
and `Open questions` are written as the item `None` and read back as
empty (`board.RenderHandoff` and `board.ParseHandoff` round-trip). A
hand-written section without the `_Updated …_` line is read for its lists
alone.

Text Flashheart writes into any section (a handoff, a note, a ticket's
description or plan, a workstream's goal) stays inside it: a level-1 or -2
heading outside code fences becomes level 3, and a code fence left open is
closed, so it cannot start or hide a section (FH-4). A ticket's
description, plan (or reproduction) and context, and a workstream's goal,
are each limited to 16 KiB, written from the board or by an agent
(`board.MaxSectionText`, FH-81); longer text belongs in a linked document.

```markdown
## Handoff

_Updated 2026-10-04 14:12 UTC by claude:3f2a… (run) on feature/x._

**Done**
- Added failing test for label-only basis
**Next**
- Pass `false` for knownSpecification; run check-my-work spec
**Files**
- src/features/study/StudyPage.tsx
**Open questions**
- None
```

### Notes written by Flashheart

Answered questions and override reasons are appended to `## Notes` as dated
bullets, e.g. `- 2026-10-04 · Question from claude:3f2a… — "Use known
assessment objectives?" Answer (Robert): "Yes".` A blocked ticket started
from the board or claimed by an agent with `force` gets the same bullet
either way: `- 2026-10-04 · Started while blocked: <reason>` (FH-81). A
note written for an agent (its `append_notes` or checkpoint `note`, or a
claim taken from another session) names the agent's short run id first:
`- 2026-10-04 · claude:3f2a…: <text>`. Every one of these bullets comes
from `board.NoteLine` (FH-85).

### Section edits

A section edited in place on the ticket's page (FH-66,
`board.NewSectionChange`) replaces only the lines between its heading and
the next level-1 or -2 heading: one blank line, the text, and one blank
line before the next heading. Every other byte of the file stays as it
was, but the `updated:` stamp. Text equal to the section's own, ignoring
blank lines around it and trailing whitespace, writes nothing. A missing
template section is added in the template's place, above the template
sections that follow it (a ticket's own section above `## Notes` and
`## Handoff`). `## Handoff` and `## Notes` are never rewritten whole:
checkpoints write the one, and notes are appended.

A workstream's section edited on its page (FH-139,
`board.NewWorkstreamSectionChange`) is replaced the same way; a
workstream has no `updated:` stamp, so nothing else changes. Every one of
its sections may be rewritten, `## Notes` too, since nothing appends to
it. A missing template section goes above the template sections that
follow it (Goal, Scope, Success Criteria, Notes), and a workstream's own
section above `## Notes`. The write takes the project lock and the
content-hash precondition (STO-3), and records no event.

## Blocking

A ticket is **blocked** when any of these holds:

1. a `depends-on` ticket is not in `review` or `done`;
2. a `depends-on-workstreams` workstream has a ticket not in `review` or
   `done`;
3. a workstream that lists it has a `depends-on-workstreams` workstream with a
   ticket not in `review` or `done`.

A workstream's order never blocks (D26): its tickets are an epic's, worked on
in sequence or in parallel, and `depends-on` (one ticket or several) is the
only way to say one must wait for another.

A workstream's `tickets:` list defines membership and display order; a ticket whose
`workstream:` field disagrees with the lists gets a warning. Flashheart's own
writes (ticket creation, the agent tools and the board's Edit tab) change
the field and the lists together, appending a joining ticket to the end of
its list. A reference to a
missing ticket or workstream (including a missing `workstream:`) is shown as a
warning and counts as blocking. Archived tickets count as done, and so do
permanently deleted ids (a project's `retired`) and every id of a deleted
project's key (`retired.yaml`), wherever a reference to them survives. A duplicated
id counts as done only when every copy does. Only tickets in `backlog`,
`up-next` and `in-progress` are shown as blocked.

## Workstream

`workstreams/<slug>.md`, where a slug is lower-case letters, digits, `.`,
`_` and `-`, starting with a letter or digit, at most 100 characters (the
same rule wherever a slug is given: a ticket's `workstream:`, MCP tools,
the UI):

```markdown
---
slug: board-ui
status: active            # informational; the UI derives status
priority: high
created: 2026-10-04
tickets:                  # ticket ids, in display order; order never blocks
  - FH-12
  - FH-13
depends-on-workstreams: []
tags: []
---

# Title
## Goal
## Scope
## Success Criteria
## Notes
```

The UI derives status (`completed` when every ticket is in review or done,
`blocked` when its own `depends-on-workstreams` are incomplete or every
unfinished ticket is blocked, otherwise `active`) and does not rewrite the
`status` field unless the user edits it. The next ticket is the first
unfinished one in order that is not blocked, else the first unfinished one.

## Review

`tickets/<folder>/review.md`, in the review template (Summary, Key Files, How
to Verify, Risks, PR Notes). Screenshots are referenced as
`![Board at desktop](files/20261004T1412-board-desktop.png)`.

## Activity

`tickets/<folder>/activity.md` holds the ticket's comments and its history,
one dated, typed entry each, oldest first (STO-8, D47). People comment on a
ticket; every ticket write by the board or an agent appends what it
changed. It sits beside the ticket so a comment never contends with an
agent rewriting `## Handoff`, and history outlives the event log's
retention: it lasts as long as the ticket, archived with it and removed
only by its permanent delete. Agents' notes stay in the ticket's
`## Notes`; handoffs and questions are read from the ticket and the log.

A ticket without the file has no activity; existing tickets need no
migration. The file is created by its first entry:

```markdown
# Activity: FH-42

### 2026-10-13T09:10:53Z · history · Robert · 0a1b2c3d

- created: backlog

### 2026-10-13T09:12:00Z · comment · Robert · 4e5f6a7b

state: open
edited: 2026-10-13T09:20:00Z
quote: Acceptance Criteria 2

> Tickets keep their place
> after a reload.

They do not keep it in Done.

### 2026-10-14T08:02:11Z · history · claude:5b0c7e2a-1f3d-4c8e-9a61-2d7f0e4b9c13 · 2a3b4c5d

- moved: up-next → in-progress
- edited: branch
```

**Entries.** An entry starts at a heading line outside code fences of
exactly the form `### <time> · <kind> · <author> · <id>` (the separator is
a space, U+00B7 and a space) and runs to the next one; text before the
first entry is the file's title and is kept as written.

- `time` is when it was written, RFC 3339 in UTC to the second.
- `kind` is lower-case letters, digits and `-`: `comment` or `history`.
  An entry of another kind, or whose time does not parse, is kept as
  written and not shown, so a later version can add kinds.
- `author` is who wrote it: a person's name (`user_name` in `config.yaml`,
  or the computer account's name; RUN-8) for the board's writes, or the
  agent's full run id (agent-protocol §2), or `unknown` for an agent call
  with no run. It is one line, at most 300 characters, with `·` written as
  `-`. A name that is not a run id and not `unknown` is a person.
- `id` is 1 to 64 letters, digits, `_` or `-`; Flashheart writes eight
  random hex digits, not used in the file, so two copies of a synced board
  do not collide.

**Comments** are written by people on the board, never by agents (D47).
After the heading and a blank line come its attributes, one `key: value`
line each up to a blank line, always at least `state`:

- `state`: `open` (new), `change-requested`, `addressed` or `resolved`
  (REV-6, FH-126); another value reads as `open`. A person asks for a
  change (from any state, so an addressed or resolved request is
  reopened) and resolves a request; an agent only marks a change request
  addressed. A comment never goes back to `open`.
- `edited`: when it was last edited, RFC 3339 UTC; absent until then. The
  board shows the comment as edited.
- `sent-to`: the run id (agent-protocol §2) of the live session, or one
  of its subagents, the person also sent the comment to when posting it
  (**Tell a session now**, D47, FH-127); absent for a comment not sent.
- `delivered`: when that session was handed the comment, RFC 3339 UTC;
  absent until then. Once it is set the comment can no longer be edited
  or deleted: the session has read what it says. Without it, a comment whose run has
  ended (or left the run window) was not delivered, and the board says
  so.
- `quote`: where the quoted part of the ticket is from (one line, at most
  200 characters, such as `Acceptance Criteria 2`); present only with a
  quote, which follows as a blockquote (`> ` before each line).
- Other keys are kept, in place, when the comment is edited.

Then its text, markdown, at most 16 KiB (`board.MaxSectionText`); a quote
is at most 4 KiB. Text and quote pass the secret scrubber (SEC-3), and a
heading of level 1 to 3 in the text outside code fences becomes level 4
and a code fence left open is closed, so it cannot start or hide an entry
(FH-4). It is shown as markdown without raw HTML (SEC-4).

A person may **delete** a comment posted by mistake (FH-140): its entry
goes, heading to the next entry, with the blank lines that separated a
last entry from the one before it, so the file reads as if it had never
been appended. A comment that is `delivered`, or whose `state` is
`change-requested` or `addressed`, is not deleted (a request is resolved
first). The write's history names the comment by id, never its text.

**History** entries list what one write changed, one bullet each, and
never the ticket's text:

| Bullet | Meaning |
|---|---|
| `- created: <status>` | the ticket was created in that column |
| `- moved: <from> → <to>` | its `status` changed (a column or `rejected`; `->` is read too) |
| `- edited: <names>` | frontmatter keys (`priority`, `pr`), `title`, sections (`description`, `acceptance-criteria`, `test-plan`, `reproduction`, `context`), `review`; `body` for other text, `frontmatter` for a key Flashheart would not name or frontmatter that does not parse |
| `- ticked: <items>`, `- unticked: <items>` | `criterion N` and `review step N`, numbered from 1, when only their boxes changed |
| `- archived`, `- restored` | archived or restored (EDIT-8) |
| `- change-requested: <comments>` | comments the person asked for as changes (a new one written as one too), as `comment <id>` (REV-6, FH-126) |
| `- addressed: <comments>` | change requests an agent marked addressed |
| `- resolved: <comments>`, `- reopened: <comments>` | change requests the person resolved, or asked for again after they were addressed or resolved |
| `- deleted: <comments>` | comments the person deleted (FH-140), as `comment <id>` |

A bullet of another form is kept and not shown. History is worked out
from the write itself, comparing the ticket (and, for a review step, its
`review.md`; for a comment's state, this file's comments) before and
after, so every write path is described alike (D58).
`updated` and `rank` are not history, and neither are `## Notes` and
`## Handoff`: a reorder within a column, a note, a checkpoint or an answer
records none. A write that changes nothing records nothing.

**Writes.** Every write is under the project lock and atomic (STO-3) and
never touches the ticket file. A history entry is staged in the same
write as the ticket change it describes, written after it (§Multi-file
writes); a history that cannot be staged fails the write, and an archive
or restore is moved back. A comment is appended to the file as it is under
the lock; an edit replaces only that comment's entry, its heading,
unknown attributes and following blank lines kept, and carries the hash of
the file the person read: a stale hash is a conflict and writes nothing.
An edit of a delivered comment is refused (`delivered`). A delete, like
an edit, carries the hash of the file the person read and removes only
that comment's entry; a delivered comment is refused (`delivered`), an
open or addressed change request too (`open_request`), and the history
entry is staged in the same write. Marking a comment
delivered is done by whoever delivers the session's inbox (agent-protocol
§4), never by a person; like a state change it rewrites that comment's
entry, merging with the file as it is under the lock, and writes nothing
when the comment was not sent to that run or is delivered already; it is
not history. A state change rewrites that comment's entry the same way but merges
with the file as it is under the lock, and a request for changes moves
the ticket and marks its comments in one write, with one history entry.
New entries follow a blank line, in the file's line endings (CRLF when its
first line ends so); every other byte, a byte-order mark included, is kept.
The file is limited to 1 MiB: a comment that would pass it is refused,
and history stops being recorded (the ticket's writes go on). Serve's own
copies of linked files (REV-5) and `migrate` record no history.

## Files and the files index

`tickets/<folder>/files/index.yaml`:

```yaml
- file: 20261004T1412-board-desktop.png
  caption: Board at 1440×900, dark theme
  kind: screenshot        # screenshot | log | other
  source: /Users/robert/src/flashheart/.playwright/board.png   # where it was copied from
  run: claude:3f2a…
  added: 2026-10-04T14:12:09Z
  sha256: 9b1c…
```

Stored names are `<UTC timestamp>-<sanitised original name>`. Files are always
copies (`REV-1`); agents' own files may be cleaned up at any time, so anything
a ticket or review refers to is copied in before it is recorded (`REV-5`).
Adding a file appends one entry and keeps the rest of the index as written,
keys Flashheart does not know and comments included; an index that is not a
list of files is reported and never replaced. A file added from its bytes
(not a local path) has no `source`. A file a person adds on the board
(`REV-7`) has the person in `run`, its original name as the caption, and
the extension of what its content is (`SEC-7`): a PNG sent as `notes.txt`
is stored as `<time>-notes.png`; a pasted screenshot is
`<time>-screenshot.png`. Adding a file does not change the ticket file
and records no history (the event log records it, `EDIT-10`).

## project.yaml

```yaml
key: FH                   # ticket id prefix (KEY-1); unique; absent until chosen (KEY-5)
next_id: 43               # next ticket number (KEY-2)
retired: [FH-7]           # permanently deleted ids, never reused (EDIT-8); absent until a delete
name: Flashheart          # display name; defaults to the directory name
repos:                    # main-checkout paths seen by hooks (PRJ-3)
  - /Users/robert/src/flashheart
settings:                 # per-project overrides of global config
  enforce_handoff: false
  quiet_minutes: 10
```

Flashheart edits only the fields it writes (`key`, `next_id`, `retired`,
`repos`), adding a missing one at the end, and keeps every other line, the
file's line endings and a byte-order mark (`STO-2`). Every reader treats
problems alike: a file that does not parse contributes nothing, and a field
of the wrong type or an invalid key is ignored; the board shows each as a
project warning. A list may be written as a single value. A list that
cannot be read is never replaced: adopting a repository or retiring an id
is refused until it is repaired.

## config.yaml

```yaml
version: 2
auto_create_projects: true
quiet_minutes: 10
lease_minutes: 30         # a claim outlives its run's last activity this long
activity_seconds: 60      # at most one activity record per run this often (agent-protocol §3)
enforce_handoff: false    # HOOK-6; a project's settings.enforce_handoff overrides it
user_name: ""             # who answers agents' questions (RUN-8); empty: the computer account's name
event_retention_days: 90
done_column_limit: 20
attachments:
  max_bytes: 20971520
ui:
  theme: system           # system | light | dark
  density: normal         # compact | normal | detailed
  colour_by: type         # type | priority | age | none
  hidden_columns: []      # real columns left off the Board: backlog (FH-41); absent shows all
  editor: formatted       # formatted | markdown: how a section opens for editing (FH-16)
  scopes:                 # remembered view and filters, per project or "all"
    flashheart: {view: board, type: {include: [bug, spike]}, workstream: {exclude: [board-ui]}, state: blocked}   # view: board, agents, workstreams or table
  expanded_workstreams:   # finished workstreams shown expanded, per project (FH-68); absent: all collapsed
    flashheart: [foundation]
```

Each of a scope's `type`, `priority`, `workstream` and `age` (today, week or
older) filters lists the values shown only (`include`, any of them) and the
values hidden (`exclude`), at most 100 each. A bare value, the form before
FH-39, reads as one included value; `hide_later` is ignored. A scope's
`state` is one of `all`, `blocked`, `unblocked`, `repair`, `working` or
`rejected` (FH-59); empty means all. The
`virtual_columns` list of earlier versions
is ignored, and dropped when preferences are next saved.

`expanded_workstreams` lists at most 500 workstream slugs per project. The
Workstreams view drops a slug once its workstream is no longer finished or
no longer exists, so the choice lasts only while the workstream stays finished.

Flashheart rewrites only the `ui` keys when preferences change, keeping the
other settings, unknown keys and comments.

`config.yaml` is read through the root like every board file (SEC-2): a
symlink to a file inside the root is followed, but one that points outside
it is refused, as unreadable settings (`serve` does not start, hooks log the
error to `hook-errors.log` and record nothing, MCP tools fail, `doctor`
reports a problem). A file larger than 1 MiB is refused the same way.

A root without `config.yaml` is version 2 unless a project in it still has v1
column folders (`todo/`, `in-progress/`, `ready-to-review/`, `done/`).

## Event log

`<project>/.flashheart/events/YYYY-MM-DD.jsonl` (UTC date), one JSON object
per line, appended under the project lock by `flashheart hook` (and later
`mcp` and `serve`). Schema and event kinds:
`docs/dev/specs/agent-protocol.md` §3. Events name tickets by id. Readers skip
malformed lines and unknown kinds. Files older than `event_retention_days` are
deleted by `serve` at startup and daily, with the session inboxes (in
`inbox/` or the old `answers/`) last written on their days, after which
`serve` moves any inbox left in `answers/` into `inbox/`; at the same time `serve` deletes activity state no
hook has written for `stale_hours` (12), left by a session that never
reported its end (agent-protocol §3, §4; FH-82). The only other deletions are
permanent deletes the user confirms: of an archived ticket (`EDIT-8`), whose
events are left to this retention, and of an archived project (`PRJ-5`),
whose event log goes with its directory.

## Archived projects

Archiving moves `<root>/<project>/` to `<root>/.archive/<project>/` under
the root lock and sets the directory's time, which dates the archive.
Restoring moves it back, refused while a live project has that name. An
archived project's ticket ids (live and archived inside it) count as done
for other projects' blocking, and its key stays taken. A permanent delete
checks its preview as a ticket's does (§Columns): the same reference scan,
for any of the project's ids, without a lock and again under the root lock
and the lock of each project it rewrites, in name order; its token also
covers the project's key and ticket ids. It removes other projects'
`depends-on` entries naming its tickets,
adds its key to `<root>/.flashheart/retired.yaml` (`keys: [AL]`), and
removes the directory. All of it is by project name, never a path, and a
symbolic link anywhere on the project's path is refused. A writer that was waiting for a
project's lock while it was archived finds it gone and writes nothing.

## Migration from v1

`flashheart migrate` shows the plan and changes nothing; `--write` applies it
(`MIG-1`):

1. Each project gets a key: its existing `key`, a `--key <project>=<KEY>`
   argument, or the derived default.
2. Tickets, including archived ones, are numbered once in `created` order
   (then by v1 slug) and written to `tickets/<id>-<slug>/<id>-<slug>.md` with `id`
   and `status` inserted at the top of the frontmatter (v1 `todo` becomes
   `backlog`, `ready-to-review` becomes `review`) and the `<type>--` prefix
   dropped from the slug. Unparseable tickets are copied byte for byte and
   stay *needs repair*.
3. `depends-on` slugs and workstream `tickets:` lists are rewritten to ids;
   reviews become `review.md`; attachments move to `files/`; ticket links in
   reviews and tickets are rewritten to ids.
4. The v1 files and folders are moved, never deleted, to
   `.flashheart/backup/v1-<UTC timestamp>/`, and `config.yaml` records
   `version: 2`.

Every frontmatter and YAML edit keeps the file's line endings and
byte-order mark (`STO-2`). `--write` runs under the root lock and the
locks of the projects it converts, and stops without writing if those
projects are no longer v1. A `project.yaml` or `config.yaml` that does
not parse stops the plan before anything is written; a workstream file
that does not parse is warned about and kept as written.

`serve` on a v1 root shows the migration command instead of the board and
writes nothing. Flashheart refuses to write to a root with a newer version
than it understands.
