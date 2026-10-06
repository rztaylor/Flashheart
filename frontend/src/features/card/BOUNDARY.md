# Card feature boundary

Owns the card panel (CARD-1): loading one ticket and reloading it when the
board changes, its header with Move to and Archive, the Ticket tab (needs
repair, blocked by with links, agents' questions about the ticket with
their answers (CARD-6), handoff with a warning when a run has
edited since it (CARD-5), tickable acceptance criteria, warnings and the
rendered markdown), the Edit tab (typed fields and the raw file, EDIT-6),
the Runs tab (the ticket's linked runs, reusing the agents run detail) and
the Review tab (attachments and the review file).

Does not own the side-panel shell, dialogs or markdown rendering
(`components/`), the move, archive and conflict flows (`editing/`),
routing (`app/`), run state (backend), or attachment upload, which
arrives in a later roadmap item.
