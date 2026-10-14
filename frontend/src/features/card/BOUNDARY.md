# Card feature boundary

Owns the card panel (CARD-1, ui-layout.md §3) and the ticket's full page
(`TicketPage`, CARD-7), which share their parts: `useTicket` (load,
reload, ticks, answers, comments and saves), the header, the ticket tab's
callouts and notices, the checklist, attachments and their drop, and the
comments (the activity timeline, a person's comment edited or deleted,
and `CommentBox` for posting, quoting and telling a session).

Panel only: properties, actions and the tabs (sections, Edit, runs,
review), whose drafts survive tab changes. Page only: the properties panel,
section outline and Jump to, Activity's filter, change requests (REV-6),
and what each in-place item edits with (`PageEdits`, `PropertyEdits`).

Does not own section, activity, comment or edit rules (`model/`),
requests (`api/`), markdown (`model/markdown`, `components/`), drafts,
saves, the open item, moves and archive (`editing/`), the editor
(`editor/`), routing (`app/`) or run state (backend).
