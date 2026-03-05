#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIALTONE_BIN="${ROOT_DIR}/dialtone_mod"
HOST="${1:-legion}"
WINDOW_LOG="/mnt/c/Users/Public/dialtone-typing-terminal.log.window.log"
LAUNCH_LOG="/mnt/c/Users/Public/dialtone-typing-terminal.log"
STATE_LOG="/mnt/c/Users/Public/dialtone-typing-terminal.log.state.json"
QUEUE_LOG="/mnt/c/Users/Public/dialtone-typing-terminal.log.queue.txt"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-20}"
DELAY_SECONDS="${DELAY_SECONDS:-1}"
TOKEN="DIALTONE_LOG_TEST_$(date +%s)"

if [[ ! -x "${DIALTONE_BIN}" ]]; then
  echo "missing executable: ${DIALTONE_BIN}" >&2
  exit 1
fi

send_command() {
  local cmd="$1"
  "${DIALTONE_BIN}" typing v1 terminal --host "${HOST}" --local --command "${cmd}"
  sleep "${DELAY_SECONDS}"
}

echo "cleaning old Dialtone typing terminals"
/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command "Get-Process -Name powershell -ErrorAction SilentlyContinue | Where-Object { \$_.MainWindowTitle -like 'DialtoneTyping*' } | Stop-Process -Force -ErrorAction SilentlyContinue"
rm -f "${STATE_LOG}" "${QUEUE_LOG}"

before_windows="$(
  /mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command '(Get-Process -Name powershell -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 }).Count'
)"

echo "running log-read test token=${TOKEN}"
send_command "Write-Host '${TOKEN} step=1'"
send_command "Write-Host '${TOKEN} step=2'"

if [[ ! -f "${WINDOW_LOG}" ]]; then
  echo "window log not found: ${WINDOW_LOG}" >&2
  exit 1
fi

if [[ ! -f "${LAUNCH_LOG}" ]]; then
  echo "launch log not found: ${LAUNCH_LOG}" >&2
  exit 1
fi

start_ts="$(date +%s)"
while true; do
  if grep -Fq "${TOKEN}" "${WINDOW_LOG}"; then
    after_windows="$(
      /mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command '(Get-Process -Name powershell -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 }).Count'
    )"
    launch_count="$(
      grep -F "launched powershell window pid=" "${LAUNCH_LOG}" | tail -n 20 | wc -l | tr -d ' '
    )"
    reuse_count="$(
      grep -F "reusing existing window pid=" "${LAUNCH_LOG}" | tail -n 20 | wc -l | tr -d ' '
    )"
    echo "PASS: found token in window transcript log"
    grep -F "${TOKEN}" "${WINDOW_LOG}" | tail -n 10
    echo "visible powershell windows before=${before_windows} after=${after_windows}"
    echo "recent launch log counts: launched=${launch_count} reused=${reuse_count}"
    exit 0
  fi
  now_ts="$(date +%s)"
  if (( now_ts - start_ts >= TIMEOUT_SECONDS )); then
    echo "FAIL: token not found in transcript within ${TIMEOUT_SECONDS}s" >&2
    echo "last transcript lines:" >&2
    tail -n 40 "${WINDOW_LOG}" >&2 || true
    exit 1
  fi
  sleep 1
done
