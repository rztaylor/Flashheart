# Table feature boundary

Owns the Table view (VIEW-5, `docs/dev/specs/ui-layout.md` §6): a sortable
list of the filtered tickets with status pill, type and priority tags,
workstream, state (blocked pill, repair, wait, live run), criteria and age,
where a row's title opens the card panel and its id the ticket's full page
(CARD-7).

Does not own filtering (`model/filters`), data loading or routing. Choosing
visible fields arrives with saved preferences (CFG-2).
