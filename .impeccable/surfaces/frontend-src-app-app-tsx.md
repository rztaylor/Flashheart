---
version: 1
slug: "frontend-src-app-app-tsx"
primary_target: "frontend/src/app/App.tsx"
related_targets: ["frontend/src/features"]
---

# Board shell, Board, Agents, card panel and Workstreams

Scope: the app shell (band, project rail, page header), Board with three
card densities and virtual columns, Agents view, card panel (Ticket, Edit,
Runs and Review tabs; Attachments to come), Workstreams and Table views.
Visitor mode: Operate. Used ambiently on a second monitor and in focused
review sessions; must answer at a glance who needs the user, what is moving,
what is stuck and where each project stands. Pinned by the user: a standard
kanban structure "with style", light and dark modes; Metro Pop for light,
Night Service in black and neutral charcoal for dark, never navy grounds
(D18).

## Direction contract

THESIS: A friendly metro system for work: each workstream is a route with
its own colour, tickets are its stations, and the board is a bright,
colourful kanban that still reads as calm, dense tooling. One structure, two
palettes.

OWN-WORLD: Metro Pop. A Metro-blue band carries the wordmark and the view
tabs; a warm paper rail lists projects with key badges, route bars and
counts; columns are cool grey rounded wells holding white rounded cards.
Route colours (bullet, card stripe, route track) always mean a workstream.
The Colour by value tints the card header and fills its named tag. Status
reads as soft pills that always carry an icon and the column's word (amber
in progress, blue review, green done, red blocked with its reason). Needs
you is the one orange attention plate. Orange is the primary action.
Archivo Variable in a heavy condensed cut for titles, tabular numerals,
sentence case. Night Service keeps every structure and swaps the palette:
black band, neutral charcoal rail, wells and cards, lime action and focus,
coral attention, lifted route colours.

STORY: The visitor sees the band with Needs you, the project's identity as
the page title, the workstream strip and colour key, five wells of cards
with stripe, id, title, tags, blockers and live runs, opens a card into the
panel (status, blocker and priority pills, questions and handoff as callouts,
criteria as a checklist), and follows each workstream as a route of
stations on its own colour-washed card.

FIRST VIEWPORT: 56px band (bolt and wordmark, view tabs, Needs you plate,
search, quiet status dot, Quit); 248px rail (key badges only while a ticket
is open below 1440px); page header (key badge, display title, one summary
line); toolbar (New ticket, filters, Show, Colour by, Density); workstream
strip; five column wells. Structure and content: `docs/dev/specs/ui-layout.md`.

FORM: Metro Pop light and Night Service charcoal dark, selected by the user
from generated comps on 2026-10-06 (references and provenance in
`.impeccable/mocks/metro-theme-rollout/`). Signature move: workstreams drawn
as metro routes with tickets as stations, a check only for finished work,
the next stop as a large ring, suspended service dashed.

PROVENANCE (metro-theme-rollout, 2026-10-06): comp round run with image
generation; the user chose Metro Pop and Night Service charcoal over Garden
Line and a navy Night Service (D18). FH-10 reconciled the comps with SPEC
into `ui-layout.md`; FH-11 to FH-14 built it code-first against those
references. Replaces the Transit Line Map direction of D14 (2026-10-04).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
