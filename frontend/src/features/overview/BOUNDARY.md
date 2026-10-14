# Overview feature boundary

Owns the Overview (VIEW-3, D32, `docs/dev/specs/ui-layout.md` §5): the
project manager's view of a scope, loading its tickets and runs and
reloading them when the board changes; the pipeline of the scope's
columns (`PipelineHeadline`); in All projects, the Projects list
(`ProjectCounts`); and the section cards (`sectionParts`): Needs your
decision (questions answered in place, permission prompts naming their
session), Ready for your review, At risk, In progress, Work with no ticket
and Up next. Focus follows a moving row (`components/useKeepFocus`). Its
load states are `OverviewSkeleton`, the shared error and the stale note.

Does not own grouping, order, reasons or wording (`model/overview`), the
column counts or run state (backend), sending answers or creating tickets
(`editing/`), the card panel (`card/`), or links, routing and which tab a
ticket opens on (`app/`).
