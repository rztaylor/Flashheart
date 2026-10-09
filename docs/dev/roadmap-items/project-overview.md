# project-overview

**Status:** Partial. Workstream: `project-overview`. Spike: FH-47.
Built: FH-49, FH-50, FH-51, FH-54. Open: FH-52, FH-53, FH-55.
Decision: D32.

## Goal

Replace the Agents view (`VIEW-3`) with an **Overview** that serves a
project manager: what needs a decision, what needs review, what is at risk,
what is in progress and what changed, with tickets as the subject and agent
sessions as evidence. Trim the run data and hooks to what that view and the
card need.

## Scope

- FH-49: the board records that a human updated a ticket, and when (an
  edit, a status change, a ticked criterion, an answered question), with
  no per-change detail. Only board changes count; editor edits do not.
- FH-50: each ticket in the board data carries a summary of its sessions:
  agent and run state, last activity, no live session, handoff staleness,
  and subagent counts (done, running, need you).
- FH-51: the Overview tab replaces the Agents tab, in the approved Calm
  layout: Needs your decision, Ready for your review, At risk, In
  progress, then collapsed Work with no ticket and Up next. Run detail
  stays on the card's Runs tab; the Needs you filter's notice of runs
  with no ticket on the board (D31) opens the Overview. `VIEW-3`, SPEC §7,
  `docs/dev/specs/ui-layout.md` §5 and the Agents feature boundary change.
- FH-52: headline metrics under the Overview title since your last change
  (done, to review, new tickets, criteria ticked), falling back to the
  last 24 hours.
- FH-53: "no handoff" and handoff enforcement count edits made through
  shell commands.
- FH-54: plan displays show only when a run has a plan.
- FH-55: throttled activity records replace one `tool.used` event per
  tool call (78% of all events).

## Acceptance criteria

- Each ticket's criteria are met and its pull request has merged.
- The Overview matches the approved concept (FH-47
  `files/20261009T1923-image.png`) in light and dark, at desktop and narrow
  widths, with an empty state for every section.
- `PROTOCOL_VERSION` changes only with FH-55 (or FH-53 if it adds an event
  field), with the protocol skill, setup output and tests in the same
  change.

## Exclusions

Event retention (SPEC §11), desktop notifications, launching agents (D1),
and editor edits as human activity.

## Dependencies

FH-51 needs FH-50. FH-52 needs FH-49 and FH-51. FH-55 needs FH-51. FH-49,
FH-50, FH-53 and FH-54 have none. No roadmap item dependencies.

## Validation

`scripts/check.sh` and `scripts/e2e.sh`; review screenshots of the
Overview in light, dark and at 390 px compared with the approved concept.
