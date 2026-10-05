# Workstreams feature boundary

Owns the Workstreams view (VIEW-4) and its signature transit diagram: each
workstream drawn as its line with tickets as stations in order, derived
status, progress, and blocked, repair and warning notes. Loads each
project's workstreams; stations open the card panel and reorder along the
line by drag or Shift and an arrow (EDIT-4).

Does not own line colour assignment (`model/lines`), blocking rules
(backend), routing, the card panel or the toast (`editing/`).
