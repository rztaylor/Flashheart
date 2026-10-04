# Lifecycle boundary

Owns the page's single Singleserve session: `connect` from
`/_singleserve/client.js` (called once, from the app root), heartbeat and
backend-loss handling, manual health checks, guarded Quit with denial
handling, the terminal transition (stop the session, attempt
`window.close()`), and the authenticated fetch handed to `api/`.

Never parses launch URLs, stores credentials, uses Web Storage, sets
`X-Singleserve-Token`, or calls `/_singleserve/` routes itself. Presentation
of lifecycle state belongs to `app/`.
