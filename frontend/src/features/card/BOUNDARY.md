# Card feature boundary

Owns the card panel (CARD-1, ui-layout.md §3) and the ticket's full page
(`TicketPage`, CARD-7). Both compose shared parts: `useTicket` (load,
reload, criterion and review-step ticks through `editing`'s
`saveWithBase`, answers, the page's field and section saves),
`TicketHeader`'s `TicketIdentity`, `TicketTab`'s parts (`RepairNotice`,
`QuestionsCallout`, `HandoffCallout`, `ReasonList`, `FormatWarnings`,
each taking its heading level), `Checklist` and `Attachments` (grid and
lightbox).

Panel only: `TicketProperties` and `TicketActions` (Move to, Position,
Archive), and its tabs (`TicketSections`, `EditTab` through
`useTicketDraft`, `RunsTab`, `RunDetail`, `ReviewTab`).

Page only: `PropertiesPanel` (Move to, a row per field, Archive),
`SectionOutline` and `JumpTo`, `useSectionInView`, `ActivityTimeline`
(with its filter's state on the page, and a person's comment edited in
place) and `CommentBox` (posting a comment with its quote, FH-125), and
what each in-place item edits with (EDIT-11): `PageEdits` (title,
markdown section, criteria list) and `PropertyEdits` (a row and its typed
control).

Does not own section order, counts, review sections or which files show
(`model/ticketSections`), section text and criteria rules
(`model/sectionEdit`), activity entries, history words and filters
(`model/activity`), what a comment can quote (`model/comments`), comment
requests (`api/edit`), markdown
(`model/markdown`, `components/`), drafts, saves, the one open item,
moves and archive (`editing/`), routing (`app/`) or run state (backend).
