# Card feature boundary

Owns the card panel (CARD-1, `docs/dev/specs/ui-layout.md` §3) and the
ticket's full page (`TicketPage`, CARD-7), both rendering one `TicketView`
at panel or reading width: loading one
ticket and reloading it when the board changes, its header (id, title,
status, blocker, priority and type pills, meta, Move to, Position
buttons (EDIT-9) and Archive), the
Ticket tab (needs repair, agents' questions about the ticket with their
answers (CARD-6), the handoff callout with a warning when a run has edited
since it (CARD-5) and its marginal remark, blocked by with links, tickable acceptance criteria,
warnings and the rendered markdown), the Edit tab (typed fields and the raw file, EDIT-6),
the Runs tab (the ticket's linked runs with their subagents as a tree, and
`RunDetail`: what one run did, its questions, plan, edited files and
activity), the Attachments tab (a grid of thumbnails
and file tiles, opening screenshots in the lightbox) and the Review tab
(screenshots, then the review file with its How to Verify steps as a
`Checklist`, REV-3; the same checklist shows acceptance criteria). A
caller may open a ticket on a given tab (the Overview's Review results).

Does not own the side-panel shell, dialogs or markdown rendering
(`components/`), the move, archive and conflict flows (`editing/`),
routing and the page's place in the shell (`app/`), run state (backend), or attaching files, which agents do
through `attach` and serve does for files a directly edited ticket links
to (REV-5).
