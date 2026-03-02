#!/usr/bin/env bash
set -euo pipefail

PORT="${1:-9222}"
URL="${2:-http://127.0.0.1:5177/#ui-hero-stage}"
ROLE="${3:-ui-dev-tailnet}"
PROFILE_DIR="${HOME}/.dialtone/chrome-tailnet-${PORT}"
LOG_FILE="${HOME}/.dialtone/chrome-tailnet-${PORT}.log"

mkdir -p "${HOME}/.dialtone"

CHROME_BIN="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
if [[ ! -x "${CHROME_BIN}" ]]; then
  echo "chrome binary not found: ${CHROME_BIN}" >&2
  exit 1
fi

if curl -fsS --max-time 1 "http://127.0.0.1:${PORT}/json/version" >/dev/null 2>&1; then
  echo "debug endpoint already up on :${PORT}"
  curl -fsS --max-time 2 "http://127.0.0.1:${PORT}/json/version" || true
  exit 0
fi

nohup "${CHROME_BIN}" \
  --remote-debugging-port="${PORT}" \
  --remote-debugging-address=0.0.0.0 \
  --remote-allow-origins='*' \
  --no-first-run \
  --no-default-browser-check \
  --user-data-dir="${PROFILE_DIR}" \
  --new-window \
  --dialtone-origin=true \
  --dialtone-role="${ROLE}" \
  "${URL}" >"${LOG_FILE}" 2>&1 < /dev/null &

for _ in $(seq 1 80); do
  if curl -fsS --max-time 1 "http://127.0.0.1:${PORT}/json/version" >/dev/null 2>&1; then
    echo "debug endpoint ready on :${PORT}"
    curl -fsS --max-time 2 "http://127.0.0.1:${PORT}/json/version" || true
    exit 0
  fi
  sleep 0.25
done

echo "failed to start debug endpoint on :${PORT}" >&2
tail -n 80 "${LOG_FILE}" || true
exit 2
