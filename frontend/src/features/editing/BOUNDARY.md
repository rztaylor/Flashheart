# Editing feature boundary

Owns writes that span views: moves (one save at a time, placed at once,
with the blocked-move confirmation, review warnings and Undo), archiving
with Undo, the New ticket dialog, answering agents' questions, and the
toast announcing each result.

Owns every ticket edit under the content-hash precondition (STO-3):
`saveWithBase` is the one save that sends a base hash; `useTicketDraft`
holds one draft's base and a conflict as a pending choice for the conflict
dialog. Owns in-place editing (`InPlace`, EDIT-11): one open item per page
or panel, the unsaved-changes prompt, holding the leave guard, late saves
(`lateSaves`, D70), and `InPlaceSection`, the one section editor for
tickets and workstreams.

Does not own request shapes (`api/edit`), a draft's fields (`model/`), the
editor (`editor/`), each item's controls or the page (`card/`,
`workstreams/`), board layout and drag (`board/`), or routing and the guard
itself (`app/`, `state/`).
