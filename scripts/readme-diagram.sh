#!/bin/sh
# Render the README's "How it works" diagram (FH-188) from
# docs/images/src/how-it-works.svg into docs/images/how-it-works-light.png
# and -dark.png with Playwright's Chromium. The README shows the one that
# matches the reader's GitHub theme. Look at both before committing them.
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

scripts/frontend-deps.sh
export PLAYWRIGHT_BROWSERS_PATH="$project_root/.cache/ms-playwright"
npm --cache "$project_root/.cache/npm" --prefix frontend exec -- playwright install chromium
node frontend/e2e/readme/diagram.mjs docs/images/src/how-it-works.svg docs/images

if command -v pngquant >/dev/null 2>&1; then
  for image in docs/images/how-it-works-*.png; do
    pngquant --quality 70-95 --speed 1 --strip --skip-if-larger --force \
      --output "$image" -- "$image" || true
  done
fi
