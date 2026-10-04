# Board format (v1)

The on-disk format Flashheart reads and writes. It is the kanban-tracker skill's
format with a few additive extensions, so existing boards work unchanged
(`STO-1`). Requirement ids refer to `docs/SPEC.md`.

## Layout

```text
<root>/                                  default ~/reports/Kanban
├── .flashheart/
│   ├── config.yaml                      global settings and UI preferences (CFG-1, CFG-2)
│   ├── cache/cwd.json                   cwd → project/branch cache (agent-protocol §2)
│   ├── hook-errors.log                  hook failures (HOOK-1), rotated at 1 MB
│   └── serve.log                        background serve diagnostics (LIFE-4), rotated at 1 MB
├── .archive/                            archived projects (PRJ-5)
├── _scratch/                            agent activity outside any git repository (PRJ-4)
└── <project>/                           e.g. ngplus
    ├── project.yaml                     optional (PRJ-3)
    ├── workstreams/<slug>.md
    ├── todo/<type>--<slug>.md
    ├── in-progress/<type>--<slug>.md
    ├── ready-to-review/<type>--<slug>.md
    ├── done/<type>--<slug>.md
    ├── reviews/<type>--<slug>.md        review guide; same filename as its ticket
    ├── attachments/<type>--<slug>/      files added with `attach` (REV-1)
    │   ├── 20261004T1412-board-desktop.png
    │   └── index.yaml                   caption, kind, run and time per file
    ├── .archive/                        archived tickets (EDIT-8), same column layout
    └── .flashheart/
        ├── lock                         advisory project lock (STO-3)
        └── events/2026-10-04.jsonl      event log (STO-5)
```

Rules:

- A ticket's column is its directory. Exactly one file per slug may exist
  across the four columns; if two exist, both are shown as *needs repair*.
- Directories starting with `.` are Flashheart's or archives; Obsidian and most
  editors hide them.
- Unknown files and directories are ignored and preserved.

## Ticket

Filename: `<prefix>--<kebab-slug>.md`, prefix one of `feat`, `test`, `bug`,
`refactor`, `infra`, `docs`, `spike`. The slug is the filename without `.md`.

```markdown
---
type: feature            # feature | test | bug | refactor | infra | docs | spike
project: ngplus
created: 2026-10-04
priority: high           # high | medium | low
session: claude-code-desktop:3321f965-…   # creating session (free text)
git-ref: cf98240
branch: feature/x        # empty until work starts
workstream: ofqual-regulatory-layer
depends-on: [feat--other-ticket]
depends-on-workstreams: []
tags: [assessment]
updated: 2026-10-04T14:12:09Z    # v1 extension, optional: last write by Flashheart
---

# Title

## Description
## Acceptance Criteria
- [ ] criterion
## Test Plan            (feature)   /   ## Reproduction   (bug)
## Context
## Notes
## Handoff              (v1 extension, written by checkpoint)
```

### Frontmatter

| Field | Required | Notes |
|---|---|---|
| `type` | yes | Must agree with the filename prefix (`feat` ↔ `feature`); disagreement is a warning, not an error. |
| `project` | yes | Informational; the directory decides the project. |
| `created` | yes | `YYYY-MM-DD`. |
| `priority` | yes | `high`, `medium`, `low`. |
| `session`, `git-ref` | yes in the skill, optional here | Free text. |
| `branch` | no | Used for provisional run links (`RUN-5`). |
| `workstream` | no | Slug of a workstream in this project. |
| `depends-on` | no | Ticket slugs in this project. `<project>/<slug>` refers to another project (v1 extension). |
| `depends-on-workstreams` | no | Workstream slugs. |
| `tags` | no | Free text. |
| `updated` | no | v1 extension. RFC 3339 UTC. |

Unknown keys are preserved in place (`STO-2`).

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

A ticket is **blocked** when any of these holds (the kanban-tracker rules):

1. a `depends-on` ticket is not in `ready-to-review/` or `done/`;
2. a `depends-on-workstreams` workstream has a ticket not in `ready-to-review/`
   or `done/`;
3. it belongs to a workstream and an earlier ticket in that workstream's
   `tickets:` list is not in `ready-to-review/` or `done/`.

A reference to a missing ticket or workstream is shown as a warning and counts
as blocking. Archived tickets count as done for blocking.

## Workstream

`workstreams/<slug>.md`:

```markdown
---
slug: ofqual-regulatory-layer
status: active            # active | blocked | completed (informational; derived in the UI)
priority: high
created: 2026-10-04
tickets:                  # ordered; order implies blocking
  - bug--board-label-claims-known-specification
  - feat--ofqual-assessment-objectives
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
`blocked` per the rules above) and does not rewrite the `status` field unless
the user edits it.

## Review

`reviews/<same filename as the ticket>.md`, in the kanban-tracker review
template (Summary, Key Files, How to Verify, Risks, PR Notes). Screenshots are
referenced with paths relative to the review file:
`![Board at desktop](../attachments/feat--x/20261004T1412-board-desktop.png)`.

## Attachments index

`attachments/<ticket>/index.yaml`:

```yaml
- file: 20261004T1412-board-desktop.png
  caption: Board at 1440×900, dark theme
  kind: screenshot        # screenshot | log | other
  run: claude:3f2a…
  added: 2026-10-04T14:12:09Z
  sha256: 9b1c…
```

Stored names are `<UTC timestamp>-<sanitised original name>`.

## project.yaml

```yaml
name: NG+                 # display name; defaults to the directory name
repos:                    # main-checkout paths seen by hooks (PRJ-3)
  - /Users/robert/src/ngplus
settings:                 # per-project overrides of global config
  enforce_handoff: false
  quiet_minutes: 10
```

## config.yaml

```yaml
version: 1
auto_create_projects: true
quiet_minutes: 10
event_retention_days: 90
done_column_limit: 20
attachments:
  max_bytes: 20971520
ui:
  theme: system           # system | light | dark
  density: normal         # compact | normal | detailed
  virtual_columns: [needs-you]
```

## Event log

`<project>/.flashheart/events/YYYY-MM-DD.jsonl` (UTC date), one JSON object
per line, appended under the project lock. Schema and event kinds:
`docs/dev/specs/agent-protocol.md` §3. Readers skip malformed lines and
unknown kinds. Files older than `event_retention_days` are deleted by `serve`
at startup and daily; this is the only deletion Flashheart performs.

## Compatibility

- **v1 readers** (the kanban-tracker skill, Obsidian, humans) can ignore every
  extension: `updated`, `## Handoff`, `done/`, `.archive/`, `attachments/`,
  `.flashheart/`, cross-project `depends-on`.
- A future incompatible change bumps `version` in `config.yaml` and ships a
  migration command; Flashheart refuses to write to a root with a newer
  version than it understands.
