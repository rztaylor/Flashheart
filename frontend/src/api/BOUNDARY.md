# API boundary

Owns typed requests to Flashheart's local `/api/` endpoints and runtime
validation of their responses, including mapping JSON errors to `ApiError`
with their payload: board reads (`board`), writes that carry the content
hash and the long-poll for changes (`edit`), agent runs and a run's
timeline (`runs`), the Overview's headline metrics (`metrics`), saved UI
preferences (`preferences`), and archived
tickets and projects with their delete previews (`archive`). Callers pass the authenticated fetch from `lifecycle/`;
this layer never issues raw `fetch`, acquires credentials or stores
anything.

Does not own React state, error presentation, backend behaviour or caching.
Features and the app shell consume these contracts.
