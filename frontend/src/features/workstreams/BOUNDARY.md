# Workstreams feature boundary

Owns the Workstreams view (VIEW-4, `docs/dev/specs/ui-layout.md` §4): each
workstream as a route card with its stations on a railway graph of their
depends-on links (`graph.ts` lays it out; a check only for finished tickets,
next stops as interchange rings, a status pill under each) and its
independent stations below; derived status, progress, and blocked, repair
and warning notes. A finished workstream collapses to its head, a
disclosure button, unless the user expanded it (`collapse.ts` decides
finished and drops stale expanded choices; the choices themselves are a
preference owned by the shell). Loads each project's workstreams;
stations open the card panel (one button covering each station, its id
linked to the full page above it, CARD-7), and independent stations reorder
by drag or Shift and an arrow (EDIT-4). Its load states (ui-layout.md §8):
`WorkstreamsSkeleton` (route cards; the shell shows it while the projects
load) and a route card skeleton per loading project, the shared error with
Try again and the last-good-copy note per project.

Does not own line colour assignment (`model/lines`), blocking rules
(backend), routing, the card panel or the toast (`editing/`).
