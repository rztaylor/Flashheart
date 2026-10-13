# Card feature boundary

Owns the card panel (CARD-1, `docs/dev/specs/ui-layout.md` §3) and the
ticket's full page (`TicketPage`, CARD-7). Both compose the same parts:

- `useTicket` loads one ticket, reloads it when the board changes, and
  wires its saves: ticking a criterion or review step (through
  `editing`'s `saveWithBase`, a conflict reloading with a toast),
  answering a question, and reloading after an Edit tab save.
- `TicketHeader`: `TicketIdentity` (id, project, title as the panel's h2 or
  the page's h1), `TicketProperties` (pills, workstream, dates, branch,
  pull request) and `TicketActions` (Move to, Position (EDIT-9), Archive).
- `TicketSections`: `SectionBody` renders one section; `SectionTabs` shows
  them as tabs in `model/ticketSections` order.
- One file per section: `TicketTab` (needs repair, questions (CARD-6), the
  handoff and its warning (CARD-5), blocked by, tickable criteria, the
  markdown), `EditTab` (typed fields and the raw file, EDIT-6, one draft
  through `editing`'s `useTicketDraft`), `RunsTab` and `RunDetail` (linked
  runs as a subagent tree, keeping focus through `useKeepFocus`; what one
  run did), `Attachments` and `ReviewTab` (screenshots and the How to
  Verify steps as a `Checklist`, REV-3).

`CardPanel` puts the header and tabs in the side panel; `TicketPage` puts
them at reading width and names the browser tab. Both show the load states
(ui-layout.md §8: `TicketSkeleton`, the error with Try again, the stale
note) the same way. A caller may open a ticket on a given tab.

Does not own section order and counts (`model/ticketSections`), the form's
fields and changes (`model/ticketForm`), the side-panel shell, dialogs or
markdown (`components/`), drafts, the save precondition, moves, archive
and the conflict dialog (`editing/`), routing (`app/`), run state
(backend), or attaching files (agents, and serve for REV-5).
