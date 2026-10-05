# Card feature boundary

Owns the card panel (CARD-1): loading one ticket, its header, the Ticket tab
(needs repair, blocked by with links, handoff, acceptance criteria, warnings
and the rendered markdown) and the Review tab (attachments and the review
file). Read-only until board-editing.

Does not own the side-panel shell or markdown rendering (`components/`),
routing (`app/`), or editing, runs and attachment upload, which arrive in
later roadmap items.
