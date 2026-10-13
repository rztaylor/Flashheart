# Components boundary

Owns shared, product-neutral UI primitives on the semantic roles of
`docs/dev/specs/ui-layout.md`: `Button`, `Icon` (authored set), `LineBullet`,
`Pill` (status, blocker and attribute tags), `ColumnWell` (the board column
surface with its head), `KeyBadge`, `StateNote`, `Tabs`,
`SidePanel`, `SegmentedControl`, fields and form fields (`ProjectField`:
the project a new ticket or workstream goes in), `Popover` (a
disclosure menu on the lifted popover surface, opened from a field-style
button or, on the band, a quiet one), `FilterChip` (the one
filter chip: show only, Cmd or Ctrl click to hide), `ChipInput` (a list
edited as removable chips), `Dialog`, `Toast`,
`Markdown` (block or one inline line), `Lightbox` (one image of a set in a
dialog that fills the viewport, with zoom, pan and their keys and pointer
handling; the zoom and pan maths is `model/zoom`), `Table` (the one table look, ui-layout.md §6: card surface,
sticky head, `HeadCell`, `Row` with dividers, hover and selection),
`FileDrop` (taking files: `AttachButton`, the file picker;
`FileDropArea`, a drop target that shows it will take files;
`UploadNotes` and `useUploadNotes`, a batch's status and refusals),
`TailText` (one line of a path, branch or id cut from its
start so its tail stays visible, ui-layout.md §8; the full value in its
title, the text in a `<bdi>` so punctuation keeps its order),
`BranchLabel` (a branch's mark and name on `TailText`),
`PullRequestLabel` (a ticket's pull request: a link in a new tab only for an
`http(s)` URL, otherwise plain text), `EmptyState`, `LoadError` (a load failure in words, an alert,
with Try again) and `StaleNote` (the last good copy's note; ui-layout.md
§8), `SkeletonTable` (the table look while rows load), `RunState` (run
state as a mark and words), `NeedsYou` (the one attention plate, ui-layout.md §7: the
attention colour, `NeedsYouPlate` inline, `NeedsYouCount` on a rail badge's corner, `plateClass` for
the band's toggle, and `AttentionDisc`; nothing else uses the attention
colour),
`TicketMarks` (a ticket's type as an icon in its paint and its priority
as three bars, for cards and the Colour by chips; `TypeTag` and
`PriorityTag`, each mark before its word in a `Tag`, FH-154),
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
