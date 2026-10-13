---
id: FH-85
status: review
type: refactor
priority: high
created: 2026-10-11
branch: feature/shared-ticket-edit
workstream: ticket-editing
depends-on: [FH-80, FH-81]
depends-on-workstreams: []
tags: [tech-debt, api, mcp, store]
session: claude:b4e92571-1663-41c3-9867-22d57d64c2eb
updated: 2026-10-13T07:23:34Z
pr: https://github.com/rztaylor/Flashheart/pull/94
---
# Board and agent ticket edits follow separately written rules that drift apart

## Description

**Problem.** Editing a ticket from the board and editing it through an agent's MCP tools run two separate copies of the same rules: which fields can change, how notes are formatted, how descriptions are cleaned up, length limits, title trimming and which project keys are allowed. The copies have already drifted apart (FH-80, FH-81).

**Why it matters.** Every fix has to be made twice, and a missed copy means a ticket looks or behaves differently depending on who edited it. Several planned editing features (FH-69, FH-66, FH-16, FH-46) would each add yet another copy.

**Proposed change.** Move the ticket-editing rules into one shared operation that both the board and the MCP tools call. Note lines, description cleanup and limits each get one definition with their own tests. Existing behaviour stays the same; new section and criteria editing is left to FH-69.

## Acceptance Criteria

- [x] Neither internal/api nor internal/mcpserver defines a field change-set type or applies mdfile field edits directly; both call one shared function
- [x] Field, limit, title and key rules have one definition each, with table tests in board/store
- [x] Every Notes bullet Flashheart writes comes from one NoteLine helper
- [x] Existing api and mcpserver tests pass unchanged; scripts/check.sh passes

## Test Plan

Keep the api and mcpserver write tests as the safety net (plus the FH-80 and FH-81 regression tests); add board/store table tests for the shared change-set, NoteLine and NormalizeBody before moving callers.

## Context

**Technical detail** (from the original description):

Goal: make ticket edits proper by moving their rules into one adapter-free operation in store/board that `api` and `mcpserver` only decode into and map errors from.

Debt source: the field change-set loop (local `type change struct{key,value,list,isList}`, `board.ListFields`/`ScalarFields`, `ValidateField`, `SetTitle`/`SetList`/`SetScalar`) is written twice, in `api.patchTicket` (internal/api/write.go) and `mcpserver.updateTicket` (internal/mcpserver/write.go). The blocked-start gate and note, Notes bullet formatting (`api/questions.go`, mcpserver `c.note`), description normalisation (`demoteHeadings` only in mcpserver), limits (16 vs 64 KiB), title trimming (`scrub.Text` vs `board.OneLine`) and the agent key pattern (mcpserver, 2–5 letters, vs `board.ValidKey`, 2–10) all live in the adapters and have already drifted (FH-80, FH-81).
Why it matters: FH-69 (section and criteria rewrites), FH-66 (in-place edits), FH-16 (Tiptap) and FH-46 (update_workstream) would each add another copy.
Target shape: e.g. `store.EditTicket(project, id, precondition, []board.FieldChange, bodyEdits)` that applies fields (routing workstream through membership), and board helpers `NoteLine(date, by, text)` and `NormalizeBody(text)` (scrub, demote headings, limit). api and mcpserver call it.
Scope: api and mcpserver ticket field edits, notes, blocked starts, descriptions; out of scope: new section or criteria operations (FH-69).

## Notes

- 2026-10-13 · claude:d3c609c1: PR 94. Evidence for each criterion. (1) api.patchTicket and mcpserver.updateTicket decode into board.FieldChange, check with board.NewFieldChanges and write with store.EditTicket; archtest TestTicketFieldEditsGoThroughEditTicket keeps ValidateField, SetTitle and SetTicketWorkstream out of api and mcpserver. (2) Each rule has one definition. Fields: NewFieldChanges/EditableField. Title: board.MaxTitle with ValidateField. Keys: board.ValidKey (KEY-1) and board.ValidAgentKey (KEY-5, moved from mcpserver). Blocked-start gate: board.StartsBlocked. Table tests: board TestNewFieldChanges, TestFieldChangesApply, TestStartsBlocked and TestParseIDAndKeys; store TestEditTicket. (3) board.NoteLine(day, by, text) replaces board.Note, and archtest TestNoteBulletsComeFromNoteLine finds the bullet separator spelled only there. (4) api and mcpserver tests unchanged; scripts/check.sh passes. Choices: surface decoding stays explicit. The API trims human input; MCP scrubs and bounds agent text with scrub.Text and items. Title edges are unchanged: PATCH rejects titles over 300 characters, the New ticket dialog cuts them with OneLine, agents' titles are cut with an ellipsis. Description normalisation was already shared (mdfile.SectionSafe FH-87, MaxSectionText FH-81), so no NormalizeBody was added. One edge case changes: the board's move now writes "Started while blocked" only when the move really starts the ticket (StartsBlocked), not for a ticket already in progress. The UI cannot reach that case. Agent protocol unchanged.

## Handoff

_Updated 2026-10-13 07:23 UTC by claude:d3c609c1 (run) on main._

**Done**

- Shared change-set in board (fields.go: FieldChange, NewFieldChanges, Apply, EditableField, StartsBlocked); MaxTitle, NoteLine, ValidAgentKey
- store.EditTicket writes the fields and the caller's body edit; a workstream change goes through membership
- api PATCH and move, and mcpserver update_ticket, move, claim and notes, moved onto the shared rules; archtest guards added
- scripts/check.sh passes; PR 94 open (not merged); review written; all four criteria ticked

**Next**

- Human: review and merge PR 94, then move FH-85 to done
- FH-69 (section and criteria edits) can build on store.EditTicket's edit argument

**Files**

- internal/board/fields.go
- internal/board/rules.go
- internal/store/edit.go
- internal/api/write.go
- internal/mcpserver/tickets.go
- internal/archtest/editing_test.go

**Open questions**

- None
