# App boundary

Owns the application shell and composition: the header with backend status
and Quit, the shutdown-denial alert, the terminal stopped screen, theme
application to `<html data-theme>`, and wiring lifecycle, API and (later)
features together.

Does not own the Singleserve session (`lifecycle/`), request contracts
(`api/`), shared primitives (`components/`) or feature behaviour
(`features/`). It is the only layer allowed to import features.
