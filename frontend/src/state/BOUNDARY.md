# State boundary

Owns how board data is loaded and kept fresh in the browser: `useResource`
loads through the authenticated fetch, keeps the last good value while
refreshing, and re-reads visible views on an interval. Long-polling on the
board revision (LIFE-3) replaces the interval with board-editing.

Does not own request shapes (`api/`), the Singleserve session
(`lifecycle/`), view models (`model/`) or presentation. Never uses Web
Storage.
