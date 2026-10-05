# Contributing to Flashheart

Flashheart is pre-release and changes quickly. Issues and pull requests are
welcome; for anything larger than a fix, open an issue first so the change
can be placed on the roadmap.

## Before you start

- Read [`docs/SPEC.md`](docs/SPEC.md) (authoritative) and
  [`AGENTS.md`](AGENTS.md). The board file format and the agent protocol are
  contracts: change them deliberately, versioned, with tests.
- Work follows [`docs/dev/roadmap.md`](docs/dev/roadmap.md); each item's brief
  holds its scope and acceptance criteria.

## Workflow

1. Branch from `main` as `feature/<short-description>`. Never commit to
   `main` directly; every change lands through a pull request.
2. Write a failing test first for features and bugs, then make it pass.
3. Keep commits focused. Update the spec, facts, roadmap brief and
   `CHANGELOG.md` in the same change when their contracts change.
4. Run the checks below, then open a pull request using the template. CI runs
   the same checks on Linux and macOS.

## Checks

Requires Go 1.26.6+ and Node.js 22+.

```sh
scripts/check.sh    # frontend lint, unit tests and build; Go format, vet, tests, -race, build
scripts/e2e.sh      # Playwright lifecycle, accessibility and screenshots (visible UI changes)
```

Tests never touch a real board root or agent configuration; use temporary
directories and copies of `testdata/boards/sample`.

## Security

Report vulnerabilities privately as described in [`SECURITY.md`](SECURITY.md).
