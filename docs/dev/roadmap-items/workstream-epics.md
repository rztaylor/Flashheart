# workstream-epics

**Status:** Partial. Ticket: FH-38. Decision: D26.

## Goal

Workstreams work like epics: tickets that make up a larger goal and may be
worked on in sequence or in parallel. Order blocks only in a workstream that
asks for it.

## Scope

- Board format: optional `ordered` in workstream frontmatter; blocking
  rule 3 only for `ordered: true`; derived status and next ticket for
  unordered workstreams (`docs/dev/specs/board-format.md` §Blocking,
  §Workstream).
- Store and MCP: `create_workstream`'s optional `ordered`; `board_context`
  marks ordered workstreams; blocked reasons follow the field.
- Protocol skill text and `docs/dev/specs/agent-protocol.md` §7.2, §7.4,
  §12, §14 (version stays 1).
- API `ordered` on workstreams; the unordered presentation in the
  Workstreams view (`docs/dev/specs/ui-layout.md` §4, `VIEW-4`).

## Acceptance criteria

- Unordered (absent or `false`) workstreams add no order blocking; ordered
  ones block as before (`CARD-4`, board-format rule 3).
- Unordered derived status, progress and next ticket (`VIEW-4`).
- MCP contract tests for `create_workstream`, `board_context`,
  `list_tickets` and `get_ticket`.
- The user approves the unordered presentation in both themes.

## Exclusions

The New workstream dialog, the membership picker and the follow-through after
New ticket stay with FH-17. No migration rewrites existing workstreams.

## Dependencies

None.

## Validation

`scripts/check.sh` and `scripts/e2e.sh`; review screenshots of the
Workstreams view in light, dark and at 390 px.
