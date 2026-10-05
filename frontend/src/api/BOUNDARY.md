# API boundary

Owns typed requests to Flashheart's local `/api/` endpoints and runtime
validation of their responses, including mapping JSON errors to `ApiError`
with their payload: board reads (`board`), writes that carry the content
hash and the long-poll for changes (`edit`), and saved UI preferences
(`preferences`). Callers pass the authenticated fetch from `lifecycle/`;
this layer never issues raw `fetch`, acquires credentials or stores
anything.

Does not own React state, error presentation, backend behaviour or caching.
Features and the app shell consume these contracts.
