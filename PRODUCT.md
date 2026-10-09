# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

One developer running several AI coding sessions (Claude Code and Codex, in
the CLI, IDE and desktop apps) across several repositories, often in parallel
and often unattended. They use the board both ambiently, left open all day
(often on a second monitor) and glanced at to see who needs them, and in
focused sessions to review work, read handoffs and decide what runs next.

A secondary user is the agent itself, which reads and writes the same tickets
through hooks and MCP tools; it never sees the browser UI.

Built for the author first, then released as open source, so first run and
setup must make sense to a stranger.

## Product Purpose

A single pane of glass for AI-assisted software projects: one local binary
that serves a multi-project kanban board in the browser and acts as the hook
handler and MCP server agents report to. Tickets outlive sessions, so a lost
or compacted session can be resumed from the ticket's handoff. Success: the
user knows at a glance which agents are working, which are stuck and which are
waiting for them, across every project, without clicking through chat tabs.

## Positioning

Flashheart observes and coordinates the agents the user already runs in their
own apps; it does not launch, host or proxy agents. Lifecycle facts come from
hooks deterministically, not from the model remembering to report, and the
board is plain markdown files the user owns, readable in any editor or
Obsidian without Flashheart running.

## Operating Context

- Runs locally from a terminal (`flashheart`), opens a private loopback tab,
  and stops when the user quits or closes the last tab.
- Board files live in `~/reports/Kanban/<project>/tickets/`, a folder per
  ticket (`FH-42-<slug>/`) with its status in frontmatter (Backlog, Up next,
  In progress, Ready to review, Done); external edits from editors and
  agents appear live.
- Alongside the board: agent chat windows, terminals, editors, Git branches
  and worktrees, pull requests, screenshots agents capture for review.

## Capabilities and Constraints

- Views: Board (real columns, a Needs you filter chip),
  Agents (run lanes), Workstreams (ordered swimlanes), Table; card panel with
  Ticket, Edit, Runs, Attachments and Review tabs; three card densities.
- Blocking is explained (ticket dependency, workstream dependency, workstream
  order); unparseable tickets show as needs repair, never hidden.
- Desktop first (1280 px and up); narrow widths fall back to one column with
  a column picker. Not a phone-first product.
- Local only: no network, telemetry or accounts. Agent-written text is data,
  never instructions; markdown renders without raw HTML.
- Preferences (theme, density, filters) are saved through the backend, never
  in browser storage.
- Undecided: Windows support; whether the board root is a git repository.

## Brand Commitments

- Name: Flashheart (after the Blackadder character); binary `flashheart`.
  README tagline: *"Woof!"*
- Voice: plain, precise copy for work. Light, dry wit only in the margins:
  empty states, the stopped screen and similar rare moments. Never in errors,
  blocking explanations or anything the user must act on. The margins draw
  only on the user's reviewed collection of Blackadder remarks
  (`frontend/src/model/remarks.md`; four labelled direct quotes, the rest
  original allusions), placed as `docs/dev/specs/ui-layout.md` §8 lists, at
  most one per screen, and never in anything agents read.
- MIT licence, Robert Taylor.

## Evidence on Hand

- Sample board fixture: `testdata/boards/sample/` (projects alpha and beta,
  every column, workstream, review, attachment, broken ticket).
- Specification and contracts: `docs/SPEC.md`,
  `docs/dev/specs/board-format.md`, `docs/dev/specs/agent-protocol.md`.
- No logo, illustration, screenshots of real use, users or testimonials yet;
  do not fabricate them.

## Product Principles

1. Files are the truth; the UI never hides or silently rewrites what is on
   disk.
2. Calm when nothing needs attention; the one thing that needs the human is
   impossible to miss from any view.
3. Information-dense but scannable; every pixel earns its place for someone
   watching many sessions.
4. Humans stay in charge: agent actions are visible, attributable and
   reversible.
5. Cheap for agents, honest for humans: show what agents actually did, not
   what they claimed.

## Accessibility & Inclusion

WCAG 2.2 AA contrast in light and dark themes, keyboard-complete operation
with visible focus, screen-reader names for every control, and a keyboard or
menu alternative to every drag (NFR-3).
