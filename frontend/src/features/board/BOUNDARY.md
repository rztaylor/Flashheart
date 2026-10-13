# Board feature boundary

Owns the Board view (VIEW-1, VIEW-6): real columns in workflow order as
rounded wells, less a hidden Backlog (FH-41), ticket cards as one set of
markers in the three layouts of `docs/dev/specs/ui-layout.md` §2 (FH-147),
their workstream stripes, the "Colour by" whole-card tint, the meta line's
drop order and each card's accessible name, the done-column limit
toggle, arrow-key movement between cards, keeping a focused card's focus
as it moves or goes (through `components/useKeepFocus`, its column head the
last place focus goes), moving tickets between columns by
drag or Shift and an arrow (EDIT-1), placing them in a column's manual
order with a drop line or Shift with Up or Down (EDIT-9), keeping the selected card's column
in view when the panel opens, live run badges on cards (VIEW-8), and drawing each card
under one covering button with its id linked to the full page above it
(CARD-7), and `BoardSkeleton`, the Board's wells while it loads, less a
hidden Backlog.

Does not own data loading, filtering (`model/filters`), the toolbar and
chip row that set filters (`filters/`), what each colour means
(`model/paint`), the card panel
(`card/`), the id link (`components/TicketLink`), run state (backend) or
routing.
