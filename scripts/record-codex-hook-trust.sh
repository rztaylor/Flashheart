#!/usr/bin/env bash
# Record how Codex keys and hashes trusted hooks, for
# testdata/hooks/codex/trust/ (agent-protocol §5.4).
#
# Runs `flashheart setup codex` into a scratch CODEX_HOME that already holds
# a Stop hook of the user's, then drives `codex app-server` over stdio the
# way the CLI's /hooks does: hooks/list to see each hook's key, hash and
# trust status, config/batchWrite to trust Flashheart's hooks, hooks/list
# again, and once more after Flashheart's commands change (another --root).
# No model is called and no login is needed; your own ~/.codex is not read
# or changed.
#
# The flashheart binary and board root in the commands are replaced with
# placeholders before Codex hashes them, so the recorded hashes are Codex's
# own for scrubbed commands; the scratch CODEX_HOME in the keys and paths is
# replaced with /Users/example/.codex afterwards (keys are not hashed).
#
# Usage: scripts/record-codex-hook-trust.sh [OUT_DIR]
# OUT_DIR defaults to .cache/hook-recordings/codex-trust. Set CODEX to the
# codex binary if it is not on PATH, and FLASHHEART to the flashheart
# binary (default: build/flashheart, else flashheart on PATH).
set -euo pipefail

out="${1:-.cache/hook-recordings/codex-trust}"

codex_bin="${CODEX:-codex}"
if [ -z "${CODEX:-}" ] && ! command -v codex >/dev/null 2>&1; then
  # Fall back to the Codex CLI bundled with the ChatGPT desktop app.
  bundled=/Applications/ChatGPT.app/Contents/Resources/codex-cli/bin/codex
  if [ -x "$bundled" ]; then
    codex_bin="$bundled"
  fi
fi
if ! command -v "$codex_bin" >/dev/null 2>&1; then
  echo "record-codex-hook-trust: cannot find '$codex_bin'; set CODEX=/path/to/codex" >&2
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
  echo "record-codex-hook-trust: cannot find flashheart; run scripts/build.sh or set FLASHHEART" >&2
  exit 1
fi

mkdir -p "$out"
out="$(cd "$out" && pwd)"
work="$(mktemp -d "${TMPDIR:-/tmp}/flashheart-record-trust.XXXXXX")"
trap 'rm -rf "$work"' EXIT
codex_home="$work/codex-home"
board="$work/board"
mkdir -p "$codex_home" "$board"
# Codex keys hooks by its home's real path (symlinks resolved).
codex_home="$(cd "$codex_home" && pwd -P)"

# A hook of the user's comes first, so Flashheart's Stop hook is group 1.
cat >"$codex_home/hooks.json" <<'EOF'
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {"type": "command", "command": "afplay /System/Library/Sounds/Glass.aiff", "timeout": 3}
        ]
      }
    ]
  }
}
EOF
CODEX_HOME="$codex_home" "$flashheart_bin" setup codex --yes --root "$board" >/dev/null

echo "Recording Codex hook trust ($("$codex_bin" --version)) into $out"
CODEX_HOME="$codex_home" python3 -I - "$codex_bin" "$codex_home" "$flashheart_bin" "$board" "$work" "$out" <<'PY'
import json, os, subprocess, sys

codex, home, binary, board, cwd, out = sys.argv[1:]
BINARY, ROOT, OTHER_ROOT = "/Users/example/bin/flashheart", "/Users/example/reports/Kanban", "/Users/example/other-board"
HOME = "/Users/example/.codex"
hooks_path = os.path.join(home, "hooks.json")


def edit_commands(old, new):
    data = json.load(open(hooks_path))
    for groups in data["hooks"].values():
        for group in groups:
            for hook in group["hooks"]:
                hook["command"] = hook["command"].replace(old, new)
    with open(hooks_path, "w") as f:
        json.dump(data, f, indent=2)
        f.write("\n")


def scrub(text):
    return text.replace(home, HOME)


def save(name, text):
    with open(os.path.join(out, name), "w") as f:
        f.write(scrub(text))


edit_commands(binary, BINARY)
edit_commands(board, ROOT)

proc = subprocess.Popen([codex, "app-server"], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                        stderr=subprocess.DEVNULL, text=True, cwd=cwd)
ids = iter(range(1, 1000))


def call(method, params):
    rid = next(ids)
    proc.stdin.write(json.dumps({"id": rid, "method": method, "params": params}) + "\n")
    proc.stdin.flush()
    while True:
        line = proc.stdout.readline()
        if not line:
            sys.exit(f"record-codex-hook-trust: codex app-server exited during {method}")
        message = json.loads(line)
        if message.get("id") == rid:
            if "error" in message:
                sys.exit(f"record-codex-hook-trust: {method}: {message['error']}")
            return message["result"]


def hooks_list(name):
    hooks = call("hooks/list", {"cwds": [cwd]})["data"][0]["hooks"]
    keep = [{k: hook.get(k) for k in ("key", "eventName", "command", "timeoutSec", "currentHash", "trustStatus")}
            for hook in hooks]
    save(name, json.dumps(keep, indent=2) + "\n")
    for hook in keep:
        print(f"  {name}: {scrub(hook['key'])} {hook['trustStatus']}")
    return hooks


call("initialize", {"clientInfo": {"name": "flashheart-recorder", "version": "0"}})
proc.stdin.write(json.dumps({"method": "initialized"}) + "\n")
proc.stdin.flush()

save("hooks.json", open(hooks_path).read())
hooks = hooks_list("hooks-list-untrusted.json")
# As /hooks does (codex-rs tui/src/hooks_rpc.rs): one upsert of hooks.state.
trust = {hook["key"]: {"trusted_hash": hook["currentHash"]} for hook in hooks
         if " hook codex " in hook.get("command", "")}
call("config/batchWrite", {"edits": [{"keyPath": "hooks.state", "mergeStrategy": "upsert", "value": trust}]})
config = open(os.path.join(home, "config.toml")).read().replace(binary, BINARY).replace(board, ROOT)
save("config.toml", config)
hooks_list("hooks-list-trusted.json")

edit_commands(ROOT, OTHER_ROOT)
save("hooks-moved.json", open(hooks_path).read())
hooks_list("hooks-list-moved.json")

# Every field that goes into the hash, and text JSON encoders escape
# differently, so a reimplementation can be checked against Codex's own.
def command(text, **fields):
    return dict({"type": "command", "command": text}, **fields)


edge = {"hooks": {
    "PreToolUse": [
        {"matcher": "Bash", "hooks": [command("echo 'a<b>&c' \"q\" \\ \t é     \U0001f600", timeout=7),
                                      command("echo async", **{"async": True, "statusMessage": "Checking…"})]},
        {"matcher": "*", "hooks": [command("echo star", additionalContextLimit=100)]},
        {"hooks": [command("echo default-limit", additionalContextLimit=2500)]},
    ],
    "Stop": [{"matcher": "ignored", "hooks": [command("echo stop", additionalContextLimit=100, timeout=0)]}],
    "SessionEnd": [{"hooks": [command("echo end", timeout=10), command("echo end-default")]}],
    "Interrupt": [{"hooks": [command("echo interrupt", timeout=0)]}],
    "UserPromptSubmit": [{"hooks": [command("echo prompt", statusMessage=None)]}],
}}
with open(hooks_path, "w") as f:
    json.dump(edge, f, indent=2, ensure_ascii=False)
    f.write("\n")
save("hooks-edge.json", open(hooks_path).read())
hooks_list("hooks-list-edge.json")
proc.stdin.close()
proc.wait(timeout=10)
PY
echo "Check the output for anything unscrubbed before committing it."
