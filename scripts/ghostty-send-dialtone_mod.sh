#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${SCRIPT_DIR}/ghostty-send-dialtone_mod.applescript"

if ! command -v osascript >/dev/null 2>&1; then
  echo "osascript is required on macOS" >&2
  exit 1
fi

COMMAND="${1:-./dialtone_mod}"
HOST="${2:-gold.shad-artichoke.ts.net}"
USER_NAME="${3:-user}"
REPO_PATH="${4:-/Users/user/dialtone}"

osascript "$SCRIPT" "$COMMAND" "$HOST" "$USER_NAME" "$REPO_PATH"
