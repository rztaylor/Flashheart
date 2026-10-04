# Workstreams feature boundary

Owns the Workstreams view (VIEW-4) and its signature transit diagram: each
workstream drawn as its line with tickets as stations in order, derived
status, progress, and blocked, repair and warning notes. Loads each
project's workstreams; stations open the card panel.

Does not own line colour assignment (`model/lines`), blocking rules
(backend), routing or the card panel. Reordering stations arrives with
board-editing (EDIT-4).
