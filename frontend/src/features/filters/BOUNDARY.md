# Filters feature boundary

Owns the view toolbar shown above the Board and Table: New ticket, the
type, priority, workstream, state and later-possibility filters, which
virtual columns are shown (VIEW-2), and the card density and Colour by
controls (VIEW-6, VIEW-7).

Does not own filter matching (`model/filters`), the search field (band in
`app/`), the ticket count (page header in `app/`, worded by
`model/summary`), or saving preferences (`app/` through `state/`, CFG-2).
