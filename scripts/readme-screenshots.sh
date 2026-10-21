#!/bin/sh
# Regenerate the README's screenshots in docs/images (FH-180). Builds the
# binary (--no-build reuses build/flashheart), then runs the dedicated
# Playwright config in frontend/e2e/readme/: it writes the invented showcase
# board into a temporary root, records live sessions through the real hooks
# and MCP server, serves the board, captures each view and stops only the
# server it started. scripts/check.sh and scripts/e2e.sh never run it.
# Other arguments go to Playwright. The PNGs are then optimised with
# pngquant when it is installed. Look at every image before committing it.
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

if [ "${1:-}" = "--no-build" ]; then
  shift
  scripts/frontend-deps.sh
  if [ ! -x build/flashheart ]; then
    echo "build/flashheart is missing: run without --no-build" >&2
    exit 1
  fi
else
  scripts/build.sh
fi

images="$project_root/docs/images"
export PLAYWRIGHT_BROWSERS_PATH="$project_root/.cache/ms-playwright"
export FLASHHEART_README_IMAGES="$images"
npm --cache "$project_root/.cache/npm" --prefix frontend exec -- playwright install chromium
npm --cache "$project_root/.cache/npm" --prefix frontend exec -- \
  playwright test --config "$project_root/frontend/e2e/readme/playwright.config.mjs" "$@"

if command -v pngquant >/dev/null 2>&1; then
  for image in "$images"/*.png; do
    # Exit 98 or 99 (no smaller result at this quality) leaves the file.
    pngquant --quality 70-95 --speed 1 --strip --skip-if-larger --force \
      --output "$image" -- "$image" || true
  done
else
  echo "pngquant is not installed: the images are not optimised" >&2
fi
ls -l "$images"
