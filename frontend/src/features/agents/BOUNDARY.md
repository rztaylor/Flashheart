# Agents feature boundary

Owns the Agents view (VIEW-3, `docs/dev/specs/ui-layout.md` §5): agent runs
as a departure board, lanes by run state with Needs you first, each lane a
rounded surface headed like a board column, one row per session with its subagents on a
spur beneath it, the ticket it is linked to (or Unassigned; a subagent
names a ticket only when it claimed another than its session's), its plan as a
route, branch and last activity, and the run detail (questions to answer,
plan stops, edited files, activity) shared with the card panel's Runs tab. Loads runs and a
run's timeline through `api/runs`.

Does not own run state or links (backend `runs`), lane grouping and wording
(`model/runs`), the state mark and plan route (`components/`), routing or
the card panel.
