# board-refresh

Status: **Done**, awaiting review; remove this brief and its roadmap entry
once approved.

## Goal

User feedback on board-core (2026-10-05): the open card panel hides columns,
and the board is too bland ("Flash by name, Flash by nature").

## Scope

- The card panel sits beside the board instead of over it: the board area
  narrows and scrolls horizontally, and keeps the column the ticket came from
  in view (`CARD-1`).
- A bolder visual identity within the Transit Line Map world (D14), designed
  with the `impeccable` skill: a project rail with its own identity, useful
  colour on cards (for example workstream livery, type and priority), depth
  (shadows) and a board background. Update `DESIGN.md`.

## Acceptance criteria

- With the panel open at 1280 px, the origin column stays fully visible and
  every other column is reachable by scrolling.
- Colour on cards carries meaning a user can name; axe-core passes in both
  themes at 1280 and 1920.

## Out of scope

Editing and live updates (board-editing).

## Evidence

- Panel beside the board at 1280 px with the origin column whole and every
  column reachable by scrolling: `e2e/board.spec.mjs` ("the panel sits
  beside the board…").
- Colour by type, priority, age and none, with a named tag and colour key:
  `model/paint.test.ts`, `e2e/board.spec.mjs`. axe-core passes in light and
  dark at 1280 and 1920; palette contrast checked (tags and headers ≥ 4.5:1).
- Design: user decisions recorded in the surface brief; finish review
  findings applied; `DESIGN.md` regenerated from the build.
