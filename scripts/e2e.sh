#!/bin/sh
# Build the binary and run the Playwright browser suite against it. Installs
# the pinned Chromium into .cache/ms-playwright on first use (downloads once).
# Screenshots land in .cache/playwright-screenshots.
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

scripts/build.sh
export PLAYWRIGHT_BROWSERS_PATH="$project_root/.cache/ms-playwright"
npm --cache "$project_root/.cache/npm" --prefix frontend exec -- playwright install chromium
npm --cache "$project_root/.cache/npm" --prefix frontend run test:e2e -- "$@"
