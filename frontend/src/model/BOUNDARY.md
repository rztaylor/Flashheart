# Model boundary

Owns pure view-model logic shared across features: workstream line
assignment and the one workstream title lookup (`lines`), filters and
search with include and exclude choices, chip toggling, workstream chip order and one-line fitting (`filters`,
VIEW-7, FH-39), running times and time order (`time`), markdown
link resolution, ticket-id linking (KEY-3), section trimming and splitting (CARD-2),
card paint for "Colour by" (`paint`, VIEW-6), ticket status tones and
priority wording (`status`), the page header's summary line (`summary`),
the words for why a run needs you
(the server derives the reason, D39) and the session part of the server's
short run id, timeline wording (`runs`), the Overview's sections, their
order, at-risk reasons and wording, and its pipeline stages (`overview`,
VIEW-3), keyboard grid movement and the neighbour that takes focus when the
focused item leaves its list (`navigation`, FH-84), and
manual-order placements and display sorts (`order`, EDIT-9), archive
search and the delete gate (`archive`, EDIT-8), a ticket's sections in
order with their counts and the files the full page lists (`ticketSections`, CARD-1, CARD-7), its activity timeline, history words and filters (`activity`), what a comment quotes and its entry in the file (`comments`), and the marginal remarks
catalogue (`remarks.md`), placements, selection and which moves to Done
earn one (`remarks`), a ticket's typed fields as form text and the
changes to save (`ticketForm`, EDIT-6), a section's text as edited in
place and the criteria list (`sectionEdit`, EDIT-11), and a failure's words for any thrown value
(`errors`: `messageOf`).
Everything here is a plain function or type, unit-tested with Vitest.

Does not own React components, data loading, requests or styling. It may
import API types; nothing here imports components, features or the app.
