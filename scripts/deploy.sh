#!/bin/sh
# Build flashheart and install it as ~/bin/flashheart without disturbing
# running agent sessions. Takes the same FLASHHEART_* variables as build.sh.
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
"$project_root/scripts/build.sh"

# Install by rename, never by copying over the binary: a running process whose
# file is overwritten in place gets SIGKILLed on macOS. After the rename, agent
# sessions keep their `flashheart mcp` on the old binary and new ones start
# the new binary.
install_dir="$HOME/bin"
mkdir -p "$install_dir"
cp "$project_root/build/flashheart" "$install_dir/flashheart.new"
mv -f "$install_dir/flashheart.new" "$install_dir/flashheart"

# Stop only the board server (SIGTERM shuts it down cleanly) so the next
# `flashheart` serves the new frontend. Never kill `flashheart mcp`: agents
# don't restart a dead MCP server and report "Transport closed".
if pkill -TERM -f "$install_dir/flashheart serve"; then
  echo "Stopped the running board; run 'flashheart' to reopen it."
fi
echo "Installed $install_dir/flashheart."
