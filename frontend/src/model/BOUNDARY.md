# Model boundary

Owns pure view-model logic shared across features: workstream line
assignment, the one workstream title lookup and the server's rule for a
taken workstream title (`lines`, D76), filters and
search with include and exclude choices, chip toggling, workstream chip order and one-line fitting (`filters`,
VIEW-7, FH-39), running times and time order (`time`), markdown
link resolution, ticket-id linking (KEY-3), section trimming and splitting (CARD-2),
card paint for "Colour by" (`paint`, VIEW-6), ticket status tones and
priority wording (`status`), the page header's summary line (`summary`),
the scope the board shows, all projects or one, with its remembered view
and filters (`scopes`),
the words for why a run needs you
(the server derives the reason, D39) and the session part of the server's
short run id, timeline wording (`runs`), the Overview's sections, their
order, at-risk reasons and wording, its pipeline stages and its Projects
list's rows and their names (`overview`, VIEW-3, FH-153), keyboard grid movement and the neighbour that takes focus when the
focused item leaves its list (`navigation`, FH-84), and
manual-order placements and display sorts (`order`, EDIT-9), archive
search and the delete gate (`archive`, EDIT-8), a ticket's sections in
order with their counts and the files the full page lists (`ticketSections`, CARD-1, CARD-7), its activity timeline, history words and filters (`activity`), what a comment quotes, its entry in the file and why it cannot be deleted (`comments`, FH-140), change requests, their counts and words (`requests`, REV-6), the sessions a comment can be told to and its delivery in words (`tell`, FH-127), and the marginal remarks
catalogue (`remarks.md`), placements, selection and which moves to Done
earn one (`remarks`), a ticket's typed fields as form text and the
changes to save (`ticketForm`, EDIT-6), a section's text as edited in
place and the criteria list (`sectionEdit`, EDIT-11), files given to a
ticket (`uploads`, REV-7), the lightbox's zoom and pan (`zoom`: Fit, the
25–800% limits and steps, zooming around a point, pan bounds; FH-146), and a failure's words for any thrown value
(`errors`: `messageOf`).
Everything here is a plain function or type, unit-tested with Vitest.

Does not own React components, data loading, requests or styling. It may
import API types; nothing here imports components, features or the app.
