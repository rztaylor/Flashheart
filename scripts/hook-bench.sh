#!/bin/sh
# Measure hook latency: p95 must stay under 50 ms over 1,000 warm
# invocations (HOOK-1). Developer tool; not run in CI.
# Usage: scripts/hook-bench.sh [-n N] [-p95 DURATION]
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"
mkdir -p .cache/go-build build
GOCACHE="$project_root/.cache/go-build" go build -trimpath -o build/flashheart-bench ./cmd/flashheart
GOCACHE="$project_root/.cache/go-build" go run ./scripts/hookbench -binary build/flashheart-bench "$@"
