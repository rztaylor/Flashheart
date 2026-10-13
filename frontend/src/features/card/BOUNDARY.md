# Card feature boundary

Owns the card panel (CARD-1, ui-layout.md §3) and the ticket's full page
(`TicketPage`, CARD-7). Both compose shared parts: `useTicket` (load,
reload, criterion and review-step ticks through `editing`'s
`saveWithBase`, answers, comments, the page's field and section saves),
`TicketHeader`'s `TicketIdentity`, `TicketTab`'s parts (`RepairNotice`,
`QuestionsCallout`, `HandoffCallout`, `ReasonList`, `FormatWarnings`,
each taking its heading level), `Checklist`, `Attachments` (grid and
lightbox), `AttachmentDrop` (files dropped or chosen into the ticket on
the panel's Attachments tab and the page's Files, FH-18), and `TicketComments` (FH-125, FH-128): `ActivityTimeline`, a
person's comment edited in place, ending in `CommentBox` (posting a
comment with its quote), under each one's in-place editing coordinator.

Panel only: `TicketProperties` and `TicketActions` (Move to, Position,
Archive), and its tabs (`TicketSections`, `EditTab` through
`useTicketDraft`, `RunsTab`, `RunDetail`, `ReviewTab`, the Comments tab
kept mounted so its drafts survive a tab change).

Page only: `PropertiesPanel` (Move to, a row per field, Archive),
`SectionOutline` and `JumpTo`, `useSectionInView`, Activity's filter
state, `ChangeRequests` (the change requests list, the Request changes
dialog and a request's state pill, REV-6), and what each in-place item
edits with (EDIT-11): `PageEdits` (title, a section in the formatted
editor of `editor/`, criteria list) and `PropertyEdits` (a row and its
typed control).

Does not own section order, counts, review sections or which files show
(`model/ticketSections`), section text and criteria rules
(`model/sectionEdit`), activity entries, history words and filters
(`model/activity`), what a comment can quote (`model/comments`), comment
requests (`api/edit`), markdown
(`model/markdown`, `components/`), drafts, saves, the one open item,
moves and archive (`editing/`), routing (`app/`) or run state (backend).
