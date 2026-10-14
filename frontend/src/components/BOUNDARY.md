# Components boundary

Owns shared, product-neutral UI primitives and patterns on the semantic
roles of `docs/dev/specs/ui-layout.md`: controls, fields, dialogs, panels,
popovers, toasts, markdown, the lightbox, file drops, tables, load states,
run state, ticket marks, question cards, ticket links and asides. Each
look or rule has one owner here: `Table` the one table look (§6),
`NeedsYou` the one attention plate and the only user of the attention
colour (§7), `TailText` the one tail-visible path or id and `useKeepFocus`
the one way a list keeps keyboard focus when items move or go (§8),
`FilterChip` the one filter chip. Components take typed props and
callbacks, use token-backed utilities only, and carry their own
accessibility semantics.

Does not own API calls, lifecycle or session state, data loading or feature
behaviour, and never imports from `features/` or `app/`. Depends only on
`styles/` and pure helpers in `model/`.
