# Model boundary

Owns pure view-model logic shared across features: workstream line
assignment (`lines`), filters and search (VIEW-7), running times, markdown
link resolution, ticket-id linking (KEY-3) and section trimming (CARD-2),
card paint for "Colour by" (`paint`, VIEW-6), ticket status tones and
priority wording (`status`), the page header's summary line (`summary`),
agent-run lanes, timeline
wording and plan stations (`runs`, VIEW-3), keyboard grid movement, and
manual-order placements and display sorts (`order`, EDIT-9), and archive
search and the delete gate (`archive`, EDIT-8).
Everything here is a plain function or type, unit-tested with Vitest.

Does not own React components, data loading, requests or styling. It may
import API types; nothing here imports components, features or the app.
