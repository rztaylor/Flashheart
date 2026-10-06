# Archive feature boundary

Owns a project's Archive view (EDIT-8): the searchable list of archived
tickets with the column each returns to, Restore, and the Delete
permanently dialog that previews what the delete touches (folder files,
depending tickets, listing workstreams), gates on the typed id and reloads
the preview when the server reports a change; and the projects archive
(PRJ-5): archived projects with Restore and Delete permanently (typed
name), the live projects with Archive…, and the archive confirmation that
warns about live work.

Does not own requests (`api/archive`, `api/edit`), search and the typed-id
rule (`model/archive`), archiving from the card panel (`card/`,
`editing/`), the toast (`editing/`) or routing and the page header
(`app/`).
