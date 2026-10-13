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
file (moving on while its own part is unchanged, and saving again on a
conflicting version that left it unchanged), adopting the saved
version, busy and error, and a conflict as a pending choice (overwrite,
reload, cancel) for the conflict dialog.

Owns in-place editing (`InPlace`, EDIT-11): one open item per page or
card panel (FH-128), the unsaved-changes prompt and holding the leave
guard (also for held drafts
outside any item, such as a comment being written or the panel's Edit
tab, and Keep editing's return to one), an item's pencil,
Add slot, double-click and focus return, and an open item's draft with
Save, Cancel, shortcuts, inline error and conflict; and `InPlaceSection`,
one level-2 section of a file's body edited by its heading in `editor/`'s
`MarkdownField`, compared and saved again over a newer file as one rule
for a ticket's sections (`card/`) and a workstream's (`workstreams/`,
FH-139); the conflict dialog says which kind of file changed.

Does not own request shapes (`api/edit`), a draft's fields (`model/`),
the editor itself (`editor/`), the other controls each item edits with or
the page (`card/`, `workstreams/`), board layout or
drag sources (`board/`), or routing and the guard itself (`app/`,
`state/`).
