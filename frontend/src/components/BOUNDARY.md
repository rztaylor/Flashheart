# Components boundary

Owns shared, product-neutral UI primitives and patterns (currently `Button`;
later IconButton, Badge, Menu, Dialog, Panel, Tabs, Markdown, EmptyState).
Components take typed props and callbacks, use token-backed utilities only,
and carry their own accessibility semantics.

Does not own API calls, lifecycle or session state, board data or feature
behaviour, and never imports from `features/` or `app/`. Depends only on
`styles/`.
