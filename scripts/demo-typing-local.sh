#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIALTONE_BIN="${ROOT_DIR}/dialtone_mod"
DELAY_SECONDS="${DELAY_SECONDS:-1}"

if [[ ! -x "${DIALTONE_BIN}" ]]; then
  echo "missing executable: ${DIALTONE_BIN}" >&2
  exit 1
fi

send_command() {
  local cmd="$1"
  echo "sending: ${cmd}"
  "${DIALTONE_BIN}" terminal v1 type --command "${cmd}"
  sleep "${DELAY_SECONDS}"
}

send_command "Write-Host 'DIALTONE_DEMO step=1 window=ready'"
send_command "\$global:DIALTONE_DEMO_COUNTER = [int](\$global:DIALTONE_DEMO_COUNTER) + 1; Write-Host ('DIALTONE_DEMO step=2 counter={0}' -f \$global:DIALTONE_DEMO_COUNTER)"
send_command "Write-Host ('DIALTONE_DEMO step=3 cwd={0}' -f (Get-Location).Path)"
send_command "Get-Date | ForEach-Object { Write-Host ('DIALTONE_DEMO step=4 timestamp=' + \$_) }"

echo "done. check logs:"
echo "  /mnt/c/Users/Public/dialtone-typing-terminal.log"
echo "  /mnt/c/Users/Public/dialtone-typing-terminal.log.window.log"
