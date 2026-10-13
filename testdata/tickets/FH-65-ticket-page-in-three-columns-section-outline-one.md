---
id: FH-65
status: review
type: feature
priority: medium
created: 2026-10-11
branch: feature/ticket-page-outline
workstream: ticket-editing
depends-on: [FH-99, FH-100]
depends-on-workstreams: []
tags: [ui, ticket-page]
session: claude:2ff6eb1c-9261-47a2-abdd-2233822a2b1b
updated: 2026-10-13T08:14:58Z
pr: https://github.com/rztaylor/Flashheart/pull/96
---
# The ticket's full page uses the width: outline, one long page, properties

## Description

**Problem.** A ticket's full page is just the card panel stretched to a medium column. It wastes a wide screen, splits the ticket across five tabs, and shows every screenshot twice on the Review tab.

**Why it matters.** The full page is where a person reads a ticket properly, especially when reviewing an agent's work. Hopping between tabs and seeing repeated images makes that slower and harder to follow.

**Proposed change.** Lay the page out in three columns, as the user chose from mocked options. On the left, an outline of the ticket's sections with counts, highlighting the one in view. In the centre, the whole ticket as one continuous page: open questions, handoff, criteria, description and other sections, then the review if there is one, then a read-only activity timeline. On the right, the ticket's properties and Move to. Each screenshot appears once. Narrower screens get a Jump to menu and stacked properties. Editing in place and human notes are separate tickets; the board's card panel is unchanged.

## Acceptance Criteria

- [x] At 1280px and wider the full page shows a left section outline, a central continuous page and a right properties panel, with no tabs
- [x] The centre shows, in order: needs repair, questions waiting for you, Handoff, Blocked by, Acceptance criteria, Description, Test plan, Context and other ticket sections; then Review (only when review.md exists) in review.md's own sections; then Activity
- [x] The outline lists every section present with counts for criteria, evidence, How to verify steps and activity, shows empty ticket sections as empty, highlights the section in view, and moves focus to a section when one is chosen, by mouse or keyboard
- [x] Each screenshot appears once: review.md's Evidence images render as a captioned gallery that opens the lightbox, and attachments the review does not reference appear in a Files entry after Review
- [x] Activity shows notes, agent questions with their answers, and handoff updates as a timeline, oldest first, with no note box
- [x] The properties panel stays in view and shows status (Move to), priority, type, workstream, branch, depends on, tags, agent, created and changed, plus Archive
- [x] Below 1280px the outline becomes a Jump to menu; on narrow screens the properties stack above the content; no width scrolls horizontally
- [x] The full page has no Edit, Runs, Attachments or Review tabs and no workstream line strip; the board's card panel is unchanged
- [x] Both themes meet WCAG 2.2 AA, axe-core passes, and the page works fully by keyboard with visible focus
- [x] SPEC CARD-7, ui-layout.md (Ticket page), DESIGN.md (Ticket Page), the card BOUNDARY.md and the changelog describe the new page

## Test Plan

Red first. Component tests for the ticket page:
- Section order with and without `review.md`.
- Outline entries and counts: criteria d/t, evidence n, verify d/t, activity n, and empty sections.
- The outline's current section as the page scrolls (mocked IntersectionObserver).
- Evidence dedupe: an image linked in review.md renders once, and an unreferenced attachment appears under Files.
- The Activity timeline order, with a question and its answer.
- No Edit or Runs tab on the page, while the card panel still has them.

Playwright, extending the ticket-page e2e: open a ticket in review and one without a review. Choose outline entries by keyboard and check focus moves. Open the lightbox from Evidence. Take screenshots at 1440 and 1280 in light and dark, plus 390 (Jump to menu, properties stacked, no horizontal scroll). axe-core in both themes. Then scripts/check.sh and scripts/e2e.sh.

## Context

**Technical detail** (from the original description):

The ticket's full page (CARD-7, `#/ticket/<id>`) is the card panel stretched to a 64rem column. It wastes the width, splits the ticket across five tabs (Ticket, Edit, Runs, Attachments, Review), and on Review shows every screenshot twice: once in the screenshot strip and again in the review's Evidence section.

Redesign the full page as three columns, chosen by the user on 2026-10-11 from three layouts mocked in Metro Pop with FH-63's real content (option B, "Outline and line"; the screenshots are attached):

- **Left: section outline.** It stays in view while the page scrolls and lists every section present, with counts (Acceptance criteria 6/6, Evidence 6, How to verify 0/5, Activity 4). Empty ticket sections show greyed out as empty. The section in view is highlighted. Groups: the ticket's sections, then Review, then Activity.
- **Centre: one continuous page, no tabs.** Needs repair, questions waiting for you, Handoff, Blocked by, Acceptance criteria, Description, Test plan, Context and any other sections. Then Review, when `review.md` exists, in its own sections (Summary, Evidence, How to verify, Risks, Key files, Tests). Then Activity.
- **Right: properties panel.** It stays in view and holds the status (Move to), priority, type, workstream, branch, depends on, tags, agent, created and changed, plus Archive. These values are read-only here; in-place editing is the follow-up ticket.

Decisions (user, 2026-10-11):
1. Below about 1280px the outline folds into a "Jump to" menu. On narrow screens the properties stack above the content. Nothing scrolls horizontally.
2. **No workstream line strip.** The mock's line of stations across the top was dropped ("this will get messy fast"). Do not build it.
3. The Review group appears only once `review.md` exists, never as empty sections.
4. Human notes: humans should be able to add notes, but how a note reaches agents is undecided (delivered to a live session at once, or read only when an agent next reads the ticket, and what happens when no session is attached). That needs a brainstorm and its own ticket. **This ticket ships no note box.** Activity is read-only.
5. SPEC, ui-layout.md, DESIGN.md, the card BOUNDARY.md and the changelog change with it.

Also decided:
- **Tabs removed from the full page.** Edit goes, replaced by in-place editing (follow-up ticket). Runs goes, as it offers the user nothing on this page. Attachments and Review become sections. The board's card panel keeps all its tabs; any panel changes are a separate ticket after this one, so editing stays possible from the board meanwhile.
- **Each screenshot once.** The review's Evidence images render as one captioned gallery that opens the lightbox. Attachments the review does not reference appear in a Files entry after Review.
- **Activity** is a timeline, oldest first, of notes, agent questions with your answers, and handoff updates. Unanswered questions stay at the top as today's callout.
- **Edit points.** No pencil icons yet; they arrive with in-place editing.

The mockup (static HTML, not shipped) is `.impeccable/mocks/ticket-page/b-outline.html` in the `ticket-detail-redesign` worktree. The mock highlights Handoff in the outline permanently; the real outline tracks the scroll.

## Notes

- 2026-10-11 · claude:2ff6eb1c: Design references (mock screenshots, 2026-10-11; the workstream line strip is already removed): ![Chosen layout, light](files/20261011T1220-ticket-page-b-light.png) ![Chosen layout, dark](files/20261011T1220-ticket-page-b-dark.png) ![Review part with the deduplicated Evidence gallery](files/20261011T1220-ticket-page-b-review.png) ![Activity timeline; the note box is out of scope](files/20261011T1220-ticket-page-b-activity.png) ![Whole page](files/20261011T1220-ticket-page-b-full.png) ![Before: the current full page](files/20261011T1220-1.png)

- 2026-10-11 · claude:2ff6eb1c: 2026-10-11: The human-notes question is now spike FH-67. Activity's note box waits for its outcome.

- 2026-10-11 · claude:2ff6eb1c: 2026-10-11: The HTML mock and its screenshots are committed for reference in .impeccable/mocks/ticket-page/ (README with provenance), docs PR https://github.com/rztaylor/Flashheart/pull/45. Once merged, that path on main replaces the worktree path given in the Description.

- 2026-10-13 · claude:d3c609c1: Design decision: Activity is derived on the client from what the ticket API already returns, with no backend change: notes from the ticket's ## Notes bullets (board.NoteLine format; board-answered questions parsed from their 'Question from ... Answer (...)' note), agent questions from the linked runs' questions and the ticket's open questions (deduped with their notes by run and text), and handoff updates from the runs' checkpoint timeline entries plus the current handoff's _Updated_ line. Run data covers the 48 h run window, so older questions and checkpoints survive only as notes and the current handoff.

- 2026-10-13 · claude:d3c609c1: All ten criteria pass with evidence in PR #96: component tests (TicketPage.dom.test.tsx), model tests (activity, ticketSections, markdown), the FH-65 Playwright test (FH-8 in review, FH-11 without one, keyboard outline, lightbox, axe-core in both themes at 1440/1280/390, no sideways scroll), scripts/check.sh, scripts/e2e.sh --repeat-each=3 (267 passed) and scripts/e2e-visual.sh with the ticket baseline updated. Decisions taken here: Activity is derived in the browser from the ticket API (D46), so questions answered in the session's own chat and checkpoints older than the 48 h run window drop out; within one day, notes come before timed entries. Files leaves out attachments the ticket body links as well as the review's. Empty template sections show only in the outline. A chosen outline entry stays current until the person scrolls, types or clicks.

## Handoff

_Updated 2026-10-13 08:14 UTC by claude:d3c609c1 (run) on main._

**Done**

- The three-column full page is built: outline, one continuous page, properties; Jump to and stacked properties below 1280px
- Model logic in model/ticketSections, model/activity and model/markdown, with unit and DOM tests
- The FH-65 Playwright test and the updated ticket visual baseline
- SPEC CARD-7, ui-layout.md, DESIGN.md, decision D46, the boundaries and the changelog are updated
- check.sh, e2e.sh --repeat-each=3 and e2e-visual.sh pass; PR #96 is open; review written

**Next**

- Human reviews PR #96 and merges
- FH-66: add in-place edits through PropertyRow (PropertiesPanel.tsx) and PageSectionView (TicketPage.tsx); empty template sections need a place in the page for adding content

**Files**

- frontend/src/features/card/TicketPage.tsx
- frontend/src/features/card/SectionOutline.tsx
- frontend/src/features/card/useSectionInView.ts
- frontend/src/features/card/PropertiesPanel.tsx
- frontend/src/features/card/ActivityTimeline.tsx
- frontend/src/model/ticketSections.ts
- frontend/src/model/activity.ts
- frontend/src/model/markdown.ts
- frontend/e2e/board.spec.mjs
- frontend/e2e/demo-board.mjs

**Open questions**

- None
