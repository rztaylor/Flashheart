---
name: Flashheart
description: A local kanban for AI-assisted projects, drawn as a transit line map.
colors:
  map-white: "#ffffff"
  platform-grey: "#f2f3f5"
  hairline: "#d7dadf"
  signal-ink: "#111111"
  ink-muted: "#565c66"
  ink-faint: "#6a6f78"
  signage-black: "#111111"
  on-band: "#ffffff"
  on-band-muted: "#a9aeb6"
  band-field: "#26282c"
  alarm-red: "#b3261e"
  alarm-surface: "#fdf0ef"
  selection-yellow: "#fcd96a"
  night-ground: "#121315"
  night-well: "#1b1c1f"
  night-card: "#202226"
  night-hairline: "#33363c"
  night-ink: "#f1f2f4"
  night-ink-muted: "#a3a8b1"
  night-ink-faint: "#8f949d"
  night-band: "#000000"
  night-on-band-muted: "#9ea3ab"
  night-band-field: "#1d1f23"
  night-alarm: "#ff8a80"
  night-alarm-surface: "#2a1517"
  night-selection: "#6b5a12"
  line-red: "#d52b1e"
  line-blue: "#0039a6"
  line-yellow: "#fccc0a"
  line-green: "#00843d"
  line-purple: "#a93aa0"
  line-orange: "#ff6319"
  line-lime: "#6cbe45"
  line-brown: "#8a5a2b"
  line-grey: "#8d8f92"
  night-line-red: "#ee4b3f"
  night-line-blue: "#4a86f0"
  night-line-yellow: "#fccc0a"
  night-line-green: "#22a65a"
  night-line-purple: "#cc5fc2"
  night-line-orange: "#ff7a3d"
  night-line-lime: "#7acb52"
  night-line-brown: "#b8854f"
  night-line-grey: "#a2a4a8"
typography:
  wordmark:
    fontFamily: "Archivo Variable, Helvetica Neue, Helvetica, Arial, sans-serif"
    fontSize: "1rem"
    fontWeight: 800
    lineHeight: "1.45rem"
    letterSpacing: "-0.01em"
    fontVariation: "'wdth' 112.5"
  headline:
    fontFamily: "Archivo Variable, Helvetica Neue, Helvetica, Arial, sans-serif"
    fontSize: "1.375rem"
    fontWeight: 600
    lineHeight: 1.25
    letterSpacing: "-0.01em"
  station-sign:
    fontFamily: "Archivo Variable, Helvetica Neue, Helvetica, Arial, sans-serif"
    fontSize: "1rem"
    fontWeight: 650
    lineHeight: "1.45rem"
    letterSpacing: "0"
    fontVariation: "'wdth' 87.5"
  station-sign-small:
    fontFamily: "Archivo Variable, Helvetica Neue, Helvetica, Arial, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 650
    lineHeight: "1.2rem"
    letterSpacing: "0"
    fontVariation: "'wdth' 87.5"
  title:
    fontFamily: "Archivo Variable, Helvetica Neue, Helvetica, Arial, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 500
    lineHeight: 1.375
  body:
    fontFamily: "Archivo Variable, Helvetica Neue, Helvetica, Arial, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: "1.35rem"
    fontFeature: "'tnum' 1"
  label:
    fontFamily: "Archivo Variable, Helvetica Neue, Helvetica, Arial, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: "1.05rem"
    fontFeature: "'tnum' 1"
  meta:
    fontFamily: "Archivo Variable, Helvetica Neue, Helvetica, Arial, sans-serif"
    fontSize: "0.6875rem"
    fontWeight: 400
    lineHeight: "1rem"
    fontFeature: "'tnum' 1"
  slug:
    fontFamily: "ui-monospace, SF Mono, Menlo, Consolas, monospace"
    fontSize: "0.6875rem"
    fontWeight: 400
    lineHeight: "1rem"
rounded:
  card: "2px"
  panel: "2px"
  control: "4px"
  bullet: "9999px"
spacing:
  unit: "4px"
  gap-tight: "8px"
  card-x: "12px"
  card-y: "10px"
  page: "16px"
  panel-gutter: "24px"
  band-height: "48px"
  rail-width: "248px"
  panel-width: "560px"
components:
  button-primary:
    backgroundColor: "{colors.signal-ink}"
    textColor: "{colors.map-white}"
    typography: "{typography.title}"
    rounded: "{rounded.control}"
    padding: "6px 12px"
  button-secondary:
    backgroundColor: "{colors.map-white}"
    textColor: "{colors.signal-ink}"
    rounded: "{rounded.control}"
    padding: "6px 12px"
  button-quiet:
    textColor: "{colors.ink-muted}"
    rounded: "{rounded.control}"
    padding: "6px 12px"
  button-band:
    backgroundColor: "{colors.signage-black}"
    textColor: "{colors.on-band}"
    rounded: "{rounded.control}"
    padding: "0 12px"
    height: "32px"
  button-band-hover:
    backgroundColor: "{colors.band-field}"
  signage-band:
    backgroundColor: "{colors.signage-black}"
    textColor: "{colors.on-band}"
    height: "{spacing.band-height}"
  search-field-band:
    backgroundColor: "{colors.band-field}"
    textColor: "{colors.on-band}"
    rounded: "{rounded.control}"
    height: "32px"
    width: "256px"
  select-field:
    backgroundColor: "{colors.map-white}"
    textColor: "{colors.signal-ink}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    height: "28px"
  segmented-option-active:
    backgroundColor: "{colors.signal-ink}"
    textColor: "{colors.map-white}"
    typography: "{typography.label}"
    padding: "2px 8px"
  ticket-card:
    backgroundColor: "{colors.map-white}"
    textColor: "{colors.signal-ink}"
    typography: "{typography.title}"
    rounded: "{rounded.card}"
    padding: "10px 12px"
  line-bullet:
    rounded: "{rounded.bullet}"
    size: "24px"
  side-panel:
    backgroundColor: "{colors.map-white}"
    textColor: "{colors.signal-ink}"
    width: "{spacing.panel-width}"
  project-rail:
    backgroundColor: "{colors.platform-grey}"
    width: "{spacing.rail-width}"
---

# Design System: Flashheart

## Overview

**Creative North Star: "The Transit Line Map"**

Flashheart is drawn as a transit system in the Vignelli 1972 diagram and Unimark signage tradition. Each workstream is a coloured line, its tickets are stations, and the shell is a black signage band over a white map ground (a near-black night map in dark mode). Order, progress and blockage read as a route map: a station served, the next stop, a stretch of suspended service.

The world is dense and calm, built to be glanced at on a second monitor all day and read closely in review sessions. One grotesk in tight sentence case, every number set as tabular data, almost no hue outside the line palette. Colour is quarantined to one job (naming workstreams); everything else is ink, rule weight, shape and words. Focus dims the rest of the board instead of filtering it away. The system refuses the grey SaaS lane-and-badge kanban: no coloured status pills, no badge rainbow, no drop-shadowed card stacks.

**Key Characteristics:**
- A black signage band (48px) carries the shell in both themes.
- Line colours mean workstreams and nothing else; ticket state is monochrome.
- Section heads are station signs: a heavy top rule over a condensed semibold label.
- Flat map surfaces with hairline rules; only the slide-over panel casts a shadow.
- Near-square corners (2px) on cards and panels; round only for bullets and stations.
- Archivo Variable, self-hosted, with tabular numerals everywhere.

## Colors

A monochrome map in two themes, plus a nine-colour line palette borrowed from the MTA that only ever names a workstream.

### Primary
- **Signal Ink** (light `signal-ink`, dark `night-ink`): text, primary buttons, the strong rule under station signs, selected-card ring, focus outline, the interchange ring of the next stop, and every state mark (diamond, hatching). It is the system's only "accent" for state.

### Secondary
- **The Line Palette** (`line-red`, `line-blue`, `line-yellow`, `line-green`, `line-purple`, `line-orange`, `line-lime`, `line-brown`, `line-grey`, with lifted `night-line-*` values for dark mode): ordered so neighbouring indices are far apart in hue. Each workstream in a project is assigned one line, stably, in creation order (a newer workstream never recolours an older one). Used as line bullet fills, transit track and station rings. Bullet initials take a paired ink per line (white on red, blue, green, purple and brown in light mode; near-black on yellow, orange, lime and grey; near-black on every line in dark mode). Bullets and stations carry a faint casing ring so yellow and lime hold their edge on white.

### Tertiary
- **Alarm Red** (light `alarm-red` on `alarm-surface`, dark `night-alarm` on `night-alarm-surface`): app error alerts only (the board root failed to load, shutdown refused, a fetch error). Never a ticket state.
- **Selection Yellow** (`selection-yellow`, dark `night-selection`): text selection highlight only.

### Neutral
- **Map White** (`map-white`; dark `night-ground` and `night-card`): page ground and card surface. In dark mode the card sits one step above the ground.
- **Platform Grey** (`platform-grey`; dark `night-well`): the project rail, the handoff block, skeletons, code, and the selected table row.
- **Hairline** (`hairline`; dark `night-hairline`): card borders, column dividers, row rules, scrollbars.
- **Ink Muted / Ink Faint** (`ink-muted`, `ink-faint`; dark `night-ink-muted`, `night-ink-faint`): metadata, counts, running times, done titles, and the quiet "waiting" note. Both pass AA on their grounds.
- **Signage Black** (`signage-black`; dark `night-band`, true black) with **On Band** text, **On Band Muted** for inactive tabs, and **Band Field** for the search well and hover fills inside the band.

### Named Rules
**The Lines Are Workstreams Rule.** A line colour appears only where a workstream is named or drawn: bullet, track, station, legend chip. Never use a line colour for status, priority, emphasis, project identity or decoration.

**The Ink, Shape and Words Rule.** Ticket state is monochrome. A real blocker is a diamond plus its reason in ink; a wait on an earlier station of the same line is muted text with no mark; needs repair is a dashed border with a hatched top band plus the reason. Every state is paired with its proof in words.

**The Alarm Is for the App Rule.** Alarm red is reserved for application error alerts. If it appears on a ticket, it is a bug.

**The Black Band Rule.** The signage band is black in both themes (true black in dark mode). The map changes with the theme; the signage does not.

## Typography

**Display Font:** Archivo Variable (with Helvetica Neue, Helvetica, Arial), self-hosted with its width axis
**Body Font:** Archivo Variable
**Label/Mono Font:** system monospace (ui-monospace, SF Mono, Menlo, Consolas) for slugs, paths and branches only

**Character:** One Helvetica-class grotesk used across its width axis: condensed for signage, normal for reading, expanded for the wordmark. Sentence case throughout; no uppercase tracking.

### Hierarchy
- **Wordmark** (800, 1rem, expanded width 112.5%): the Flashheart name in the band. Nowhere else.
- **Headline** (600, 1.375rem, tight line height, -0.01em): the ticket title at the top of the card panel.
- **Station Sign** (650, 1rem, condensed 87.5%): board column heads, empty-state titles, project names (1.125rem) in Workstreams, workstream names.
- **Station Sign Small** (650, 0.8125rem, condensed 87.5%): panel section heads, tabs, table column heads, rendered-markdown headings and the current-project sign in the band.
- **Title** (500, 0.8125rem, snug): ticket card titles and station names; done tickets drop to muted ink.
- **Body** (400, 0.875rem / 1.35rem): panel and markdown reading text, the base size of the app.
- **Label** (400, 0.75rem): controls, filters, counts, state notes on cards.
- **Meta** (400, 0.6875rem): running times, slugs (in monospace), criteria and attachment counts.

### Named Rules
**The Station Sign Rule.** A section head is a condensed semibold label standing under a strong ink rule. Rule weight follows rank: 5px over board columns, 3px under table heads and active tabs, 2px over panel sections, markdown H2s and the band's project sign.

**The Numbers Are Data Rule.** Tabular numerals are on globally. Counts sit right-aligned against their label; running times sit right-aligned on the card's title row.

## Layout

A fixed three-row app grid: the 48px signage band, an optional alert strip, then the work area. At 48rem and up the work area is a 248px project rail (platform grey, hairline right edge) beside the main view; below that the rail hides and a compact control row takes its place.

The Board is four equal columns (minimum 15rem each, horizontal scroll if needed) separated by hairline verticals, with 16px page padding and 12px column gutters. A line legend strip of pill chips sits above the columns; choosing a line dims other cards to 35% rather than hiding them. Under 48rem the board shows a single column chosen with a column picker. Cards stack with an 8px gap.

Workstreams stacks projects (40px apart) and their lines (separated by hairline rules), each line a horizontal scroller of 176px stations, with an edge fade and "more" hint when the route runs off-screen. Table is a full-width sortable table with a sticky head. The card panel slides over from the right at 560px (full width on phones) without covering the band, and leaves the board visible.

Spacing runs on a 4px unit: 8px tight gaps, 10/12px card padding, 16px page edges, 24px panel gutters.

## Elevation & Depth

The map is flat. Surfaces are separated by hairline rules and one tonal step (platform grey rail and wells against white ground; in dark mode the card lifts one step above the ground). Selection is a 1px ink ring, not a lift. The single cast shadow belongs to the slide-over card panel, which sits above the board.

### Shadow Vocabulary
- **Panel** (`box-shadow: -12px 0 32px -8px rgb(0 0 0 / 0.22), -1px 0 0 rgb(0 0 0 / 0.06)`): the card panel's left edge only.
- **Selection ring** (`box-shadow: 0 0 0 1px` signal ink): the selected ticket card, doubling its border.
- **Station halo** (`box-shadow: 0 0 0 3px` ground): clears the track around the next-stop ring.

### Named Rules
**The Flat Map Rule.** Nothing on the map casts a shadow. Only the layer that slides over the map does.

## Shapes

Near-square: 2px corners on cards, panels, wells and code blocks; 4px on controls (buttons, fields, segmented controls, rail items). Round is reserved for the transit vocabulary: line bullets, stations, route-bar terminals and legend chips. Borders are 1px hairlines; strong ink rules (2 to 5px) mark signs and selection. Dashes have two meanings only: a dashed card border means needs repair, and dashed track or a dashed station ring means suspended service or a missing station. Hatching is monochrome: the strong 45-degree ink hatch marks the needs-repair band and the in-progress stretch of a route bar.

## Components

### Buttons
Plain signage hardware, small and square-shouldered.
- **Shape:** gently squared (4px), 6px by 12px padding, 0.8125rem medium label, optional 14px icon.
- **Primary:** signal ink fill with ground-coloured text; hover lightens to 85% ink.
- **Secondary (default):** card surface with a hairline border; hover darkens the border to muted ink.
- **Quiet:** text only in muted ink; hover to full ink.
- **Band:** for controls inside the signage band (Quit): transparent with a 40% on-band-muted border, 32px tall; hover fills with band field and brightens the border.
- **Focus:** a 2px outline offset 2px in signal ink (on-band white inside the band). Disabled drops to 50% opacity.

### Chips
- **Line legend chips:** pill-shaped, card surface, hairline border, a line bullet at the left, the workstream name and a done/total count. The pressed chip takes an ink border; other chips fade to 50% while one is focused.

### Cards / Containers
- **Ticket card:** 2px corners, card surface, hairline border, 10px by 12px padding. Row one: line bullet, title, right-aligned running time. Row two: project (in All view), monospace slug, priority in words (High in semibold ink). Then, by density, the state note, criteria count, excerpt, handoff next step and attachments. Hover darkens the border; selection is an ink border plus 1px ink ring; needs repair swaps to a dashed muted border with a 6px hatched band across the top.
- **Three densities:** compact (title row and meta, a small diamond "Blocked" inline), normal (adds the blocker or wait note and criteria), detailed (adds excerpt, next step, attachments and review).
- **Handoff block:** platform grey well, 2px corners, 16px padding, station-sign head.

### Inputs / Fields
- **Search (band):** 32px tall, 256px wide, band-field well, borderless, on-band text, magnifier icon inset left; focus shows an on-band-muted border and white outline.
- **Search (plain) and selects:** card surface, hairline border, 4px corners, 28 to 32px tall, chevron inset right. A select with an active value takes an ink border so applied filters read at a glance.
- **Segmented control:** joined segments in a hairline frame; the chosen segment is an ink fill with ground text.
- **Checkboxes:** native, tinted signal ink.

### Navigation
- **Signage band:** wordmark, a hairline divider, the current project as a small station sign under a 2px white rule, then view tabs (Board, Workstreams, Table) with 15px icons. Active tab: white text over a 3px white underline; inactive: on-band-muted. Search, a quiet status dot (shape and brightness only, trouble shown in words) and Quit sit right. On phones the labels collapse to icons.
- **Project rail:** All projects, then each project with its ticket count, blocked and repair counts (diamond and repair marks), and a route bar. The active project is a card-surface tile with a hairline inset ring.
- **Panel tabs (Ticket, Review):** small station-sign labels over a hairline, the active tab underlined 3px in ink.

### Line Bullet
The round transit bullet: line colour fill, paired ink initials (one or two letters from the slug) in a bold condensed cut, a faint casing ring. Sizes 20, 24 and 32px. Dims to 30% when its line is out of focus.

### Route Bar
A project's progress as a stretch of track between two terminal stations: review and done in solid ink, in progress in strong hatching, the rest on a hairline track. Terminals fill in ink once served. Monochrome so it never competes with line colours.

### Transit Line (signature)
Workstreams drawn as transit lines with tickets as stations, in order. Served stations (review, done, archived) are filled line-colour discs with a ground-coloured centre dot; the next stop is an interchange ring in signal ink with a ground halo; stations ahead are open rings in the line colour. Track already travelled is solid line colour; the route ahead is the same colour at 35% as a solid track. Only suspended service is dashed: the whole line when its workstream dependencies are unmet, or the track into a station held by something outside the line. A missing station is a dashed faint ring reading "Does not exist". A blocked next stop carries a small diamond "blocked" above it.

**The Suspended Service Rule.** Dashed track means suspended service and nothing else. The route ahead is never dashed.

## Do's and Don'ts

### Do:
- **Do** draw every workstream reference with its line bullet or line colour, assigned per project by the stable line assignment.
- **Do** pair every ticket state with its proof in words: diamond plus reason for blockers, muted words for in-line waits, dashed border plus hatched band plus reason for needs repair.
- **Do** head sections with the station sign: condensed semibold label under a strong ink rule, weight by rank (5px, 3px, 2px).
- **Do** keep the signage band black in both themes, with on-band text and band-field wells.
- **Do** dim out-of-focus items (35% cards, 30% bullets, 50% chips) instead of removing them.
- **Do** keep corners at 2px for cards and panels and 4px for controls.

### Don't:
- **Don't** use a line colour for status, priority, project identity or decoration.
- **Don't** colour ticket states: no red blocked badges, no green done pills, no amber warnings. Alarm red is for app error alerts only.
- **Don't** add drop shadows to cards or anything else on the map; only the slide-over panel casts one.
- **Don't** dash the route ahead; dashes mean suspended service, a missing station, or needs repair.
- **Don't** set labels in uppercase or tracked-out caps; signage here is sentence case in the condensed cut.
- **Don't** use proportional numerals for counts or times.
