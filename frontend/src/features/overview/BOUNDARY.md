# Overview feature boundary

Owns the Overview (VIEW-3, D32, `docs/dev/specs/ui-layout.md` §5): the
project manager's view of a scope (All projects or one project), loading
its tickets and runs through `api/board` and `api/runs` and reloading them
when the board changes, and drawing the sections in one column of rounded
section cards: Needs your decision (questions answered in place through the
shared question card; permission prompts saying which session to answer
in, never a button), Ready for your review (Review results), At risk, In
progress, and the collapsed Work with no ticket (Create ticket) and Up next
tiles, with their open and closed state, rows, empty sentences and quiet
remarks.

Does not own grouping, order, reasons or wording (`model/overview`), run
state or the ticket session summaries (backend `index`, RUN-9), sending
answers or creating tickets (`editing/`), the card panel and its tabs or
run detail (`card/`), routing and which tab a ticket opens on (`app/`).
