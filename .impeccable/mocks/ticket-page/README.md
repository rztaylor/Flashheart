# Ticket page reference

The ticket's full page in three columns: a section outline, one continuous
page and a properties panel. The user chose it on 2026-10-11 for FH-65, out
of three layouts (a margin column, this outline layout, and a page led by
the ticket's status). It is a design reference, not shipped UI.

| File | What it is |
| --- | --- |
| [b-outline.html](b-outline.html) | The static HTML mock, with FH-63's real content |
| [ticket-shots/ticket-page-b-light.png](ticket-shots/ticket-page-b-light.png) | First viewport, light, 1440 × 1000 |
| [ticket-shots/ticket-page-b-dark.png](ticket-shots/ticket-page-b-dark.png) | First viewport, dark, 1440 × 1000 |
| [ticket-shots/ticket-page-b-review.png](ticket-shots/ticket-page-b-review.png) | The Review part, with each screenshot once in the Evidence gallery |
| [ticket-shots/ticket-page-b-activity.png](ticket-shots/ticket-page-b-activity.png) | The Activity timeline |
| [ticket-shots/ticket-page-b-full.png](ticket-shots/ticket-page-b-full.png) | The whole page, light |

To view the mock, serve this folder (it loads its stylesheet, script and
font by relative path) and open `b-outline.html`. For example, run
`python3 -m http.server 8765` here and open
http://127.0.0.1:8765/b-outline.html. The Light and Dark buttons in the
band switch the theme.

FH-65 is the authority on scope. The mock differs from it in these places:

- The outline always highlights Handoff; the real outline follows the
  scroll.
- The mock has a note box under Activity. FH-65 ships none until the
  human-notes spike (FH-67) decides how notes reach agents.
- The mock has no Jump to menu or stacked layout for narrow widths.
- The mock never had pencil edit points; in-place editing is FH-66.

The workstream line strip in the chosen comp was dropped at the user's
request and is not in the mock.

## Provenance

- Built by hand in HTML and CSS from Flashheart's tokens. `assets/tokens.css`
  copies the token blocks of `frontend/src/styles/tokens.css` as of
  2026-10-11. No image generation was used.
- `assets/archivo-latin-wdth-normal.woff2` comes from
  `@fontsource-variable/archivo` 5.3.0, under the SIL Open Font License,
  the font the app already ships.
- The six `assets/*-overview-pipeline-*.jpg` files are FH-63's review
  screenshots, resized to 900 px wide.
- The screenshots in `ticket-shots/` were captured with Playwright's
  Chromium at 1440 px wide. FH-65 holds copies of them on the board.
- The layout came from an `impeccable` surface round (seed `bb9bc6bf`, mode
  operate). The two layouts not chosen are not kept.
