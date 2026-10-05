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

OWN-WORLD: Unimark/Vignelli signage. A black signage band and a black
project rail form an L-shaped signage frame around a lit platform: the board
ground is platform grey with a faint map-dot grid, and cards lift off it with
soft shadows. One grotesk (Helvetica class) in tight sentence case, tabular
numerals for every count and age. Round line bullets and a 4px left stripe in
the MTA line palette always mean a workstream. A second, muted paint palette
shows the board's "Colour by" attribute (type by default; priority, age or
none), always as a tinted card header plus a tag that names it, with a colour
key on the board. Ticket state stays monochrome shape plus words (diamond for
blocked, dashed border and hatched band for needs repair). The only other hue
is the signal-yellow bolt of the brand mark. (Revised with the user,
2026-10-05: "Flash by name, Flash by nature".)

STORY: The visitor sees per-project route bars and station-code keys in the
black rail, columns as station signs, cards whose stripe names the line and
whose header colour names the chosen attribute, blocked tickets with their
reason inline, and opens a card to read its handoff, criteria and review in a
panel beside the board, which narrows and keeps the card's column in view.

FIRST VIEWPORT: 48px black band: bolt and wordmark, the current project's key
and name as a station sign, view tabs (Board, Workstreams, Table), search,
quiet status dot and Quit right. 248px black rail: All projects, then
projects with key badge, route bar and counts. Main: filter bar with Colour by
and density, a strip with the line legend and colour key, then five columns
on the platform headed by a rule and station-sign label with a right-aligned
count. Cards: workstream stripe, tinted header with line bullet, title and
running time; id, colour tag, type and priority; blocked reason line. The card
panel sits beside the board (clamp(26rem, 36vw, 35rem)); on phones it covers
the view.

FORM: Transit line map (Vignelli 1972 diagram and Unimark signage); position
6 of 7 on the ordered list; seed key 4687b54f. Raises: numbers set as data
(Ikeda); running times right-aligned (cassette j-card); colour quarantined to
jobs (lexicon); every status paired with its proof (monochrome product); focus
dims instead of filtering (streaming wall). Signature move: workstreams drawn
as transit lines with tickets as stations, done stations filled, the next
station an interchange ring, blocked segments dashed as suspended service.

PROVENANCE: The 2026-10-05 board refresh was code-led on this machine (no image
generation; `config.local.json` buildPath code). Its changes came from the
user's own decisions in conversation (panel beside the board; black rail with
key badges; workstream stripe; "Colour by" with type as default and a tinted
header plus named tag) rather than an approved comp; the comp round was
skipped.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
