# State boundary

Owns how board data is loaded and kept fresh in the browser: `useRevision`
long-polls the board revision (LIFE-3) and `useResource` loads through the
authenticated fetch, keeps the last good value while reloading, and reloads
when the revision moves or the page becomes visible. `usePreferences` loads
and saves UI preferences through the backend (CFG-2).

Does not own request shapes (`api/`), the Singleserve session
(`lifecycle/`), view models (`model/`) or presentation. Never uses Web
Storage.
