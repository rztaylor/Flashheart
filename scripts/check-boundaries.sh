#!/bin/sh
# Check every boundary note against its size limit
# (.agents/facts/architecture.md): each frontend BOUNDARY.md and each Go
# package's doc.go holds at most 150 and 250 words respectively, and at
# most 2 KiB (2048 bytes) of file. A word is a whitespace-separated token
# with a letter or digit in it, so markdown bullets and the `//` of a Go
# comment do not count, nor does doc.go's `package` line.
#
#   scripts/check-boundaries.sh          check, listing every note over
#   scripts/check-boundaries.sh --list   print each note's words and bytes
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

max_bytes=2048
boundary_words=150
docgo_words=250

list=false
if [ "${1:-}" = "--list" ]; then
  list=true
fi

words() {
  awk '
    /^package [[:alnum:]_]+$/ { next }
    {
      sub(/^[[:space:]]*\/\//, "")
      for (i = 1; i <= NF; i++) if ($i ~ /[[:alnum:]]/) n++
    }
    END { print n + 0 }
  ' "$1"
}

failed=0
check() {
  file=$1
  limit=$2
  w=$(words "$file")
  b=$(wc -c < "$file" | tr -d ' ')
  if $list; then
    printf '%5d words %5d bytes  %s\n' "$w" "$b" "$file"
  fi
  if [ "$w" -gt "$limit" ] || [ "$b" -gt "$max_bytes" ]; then
    printf '%s: %d words (limit %d), %d bytes (limit %d)\n' \
      "$file" "$w" "$limit" "$b" "$max_bytes" >&2
    failed=1
  fi
}

for file in $(find frontend/src -name BOUNDARY.md -not -path '*/node_modules/*' | sort); do
  check "$file" "$boundary_words"
done
for file in $(find cmd internal scripts -name doc.go | sort); do
  check "$file" "$docgo_words"
done

if [ "$failed" -ne 0 ]; then
  echo "Boundary notes over their limit (.agents/facts/architecture.md); trim them." >&2
  exit 1
fi
