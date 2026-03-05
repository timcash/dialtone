#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "ghostty-send.sh is macOS only" >&2
  exit 1
fi

if ! command -v osascript >/dev/null 2>&1; then
  echo "osascript is required" >&2
  exit 1
fi

if [[ $# -lt 1 ]]; then
  echo "Usage: ./ghostty-dialtone_mod <command>" >&2
  exit 1
fi

COMMAND="$*"

open -a Ghostty
sleep 0.2

osascript <<APPLESCRIPT
tell application "Ghostty"
  activate
end tell

delay 0.2

tell application "System Events"
  tell process "Ghostty"
    keystroke "$COMMAND"
    key code 36
  end tell
end tell
APPLESCRIPT

echo "sent to Ghostty: $COMMAND"
