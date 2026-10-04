#!/bin/sh
# Build the frontend and the flashheart binary into build/. Release builds set
# FLASHHEART_VERSION, FLASHHEART_COMMIT and FLASHHEART_BUILD_DATE.
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

version=${FLASHHEART_VERSION:-dev}
commit=${FLASHHEART_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}
build_date=${FLASHHEART_BUILD_DATE:-unknown}
buildinfo=github.com/rztaylor/flashheart/internal/buildinfo

if [ ! -d frontend/node_modules ]; then
  npm --cache "$project_root/.cache/npm" --prefix frontend ci
fi
npm --cache "$project_root/.cache/npm" --prefix frontend run build

mkdir -p "$project_root/.cache/go-build" "$project_root/build"
GOCACHE="$project_root/.cache/go-build" go build -trimpath \
  -ldflags "-X $buildinfo.Version=$version -X $buildinfo.Commit=$commit -X $buildinfo.BuildDate=$build_date" \
  -o "$project_root/build/flashheart" ./cmd/flashheart
