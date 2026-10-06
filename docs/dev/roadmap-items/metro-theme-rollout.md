# metro-theme-rollout

Status: **Pending**. Approved direction, 2026-10-06; no UI implementation yet.
Planning ticket: FH-9. Delivery tickets: FH-10 through FH-14.

## Goal

Make Flashheart colourful and inviting while keeping one consistent product:
**Metro Pop** for light mode and **Night Service in black/charcoal** for dark
mode. Metro Pop governs the shared layout, hierarchy and contents of cards,
workstreams, panels and controls. Dark mode changes the palette, not the
structure, information, workflows or available actions.

## Approved references and authority

The four original PNGs and their generation prompts/checksums are saved in
[`../../../.impeccable/mocks/metro-theme-rollout/`](../../../.impeccable/mocks/metro-theme-rollout/).

- [Metro Pop board](../../../.impeccable/mocks/metro-theme-rollout/metro-pop-board.png).
- [Metro Pop workstreams and detail](../../../.impeccable/mocks/metro-theme-rollout/metro-pop-workstreams.png).
- [Night Service charcoal board](../../../.impeccable/mocks/metro-theme-rollout/night-service-charcoal-board.png).
- [Night Service charcoal workstreams and detail](../../../.impeccable/mocks/metro-theme-rollout/night-service-charcoal-workstreams.png).
- [Generation provenance](../../../.impeccable/mocks/metro-theme-rollout/provenance.json).

User selection supersedes D14's visual direction for this planned rollout.
Garden Line and the original navy Night Service are not selected. The user
explicitly dislikes navy/blue grounds in dark themes. Use neutral black and
charcoal grounds, wells, panels and borders; route colours remain accents.

The mockups are visual references, not literal product specifications. Their
sample text, counts, ticket states, invented types, taglines and dates are
illustrative. SPEC remains authoritative for functionality and data. The
images vary even within Metro Pop (navigation placement, station semantics,
card stripes); FH-10 must resolve these into a single documented structure.
Do not re-run concept selection or use ui-concept-design for this rollout.

## Scope and execution

| Order | Ticket | Deliverable | Depends on |
| --- | --- | --- | --- |
| 1 | FH-10 | Shared layout/content contract derived from Metro Pop | Approved references |
| 2 | FH-11 | Light/dark tokens, shared primitives and shell | FH-10, FH-6 |
| 3 | FH-12 | Board, filters and standardised ticket cards | FH-11 |
| 4 | FH-13 | Workstreams and ticket detail, including existing review/attachment flows | FH-12 |
| 5 | FH-14 | Agents/Table consistency, full visual/accessibility verification and rollout closure | FH-13 |

Preserve the existing roadmap sequence: implementation follows
`review-and-orchestration` (FH-6), which supplies the review/attachments
capabilities being styled. FH-10 may be prepared beforehand. No runtime
dependency on Codex support is introduced. Avoid duplicating work owned by
agent-runs, mcp-protocol, codex-support or review-and-orchestration; this item
changes presentation, not those protocol/backend capabilities.

## Shared layout and content contract (FH-10)

- Define one shell: project rail, project identity, navigation, global Needs
  you, search, backend status, Quit, theme selector, filters and density.
  Apply it to Board, Agents, Workstreams and Table in both themes.
- Define the exact card field order and treatment for Compact, Normal and
  Detailed densities: title, ID, project where needed, workstream badge and
  stripe, type/colour-by value, priority, timestamps, blockers, run state,
  current plan step, criteria, excerpt, handoff and attachment count as
  appropriate. A visual theme must not omit or reorder information.
- Retain Backlog → Up next → In progress → Ready to review → Done, and the
  optional Needs you/Agent working mirrored columns (`VIEW-1`, `VIEW-2`).
- Define real route order, filled served stations, current station rings,
  future stations, suspended/missing states and explicit blockers. Never
  reproduce a generated checked station whose actual ticket is unfinished.
- Define panel header/actions and Ticket, Edit, Runs, Attachments and Review
  tabs, preserving availability rules. Keep Handoff and human questions
  prominent and acceptance criteria directly actionable (`CARD-1`–`CARD-6`).
- Carry the same hierarchy into Agents and Table; their absence from the
  selected images is not permission to omit or invent their functionality.
- Define semantic colour roles once: workstream identity, colour-by paint,
  state and selection must be distinguishable and labelled. Correct mockup
  inconsistencies such as an AR card with an orange stripe. Do not infer a
  new status or type system from a generated badge.
- Specify empty, loading, error, repair, blocked, long-content and narrow
  states; preserve keyboard focus, reduced motion, density and contrast.

## Acceptance criteria

- `VIEW-1`–`VIEW-8`, `CARD-1`–`CARD-6`, `EDIT-1`–`EDIT-8`: existing actions,
  data, IDs, workflow order, mirrored tickets, search/filtering, moves,
  undo, editing and conflicts remain intact in both themes.
- `CFG-2`: System/Light/Dark selection uses the existing preference path;
  switching theme preserves view, filters, selection and content structure.
- `NFR-3`: both palettes meet WCAG 2.2 AA, with keyboard-complete operation,
  visible focus and non-colour state cues.
- `LIFE-1`: backend status, Quit, backend-loss and terminal states remain
  present and usable. No new cloud, account or agent-launching functions.
- Light appearance follows Metro Pop; dark surfaces are black/neutral
  charcoal with no navy cast. Both use the FH-10 component/content contract.
- DESIGN.md, design metadata, the surface brief, frontend facts, relevant
  specs and changelog describe the final implemented system at closure.

## Validation

Feature/behaviour changes are test-first. Add focused failing Vitest or
Playwright tests before implementation; use actual board fixtures and cover
long titles, multiple projects/routes, empty columns, repair, dependencies,
active runs, questions and handoffs. Do not validate only the six sample cards.

Inspect screenshots at 1280 and 1920 in both themes, plus the narrow layout,
for all four views and ticket detail. Compare layout/content parity across
themes and visual character against the saved references. Exercise keyboard
navigation, theme persistence, density, filters, moves and questions. Run
axe-core, `scripts/check.sh` and `scripts/e2e.sh`; save review evidence on the
delivery tickets. A screenshot is not proof of contrast or functionality.

## Out of scope

Implementing this rollout in the planning change; hosting/deployment to a
cloud service; protocol/storage changes; new statuses or agent controls;
copying generated sample data into the product; separate light/dark layouts.
