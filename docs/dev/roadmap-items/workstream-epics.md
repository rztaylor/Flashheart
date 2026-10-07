# workstream-epics

**Status:** Partial. Ticket: FH-38. Decision: D26.

## Goal

Workstreams work like epics: tickets that make up a larger goal and may be
worked on in sequence or in parallel. Only `depends-on` orders them, and the
Workstreams view shows those dependencies as a railway graph.

## Scope

- Board format: workstream order never blocks (the old blocking rule 3 is
  gone); derived status and next ticket for epics
  (`docs/dev/specs/board-format.md` §Blocking, §Workstream).
- MCP: `create_workstream` and the skill text describe workstreams as epics
  ordered by `depends_on`; blocked reasons follow.
- API: each workstream ticket's dependencies on the line (`dependsOn`) and
  unfinished ones elsewhere (`outside`).
- Workstreams view: the railway graph (`docs/dev/specs/ui-layout.md` §4,
  `VIEW-4`), independent stations below it, `EDIT-4` for those only.

## Acceptance criteria

- No workstream blocks by order; depends-on, depends-on-workstreams and a
  workstream's own dependencies still block (`CARD-4`).
- Epic derived status, progress and next ticket (`VIEW-4`).
- MCP contract tests for `create_workstream`, `board_context`,
  `list_tickets` and `get_ticket`.
- The graph draws chains, parallel tracks, joins for several dependencies
  and dependencies outside the line, in both themes and at phone width, and
  the user approves it.

## Exclusions

The New workstream dialog, the membership picker and the follow-through after
New ticket stay with FH-17. No migration adds depends-on to existing
workstreams.

## Dependencies

None.

## Validation

`scripts/check.sh` and `scripts/e2e.sh`; review screenshots of the
Workstreams view in light, dark and at 390 px.
