# Changelog

All notable changes to this project are documented here. The project follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

- `flashheart` binary: `serve` (default) opens a private, authenticated board
  shell in the browser and returns the terminal, running the server in the
  background until Quit or the last tab closes; `--foreground` keeps it
  attached. The background server records later errors in
  `<root>/.flashheart/serve.log`. `version` prints the build and agent protocol version; `mcp`,
  `hook`, `setup` and `doctor` report that they are not yet available.
- Board root resolution from `--root`, `FLASHHEART_ROOT` or
  `~/reports/Kanban`, and read-only loading of `.flashheart/config.yaml` with
  documented defaults and validation.
- Browser shell with quiet backend status and health check, guarded Quit that
  explains a refusal, and stopped and connection-lost screens that explain how
  to close the tab. Light and dark themes follow the system setting.
- Board UI in the Transit Line Map style: every project in a rail with
  route bars, a Board in three densities, a card panel with the ticket,
  blocked-by explanations, handoff, criteria and review, Workstreams drawn as
  transit lines, a sortable Table, filters and search, light and dark themes
  and full keyboard movement.
- Board refresh: the card panel sits beside the board, which narrows and
  keeps the card's column in view; a black project rail with project-key
  badges; cards lift off a dotted platform with the workstream's line down
  their left edge; **Colour by** type (default), priority, age or none tints
  card headers and names the value, with a colour key.
- Read-only board API over the root: projects with counts, project and
  all-projects boards with blocked-by explanations, ticket detail with
  review, handoff and attachments, workstreams with derived status, and
  allow-listed attachment files. Unparseable, oversized or escaping files are
  reported as needing repair instead of being hidden.
- Board format v2: a folder per ticket (`tickets/FH-42-slug/FH-42-slug.md`
  with `review.md` and `files/`), status in frontmatter, and five columns:
  Backlog, Up next, In progress, Ready to review and Done.
- Ticket ids like `FH-42` from a per-project key, shown on cards, the panel,
  the table and in URLs; search by id; a bare id in ticket or review text
  links to that ticket.
- `flashheart migrate [--write] [--key project=KEY]` converts a v1 board:
  numbers tickets oldest first, rewrites references to ids and moves the v1
  files into `.flashheart/backup/`, deleting nothing. A board with v1
  projects shows the command to run.
- Validation: strict `scripts/check.sh`, `scripts/build.sh`, and
  `scripts/e2e.sh` for the Playwright lifecycle, accessibility and screenshot
  suite.
- Product specification, board format (v2) and agent protocol (v1), roadmap,
  decisions, contributor facts and the implementation kickoff prompt.
