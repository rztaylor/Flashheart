# Node and frontend-tooling facts

- Build-time only: Node.js 22 or newer; running Flashheart needs no Node.
- Package root and lockfile: `frontend/package.json` and
  `frontend/package-lock.json`; use `npm ci` for deterministic installs.
- TypeScript strict. Biome for lint and format (typescript-eslint does not
  support the TypeScript 7 toolchain used across these projects).
- Frontend validation: Biome, Vitest, TypeScript + Vite production build, and
  Playwright for lifecycle and visual checks.
- Dependencies: exact versions, pinned in `package-lock.json`, at least two
  weeks old when added; test-only packages are dev dependencies. Each new
  package is recorded with its alternatives in `docs/dev/decisions.md`
  (D10 the libraries, D42 happy-dom and Testing Library).
