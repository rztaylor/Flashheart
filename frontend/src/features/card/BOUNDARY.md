# Card feature boundary

Owns the card panel (CARD-1): loading one ticket and reloading it when the
board changes, its header with Move to and Archive, the Ticket tab (needs
repair, blocked by with links, handoff, tickable acceptance criteria,
warnings and the rendered markdown), the Edit tab (typed fields and the raw
file, EDIT-6) and the Review tab (attachments and the review file).

Does not own the side-panel shell, dialogs or markdown rendering
(`components/`), the move, archive and conflict flows (`editing/`),
routing (`app/`), or runs and attachment upload, which arrive in later
roadmap items.
