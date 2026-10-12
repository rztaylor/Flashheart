# Components boundary

Owns shared, product-neutral UI primitives on the semantic roles of
`docs/dev/specs/ui-layout.md`: `Button`, `Icon` (authored set), `LineBullet`,
`Pill` (status, blocker and attribute tags), `ColumnWell` (the board column
surface with its head), `KeyBadge`, `StateNote`, `Tabs`,
`SidePanel`, `SegmentedControl`, fields and form fields, `Popover` (a
disclosure menu on the lifted popover surface), `FilterChip` (the one
filter chip: show only, Cmd or Ctrl click to hide), `Dialog`, `Toast`,
`Markdown` (block or one inline line), `Lightbox` (one image of a set in a
dialog), `Table` (the one table look, ui-layout.md §6: card surface,
sticky head, `HeadCell`, `Row` with dividers, hover and selection),
`RouteBar`, `TailText` (one line of a path, branch or id cut from its
start so its tail stays visible, ui-layout.md §8; the full value in its
title, the text in a `<bdi>` so punctuation keeps its order),
`BranchLabel` (a branch's mark and name on `TailText`), `EmptyState`, `LoadError` (a load failure in words, an alert,
with Try again) and `StaleNote` (the last good copy's note; ui-layout.md
§8), `SkeletonTable` (the table look while rows load), `RunState` (run
state as a mark and words), `NeedsYou` (the one attention plate, ui-layout.md §7: the
attention colour, `NeedsYouPlate` inline and on the rail, `plateClass` for
the band's toggle, and `AttentionDisc`; nothing else uses the attention
colour),
`QuestionCard` (an
agent's question with its answer box), `TicketLink` (a ticket id as a link
to its full page, a new tab unless inside `TicketLinksInPlace`, and
`LinkedText` for ids in plain text), `Aside` (a marginal remark, at
most one per screen through `AsideProvider`) and `useKeepFocus` (the one
way a list keeps keyboard focus when its items move or go, ui-layout.md §8
"Keeping focus": callers mark lists, items and headings with `focusList`,
`focusItem` and `focusHeading`; the neighbour rule is `model/navigation`). Components take typed props and callbacks, use token-backed
utilities only, and carry their own accessibility semantics.

Does not own API calls, lifecycle or session state, data loading or feature
behaviour, and never imports from `features/` or `app/`. Depends only on
`styles/` and pure helpers in `model/`.
