# Card feature boundary

Owns the card panel (CARD-1, `docs/dev/specs/ui-layout.md` §3) and the
ticket's full page (`TicketPage`, CARD-7). Both compose the same parts;
neither branches on the other's layout:

- `useTicket` loads one ticket, reloads it when the board changes, and
  wires its saves (tick a criterion or review step, answer a question,
  reload after an Edit tab save), each reporting failures through
  `editing`.
- `TicketHeader`: `TicketIdentity` (id and project, the title as the panel's
  h2 with the id linking to the full page, or as the page's h1),
  `TicketProperties` (status, blocker, priority and type pills; workstream,
  created, branch and last change) and `TicketActions` (Move to, Position
  buttons (EDIT-9) and Archive).
- `TicketSections`: `SectionBody` renders one section and `SectionTabs`
  shows the sections (in `model/ticketSections` order, with counts) as
  tabs over one tab panel.
- The sections, one per file: `TicketTab` (needs repair, agents' questions
  about the ticket with their answers (CARD-6), the handoff callout with a
  warning when a run has edited since it (CARD-5) and its marginal remark,
  blocked by with links, tickable acceptance criteria, warnings and the
  rendered markdown), `EditTab` (typed fields and the raw file, EDIT-6),
  `RunsTab` (the ticket's linked runs with their subagents as a tree, and
  `RunDetail`: what one run did, its questions, plan, edited files and
  activity), `Attachments` (a grid of thumbnails and file tiles, opening
  screenshots in the lightbox) and `ReviewTab` (screenshots, then the
  review file with its How to Verify steps as a `Checklist`, REV-3; the
  same checklist shows acceptance criteria).

`CardPanel` puts the header and tabs in the side panel; `TicketPage` puts
them at reading width under a link to the board and names the browser tab.
A caller may open a ticket on a given tab (the Overview's Review results).

Does not own which sections a ticket offers or their counts
(`model/ticketSections`), the side-panel shell, dialogs or markdown
rendering (`components/`), the move, archive and conflict flows
(`editing/`), routing and the page's place in the shell (`app/`), run state
(backend), or attaching files, which agents do through `attach` and serve
does for files a directly edited ticket links to (REV-5).
