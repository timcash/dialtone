#!/usr/bin/env bash
set -euo pipefail

HOST="legion"
BASE_URL="http://127.0.0.1:5176"
APM="60"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)
      HOST="$2"
      shift 2
      ;;
    --url)
      BASE_URL="$2"
      shift 2
      ;;
    --apm)
      APM="$2"
      shift 2
      ;;
    *)
      echo "usage: $0 [--host legion] [--url http://127.0.0.1:5176] [--apm 60]" >&2
      exit 1
      ;;
  esac
done

PAUSE_SECONDS="$(awk "BEGIN { if ($APM <= 0) print 0; else print 60 / $APM }")"

cd /home/user/dialtone

run() {
  ./dialtone.sh chrome src_v3 "$@"
}

click() {
  local label="$1"
  run click-aria --host "$HOST" --label "$label"
}

pause() {
  if [[ "$PAUSE_SECONDS" == "0" || "$PAUSE_SECONDS" == "0.0" ]]; then
    return
  fi
  sleep "$PAUSE_SECONDS"
}

echo "[demo] open $BASE_URL/#test-home-docs"
run open --host "$HOST" --url "$BASE_URL/#test-home-docs"
pause

walk_section() {
  local button="$1"
  echo "[demo] menu -> $button"
  click "Toggle Global Menu"
  pause
  click "$button"
  pause
}

walk_section "Open Overview"
walk_section "Open Docs"
walk_section "Open Telemetry"
walk_section "Open Steering"
walk_section "Open Key Params"
walk_section "Open Three"
walk_section "Open Three Calc"
walk_section "Open Signals"
walk_section "Open Camera"
walk_section "Open Settings"

echo "[demo] final status"
run status --host "$HOST"
