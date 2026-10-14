# Overview feature boundary

Owns the Overview (VIEW-3, D32, `docs/dev/specs/ui-layout.md` §5): the
project manager's view of a scope (All projects or one project), loading
its tickets and runs through `api/board` and `api/runs` and reloading them
when the board changes, and drawing the pipeline of the scope's columns
(`PipelineHeadline`, from the project summaries the shell passes), in the
All projects scope only the Projects list (`ProjectCounts`, FH-153: every
project's counts and live agents (FH-162) from the same summaries, as a
table or stacked by the card's own width, each row a link to the
project's Overview) and the
sections in one column of rounded section cards, which share their card,
head, mark and count badge (`sectionParts`): Needs your decision (questions answered in place through the
shared question card; permission prompts saying which session to answer
in, never a button), Ready for your review (Review results), At risk, In
progress, and the collapsed Work with no ticket (Create ticket) and Up next
tiles, with their open and closed state, rows, empty sentences and quiet
remarks; a focused row keeps focus as it moves between sections or goes
(through `components/useKeepFocus`, the section head the last place); and its load states (ui-layout.md §8): `OverviewSkeleton`, which
the shell also shows while the projects load, the shared error with Try
again and the last-good-copy note.

Does not own grouping, order, reasons or wording (`model/overview`,
including each project's row and its name), the column counts (backend
`api` project summaries), the Board and project links (`app/`),
run
state or the ticket session summaries (backend `index`, RUN-9), sending
answers or creating tickets (`editing/`), the card panel and its tabs or
run detail (`card/`), routing and which tab a ticket opens on (`app/`).
