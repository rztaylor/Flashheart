# API boundary

Owns typed requests to Flashheart's local `/api/` endpoints and runtime
validation of their responses, including mapping JSON errors to `ApiError`
with their payload: board reads (`board`), writes that carry the content
hash, people's comments, and the long-poll for changes (`edit`), agent runs
and a run's timeline (`runs`), files uploaded into a ticket and a
project's badge image (`files`),
saved UI preferences (`preferences`), the server's identity and settings
(`info`), and archived tickets and projects with their delete previews
(`archive`). Callers pass the authenticated fetch from `lifecycle/`; this
layer never issues raw `fetch`, acquires credentials or stores anything.

Each response has an exported guard checking every field the server
sends, tested over `testdata/api/`; the lists copied from the server
(`ERROR_CODES` among them) are tested against `testdata/vocabulary.json`
(FH-97).

Does not own React state, error presentation, backend behaviour or caching.
Features and the app shell consume these contracts.
