# Board feature boundary

Owns the Board view (VIEW-1, VIEW-6): real columns in workflow order,
ticket cards in three densities with workstream stripes and bullets, the
"Colour by" header tint and tag with its colour key, blocked proof and running
times, the line legend that dims other workstreams, the done-column limit
toggle, arrow-key movement between cards, moving tickets between columns by
drag or Shift and an arrow (EDIT-1), and keeping the selected card's column
in view when the panel opens.

Does not own data loading, filtering (`model/filters`), what each colour
means (`model/paint`), the card panel
(`card/`) or routing. Virtual columns and live badges arrive with runs.
