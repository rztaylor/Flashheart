#!/usr/bin/env bash
# Record real Codex hook payloads for testdata/hooks/codex/.
#
# Creates a scratch git repository, a scratch board and a recorder that copies
# each hook payload to <out>/<Event>/<time>-<pid>.json, with the CODEX_*
# environment the hook saw beside it as <time>-<pid>.env. The flashheart MCP
# server is started through a wrapper that copies what Codex sends it to
# <out>/mcp/<time>-<pid>.jsonl (and its environment), so recordings show
# whether hooks and the MCP server share a session identifier.
#
# Your own ~/.codex is not changed: the session runs with a temporary
# CODEX_HOME that links only your login (auth.json, unlinked again on exit),
# so its config, trust entries and session files stay in the scratch
# directory. The hooks and the MCP server are added with -c, and
# --dangerously-bypass-hook-trust runs the recorder hooks without a trust
# review. The MCP server writes to the scratch board, never your real one.
# Recordings of the ChatGPT app cannot use this: see the manifest.
#
# Usage: scripts/record-codex-hooks.sh [exec|tui] [OUT_DIR] [CODEX_OPTION...]
#   exec  (default) runs a scripted non-interactive session (codex exec);
#   tui   starts an interactive session for the events exec cannot reach
#         (permission prompts, /compact, interrupting a turn).
# OUT_DIR defaults to .cache/hook-recordings/codex. Set CODEX to the codex
# binary if it is not on PATH, and FLASHHEART to the flashheart binary
# (default: build/flashheart, else flashheart on PATH). Further options go
# to codex, such as -c model_auto_compact_token_limit=20000 to reach
# compaction in exec.
#
# Payloads contain your paths and prompts: scrub them as described in
# testdata/hooks/codex/MANIFEST.md before committing any.
set -euo pipefail

mode="${1:-exec}"
out="${2:-.cache/hook-recordings/codex}"
shift $(($# < 2 ? $# : 2))
case "$mode" in
  exec|tui) ;;
  *) echo "usage: scripts/record-codex-hooks.sh [exec|tui] [OUT_DIR] [CODEX_OPTION...]" >&2; exit 2 ;;
esac

codex_bin="${CODEX:-codex}"
if [ -z "${CODEX:-}" ] && ! command -v codex >/dev/null 2>&1; then
  # Fall back to the Codex CLI bundled with the ChatGPT desktop app.
  bundled=/Applications/ChatGPT.app/Contents/Resources/codex-cli/bin/codex
  if [ -x "$bundled" ]; then
    codex_bin="$bundled"
  fi
fi
if ! command -v "$codex_bin" >/dev/null 2>&1; then
  echo "record-codex-hooks: cannot find '$codex_bin'; set CODEX=/path/to/codex" >&2
  exit 1
fi
flashheart_bin="${FLASHHEART:-}"
if [ -z "$flashheart_bin" ]; then
  if [ -x build/flashheart ]; then
    flashheart_bin="$PWD/build/flashheart"
  else
    flashheart_bin="$(command -v flashheart || true)"
  fi
fi
if [ -z "$flashheart_bin" ] || [ ! -x "$flashheart_bin" ]; then
  echo "record-codex-hooks: cannot find flashheart; run scripts/build.sh or set FLASHHEART" >&2
  exit 1
fi

mkdir -p "$out"
out="$(cd "$out" && pwd)"
work="$(mktemp -d "${TMPDIR:-/tmp}/flashheart-record-codex.XXXXXX")"
repo="$work/demo"
board="$work/board"
mkdir -p "$repo" "$board"
git -C "$repo" init -q -b feature/demo
printf '# demo\n' >"$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" -c user.name=recorder -c user.email=recorder@example.com commit -q -m init

recorder="$work/record.sh"
cat >"$recorder" <<'EOF'
#!/bin/sh
dir="$RECORD_OUT/$1"
mkdir -p "$dir"
# Unique per invocation, so parallel hooks never overwrite each other; names
# sort by time.
name="$(date +%Y%m%dT%H%M%S)-$$"
cat >"$dir/$name.json"
env | grep '^CODEX' | sort >"$dir/$name.env" || true
# For flashheart tools, PreToolUse rewrites the input to add a run id (does
# it reach the MCP server?) and PermissionRequest allows the call, which
# codex exec would otherwise refuse.
case "$1" in
  PreToolUse|PermissionRequest)
    python3 -I - "$1" "$dir/$name.json" <<'PY'
import json, sys
event, path = sys.argv[1], sys.argv[2]
payload = json.load(open(path))
if not payload.get("tool_name", "").startswith("mcp__flashheart__"):
    sys.exit(0)
if event == "PreToolUse":
    stamped = dict(payload.get("tool_input") or {}, run="codex:recorded-stamp")
    out = {"permissionDecision": "allow", "updatedInput": stamped}
else:
    out = {"decision": {"behavior": "allow"}}
out["hookEventName"] = event
print(json.dumps({"hookSpecificOutput": out}))
PY
    ;;
esac
exit 0
EOF
chmod +x "$recorder"

mcp="$work/mcp.sh"
cat >"$mcp" <<EOF
#!/bin/sh
dir='$out/mcp'
mkdir -p "\$dir"
name="\$(date +%Y%m%dT%H%M%S)-\$\$"
env | grep '^CODEX' | sort >"\$dir/\$name.env" || true
pwd >"\$dir/\$name.cwd"
tee "\$dir/\$name.jsonl" | '$flashheart_bin' --root '$board' mcp
EOF
chmod +x "$mcp"

events="SessionStart SessionEnd UserPromptSubmit PreToolUse PermissionRequest PostToolUse PreCompact PostCompact SubagentStart SubagentStop Stop Interrupt"
overrides=()
for event in $events; do
  # Paths are single-quoted for the shell that runs the hook, so spaces in
  # them are safe.
  overrides+=(-c "hooks.$event=[{hooks=[{type=\"command\",command=\"RECORD_OUT='$out' '$recorder' $event\",timeout=5}]}]")
done
overrides+=(-c "mcp_servers.flashheart.command=\"$mcp\"")

# Codex saves project trust to $CODEX_HOME/config.toml even with
# --ignore-user-config or a -c override, so give it a scratch home.
codex_home="$work/codex-home"
mkdir -p "$codex_home"
real_home="${CODEX_HOME:-$HOME/.codex}"
if [ ! -f "$real_home/auth.json" ]; then
  echo "record-codex-hooks: no login in $real_home/auth.json; run 'codex login' first" >&2
  exit 1
fi
ln -s "$real_home/auth.json" "$codex_home/auth.json"
trap 'rm -f "$codex_home/auth.json"' EXIT
export CODEX_HOME="$codex_home"

echo "Recording Codex hook payloads into $out"
echo "Scratch repository: $repo"
echo "Scratch board: $board"
cd "$repo"
if [ "$mode" = exec ]; then
  prompt='This is a scratch repository for recording tool use. Do each step, briefly:
1. Make a plan of three steps with your planning tool, and mark each done as you go.
2. Create notes.txt containing "hello", then change README.md to say "# demo app". Use your file editing tool.
3. Run the shell command: ls -la
4. Read the file missing.txt (it does not exist; report the error).
5. Call the flashheart board_context tool once.
6. If you can start a subagent, ask one to list the files in this folder; otherwise say you cannot.
Then stop.'
  "$codex_bin" exec --dangerously-bypass-hook-trust \
    -s workspace-write "${overrides[@]}" "$@" "$prompt" </dev/null
else
  cat <<EOF

In the session, please:
  1. ask it to run 'rm -rf build' and answer the permission prompt (once allow, once deny);
  2. ask it to write a file and edit README.md;
  3. ask it to use a subagent to list the files;
  4. ask for something long, and press Esc to interrupt it;
  5. run /compact;
  6. exit with /quit.
EOF
  "$codex_bin" --dangerously-bypass-hook-trust \
    -a on-request -s read-only "${overrides[@]}" "$@"
fi
echo "Recorded:"
find "$out" -type f | sort
