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
stations open the card panel (CARD-7), and independent stations reorder
by drag or Shift and an arrow (EDIT-4), keeping focus through
`components/useKeepFocus`. Its load states (ui-layout.md §8):
`WorkstreamsSkeleton` (route cards; the shell shows it while the projects
load) and a route card skeleton per loading project, the shared error with
Try again and the last-good-copy note per project.

Owns the New workstream dialog (EDIT-14, FH-17); the shell opens it and
the page it creates.

Owns the workstream page too (EDIT-13, FH-139, ui-layout.md §4
"Workstream page"): it loads one workstream (its line, body and hash),
shows the route card's head and stations, and edits each section in place
through `editing/`'s `InPlaceEditing` and `InPlaceEditor` with `editor/`'s
`MarkdownField`, saving one section on the hash it opened on, and its
name in place (`WorkstreamName`, FH-167): saved with the slug the server
makes from it, telling the shell the new slug so the page can follow. A
workstream gone from its slug while the page is open (renamed or removed
elsewhere, FH-174, D80) is followed the same way when `successor.ts` finds
what it became among the project's workstreams (its name, or all its
tickets), else the page says it is no longer there with a link to the
project's workstreams. Which
sections show, and where a missing template one goes, is
`model/workstreamSections`. The route card's name links to the page.

Does not own line colour assignment (`model/lines`), blocking rules
(backend), routing, the card panel, the in-place editing, conflict dialog
and toast (`editing/`), the editor (`editor/`), or which sections may be
rewritten, slugs, how a name makes one, or the template (backend), or ticket membership (`card/`).
