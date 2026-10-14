# Board feature boundary

Owns the Board view (VIEW-1, VIEW-6): workflow columns as wells less a
hidden Backlog, ticket cards in the three layouts of
`docs/dev/specs/ui-layout.md` §2 with their stripes, tint, meta line,
accessible name and live run badges (VIEW-8), the done-column limit,
arrow-key movement and keeping focus (`components/useKeepFocus`), moving
tickets between columns and within a column's manual order by drag or
keyboard (EDIT-1, EDIT-9), keeping the selected card in view, each card's
covering button with its id linking to the full page (CARD-7), and
`BoardSkeleton`.

Does not own data loading, filtering (`model/filters`), the toolbar and
chips (`filters/`), what each colour means (`model/paint`), saving moves
(`editing/`), the card panel (`card/`), the id link
(`components/TicketLink`), run state (backend) or routing.
