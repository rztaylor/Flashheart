# Workstreams feature boundary

Owns the Workstreams view (VIEW-4, `docs/dev/specs/ui-layout.md` §4): each
workstream as a route card with its stations on a railway graph of their
depends-on links (`graph.ts` lays it out; a check only for finished tickets,
next stops as interchange rings, a status pill under each) and its
independent stations below; derived status, progress, and blocked, repair
and warning notes. Loads each project's workstreams; stations open the card
panel (one button covering each station, its id linked to the full page
above it, CARD-7), and independent stations reorder by drag or Shift and an arrow
(EDIT-4).

Does not own line colour assignment (`model/lines`), blocking rules
(backend), routing, the card panel or the toast (`editing/`).
