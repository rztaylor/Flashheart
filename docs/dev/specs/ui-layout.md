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

One shell for Overview, Workstreams, Table and Board, in both themes.

```
┌ Band ─────────────────────────────────────────────────────────────────┐
│ ⚡ Flashheart │ Overview  Workstreams  Table  Board │ [● N need you] [Search] status [⚙] [Quit] │
├ Rail ────────┬ Main ──────────────────────────────────────┬ Panel ────┤
│ All projects │ Page header: scope identity · view summary │ (ticket)  │
│ ──────────── │ View toolbar (Board, Table)                │           │
│ FH Project   │ View body                                  │           │
│   route bar  │                                            │           │
│ …            │                                            │           │
└──────────────┴────────────────────────────────────────────┴───────────┘
```

**Band** (3.5rem tall, left to right): the bolt and the *Flashheart*
wordmark in sentence case; the view tabs Overview, Workstreams, Table, Board,
each an icon and a label, the current one marked; then, right-aligned, the
Needs you plate while any run anywhere needs the user (disc mark, count and
"need(s) you"). It is the Needs you filter (`VIEW-2`, FH-44), a toggle:
pressed, it takes an inset ring in the on-attention ink and a close mark
(the ring alone on a phone) and shows All projects' Board (the Table when
that is open) with only tickets that need the user; pressed again, it
returns to the project it was pressed in. It stays while pressed, so it can
be switched off once nothing needs the user. Then search (Board and Table
only), the backend status (quiet dot while healthy, words when not,
`LIFE-1`), the **Settings** gear and Quit. The band carries no tagline and
no project name.

**Settings** (FH-149): an icon button named *Settings* (its name also its
tooltip), at every width and on every screen under the band, the ticket
page included. It opens a menu below it, aligned to its right edge and
never wider than the screen, holding the app-wide settings: the theme
choice (System, Light, Dark; `CFG-2`), which applies at once and is saved
with the other preferences, so the menu stays open; the board root (cut
from the start, the whole path in its tooltip); and the version line
(version and agent protocol, once the server has answered). It is a
disclosure like the toolbar's menus: Enter or Space opens it, Escape, a
click outside or tabbing away closes it, and Escape returns focus to the
gear. Per-view settings (Show columns, Colour by, Density) stay in the
Board's View options.

**Rail** (15.5rem; 4rem of key badges while a ticket is open below
1440px): *All projects*, a divider, then each project, most recently active
first: key badge, name, ticket count, route bar, and a line with its Needs
you plate (count), in-progress count, blocked count (diamond) and repair
count. The current scope is filled. The rail has no footer: the theme,
board root and version are in the band's Settings menu. Below 48rem the
rail hides and a control row with Project and Column pickers takes its
place.

**Page header** (top of Main, every view): the scope's identity, a key badge
and the project's display name set as the page title (*All projects* with
the board icon in the all scope), then one quiet summary line for the view:

| View | Summary line |
| --- | --- |
| Board | `N tickets` (`n of N tickets` when filtered) |
| Overview | `n in progress · r ready to review` |
| Workstreams | `N workstreams` and `d of t stations served` |
| Table | as Board |

The header's far end holds *Archive* and, on Board and Table, **New ticket**
(primary) (FH-39), or on Workstreams **New workstream** (primary, FH-17).

**View toolbar** (Board and Table), one row (wrapping only on a phone): one
button per filter, named for what it filters with no separate label, Type,
Priority, Workstream and State, then *Clear filters* while any filter or chip
applies; right-aligned, Board only, one icon button, *View options* (⋯),
whose menu holds Show columns (Backlog), Colour by and
Density (FH-39, FH-41). An applied filter's button takes the select border and ring
and a count badge (*State: Blocked* for the one-choice State). State
offers All states, Blocked, Not blocked, Needs repair, Agent working and
Rejected (tickets in Done whose status is rejected, FH-59; the Done limit
still applies, so older ones need *Show all*). Type,
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

**Needs you notice** (Board and Table, while the Needs you filter is on and
runs need the user with no ticket on the board): a well-coloured strip under
the chip row, the Needs you mark on its attention disc, *N agent(s) need(s)
you with no ticket on the board.* and a quiet **Open Overview** button
(FH-44), whose Needs your decision lists them (D32).

**Panel** beside the main view at clamp(26rem, 32vw, 35rem), narrowing it
(`CARD-1`); it covers the view below the band under 48rem.

**Alerts** (shutdown refused, a stale copy shown, a board that failed to
load) sit as a full-width strip under the band, above the rail and Main.

## 2. Board

- Columns in workflow order: Backlog, Up next, In progress, Ready to review,
  Done (`VIEW-1`). A ticket shows once, in its own column. Needs you is the
  band plate's filter (`VIEW-2`, FH-44) and Agent working a State filter
  option (*State: Agent working*, FH-42), not columns.
- The Backlog can be hidden with View options › Show columns (shown by
  default; FH-41) so the columns being worked on get its width. Hidden, it
  leaves every width, the narrow Column picker included, and Shift with an
  arrow or a drag never targets it. Its tickets still count in the page
  header and the chip ordering, and the Table and Workstreams are unchanged.
- A column is a rounded well. Its head: count badge, then the title in the
  display cut. Done reads `n of N`
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
| 3 | Tags: workstream bullet, type tag, age tag (Colour by Age), and the change requests still waiting as a pill (`REV-6`: *2 changes requested* in progress's amber, *1 addressed* in review's blue, *1 open · 1 addressed*, with the speech-balloon icon; also in the card's name) | ✓ | ✓ | ✓ |
| 4 | Needs repair note | ✓ | ✓ | ✓ |
| 5 | Blocker pill (diamond, reason, `+n more`) or quiet wait note | short pill *Blocked* | ✓ | ✓ |
| 6 | Live badge: run state and agent, its Needs you reason (the words its Runs tab uses, `RUN-3`), last activity; plan step `done/total · step` when the run has a plan (`VIEW-8`) | mark only (Needs you plate) | ✓ | ✓ |
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
   monospace, the pull request as *PR #7* or its address, a link in a new
   tab only for an `http(s)` URL (FH-114), changed time); actions (Move to, Archive) when editable. Top
   right, *Open full page in a new tab* (expand icon) and close; Escape
   closes.
2. **Tabs**: Ticket, Comments (with its comment count; when the ticket has
   comments or is editable), Edit (editable only), Runs (with its session
   count), Attachments (once `review-and-orchestration` builds it), Review
   (when a review file exists). Hidden tabs never leave a gap.
3. **Ticket tab**, in order: Needs repair (dashed, hatched, the problems, the
   frontmatter) · questions for you, each with its answer box or option
   buttons (`CARD-6`) · Handoff, the accent callout with the next steps large
   and the full handoff behind a disclosure, with a warning when the run has
   edited since (`CARD-5`) · Blocked by (diamond, reasons with ids linked, Open) and Waits for
   (`CARD-4`) · Acceptance criteria with `d of t` and tickable boxes
   (`CARD-3`) · format warnings · the ticket's markdown (`CARD-2`).
4. **Comments tab** (FH-128): the full page's Activity limited to people's
   comments, with the same parts: the timeline of comments oldest first
   (author, time, *edited*, the quote in its well, the text, and a sent
   comment's delivery line; *No comments
   yet.* when there are none), a person's comment editable in place with
   its pencil, Save, Cancel, shortcuts and the conflict dialog, until it
   is delivered, and deleted with the full page's *Delete comment*
   (FH-140), and, where
   the ticket can be edited, the comment box with **Tell a session now**,
   **Comment**, *Quote* and the shortcut hint. History, notes, handoffs and questions stay on the
   full page. The tab's comment being written or edited stays while
   another tab is shown; closing the panel, Escape or opening another
   ticket with unsaved comment text asks first (*Unsaved changes to the
   comment*: **Discard changes** or **Keep editing**), and focus returns
   to the card only once the panel closes. Unsaved Edit tab changes join
   the same prompt (item 5).
5. **Edit tab**: frontmatter form and raw markdown (`EDIT-6`), conflict
   dialog on save (`EDIT-7`). Its unsaved changes (FH-138) stay while
   another tab is shown; closing the panel, Escape or opening another
   ticket asks first (*Unsaved changes to the ticket*, naming the
   comment too when one is being written): **Discard changes** drops them,
   **Keep editing** shows the Edit tab again with focus on its control
   last used (or its first field), unless the prompt was opened from a
   field, where focus stays. The ticket New ticket created opens on this
   tab with focus in its Title, ready to edit (FH-17). A save that meets a conflict after another
   tab was chosen shows the Edit tab again with the conflict dialog.
6. **Runs tab**: runs newest first, each a card with its state (and its
   Needs you reason, in the card's words, `RUN-3`), agent,
   short id, link, branch and time, then its detail and its subagents as a
   tree on a spur: each row its state, type, the ticket it claimed when that
   is another, and its plan step (or tool and edit counts), opening to its
   own plan, files and activity (agent-protocol §10). Activity names each
   event in plain words, never by its event kind: "Claimed FH-1", "Moved
   FH-1 to In progress", "Wrote the review of FH-1"; a hookless run's
   branch switch reads "Switched to branch feature/next", not as a second
   connection. A run's plan shows only when it has one; a run with no
   plan shows no plan section. A subagent listed on its own ticket names
   the session it belongs to.
7. **Attachments tab** (shown when the ticket has files or can be
   edited): a two-column grid of thumbnails (screenshots) and file tiles,
   each with caption, file name and kind; a thumbnail opens a lightbox
   dialog; other files open in a new tab. The **lightbox** (FH-146) fills
   the viewport less a 1rem margin at every width: the caption as its
   title, the image on a well stage, its file name with a muted hint
   (*Drag to pan, scroll to zoom; arrows pan when zoomed in, 0 fits.*,
   from 640px), and a footer of **Zoom out**, the zoom as a tabular
   percentage, **Zoom in** and **Fit**, then, when there is more than
   one image, *n of m* with Previous and Next. Zoom is relative to the
   image's natural size (100% is one image pixel to a CSS pixel), from
   25% to 800% (down to Fit for an image too large to fit at 25%), in
   steps of 25, 33, 50, 67, 75, 100, 125, 150, 200, 300, 400, 600 and
   800% with Fit among them; Fit shows the whole image, never larger
   than 100%, and is where every image opens. Keys while focus is in the
   dialog: Cmd or Ctrl (or no modifier) with + or = zooms in and with −
   zooms out, never the browser's page; 0 or Cmd or Ctrl and 0 fits; the
   arrows pan by 80px (320px with Shift) while zoomed in past Fit, and at
   Fit or smaller Left and Right step between images; Escape closes and
   focus returns to the thumbnail. The wheel zooms around the pointer (a
   trackpad pinch too), dragging pans (a grab cursor while there is
   anything to pan), two fingers pinch on a touch screen; panning stops
   where the image's edge meets the stage's. A button at its limit stays
   focusable, marked unavailable. Nothing animates. Where the ticket can
   be edited, the **file drop** follows the grid
   (`REV-7`): the whole tab takes dropped files, and a dashed slot (rule
   border, card corners) with the attachment icon reads *Drop files here
   to attach them* over a muted line of the allowed types and size, with
   **Attach files** (secondary) at its end; while files are dragged over
   the tab the slot turns solid, on the select surface with a select
   border and ring. Under it, a muted status line (*Uploading 2 files…*,
   *Attached 2 files.*) and each refusal in danger words (an alert, with
   a warning icon and a Dismiss button that returns focus to Attach
   files).
8. **Review tab**: screenshots above the review markdown; the *How to
   Verify* steps show as a checklist with `d of t`, between the section's
   own text, and a tick is written into the step's line in `review.md`
   (`REV-3`).

### Ticket page

A ticket's full page (`CARD-7`, `#/ticket/<id>`, FH-65) shows the whole
ticket as one continuous page, with no tabs, under the band (no view
current; its views leave the ticket; its Settings gear is the views'). It scrolls as a whole in a frame of
at most 94rem.

```
┌ Band ────────────────────────────────────────────────────────────────┐
├────────────┬──────────────────────────────────────────┬──────────────┤
│ Outline    │ Header: board link, id, title (h1)       │ Properties   │
│ (in view)  │ Handoff, criteria, description, …        │ (in view)    │
│            │ Review, in its own sections              │              │
│            │ Files · Activity                         │              │
└────────────┴──────────────────────────────────────────┴──────────────┘
```

- **Header**, over the centre and right columns: a quiet *Show on the
  <project> board* link (board icon), the id (plain, being this page's own)
  and project, and the title as the page's h1 in the display page cut.
- **Centre**, in order, each part only when it has something: Needs repair ·
  **Change requests** (`REV-6`, FH-126), while any comment is or was one:
  its count in words beside the heading (*1 open · 1 addressed*, *All
  resolved*) and, on a ticket in review where the page can save, a
  secondary **Request changes** after it; then one bordered list on the
  card surface, open requests first, then addressed, each a row with its
  state pill (*Change requested* amber with a speech balloon, *Addressed*
  blue with the review icon), who asked and when, what waits (*Waiting for
  the agent*, *The agent addressed it: resolve or reopen it*), the text as
  markdown and, where the page can save, **Resolve** (secondary) and, for
  an addressed one, **Reopen** (quiet), each named for its request
  (*Resolve “Rename the button…”*); then resolved ones, one quiet line
  each (*Resolved* green with a check, the text cut to a line, who asked,
  **Reopen**). A request that changes state moves in the list and keyboard
  focus follows it; a failed change keeps its reason as a danger line
  beside its buttons · questions for you (`CARD-6`) · Handoff (`CARD-5`) · Blocked by
  (`CARD-4`) · Acceptance criteria with `d of t`, tickable (`CARD-3`) ·
  format warnings · Description · Test plan (Reproduction for a bug) ·
  Context · the ticket's other sections in file order. Each is an h2. Then,
  only when `review.md` exists, **Review** (h2, review icon) after a rule:
  the review's lead text, then its own sections as h3s in its order, the
  template's names in sentence case (Summary, Key files, Evidence, How to
  verify, Risks / things to watch, Tests). *Evidence* shows its words,
  then its screenshots once, as a captioned gallery (caption, file name and
  kind) that opens the lightbox; *How to verify* is the steps' checklist
  with its count between the section's own text (`REV-3`). Then **Files**
  (h2): the attachments neither the review nor the ticket links, in the
  attachment grid, followed by the file drop of the Attachments tab where
  the ticket can be edited (then Files shows even with no files). Then **Activity** (h2), with its filter beside the
  heading, a segmented control *Show in Activity*: **All**, **Comments**
  (people's comments) or **History** (the ticket's history); a muted line
  under the heading says what it lists. A timeline on one rule, oldest
  first by day (a note carries only its day, so a day's notes come before
  its timed entries), of people's comments (author, time, *edited* once
  edited, the quote in a well with a full rule border captioned with where
  it is from, then the text; a pencil, *Edit comment*, on a person's
  comment edits its text in place as other items do, FH-125; beside it a
  bin, *Delete comment*, revealed as the pencil is, asks in a dialog
  (*Delete this comment?*: an excerpt of the comment, what deleting
  does, **Keep it** focused and the red **Delete comment**) and deletes
  it, FH-140; on a comment its session has read, or a change request
  open or addressed, the bin is marked unavailable and pressing it shows
  why in a small note under it (*It can't be deleted: <run> has read
  it.*, *…while the change it asks for is open. Resolve it first.*); a
  change
  request's state pill after its time, `REV-6`; a comment sent to a
  session ends with a muted line with an icon, FH-127: *Waiting for <run>
  to take it with its next prompt or tool call.*, *Delivered to <run> at
  <time>. It can no longer be edited.* (no pencil), or *Not delivered:
  <run> ended before taking it. It stays on the ticket.*), notes
  (author, day, text), history (author, what the write changed in words,
  "moved it from Up next to In progress", "edited the description and the
  priority", "ticked criterion 2", and time; never the ticket's text),
  agents' questions (asker, time, the question and, in a tinted reply box,
  the answer and who gave it, *Answered in the session's own chat*, or
  *Waiting for your answer, above* while it is open) and handoff updates
  (run, time, and the next steps of the current handoff). A comment's
  node is on the selection surface, as a person's answer is; history's on
  the well. A filter with nothing to show says so (*No comments yet.*,
  *No history yet.*). Comments and history come from the ticket's
  `activity.md` and last as long as the ticket; notes, the user's answers
  on the board and the current handoff are kept in the ticket; earlier
  questions and checkpoints, and agents' moves from before the history
  begins, show while the runs that made them are within the run window.
  Where the page can save, the timeline ends with the **comment box**: a
  comment node, then a text area *Add a comment*, then **Tell a session
  now** (FH-127), a checkbox with a muted line beside it: *Also hands it
  to the session working on <id>.*; checked, a *Session* select of the
  live sessions holding the ticket, each an option group *Session
  <run>* with the session (*<run> · <state>*) and its live subagents
  (*<type> subagent · <state>*), the holder chosen first, and the line
  *It gets your words with its next prompt or tool call.*; with no live
  session the checkbox is disabled and the line says *No live session
  holds <id> now.* Then **Comment**
  (primary; disabled while empty, *Posting…* meanwhile; **Comment and
  tell**, *Sending…*, while telling; the choice clears once posted), *Quote*, a select
  of the parts of the ticket a comment can quote (*No quote*, then
  Sections, Criteria and How to verify steps, numbered), and the hint
  *⌘Enter posts* (Ctrl on other systems). A chosen quote shows above the
  text as it will in the comment. A failed post keeps the text with the
  reason as a danger line under it. Text in the box is unsaved work:
  leaving the page asks first (*Unsaved changes to the comment*).
- **Outline** (left, 13rem, from 1280px), staying in view: *Sections*, a
  list of the centre's sections in order: the ticket's, then *Review*
  (its icon) with its sections indented, then Files and Activity. Counts
  sit right in tabular numerals: criteria, How to verify and Change
  requests (resolved of all) `d/t` (green when all done), Evidence, Files and Activity a number. A template
  section with nothing in it (criteria, Description, Test plan or
  Reproduction, Context) is listed as *<Name>, empty* in faint italics,
  not a control. Activity counts what its filter shows. The section crossing the top third of the page is current
  (`aria-current`, the selection surface); between sections the last one
  stays. Choosing a section, by mouse or keyboard, scrolls it to the top,
  moves focus to its heading and keeps it current until the person moves
  the page.
- **Properties** (right, 20rem; 19rem from 1024px), staying in view, on the
  column surface with 14px corners under a visually hidden h2: *Move to*
  as a select in the status's own fill, icon and word (the status pill
  when read-only), on a ticket in review where the page can save a full
  width secondary **Request changes** (speech balloon; `REV-6`) under it,
  *Blocked* when blocked, then rows of Priority, Type,
  Workstream (bullet and name), Branch, Pull request (when set), Depends on
  (ids linked), Tags, Agent (the run speaking for the ticket, its state
  mark and word), Created and Changed, *None* where empty; then
  **Archive**.
- **Request changes** (`REV-6`) opens a dialog, *Request changes on
  <id>*: a muted line saying the ticket goes back to In progress on its
  branch and who gets the changes; *Comments to send back*, a checkbox per
  person's comment, newest first, its text cut to a line over its author
  and state pill, open requests checked; *New change request*, an
  optional text area with the hint *⌘Enter requests the changes*; then
  **Cancel** and **Request changes** (primary; *Requesting…* meanwhile).
  Sending nothing says *Choose a comment or write a change to request.*; a
  failed request keeps the dialog with its reason. Done, the dialog closes
  and a toast says where the changes went (*Requested 2 changes on FH-8;
  sent to claude:aa11bb22.*, or *the next session on it will see them.*).
- **In-place editing** (`EDIT-11`, FH-66), where the server saves: the
  title, each ticket section (criteria, Description, Test plan or
  Reproduction, Context, the ticket's own) and the properties Priority,
  Type, Workstream, Branch, Pull request (when set), Depends on and Tags
  each show a quiet 28px pencil, *Edit <name>*, after the heading or at
  the row's end, while the item is hovered or holds focus (always on
  touch). The pencil, a double-click on the content or Enter opens the
  item in place: its control on a well with a rule border (the title a
  one-line field in the display cut, a section the formatted editor, the
  criteria a list of rows with a done box, the text, Move up, Move down
  and Remove, then *Add a criterion*; a property's name above its select,
  text field or chips, spanning the panel), then **Save**, **Cancel** and
  the shortcut hint (*⌘Enter saves · Esc cancels*, Ctrl on other
  systems). A failed save's reason is a danger line above the buttons;
  Save reads *Saving…* meanwhile. Chips are pills with a remove button,
  then a field where Enter or a comma adds one; Depends on offers the
  project's tickets. An empty template section is a dashed *Add
  <section>* slot in its place. Handoff, Blocked by, the review, Files and
  Activity have no pencil; in Activity only a person's comment has one.
  Opening another item or leaving the page with unsaved changes asks in a
  dialog, *Unsaved changes to the <name>* (a comment being written counts
  when leaving, named *the comment*): **Discard changes** or **Keep
  editing** (primary).
- **The formatted editor** (`EDIT-12`, FH-16): a row of a muted hint
  (*Type / for blocks · select text to format · Alt+Enter opens a link
  or ticks a task*) and the **Formatted** / **Markdown** segmented switch,
  wrapping on narrow screens, over the text on the card surface with a
  rule border and control corners. Formatted reads as the section's
  rendered markdown does (same type scale, task checkboxes, bullets,
  images, links and ticket ids), with the edited lines' marks in faint
  ink; Markdown is the text in the code face. The slash menu and the
  selection toolbar (icon buttons for bold, italic, code, link, bulleted
  list and task list) are popovers; the toolbar sits above the
  selection, or below it near the top. While the editor's code loads, a
  same-sized slot reads *Opening the editor…*. Under the text, a row of
  **Attach files** (secondary) and a muted hint (*Paste or drop files to
  attach them*, with the allowed types and size), then the file drop's
  status line and refusals (Dismiss returns focus to the text). While
  files are dragged over the text its border turns select with a select
  ring. A file uploading shows in the text, where it will go, as a quiet
  chip on the well (*Uploading screenshot.png…*), never as text.
- **Below 1280px** the outline becomes a **Jump to** button (naming the
  section in view) at the top of the page, staying in view, opening the
  same list as a menu. From 1024px the properties stay on the right;
  narrower, they stack above the page. Nothing scrolls sideways at any
  width.

No close button, no Position buttons, no workstream line strip and no
Comments, Edit, Runs, Attachments or Review tabs (the card panel keeps its
tabs; the page's Activity holds the comments).
Ticket links on the page stay in its tab; the browser tab reads
`<id> · <title> · Flashheart`.

### Ticket ids

A ticket id shown anywhere is a link to the ticket's full page in a new tab
(`KEY-3`): semibold, tabular, underlined on hover. A link never sits inside
a button. Where the whole of a card or station opens the panel, one button
covers its drawing (which is hidden from assistive technology, the button
carrying the name) and the id link sits above the button, off the tab order:
the card stays one keyboard stop, and the panel header's id link serves the
keyboard. In table rows and the Overview the title is the button and the
id a separate link.

## 4. Workstreams

- Page header, then one section per project (heading with key badge and
  name in the all scope) and one route card per workstream, in workstream
  order (`VIEW-4`).
- Route card head: large bullet, name in the display cut, derived status in
  words with its mark (*Running*, *Blocked* with diamond, *Completed* with
  check), and `d of t served` right. Repair, blocker and warning notes follow.
  The name is a link to the workstream's page (underlined on hover); a
  finished line's head, being its disclosure button, has instead a quiet
  expand-icon link after it, *Open <name>* (FH-139).
- **Finished lines collapse** (FH-68, D35). A workstream whose tickets are all
  Done, Rejected or Archived, with nothing to repair, shows only its head
  with a chevron at the right (pointing right when collapsed, down when
  expanded). The head is then a disclosure button inside the heading
  (`aria-expanded`, named by the head's words, the workstream's name
  first); the mouse, Enter or Space expands the full card and collapses it
  again. The chevron turns with the default transition, which reduced
  motion sets to none. Any ticket in Ready to review or earlier keeps the
  card in full with no control, though its head already reads *Completed*.
  The expand choice is a saved preference (`CFG-2`) that lasts only while
  the workstream stays finished: reopened, it shows in full, and once
  finished again it collapses. Collapsing never changes the order of lines
  or the per-project grouping.
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
  - **Rejected**: filled line-colour station with a cross (finished, not
    accepted; never a check).
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
  Shift with Left/Right or drag reorders them (`EDIT-4`). A line with no
  tickets says *No tickets on this line yet.*
- **New workstream** (`EDIT-14`, FH-17) opens a dialog like New ticket's:
  *Project* (only in the all scope), **Title** (focused), *Goal* (a text
  area, *The shared outcome its tickets deliver.*), *Priority* (default
  medium) and, when the project has workstreams, *Depends on workstreams*:
  a bordered, scrolling list of checkboxes named by title, wrapping long
  names, with the hint *Its tickets wait until every ticket in these is in
  review or done.* **Create workstream** (primary, off until there is a
  title; Enter in the title submits) and Cancel; Escape closes and focus
  returns to the button. A refusal shows as an alert in words under the
  form, which keeps what was typed. Created, a toast *Created workstream
  <name>.* and the workstream's page opens. The empty view's sentence
  names New workstream and the Edit tab's Workstream field.

### Workstream page

A workstream's page (`EDIT-13`, `#/workstream/<project>/<slug>`, FH-139)
sits under the band like a ticket's page (no view current; its views leave
it) and scrolls as a whole in a frame of at most 80rem.

```
┌ Band ────────────────────────────────────────────────────────────────┐
│ Show in the <project> workstreams                                    │
│ ● Name (h1)                                         d of t served    │
│   Running                                                            │
│ ┌ Stations (route card surface) ───────────────────────────────────┐ │
│ │ notes · railway graph · independent stations                     │ │
│ └──────────────────────────────────────────────────────────────────┘ │
│ Goal ✎                                                               │
│ Scope ✎ … Success Criteria ✎ … Notes ✎   (at most 48rem wide)        │
└──────────────────────────────────────────────────────────────────────┘
```

- **Header**: a quiet *Show in the <project> workstreams* link (lines
  icon) to the project's Workstreams view; the route card's head with the
  name as the page's h1 in the display cut, its status and progress.
- **Stations**: the route card's body on its line-coloured surface (repair,
  blocker and warning notes, the railway graph and independent stations);
  a station opens the ticket's full page.
- **Sections**, each an h2 with its pencil, as on a ticket's page: open,
  the formatted editor with Save and Cancel; empty, a dashed **Add
  <section>** slot. A read-only board shows the sections without pencils
  and leaves empty ones out. The conflict dialog is titled *<name> changed
  while you were editing* and says another saved *this workstream*.
- Loading shows the head, a track and two section shapes; an error keeps
  an h1 of the slug with the shared error and Try again.

## 5. Overview

The project manager's view (`VIEW-3`, D32; approved concept: FH-47's Calm
direction). Page header; then one column, filling Main with the same gutter
as the other views, opening with the pipeline (FH-63): one row of five
interlocking chevron cards, pointing left to right, one per board column in
order (Backlog, Up next, In progress, Ready to review, Done, the last
without Rejected tickets), each a large
tabular count over the column's word with its status icon, in the State
role (§7). Each card leads to the Board for the scope. The row stays one row
at every width; below 40rem the icons go and the words wrap. Then the
rounded section cards in fixed order:

1. **Needs your decision**: questions, each a question card answerable in
   place (`CARD-6`), and permission prompts, which say *Answer in the
   session · project · branch* and offer no button.
2. **Ready for your review**: oldest waiting first; criteria met, waiting
   time and one primary **Review results** button opening the ticket's
   Review tab.
3. **At risk**: the reason in words (no live session, session gone quiet,
   ended without a handoff, top of Up next blocked by …) and *Last
   activity …* (a session's), *Last changed …* (the ticket's own change,
   when no session has activity) or *Not started*; an agent is named only
   for a live session.
4. **In progress**: the session's state and its subagents in words
   (*Working · subagents 3 done · 2 running*), agent, criteria and last
   activity.
5. **Work with no ticket** and **Up next**, collapsed tiles: a live session
   with no ticket (session id, project · branch, agent, run state, time,
   **Create ticket** opening New ticket in its project); the top three of
   each project's Up next, blocked (its reason) or *Ready to start*.

A section card's head sits on the column surface: a round mark (Needs your
decision in the attention disc while anything needs you, review and in
progress in their status colours, the rest ink on a card disc), the title
in the display head cut and a count badge. At risk and In progress open
when they have something, the tiles start closed, and each of those four
toggles with a chevron; the choice is not remembered. An empty section is
its head with one calm sentence and nothing to open. Rows align from 48rem
(id · title over one line of words · agent chip, with *MCP only* beneath it
for a session without hooks (D34) · meta · action) and stack
under the title below. The id opens the full page, the title the panel.
No coloured edge strips; the attention colour stays with Needs you.

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
| Attention | A run or question that needs the user | Needs you plate (band, rail, card, panel, question card) and the Overview's Needs your decision mark | Disc mark and words |
| Line | A workstream | Bullet, card stripe, track, station, route card wash | Bullet initials and name |
| Paint | The Colour by value | Card header tint, filled tag, colour chip bullet | The value's word |
| State | Ticket workflow state | Status pills: Backlog and Up next neutral, In progress amber, Ready to review blue, Done green, Rejected neutral with a cross (also on its card in Done); the Overview's pipeline stages | Icon and column word |
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
| Loading | Skeletons in the shape of the view (columns, less a hidden Backlog; section cards, route cards, a table for the Table and archives, panel header) |
| Empty | Centred display-cut title and one plain sentence of what fills it |
| Error | Danger strip with the error in words and *Try again*; the last good copy stays visible with a note |
| Board root missing | Empty state naming the path |
| Older board format | Notice with the migrate command |
| Repair | Dashed, hatched, never painted, never hidden |
| Blocked | Blocker pill with the reason; on stations a pill above the next stop |
| Long content | Titles wrap (card three lines, then clamped); ids never cut; paths, branches and run ids cut from the start |
| Narrow (< 48rem) | Rail hidden, control row with Project and Column pickers, band tabs as icons, Needs you plate keeps its words, the Settings gear stays with its menu inside the screen, panel covers the view |
| Backend lost / stopped | Band status in words; the stopped screen replaces the app (`LIFE-1`) |
| Marginal remark | At most one per screen: one quiet line in faint ink under a real quiet state, chosen once from its placement's lines (below) and kept while shown, never the line shown last. Never in errors, blockers, permissions, controls or anything to act on; identical in both themes; no motion |

Marginal remarks come from the reviewed collection in
`frontend/src/model/remarks.md` (ids are its numbers). Placements, highest
priority first when several are on screen:

| Placement | When | Lines |
| --- | --- | --- |
| Move toast (not announced) | A move to Done that finishes its workstream: completion. Any other move to Done: progress, at most every ten minutes | 61–70; 51–53, 55, 57, 58 |
| Card panel | A current handoff (not stale) with a next step; an in-progress ticket with no handoff | 75, 77, 79, 80; 73 |
| Open run (Runs tab) | A plan | 32, 34–36, 39, 40 |
| Empty views | No tickets; nothing matches the filters or search (also the archive's search); no workstreams; the Overview's In progress is empty and no session in scope is live | 21–30 (reviewed ones); 82–88; 31; 41–50 |
| Columns and sections | Ready to review is empty; the Overview's Needs your decision is empty and nothing anywhere needs you | 71, 72; 11–19 |
| Stopped screen | After a successful Quit only (never on a lost or failed connection) | 92–100 |
| Wordmark tooltip | Always; an Easter egg, never visible text | 1–7 |

Deferred: 33 (a run with no plan shows no plan section), 54 (claims a
plan's first task) and 60 (claims a reader for a handoff).

Focus is always visible; reduced motion removes transitions, the panel
reveal and the Working beat; every control has an accessible name; contrast
meets WCAG 2.2 AA in both palettes (`NFR-3`).

**Keeping focus** (FH-84). Board cards, Overview rows, Runs-tab subagent
rows and independent stations keep keyboard focus when the data under them
changes, whether by a live reload, a filter or the view's own keyboard
move. A focused item that re-renders elsewhere (a card in another column, a
row in another section, a session lower in the list) keeps focus on the
same control. One that disappears hands focus to a neighbour by one rule:
the next item in its list (column, section or subagent tree) as the list
stood before the change, else the previous one, else the list's heading
(the column or section title); the neighbour takes the same kind of
control (a card's button, a row's title). An item whose list went with it
hands over as its enclosing item would (a subagent row whose session
unlinked goes to the next session). Focus moves without scrolling the page
and is then brought into view only as far as needed; nothing else is
announced. Nothing moves when focus was not in that list: focus in a
dialog, the panel, the search box or anywhere else stays put, and focus
the user let go of is not taken back.

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
  footer, so it shows at every width and with the rail collapsed; the
  theme, board root and version, which the references put in the rail's
  footer, are in the band's Settings menu (FH-149).
- **Omissions the references do not show but the product keeps**: project
  route bars and counts in the rail, the filter buttons, the
  ticket count, the Needs you filter, the Backlog toggle, the colour chips,
  repair, wait notes, the project name on All-projects cards,
  attachments and review markers, Archive, the Edit and Runs tabs' content,
  the board root and version, the Overview and Table.
- **Additions the references show but the product does not have**: a card
  overflow menu, a ⌘K search shortcut, a single filters popover, workstream
  descriptions and a ticket details list. None are added by this rollout.
