# Filters feature boundary

Owns the filter bar shown above the Board and Table: type, priority,
workstream, state and later-possibility controls, the result count, and the
card density control (VIEW-6, VIEW-7).

Does not own filter matching (`model/filters`), the search field (header in
`app/`), or saving preferences, which arrives with backend preference writes
(CFG-2).
