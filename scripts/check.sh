#!/bin/sh
# Full local validation: frontend lint, unit tests and build, then Go format,
# vet, tests (plain and -race, then the indexing timing test alone) and the
# binary build. The browser suite runs
# separately with scripts/e2e.sh.
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

npm --cache "$project_root/.cache/npm" --prefix frontend ci
npm --cache "$project_root/.cache/npm" --prefix frontend run lint
npm --cache "$project_root/.cache/npm" --prefix frontend test
npm --cache "$project_root/.cache/npm" --prefix frontend run build

unformatted=$(gofmt -l cmd internal)
if [ -n "$unformatted" ]; then
  echo "Go files need formatting:" >&2
  echo "$unformatted" >&2
  exit 1
fi

mkdir -p "$project_root/.cache/go-build" "$project_root/build"
GOCACHE="$project_root/.cache/go-build" go vet ./...
# The NFR-1 timing test runs on its own after the parallel passes: sharing a
# small CI runner with every other package's tests makes its time noise.
timing='^TestIndexesFiveThousandTicketsQuickly$'
GOCACHE="$project_root/.cache/go-build" go test -skip "$timing" ./...
GOCACHE="$project_root/.cache/go-build" go test -race -skip "$timing" ./...
GOCACHE="$project_root/.cache/go-build" go test -count=1 -v -run "$timing" ./internal/index
GOCACHE="$project_root/.cache/go-build" go build -o "$project_root/build/flashheart" ./cmd/flashheart
