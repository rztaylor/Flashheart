---
id: FH-100
status: review
type: refactor
priority: high
created: 2026-10-11
branch: feature/ticket-draft-hook
workstream: ticket-editing
depends-on: [FH-101]
depends-on-workstreams: []
tags: [tech-debt, ui, editing]
session: claude:b4e92571-1663-41c3-9867-22d57d64c2eb
updated: 2026-10-13T07:26:09Z
pr: https://github.com/rztaylor/Flashheart/pull/95
---
# Each place you can change a ticket handles edit conflicts its own way

## Description

**Problem.** When you save a ticket in the browser, Flashheart checks nobody else changed it since you opened it and offers a conflict view if they did. That logic lives privately inside the Edit tab. Ticking or verifying an acceptance criterion handles a conflict another way, as a passing toast, and the editing feature has a third way of reporting failures.

**Why it matters.** Ticket editing (FH-66) will let you edit each field and section in place, with inline errors and a conflict dialog, and FH-16 needs to merge changes section by section. Without one shared owner, each would rewrite this logic or depend on the Edit tab, which the planned ticket page removes.

**Proposed change.** Move the draft and its save rules into one shared piece: it remembers which version you started from, adopts the saved version after a save and turns a conflict into a pending choice. The Edit tab uses it now and in-place editors later, and ticking and verifying save through the same path. Behaviour stays the same.

## Acceptance Criteria

- [x] The editing e2e test for saving fields and side-by-side conflicts passes unchanged
- [x] formOf and changes have unit tests
- [x] The hook's adopt-after-save and conflict paths have tests
- [x] Tick, verify and EditTab saves handle a 409 through one helper; scripts/check.sh passes

## Test Plan

Unit tests for formOf/changes first; hook tests need the DOM test environment from the frontend test-infrastructure ticket.

## Context

**Technical detail** (from the original description):

Goal: give ticket edits in the browser one owner for the content-hash precondition.

Debt source: `features/card/EditTab.tsx` privately owns the draft model (`Form`, `formOf`, `changes`, `split`) and the precondition flow (base, adopt/latest refs, run, Pending conflict). Tick and verify in `CardPanel.tsx` handle a conflict a second way (as a toast), and `features/editing/useEditing.ts` has its own `failure`.
Why it matters: FH-66 needs per-field and per-section edits with one open edit, inline errors and the EDIT-7 conflict dialog; FH-16 needs a section-level rebase. Both would otherwise rewrite this logic or depend on a tab FH-65 removes.
Target shape: pure `model/ticketForm.ts` (formOf, changes, split); a `features/editing/useTicketDraft` hook (base hash, adopt after save, conflict to pending, busy/error) used by EditTab now and in-place editors later; one save-with-precondition helper for tick and verify.
Scope: features/card, features/editing, model; behaviour unchanged.

## Notes

- 2026-10-13 · claude:d3c609c1: 2026-10-13: PR #95. useEditing's move, archive and answer failures keep their own `notSaved` words. They do not carry a content hash (moves send base ""), so they stay outside saveWithBase. Pre-existing quirk, kept as is: after Reload in the conflict dialog, the draft adopts the next version of the file even if you edited in between. FH-66 may want to revisit this.

## Handoff

_Updated 2026-10-13 07:26 UTC by claude:d3c609c1 (run) on main._

**Done**

- PR #95 open (not merged): model/ticketForm, editing/precondition saveWithBase, editing/useTicketDraft; EditTab and tick/verify moved onto them
- Unit and DOM tests added; scripts/check.sh passes; e2e --repeat-each=3 passes (264 passed, 30 visual skipped)
- All four criteria ticked; review written

**Next**

- Human: review and merge PR #95
- FH-66: build in-place editors on useTicketDraft

**Files**

- frontend/src/model/ticketForm.ts
- frontend/src/features/editing/precondition.ts
- frontend/src/features/editing/useTicketDraft.ts
- frontend/src/features/card/EditTab.tsx
- frontend/src/features/card/useTicket.ts

**Open questions**

- None
