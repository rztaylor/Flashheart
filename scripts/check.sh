#!/bin/sh
# Full local validation. Until the foundation roadmap item lands, halves whose
# project files do not exist yet are skipped; foundation makes this strict.
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

ran=0

if [ -f frontend/package.json ]; then
  ran=1
  npm --cache "$project_root/.cache/npm" --prefix frontend ci
  npm --cache "$project_root/.cache/npm" --prefix frontend run lint
  npm --cache "$project_root/.cache/npm" --prefix frontend test
  npm --cache "$project_root/.cache/npm" --prefix frontend run build
else
  echo "check: no frontend/package.json yet; skipping frontend checks" >&2
fi

if [ -f go.mod ]; then
  ran=1
  unformatted=$(gofmt -l cmd internal 2>/dev/null || true)
  if [ -n "$unformatted" ]; then
    echo "Go files need formatting:" >&2
    echo "$unformatted" >&2
    exit 1
  fi
  mkdir -p "$project_root/.cache/go-build" "$project_root/build"
  GOCACHE="$project_root/.cache/go-build" go vet ./...
  GOCACHE="$project_root/.cache/go-build" go test ./...
  GOCACHE="$project_root/.cache/go-build" go test -race ./...
  GOCACHE="$project_root/.cache/go-build" go build -o "$project_root/build/flashheart" ./cmd/flashheart
else
  echo "check: no go.mod yet; skipping Go checks" >&2
fi

if [ "$ran" -eq 0 ]; then
  echo "check: nothing to validate yet (pre-foundation)" >&2
fi
