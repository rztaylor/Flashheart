#!/bin/sh
# Make frontend/node_modules match frontend/package-lock.json. The other
# scripts call this before using the frontend packages.
#
# After a successful `npm ci` it writes a stamp, frontend/node_modules/
# .flashheart-install, holding the lockfile's checksum and the Node major
# version. It runs `npm ci` again only when the stamp is missing or differs:
# after the lockfile changes (a pull that adds or upgrades packages), after a
# Node major upgrade, or after an install that failed or predates the stamp.
# The comparison uses content, not file times, so a branch switch that leaves
# the lockfile unchanged does not reinstall. An up-to-date install prints
# nothing.
#
#   scripts/frontend-deps.sh            install only when out of date
#   scripts/frontend-deps.sh --always   always run npm ci (scripts/check.sh)
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

always=false
case "${1:-}" in
  '') ;;
  --always) always=true ;;
  *)
    echo "usage: scripts/frontend-deps.sh [--always]" >&2
    exit 2
    ;;
esac

stamp=frontend/node_modules/.flashheart-install
node_major=$(node --version | sed 's/^v\([0-9]*\).*/\1/')
want="package-lock.json cksum $(cksum < frontend/package-lock.json); node $node_major"

if [ "$always" = false ] && [ -f "$stamp" ] && [ "$(cat "$stamp")" = "$want" ]; then
  exit 0
fi
if [ "$always" = false ]; then
  echo "frontend packages are missing or out of date with package-lock.json; running npm ci" >&2
fi

# Remove the old stamp first so a failed install is retried next time.
rm -f "$stamp"
npm --cache "$project_root/.cache/npm" --prefix frontend ci
printf '%s\n' "$want" > "$stamp"
