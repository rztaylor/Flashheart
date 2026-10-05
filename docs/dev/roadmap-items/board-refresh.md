# board-refresh

Status: **Pending**. Depends on `board-format-v2`.

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
