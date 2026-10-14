# Model boundary

Owns pure view-model logic shared across features, one module per concern:
workstream lines and titles (`lines`), filters and search, card paint,
status tones, times, markdown links and ticket ids (`markdown`), the scope
and its remembered view (`scopes`), run-state words (`runs`; the server
derives the reason, D39), the Overview's sections and rows, grid movement
and focus neighbours (`navigation`), manual order, archive search and the
delete gate, a ticket's sections, activity, comments, change requests and
telling a session, form fields and section edits (`ticketForm`,
`sectionEdit`, `workstreamSections`), uploads, lightbox zoom, the
marginal remarks (`remarks`, `remarks.md`) and a failure's words
(`errors`). Everything here is a plain function or type, unit-tested with
Vitest.

Does not own React components, data loading, requests or styling. It may
import API types; nothing here imports components, features or the app.
