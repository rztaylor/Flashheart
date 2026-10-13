# Editing feature boundary

Owns the board's write interactions that span views: moving tickets, one
save at a time (`pending`), with the optimistic column and place (EDIT-9), the blocked-move confirmation of EDIT-2, review
warnings of EDIT-3 and Undo), archiving with Undo (EDIT-8), the New ticket
dialog with project key choice (EDIT-5, KEY-5), answering agents' questions
from the card panel and the Overview (CARD-6), and the toast that announces
each result.

Owns ticket edits under the content-hash precondition (STO-3, EDIT-7):
`saveWithBase` is the one save that sends a base hash and sorts saved,
conflict and failure; `useTicketDraft` is one draft's base, following the
file, adopting the saved version, busy and error, and a conflict as a
pending choice (overwrite, reload, cancel) for the conflict dialog.

Does not own request shapes (`api/edit`), a draft's fields (`model/`),
board layout or drag sources (`board/`), the card panel and its tabs
(`card/`), or routing (`app/`).
