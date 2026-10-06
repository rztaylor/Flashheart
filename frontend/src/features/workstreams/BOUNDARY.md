# Workstreams feature boundary

Owns the Workstreams view (VIEW-4, `docs/dev/specs/ui-layout.md` §4): each
workstream as a route card with its line's stations in ticket order (a check
only for finished tickets, the next stop as the interchange ring, a status
pill under each), derived status, progress, and blocked, repair and warning
notes. Loads each
project's workstreams; stations open the card panel and reorder along the
line by drag or Shift and an arrow (EDIT-4).

Does not own line colour assignment (`model/lines`), blocking rules
(backend), routing, the card panel or the toast (`editing/`).
