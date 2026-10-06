# Components boundary

Owns shared, product-neutral UI primitives on the semantic roles of
`docs/dev/specs/ui-layout.md`: `Button`, `Icon` (authored set), `LineBullet`,
`Pill` (status, blocker and attribute tags), `ColumnWell` (the board column
and Agents lane surface with its head), `KeyBadge`, `StateNote`, `Tabs`,
`SidePanel`, `SegmentedControl`, fields and form fields, `Dialog`, `Toast`,
`Markdown`, `RouteBar`, `EmptyState`, `RunState` (run state as a mark and
words, Needs you as the attention plate),
`PlanRoute` (an agent's plan as a monochrome route) and `QuestionCard` (an
agent's question with its answer box). Components take typed props and callbacks, use token-backed
utilities only, and carry their own accessibility semantics.

Does not own API calls, lifecycle or session state, data loading or feature
behaviour, and never imports from `features/` or `app/`. Depends only on
`styles/` and pure helpers in `model/`.
