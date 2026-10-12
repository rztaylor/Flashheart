#!/bin/sh
# Compare the stable screenshot set (frontend/e2e/visual.spec.mjs) with its
# committed baselines in the Playwright container that matches the pinned
# @playwright/test, so fonts and rendering are the same on every machine and
# in CI (D42). Needs Docker; pulls the image on first use. Arguments go to
# Playwright: after an intended visual change, rewrite the baselines with
#   scripts/e2e-visual.sh --update-snapshots
# and review the changed PNGs before committing them. Diffs of a failed
# comparison land in .cache/playwright-results.
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

scripts/frontend-deps.sh
version=$(node -p 'require("./frontend/node_modules/@playwright/test/package.json").version')
image="mcr.microsoft.com/playwright:v$version-noble"

# The container runs a Linux build on amd64, CI's architecture, emulated on
# other machines so every run renders alike.
npm --cache "$project_root/.cache/npm" --prefix frontend run build
mkdir -p "$project_root/.cache/go-build" "$project_root/build/linux-amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOCACHE="$project_root/.cache/go-build" \
  go build -trimpath -o "$project_root/build/linux-amd64/flashheart" ./cmd/flashheart

docker run --rm --platform linux/amd64 --ipc=host \
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp \
  -e FLASHHEART_VISUAL=1 \
  -e FLASHHEART_E2E_BINARY=/work/build/linux-amd64/flashheart \
  -e PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
  -v "$project_root:/work" \
  -w /work/frontend \
  "$image" \
  node node_modules/@playwright/test/cli.js test visual.spec.mjs "$@"
