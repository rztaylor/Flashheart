#!/bin/sh
# Build the release archives into build/dist, as the release workflow does
# (D86): the frontend once, then flashheart cross-compiled without cgo for
# darwin/arm64, darwin/amd64, linux/amd64 and linux/arm64 (or the targets
# named), each packaged as flashheart_<version>_<os>_<arch>.tar.gz with
# LICENSE and README.md, and one SHA256SUMS over them. Archives and sums
# from an earlier run in the output directory are replaced.
#
#   scripts/package.sh [--skip-frontend] [-out DIR] [os/arch ...]
#
# FLASHHEART_VERSION (default dev; X.Y.Z for a release), FLASHHEART_COMMIT
# (default the short HEAD) and FLASHHEART_BUILD_DATE (default now, UTC) are
# stamped into the binary and name the archives. --skip-frontend reuses the
# frontend already built into internal/webui/assets/generated.
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

build_frontend=true
if [ "${1:-}" = "--skip-frontend" ]; then
  build_frontend=false
  shift
fi

version=${FLASHHEART_VERSION:-dev}
commit=${FLASHHEART_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}
build_date=${FLASHHEART_BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}

if $build_frontend; then
  scripts/frontend-deps.sh
  npm --cache "$project_root/.cache/npm" --prefix frontend run build
fi

mkdir -p "$project_root/.cache/go-build"
GOCACHE="$project_root/.cache/go-build" go run ./scripts/release package \
  -version "$version" -commit "$commit" -date "$build_date" \
  -root "$project_root" "$@"
