# Model boundary

Owns pure view-model logic shared across features: workstream line
assignment (`lines`), filters and search with include and exclude choices,
chip toggling, workstream chip order and one-line fitting (`filters`,
VIEW-7, FH-39), running times, markdown
link resolution, ticket-id linking (KEY-3) and section trimming (CARD-2),
card paint for "Colour by" (`paint`, VIEW-6), ticket status tones and
priority wording (`status`), the page header's summary line (`summary`),
why a run needs you, timeline
wording and plan stations (`runs`), the Overview's sections, their
order, at-risk reasons and wording (`overview`, VIEW-3), keyboard grid movement, and
manual-order placements and display sorts (`order`, EDIT-9), archive
search and the delete gate (`archive`, EDIT-8), and the marginal remarks
catalogue (`remarks.md`), placements and selection (`remarks`).
Everything here is a plain function or type, unit-tested with Vitest.

Does not own React components, data loading, requests or styling. It may
import API types; nothing here imports components, features or the app.
