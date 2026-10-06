# Editing feature boundary

Owns the board's write interactions that span views: moving tickets (with
the optimistic column, the blocked-move confirmation of EDIT-2, review
warnings of EDIT-3 and Undo), archiving with Undo (EDIT-8), the save
conflict dialog (EDIT-7), the New ticket dialog with project key choice
(EDIT-5, KEY-5), answering agents' questions from the card panel and the
Agents view (CARD-6), and the toast that announces each result.

Does not own request shapes (`api/edit`), board layout or drag sources
(`board/`), the card panel and its Edit tab (`card/`), or routing (`app/`).
