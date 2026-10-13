# Card feature boundary

Owns the card panel (CARD-1, ui-layout.md §3) and the ticket's full page
(`TicketPage`, CARD-7). Both compose shared parts: `useTicket` (load,
reload, criterion and review-step ticks through `editing`'s
`saveWithBase`, answers), `TicketHeader`'s `TicketIdentity`, `TicketTab`'s
parts (`RepairNotice`, `QuestionsCallout`, `HandoffCallout`, `ReasonList`,
`FormatWarnings`, each taking its heading level), `Checklist` and
`Attachments` (grid and lightbox).

Panel only: `TicketProperties` and `TicketActions` (Move to, Position,
Archive), and its tabs (`TicketSections`, `EditTab` through
`useTicketDraft`, `RunsTab`, `RunDetail`, `ReviewTab`).

Page only: `PropertiesPanel` (Move to, a `PropertyRow` per field,
Archive), `SectionOutline` and `JumpTo`, `useSectionInView` (the section
in view; focusing a chosen one) and `ActivityTimeline`. Page sections and
property rows are where in-place edits go (FH-66).

Does not own section order, counts, review sections or which files show
(`model/ticketSections`), activity entries (`model/activity`), markdown
(`model/markdown`, `components/`), drafts, saves, moves and archive
(`editing/`), routing (`app/`) or run state (backend).
