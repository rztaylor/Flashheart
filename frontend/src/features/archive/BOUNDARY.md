# Archive feature boundary

Owns the permanent-delete dialog (`TypedDeleteDialog`: preview, typed
word, red confirmation, reload on a change), a project's Archive view
(EDIT-8: searchable archived tickets with their return column, links to
the full page, Restore and Delete permanently with the previewed effects)
and the projects archive (PRJ-5: Restore, Delete permanently, and Archive…
with its warning about live work). Both load, fail and go stale in the
shared way (ui-layout.md §8), loading as `ArchiveSkeleton`.

Does not own requests (`api/archive`, `api/edit`), search and the typed-id
rule (`model/archive`), archiving from the card panel (`card/`,
`editing/`), the toast (`editing/`) or routing and the page header
(`app/`).
