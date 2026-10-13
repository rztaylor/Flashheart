---
slug: workstream-editing
status: active
priority: medium
created: 2026-10-06
tickets:
  - FH-17
  - FH-46
  - FH-68
  - FH-139
depends-on-workstreams: []
tags: [ui, workstreams, user-request]
---

# Workstream Editing

## Goal

Humans and agents create and change workstreams: creating them from the board, ordered or unordered (FH-17), and an MCP tool that edits an existing workstream's title, goal, priority, tags, dependencies and members (FH-46).

## Scope

FH-17 creates workstreams from the board. FH-46 adds `update_workstream`, which also covers renames like this line's own without editing board files. The ticket editor (FH-16) and paste and drop attachments (FH-18) moved to the `ticket-page` workstream on 2026-10-11.

## Success Criteria

- [ ] Every ticket in the line reviewed

## Notes

- 2026-10-06: Created ordered at the user's request; they will implement the tickets in this order. Once FH-17 ships `ordered: false`, this line could become unordered, but that isn't needed.
- 2026-10-11: Renamed from "Ticket authoring" (`ticket-authoring`) to "Workstream Editing" (`workstream-editing`) at the user's request, after FH-16 and FH-18 moved to `ticket-page`. Edited directly with the user's one-time approval, because no tool can edit a workstream yet (FH-46).
