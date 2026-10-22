# Workstreams and changing scope

Read this before you plan work across tickets, add dependencies, or change
a ticket's scope.

## Workstreams

- A workstream groups tickets with a shared goal, like an epic, worked on
  in sequence or in parallel. Its order never blocks.
- Work spanning several tickets belongs in a workstream. Join one
  `board_context` lists by passing `workstream` to `create_ticket` or
  `update_ticket`, or make one with `create_workstream` and change it with
  `update_workstream` (`rename_to` renames its slug). Leave single tickets
  out of workstreams.

## Dependencies

- A ticket waits only for the tickets its `depends_on` names and for
  workstreams it or its workstream depends on.
- Give each ticket the dependencies it really needs, naming several tickets
  in `depends_on` when it needs them all.
- A workstream waiting for another takes the dependency itself, rather
  than each of its tickets.
- A blocked ticket needs `force` and a reason to claim.

## When the scope changes

Rewrite the ticket instead of adding a note that contradicts it:
`update_ticket`'s `sections` replaces sections (Description, Test Plan,
Context…); `criteria` adds, rewords, removes or moves criteria, keeping
ticks. Handoff and Notes are never rewritten: `checkpoint` writes the
handoff and `append_notes` adds to the notes.
