#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "ghostty-dialtone2.sh is for macOS only" >&2
  exit 1
fi

if ! command -v osascript >/dev/null 2>&1; then
  echo "osascript is required" >&2
  exit 1
fi

if [[ ! -x "$HOME/dialtone/dialtone2.sh" ]]; then
  echo "missing executable: ~/dialtone/dialtone2.sh" >&2
  exit 1
fi

open -a Ghostty
sleep 0.4

osascript <<'APPLESCRIPT'
tell application "Ghostty" to activate
delay 0.2
tell application "System Events"
  keystroke "cd ~/dialtone && ./dialtone2.sh"
  key code 36
end tell
APPLESCRIPT

echo "sent to Ghostty: cd ~/dialtone && ./dialtone2.sh"
