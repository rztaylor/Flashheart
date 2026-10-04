# Board feature boundary

Owns the Board view (VIEW-1, VIEW-6): real columns in workflow order,
ticket cards in three densities with line bullets, blocked proof and running
times, the line legend that dims other workstreams, the done-column limit
toggle, and arrow-key movement between cards.

Does not own data loading, filtering (`model/filters`), the card panel
(`card/`) or routing. Virtual columns and live badges arrive with runs.
