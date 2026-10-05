#!/usr/bin/env bash
# Record real Claude Code hook payloads for testdata/hooks/claude/.
#
# Creates a scratch git repository and a settings file whose hooks copy each
# payload to <out>/<Event>/NNN.json, then starts an interactive Claude Code
# session there. Your own ~/.claude configuration is not changed (the
# hooks are added for this session only, with --settings).
#
# Usage: scripts/record-claude-hooks.sh [OUT_DIR]   (default .cache/hook-recordings)
# Set CLAUDE to the claude binary if it is not on PATH.
#
# Payloads contain your paths and prompts: scrub them as described in
# testdata/hooks/claude/MANIFEST.md before committing any.
set -euo pipefail

out="${1:-.cache/hook-recordings}"
claude_bin="${CLAUDE:-claude}"
if ! command -v "$claude_bin" >/dev/null 2>&1; then
  echo "record-claude-hooks: cannot find '$claude_bin'; set CLAUDE=/path/to/claude" >&2
  exit 1
fi

mkdir -p "$out"
out="$(cd "$out" && pwd)"
work="$(mktemp -d "${TMPDIR:-/tmp}/flashheart-record.XXXXXX")"
repo="$work/demo"
mkdir -p "$repo"
git -C "$repo" init -q -b feature/demo
printf '# demo\n' >"$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" -c user.name=recorder -c user.email=recorder@example.com commit -q -m init

recorder="$work/record.sh"
cat >"$recorder" <<'EOF'
#!/bin/sh
dir="$RECORD_OUT/$1"
mkdir -p "$dir"
n=$(ls "$dir" | wc -l | tr -d ' ')
cat >"$dir/$(printf %03d "$n").json"
exit 0
EOF
chmod +x "$recorder"

events="SessionStart UserPromptSubmit PreToolUse PostToolUse PostToolUseFailure PermissionRequest PermissionDenied Notification TaskCreated TaskCompleted SubagentStart SubagentStop PreCompact PostCompact Stop SessionEnd"
settings="$work/settings.json"
{
  printf '{"hooks":{'
  sep=""
  for event in $events; do
    printf '%s"%s":[{"hooks":[{"type":"command","command":"RECORD_OUT=%s %s %s","timeout":5}]}]' \
      "$sep" "$event" "$out" "$recorder" "$event"
    sep=","
  done
  printf '}}\n'
} >"$settings"

cat <<EOF
Recording hook payloads into $out
Scratch repository: $repo

In the session, please:
  1. ask for a task list of three items, then work through them;
  2. ask it to write a file and edit README.md;
  3. ask it to run 'rm -rf build' and answer the permission prompt (once allow, once deny);
  4. ask it to read a missing file;
  5. ask it to use a subagent to list the files;
  6. wait at the prompt for a minute (idle notification), then run /compact;
  7. exit with /exit.
EOF

cd "$repo"
"$claude_bin" --settings "$settings"
echo "Recorded:"
find "$out" -type f | sort
