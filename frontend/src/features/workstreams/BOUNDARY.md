# Workstreams feature boundary

Owns the Workstreams view (VIEW-4, `docs/dev/specs/ui-layout.md` §4): each
workstream as a route card with its stations on a railway graph of their
depends-on links (`graph.ts`), independent stations below, derived status,
progress and notes; collapsing a finished workstream (`collapse.ts`; the
expanded choices are the shell's preference); opening stations in the
card panel and reordering independent ones (EDIT-4); and its load states.

Owns the New workstream dialog (EDIT-14) and the workstream page
(EDIT-13): one workstream's head, stations and sections, each edited in
place through `editing/` on the hash it opened on; which sections show is
`model/workstreamSections`.

Does not own line colours (`model/lines`), blocking rules, slugs, the
template or which sections may be rewritten (backend), routing, the card
panel and ticket membership (`card/`), in-place editing, the conflict
dialog and toast (`editing/`) or the editor (`editor/`).
