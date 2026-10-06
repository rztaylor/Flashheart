# Components boundary

Owns shared, product-neutral UI primitives in the transit-map vocabulary:
`Button`, `Icon` (authored set), `LineBullet`, `StateNote`, `Tabs`,
`SidePanel`, `SegmentedControl`, fields and form fields, `Dialog`, `Toast`,
`Markdown`, `RouteBar`, `EmptyState`, `RunState` (run state as ink and shape),
`PlanRoute` (an agent's plan as a monochrome route) and `QuestionCard` (an
agent's question with its answer box). Components take typed props and callbacks, use token-backed
utilities only, and carry their own accessibility semantics.

Does not own API calls, lifecycle or session state, data loading or feature
behaviour, and never imports from `features/` or `app/`. Depends only on
`styles/` and pure helpers in `model/`.
