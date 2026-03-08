#!/usr/bin/env bash
set -euo pipefail

HOST="legion"
BASE_URL="http://127.0.0.1:3000"
APM="60"
ROLE="robot-dev"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)
      HOST="${2:?missing value for --host}"
      shift 2
      ;;
    --url)
      BASE_URL="${2:?missing value for --url}"
      shift 2
      ;;
    --apm)
      APM="${2:?missing value for --apm}"
      shift 2
      ;;
    --role)
      ROLE="${2:?missing value for --role}"
      shift 2
      ;;
    *)
      echo "usage: $0 [--host legion] [--role robot-dev] [--url http://127.0.0.1:3000] [--apm 60]" >&2
      exit 1
      ;;
  esac
done

if ! awk "BEGIN { exit !($APM > 0) }"; then
  echo "--apm must be > 0" >&2
  exit 1
fi

PAUSE="$(awk "BEGIN { printf \"%.3f\", 60 / $APM }")"

run() {
  ./dialtone.sh chrome src_v3 "$@" --role "$ROLE"
}

pause() {
  sleep "$PAUSE"
}

click_menu_item() {
  local label="$1"
  echo "[robot-demo] menu -> ${label}"
  run click-aria --host "$HOST" --label "Toggle Global Menu"
  pause
  run click-aria --host "$HOST" --label "$label"
  pause
  run status --host "$HOST"
}

echo "[robot-demo] open ${BASE_URL}/#hero"
run open --host "$HOST" --url "${BASE_URL}/#hero"
pause

click_menu_item "Navigate Hero"
click_menu_item "Navigate Docs"
click_menu_item "Navigate Telemetry"
click_menu_item "Navigate Steering Settings"
click_menu_item "Navigate Key Params"
click_menu_item "Navigate Three"
click_menu_item "Navigate Terminal"
click_menu_item "Navigate Camera"
click_menu_item "Navigate Settings"

echo "[robot-demo] final status"
run status --host "$HOST"
