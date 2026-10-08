# UI layout and content contract

The structure, content and semantic colour roles every theme of the browser
board shares (D18). `docs/SPEC.md` decides what the board does; this file
decides where each thing sits and what it shows; `DESIGN.md` decides how it
looks in each palette. A theme changes tokens only: it never adds, drops,
reorders or renames anything below.

Derived on 2026-10-06 from the approved Metro Pop references in
`.impeccable/mocks/metro-theme-rollout/`, reconciled with SPEC. Light is
Metro Pop; dark is Night Service in black and neutral charcoal. Where the
references disagree with each other or with SPEC, this file records the
resolution under *Corrections to the references*.

## 1. Shell

One shell for Board, Agents, Workstreams and Table, in both themes.

```
┌ Band ─────────────────────────────────────────────────────────────────┐
│ ⚡ Flashheart │ Board  Agents  Workstreams  Table │   [● N need you] [Search] status [Quit] │
├ Rail ────────┬ Main ──────────────────────────────────────┬ Panel ────┤
│ All projects │ Page header: scope identity · view summary │ (ticket)  │
│ ──────────── │ View toolbar (Board, Table)                │           │
│ FH Project   │ View body                                  │           │
│   route bar  │                                            │           │
│ …            │                                            │           │
│ Theme · root │                                            │           │
└──────────────┴────────────────────────────────────────────┴───────────┘
```

**Band** (3.5rem tall, left to right): the bolt and the *Flashheart*
wordmark in sentence case; the view tabs Board, Agents, Workstreams, Table,
each an icon and a label, the current one marked; then, right-aligned, the
Needs you plate while any run anywhere needs the user (disc mark, count and
"need(s) you"; it opens Agents across all projects), search (Board and Table
only), the backend status (quiet dot while healthy, words when not,
`LIFE-1`) and Quit. The band carries no tagline and no project name.

**Rail** (15.5rem; 4rem of key badges while a ticket is open below
1440px): *All projects*, a divider, then each project, most recently active
first: key badge, name, ticket count, route bar, and a line with its Needs
you plate (count), in-progress count, blocked count (diamond) and repair
count. The current scope is filled. The footer holds the theme choice
(System, Light, Dark; `CFG-2`), the board root (cut from the start) and the
version line. Below 48rem the rail hides and a control row with Project and
Column pickers takes its place.

**Page header** (top of Main, every view): the scope's identity, a key badge
and the project's display name set as the page title (*All projects* with
the board icon in the all scope), then one quiet summary line for the view:

| View | Summary line |
| --- | --- |
| Board | `N tickets` (`n of N tickets` when filtered) |
| Agents | `N live runs` and, when any, `k need(s) you` |
| Workstreams | `N workstreams` and `d of t stations served` |
| Table | as Board |

The header's far end holds *Archive* and, on Board and Table, **New ticket**
(primary) (FH-39).

**View toolbar** (Board and Table), one row (wrapping only on a phone): one
button per filter, named for what it filters with no separate label, Type,
Priority, Workstream and State, then *Clear filters* while any filter or chip
applies; right-aligned, Board only, one icon button, *View options* (⋯),
whose menu holds Show columns (Needs you), Colour by and
Density (FH-39). An applied filter's button takes the select border and ring
and a count badge (*State: Blocked* for the one-choice State). Type,
Priority and Workstream open a list of values that toggle as chips do: a
click shows only that value (with any others shown), a Cmd click (Ctrl off
macOS) or Cmd/Ctrl with Enter hides it, struck through, and a click on a
chosen value restores it; values of one filter match any, filters combine.
Menus open below their button on the lifted popover surface and close on
Escape (focus returns to the button), a click outside or tabbing away.

**Chip row** (Board): on the left the label *Workstreams* and one chip per
workstream of the project in scope (bullet and name), busiest first (most
tickets neither done nor in the backlog), finished workstreams left out
unless chosen; right-aligned, one chip per Colour by value present (a round
paint bullet and the value's word). Both are filters with the menus' click
rules: a shown-only chip takes the selection ring, a hidden one is dashed and
struck through. The row never wraps: chips that do not fit stay out of view,
colour chips before workstreams; the menus list every value. Hidden when
there are neither workstream chips nor colour chips.

**Panel** beside the main view at clamp(26rem, 32vw, 35rem), narrowing it
(`CARD-1`); it covers the view below the band under 48rem.

**Alerts** (shutdown refused, a stale copy shown, a board that failed to
load) sit as a full-width strip under the band, above the rail and Main.

## 2. Board

- Columns in workflow order: Backlog, Up next, In progress, Ready to review,
  Done (`VIEW-1`). Needs you stands between In progress and Ready to review
  while it holds tickets and is switched on (`VIEW-2`); it holds mirrored
  cards and never accepts a drop. Agent working is a State filter option
  (*State: Agent working*), not a column (FH-42).
- A column is a rounded well. Its head: count badge, then the title in the
  display cut (virtual columns add their run mark). Done reads `n of N`
  while the done limit hides tickets, with *Show all N* / *Show recent only*
  at the foot. An empty column says *No tickets* (*Nothing in progress* for
  In progress).
- Columns are at least 15rem, scroll horizontally with snap; under 48rem one
  column shows, chosen with the Column picker.
- The board is one scrolling surface: columns grow with their cards and
  scroll down together, from anywhere over the board, and every well
  stretches to the tallest column (or the board's height). No column scrolls
  on its own. Column heads stay pinned at the top while the board scrolls; a
  drag held at an edge scrolls the board. The ticket panel scrolls
  separately.
- Dropping: the hovered column shows a drop outline and a select-coloured
  line where the card will land (none in Done, or where the drop would
  change nothing) (`EDIT-9`). Keyboard: arrows move between cards, Shift with
  Left/Right moves the ticket to the next column (`EDIT-1`), Shift with
  Up/Down moves it within its column, Enter opens it. The panel's actions
  add **Position** buttons (Top, Up, Down, Bottom; each named by its full
  action) while the board shows the ticket outside Done.

### Ticket card

One anatomy at every density, the same in both themes. Rows appear in this
order; a density shows or hides rows (as the table says) but never
reorders them.

| # | Row | Compact | Normal | Detailed |
| --- | --- | --- | --- | --- |
| 1 | Header: id, project name (All projects only), running time right | ✓ | ✓ | ✓ |
| 2 | Title, wrapping to at most three lines (full title in its tooltip and the card's name) | ✓ | ✓ | ✓ |
| 3 | Tags: workstream bullet, type tag, age tag (Colour by Age), *in <column>* on a mirror | ✓ | ✓ | ✓ |
| 4 | Needs repair note | ✓ | ✓ | ✓ |
| 5 | Blocker pill (diamond, reason, `+n more`) or quiet wait note | short pill *Blocked* | ✓ | ✓ |
| 6 | Live badge: run state and agent, permission reason, last activity; plan step `done/total · step` (`VIEW-8`) | mark only (Needs you plate) | ✓ | ✓ |
| 7 | Question waiting plate, when a question needs the user and its run does not | — | ✓ | ✓ |
| 8 | Description excerpt, three lines | — | — | ✓ |
| 9 | Handoff next step (arrow) | — | — | ✓ |
| 10 | Footer: criteria `d/t` left; attachments count and *Review* (Detailed); priority tag right | priority only | ✓ | ✓ |

- The workstream stripe runs the full left edge and is always the bullet's
  line colour. A ticket with no workstream has neither.
- Colour by paints the header row with the value's tint and fills that
  value's tag with its strong shade (`VIEW-6`); unpainted tags are neutral
  outlined chips. High priority is semibold in either form. A needs-repair
  card is never painted.
- A card has no overflow menu: moves are drag, Shift with an arrow, or the
  panel's Move to.
- Selected card: selection ring. Dimmed (another line pressed): 35% opacity.
  Needs repair: dashed border and hatched top band, no paint.
- Ids never truncate; titles wrap; an excerpt or step that is too long is
  clamped with its full text in the panel.

## 3. Card panel

1. **Header**: id (a link to the full page, `CARD-7`) and project name; the
   title in the display cut; a pill row
   (status pill for its column, the blocker pill when blocked, priority tag,
   type tag); a meta row (workstream bullet and name, created date, branch in
   monospace, changed time); actions (Move to, Archive) when editable. Top
   right, *Open full page in a new tab* (expand icon) and close; Escape
   closes.
2. **Tabs**: Ticket, Edit (editable only), Runs (with its session count),
   Attachments (once `review-and-orchestration` builds it), Review (when a
   review file exists). Hidden tabs never leave a gap.
3. **Ticket tab**, in order: Needs repair (dashed, hatched, the problems, the
   frontmatter) · questions for you, each with its answer box or option
   buttons (`CARD-6`) · Handoff, the accent callout with the next steps large
   and the full handoff behind a disclosure, with a warning when the run has
   edited since (`CARD-5`) · Blocked by (diamond, reasons with ids linked, Open) and Waits for
   (`CARD-4`) · Acceptance criteria with `d of t` and tickable boxes
   (`CARD-3`) · format warnings · the ticket's markdown (`CARD-2`).
4. **Edit tab**: frontmatter form and raw markdown (`EDIT-6`), conflict
   dialog on save (`EDIT-7`).
5. **Runs tab**: runs newest first, each a card with its state, agent,
   short id, link, branch and time, then its detail and its subagents as a
   tree on a spur: each row its state, type, the ticket it claimed when that
   is another, and its plan step (or tool and edit counts), opening to its
   own plan, files and activity (agent-protocol §10). A subagent listed on
   its own ticket names the session it belongs to.
6. **Attachments tab** (shown when the ticket has files): a two-column grid
   of thumbnails (screenshots) and file tiles, each with caption, file name
   and kind; a thumbnail opens a lightbox dialog (Escape closes, arrows and
   Previous/Next step when there is more than one, focus returns to the
   thumbnail); other files open in a new tab.
7. **Review tab**: screenshots above the review markdown; the *How to
   Verify* steps show as a checklist with `d of t`, between the section's
   own text, and a tick is written into the step's line in `review.md`
   (`REV-3`).

### Ticket page

A ticket's full page (`CARD-7`, `#/ticket/<id>`) is the panel at reading
width: the band (no view current; its views leave the ticket), then a
column of at most 64rem, centred, that scrolls as a whole. A quiet *Show on
the <project> board* link (board icon) leads, then the panel's header with
the title as the page's h1 (the id plain, being this page's own), the tabs
and the tab's content, under a visually hidden h2 naming the tab. No close
button and no Position buttons. Ticket links on the page stay in its tab;
the browser tab reads `<id> · <title> · Flashheart`.

### Ticket ids

A ticket id shown anywhere is a link to the ticket's full page in a new tab
(`KEY-3`): semibold, tabular, underlined on hover. A link never sits inside
a button. Where the whole of a card or station opens the panel, one button
covers its drawing (which is hidden from assistive technology, the button
carrying the name) and the id link sits above the button, off the tab order:
the card stays one keyboard stop, and the panel header's id link serves the
keyboard. In table rows and the Agents view the title is the button and the
id a separate link.

## 4. Workstreams

- Page header, then one section per project (heading with key badge and
  name in the all scope) and one route card per workstream, in workstream
  order (`VIEW-4`).
- Route card head: large bullet, name in the display cut, derived status in
  words with its mark (*Running*, *Blocked* with diamond, *Completed* with
  check), and `d of t served` right. Repair, blocker and warning notes follow.
- Stations form a railway graph of the tickets' `depends-on` links on the
  line (D26), left to right, scrolling with an edge fade and a *more* hint.
  A ticket's column is one past the longest chain it depends on; a chain
  runs along one track, independent work runs on parallel tracks below, and
  a ticket that depends on several others is where their tracks join. Track
  bends at right angles with rounded corners in the gaps between columns;
  the trunk keeps to the top track. Each station: mark, title (two lines),
  id, status pill, with a *Needs FH-n* blocked pill above it for an
  unfinished dependency outside the line.
  - **Done or archived**: filled line-colour station with a check.
  - **Ready to review**: filled line-colour station with a ground dot (served,
    not finished; never a check).
  - **Next stop**: the larger interchange ring in the line colour with a
    halo, on every unfinished ticket that can start.
  - **Ahead**: open ring in the line colour (waits for a station on the line).
  - **Held**: dashed ring in the line colour (held from outside the line, or
    the line is suspended).
  - **Missing**: dashed faint ring, *Does not exist*.
- Track into a served station is solid line colour; track ahead is the same
  colour at 35%; only suspended service is dashed (unmet workstream
  dependency, or a station held from outside the line), with dashes kept in
  step where tracks share a stretch.
- Tickets with no links on the line follow below the graph, set off by a
  faint rule when there is a graph, wrapping onto rows.
  Shift with Left/Right or drag reorders them (`EDIT-4`).

## 5. Agents

Page header; then lanes in fixed order Needs you, Working, Quiet, Waiting,
Ended (`VIEW-3`), each a rounded surface headed like a board column (run
mark, title, count badge). An empty lane keeps its head above one quiet
sentence. Each run is one row: state and reason · agent and short id (the
expand control) · project key (all scope) · ticket id and title or
*Unassigned* · plan route and step · branch · last activity right. Subagents
hang under their session. An open row shows plan, edited files, meta and
activity. Older ended runs sit behind *Show older ended runs*.

## 6. Table

Page header and view toolbar (no View options or chip row); then the table
on a rounded surface with a sticky head: Ticket (title over id), Project (all
scope), Status (status pill), Type (tag), Priority (tag), Workstream (bullet
and name), State (blocker pill, repair mark, live mark), Criteria, Changed.
Head buttons sort (`VIEW-5`); the selected row takes the selection tint.

## 7. Semantic colour roles

Each role is a token family defined once per palette; a role is never used
for another role's job, and none is ever the only carrier of meaning.

| Role | Carries | Where | Always paired with |
| --- | --- | --- | --- |
| Frame | Brand surface and the current scope | Band and its current view tab (raspberry light, black dark), rail and its current tile | The tab's or project's name |
| Action | The primary action | Primary button | Its label |
| Attention | A run or question that needs the user | Needs you plate (band, rail, card, lane, panel) | Disc mark and words |
| Line | A workstream | Bullet, card stripe, track, station, route card wash | Bullet initials and name |
| Paint | The Colour by value | Card header tint, filled tag, colour chip bullet | The value's word |
| State | Ticket workflow state | Status pills: Backlog and Up next neutral, In progress amber, Ready to review blue, Done green | Icon and column word |
| Blocked | A real blocker | Blocker pill | Diamond and the reason |
| Danger | Application errors, and actions that cannot be undone | Alert strips, failed loads; the red Delete permanently button and its red confirmation (`EDIT-8`, `PRJ-5`) | Words |
| Focus | Keyboard focus | 2px outline | — |
| Selection | What is chosen or applied | Open ticket's card ring and table row, current panel tab, chosen density, applied filter, pressed workstream chip, drop target | `aria-current`, `aria-selected`, `aria-pressed` or the control's value |

Run state other than Needs you stays an ink mark with its word. Waiting on an
earlier station of the same line is muted text with no colour. Needs repair
is dashed and hatched in ink.

## 8. States

| State | Contract |
| --- | --- |
| Loading | Skeletons in the shape of the view (columns, lanes, route cards, panel header) |
| Empty | Centred display-cut title and one plain sentence of what fills it |
| Error | Danger strip with the error in words and *Try again*; the last good copy stays visible with a note |
| Board root missing | Empty state naming the path |
| Older board format | Notice with the migrate command |
| Repair | Dashed, hatched, never painted, never hidden |
| Blocked | Blocker pill with the reason; on stations a pill above the next stop |
| Long content | Titles wrap (card three lines, then clamped); ids never cut; paths, branches and run ids cut from the start |
| Narrow (< 48rem) | Rail hidden, control row with Project and Column pickers, band tabs as icons, Needs you plate keeps its words, panel covers the view |
| Backend lost / stopped | Band status in words; the stopped screen replaces the app (`LIFE-1`) |
| Marginal remark | At most one per screen: one quiet line in faint ink under a real quiet state, chosen once from its placement's lines (below) and kept while shown, never the line shown last. Never in errors, blockers, permissions, controls or anything to act on; identical in both themes; no motion |

Marginal remarks come from the reviewed collection in
`frontend/src/model/remarks.md` (ids are its numbers). Placements, highest
priority first when several are on screen:

| Placement | When | Lines |
| --- | --- | --- |
| Move toast (not announced) | A move to Done that finishes its workstream: completion. Any other move to Done: progress, at most every ten minutes | 61–70; 51–53, 55, 57, 58 |
| Card panel | A current handoff (not stale) with a next step; an in-progress ticket with no handoff | 75, 77, 79, 80; 73 |
| Open run (Agents) | A plan; no plan | 32, 34–36, 39, 40; 33 |
| Empty views | No tickets; nothing matches the filters or search (also the archive's search); no workstreams; no agent runs | 21–30 (reviewed ones); 82–88; 31; 41–50 |
| Columns and lanes | Ready to review is empty; the Needs you lane is empty and nothing anywhere needs you | 71, 72; 11–19 |
| Stopped screen | After a successful Quit only (never on a lost or failed connection) | 92–100 |
| Wordmark tooltip | Always; an Easter egg, never visible text | 1–7 |

Deferred: 54 (claims a plan's first task) and 60 (claims a reader for a
handoff).

Focus is always visible; reduced motion removes transitions, the panel
reveal and the Working beat; every control has an accessible name; contrast
meets WCAG 2.2 AA in both palettes (`NFR-3`).

## 9. Corrections to the references

The references are images, not specifications. Where they show something
other than the above, the above wins:

- **Invented copy**: taglines ("Local AI for real progress", "Turn ideas into
  working software", "Three focused routes…"), column subtitles ("Ideas, not
  yet scheduled"), workstream descriptions, "Design study · Sample data",
  "Strong Handoff" (the section is *Handoff*). None appear in the product.
- **Spelling**: "FLASHEART" is *Flashheart*, in sentence case.
- **Types and states**: "Chore", "Integration", "Task" and "Dependency" are
  not ticket types; types are feature, bug, test, refactor, infra, docs and
  spike (or the file's own word). No new status, type or run state comes from
  a generated badge.
- **Sample data**: ticket titles, ids, counts, dates ("Apr 22, 2025"),
  criteria, workstream names and answer options are illustrative.
- **Colour slips**: a card's stripe always matches its bullet (the light
  board shows an AR card with an orange stripe); a priority is not shown in
  the blocked red; the dark board's criteria rings are plain counts.
- **Stations**: a check means Done or Archived only; a dashed route ahead
  (dark workstreams) is solid at 35%, since dashes mean suspended service.
- **Placement**: view tabs sit in the band (the light workstreams image puts
  them under it); the backend status stays in the band rather than the rail
  footer, so it shows at every width and with the rail collapsed.
- **Omissions the references do not show but the product keeps**: project
  route bars and counts in the rail, the filter buttons, the
  ticket count, virtual columns and their toggles, the colour chips, mirrored
  cards, repair, wait notes, the project name on All-projects cards,
  attachments and review markers, Archive, the Edit and Runs tabs' content,
  the board root and version, Agents and Table.
- **Additions the references show but the product does not have**: a card
  overflow menu, a ⌘K search shortcut, a single filters popover, workstream
  descriptions and a ticket details list. None are added by this rollout.
