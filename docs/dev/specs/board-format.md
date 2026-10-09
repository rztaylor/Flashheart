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
├── _scratch/                             agent activity outside any git repository (PRJ-4)
└── <project>/                            e.g. flashheart
    ├── project.yaml                      key, display name, repos, next id (PRJ-3, KEY-1)
    ├── workstreams/<slug>.md
    ├── tickets/
    │   └── FH-42-card-panel/             one folder per ticket: <id>-<slug>
    │       ├── FH-42-card-panel.md       the ticket, named after its folder
    │       ├── review.md                 review guide (REV-4), optional
    │       └── files/                    copied files (REV-1, REV-5)
    │           ├── index.yaml
    │           └── 20261005T1412-board-desktop.png
    ├── .archive/tickets/FH-7-old-idea/   archived tickets (EDIT-8)
    └── .flashheart/
        ├── lock                          advisory project lock (STO-3)
        └── events/2026-10-05.jsonl       event log (STO-5)
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
- Directories starting with `.` are Flashheart's or archives. Unknown files and
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
status: up-next          # backlog | up-next | in-progress | review | done
type: feature            # feature | test | bug | refactor | infra | docs | spike
priority: high           # high | medium | low
created: 2026-10-04
branch: feature/x        # empty until work starts
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
| `status` | yes | The column. Missing reads as `backlog` with a warning; an unknown value marks the ticket *needs repair* (shown in Backlog). |
| `type` | yes | `feature`, `test`, `bug`, `refactor`, `infra`, `docs`, `spike`. |
| `priority` | yes | `high`, `medium`, `low`. |
| `created` | yes | `YYYY-MM-DD`. Orders numbering at migration and ties in sorting. |
| `branch` | no | Used for provisional run links (`RUN-5`). |
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

A move is a frontmatter edit of `status` under the lock, applied to the file
as it is then (`STO-3`); other edits from the UI and agents carry the content
hash they read. Every ticket write by Flashheart stamps `updated`; archiving
stamps it too, which dates the archive. The folder never moves except to
`.archive/` and back, and is removed only by a permanent delete of an
archived ticket (`EDIT-8`): the delete takes the root lock and the lock of
every project it rewrites (in name order), removes the id from every
ticket's `depends-on` and the project's workstream `tickets:` lists, adds
it to `retired` in `project.yaml`, and removes the folder. It takes an id,
never a path, and refuses any symbolic link on the folder's path.

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
log keeps history.

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
assessment objectives?" Answer (Robert): "Yes".`

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
writes (ticket creation and the agent tools) change the field and the lists
together, appending a joining ticket to the end of its list. A reference to a
missing ticket or workstream (including a missing `workstream:`) is shown as a
warning and counts as blocking. Archived tickets count as done, and so do
permanently deleted ids (a project's `retired`) and every id of a deleted
project's key (`retired.yaml`), wherever a reference to them survives. A duplicated
id counts as done only when every copy does. Only tickets in `backlog`,
`up-next` and `in-progress` are shown as blocked.

## Workstream

`workstreams/<slug>.md`:

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

## config.yaml

```yaml
version: 2
auto_create_projects: true
quiet_minutes: 10
lease_minutes: 30         # a claim outlives its run's last activity this long
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
  scopes:                 # remembered view and filters, per project or "all"
    flashheart: {view: board, type: {include: [bug, spike]}, workstream: {exclude: [board-ui]}, state: blocked, needs_you: only}   # view: board, agents, workstreams or table; needs_you: only or hidden
```

Each of a scope's `type`, `priority`, `workstream` and `age` (today, week or
older) filters lists the values shown only (`include`, any of them) and the
values hidden (`exclude`), at most 100 each. A bare value, the form before
FH-39, reads as one included value; `hide_later` is ignored. `needs_you` is
the Needs you chip (FH-44). The `virtual_columns` list of earlier versions
is ignored, and dropped when preferences are next saved.

Flashheart rewrites only the `ui` keys when preferences change, keeping the
other settings, unknown keys and comments.

A root without `config.yaml` is version 2 unless a project in it still has v1
column folders (`todo/`, `in-progress/`, `ready-to-review/`, `done/`).

## Event log

`<project>/.flashheart/events/YYYY-MM-DD.jsonl` (UTC date), one JSON object
per line, appended under the project lock by `flashheart hook` (and later
`mcp` and `serve`). Schema and event kinds:
`docs/dev/specs/agent-protocol.md` §3. Events name tickets by id. Readers skip
malformed lines and unknown kinds. Files older than `event_retention_days` are
deleted by `serve` at startup and daily. The only other deletions are
permanent deletes the user confirms: of an archived ticket (`EDIT-8`), whose
events are left to this retention, and of an archived project (`PRJ-5`),
whose event log goes with its directory.

## Archived projects

Archiving moves `<root>/<project>/` to `<root>/.archive/<project>/` under
the root lock and sets the directory's time, which dates the archive.
Restoring moves it back, refused while a live project has that name. An
archived project's ticket ids (live and archived inside it) count as done
for other projects' blocking, and its key stays taken. A permanent delete
takes the root lock and the lock of each project it rewrites (in name
order), removes other projects' `depends-on` entries naming its tickets,
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

`serve` on a v1 root shows the migration command instead of the board and
writes nothing. Flashheart refuses to write to a root with a newer version
than it understands.
