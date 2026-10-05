# Filters feature boundary

Owns the filter bar shown above the Board and Table: type, priority,
workstream, state and later-possibility controls, the result count, which
virtual columns are shown (VIEW-2), and the card density and Colour by
controls (VIEW-6, VIEW-7).

Does not own filter matching (`model/filters`), the search field (header in
`app/`), or saving preferences (`app/` through `state/`, CFG-2).
