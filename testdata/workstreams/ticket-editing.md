---
slug: ticket-editing
status: active
priority: high
created: 2026-10-11
tickets:
  - FH-80
  - FH-81
  - FH-85
  - FH-86
  - FH-87
  - FH-88
  - FH-101
  - FH-99
  - FH-100
  - FH-65
  - FH-66
  - FH-67
  - FH-16
  - FH-18
  - FH-124
  - FH-125
  - FH-126
  - FH-127
  - FH-128
  - FH-135
  - FH-137
  - FH-138
  - FH-140
  - FH-142
  - FH-146
depends-on-workstreams: []
tags: [tech-debt, ui, ticket-page]
---

# Ticket Editing

## Goal

The ticket's full page becomes the place to read, review and edit a ticket, built on shared groundwork rather than copies:

- Edit domain: ticket edits from the browser and from agents go through one store/board operation with one transaction, one schema owner and the board format's rules (FH-80, FH-81, FH-85 to FH-88).
- Page groundwork: the ticket view is split into sections, a loader and a draft hook, with DOM interaction tests, so the page work composes existing pieces instead of rewriting CardPanel (FH-101, FH-99, FH-100).
- Ticket page: a three-column layout with a section outline, one continuous page and a properties panel (FH-65), in-place editing of the title, properties and sections (FH-66), creating and editing tickets and workstreams in a rich editor (FH-16), pasted and dropped files (FH-18), and human notes in the Activity timeline once their delivery to agents is designed (FH-67).

## Scope

## Success Criteria

- [ ] Every ticket in the line reviewed

## Notes

Merged on 2026-10-12 from the ticket-edit-domain, ticket-page-groundwork and ticket-page workstreams at the user's request.
