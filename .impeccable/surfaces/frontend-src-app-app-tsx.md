---
version: 1
slug: "frontend-src-app-app-tsx"
primary_target: "frontend/src/app/App.tsx"
related_targets: ["frontend/src/features"]
---

# Board shell, Board, card panel and Workstreams

Scope: the app shell (project rail, signage header), Board with three card
densities, card panel (Ticket and Review tabs), Workstreams and Table views.
Visitor mode: Operate. Used ambiently on a second monitor and in focused
review sessions; must answer at a glance who needs the user, what is moving,
what is stuck and where each project stands. Pinned by the user: a standard
kanban structure "with style", light and dark modes.

## Direction contract

THESIS: The board is a transit system: each workstream is a coloured line and
its tickets are stations, so order, progress and blockage read as a route map.
It refuses the grey SaaS lane-and-badge kanban.

OWN-WORLD: Unimark/Vignelli signage. A black signage band carries the shell;
white map ground in light mode, night-map charcoal in dark. One grotesk
(Helvetica class) in tight sentence case, tabular numerals for every count and
age. Round line bullets in the MTA line palette are the only hues and always
mean a workstream; ticket state is monochrome shape plus words (diamond for
blocked, hatched border for needs repair, inverted black card reserved for
Needs you).

STORY: The visitor sees per-project route bars in the rail, columns as station
signs, blocked tickets with their reason inline, and opens a card to read its
handoff, criteria and review without losing the board.

FIRST VIEWPORT: 48px black band: wordmark and current project as a station
sign at left, view tabs (Board, Workstreams, Table) centre-left, search,
filters, density, quiet status dot and Quit right. 248px rail: All projects,
then projects with a four-segment route bar and counts. Main: a line legend
strip (click a line to focus it; other cards dim), then four columns headed by
a rule and station-sign label with a right-aligned count. Cards: line bullet,
title, slug, priority mark, running time right-aligned, blocked reason line.
Card panel slides over from the right at 560px.

FORM: Transit line map (Vignelli 1972 diagram and Unimark signage); position
6 of 7 on the ordered list; seed key 4687b54f. Raises: numbers set as data
(Ikeda); running times right-aligned (cassette j-card); colour quarantined to
jobs (lexicon); every status paired with its proof (monochrome product); focus
dims instead of filtering (streaming wall). Signature move: workstreams drawn
as transit lines with tickets as stations, done stations filled, the next
station an interchange ring, blocked segments dashed as suspended service.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
