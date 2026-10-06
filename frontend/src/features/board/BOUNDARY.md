# Board feature boundary

Owns the Board view (VIEW-1, VIEW-6): real columns in workflow order as
rounded wells, ticket cards in the one anatomy of
`docs/dev/specs/ui-layout.md` §2 at three densities with workstream stripes
and bullets, the
"Colour by" header tint and tag with its colour key, blocked proof and running
times, the workstream strip whose chips dim other workstreams, the done-column limit
toggle, arrow-key movement between cards, moving tickets between columns by
drag or Shift and an arrow (EDIT-1), keeping the selected card's column
in view when the panel opens, live run badges on cards (VIEW-8), and the
Needs you and Agent working virtual columns of mirrored cards (VIEW-2).

Does not own data loading, filtering (`model/filters`), what each colour
means (`model/paint`), the card panel
(`card/`), run state (backend) or routing.
