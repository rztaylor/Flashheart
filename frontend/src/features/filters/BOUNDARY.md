# Filters feature boundary

Owns the view toolbar shown above the Board and Table (FH-39): the type,
priority, workstream and state filter buttons and their menus, Clear
filters, and the Board's View options menu (which virtual columns are shown,
VIEW-2; card density and Colour by, VIEW-6); and the Board's chip row
(`FilterChips`): workstream chips and the Colour by value chips as filters,
kept to one line (VIEW-7).

Does not own filter matching, chip toggling or workstream ordering
(`model/filters`), the chip and menu primitives (`components/FilterChip`,
`components/Popover`), the search field (band in `app/`), New ticket and
the ticket count (page header in `app/`), or saving preferences (`app/`
through `state/`, CFG-2).
