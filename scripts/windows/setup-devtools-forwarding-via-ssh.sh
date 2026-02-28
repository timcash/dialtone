#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NODE="legion"
PORTS="9333"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/windows/setup-devtools-forwarding-via-ssh.sh [--node legion] [--ports 9333,9334]

Applies Windows DevTools forwarding rules over mesh SSH:
  - netsh portproxy 0.0.0.0:PORT -> 127.0.0.1:PORT
  - matching Windows firewall allow rule
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --node)
      NODE="${2:-}"
      shift 2
      ;;
    --ports)
      PORTS="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -z "${NODE}" ]]; then
  echo "--node is required" >&2
  exit 1
fi
if [[ -z "${PORTS}" ]]; then
  echo "--ports is required" >&2
  exit 1
fi

IFS=',' read -r -a PORT_ARR <<< "${PORTS}"
PORT_LIST=""
for p in "${PORT_ARR[@]}"; do
  p="$(echo "$p" | xargs)"
  if [[ ! "$p" =~ ^[0-9]+$ ]] || (( p < 1 || p > 65535 )); then
    echo "Invalid port: $p" >&2
    exit 1
  fi
  if [[ -n "$PORT_LIST" ]]; then
    PORT_LIST+=","
  fi
  PORT_LIST+="$p"
done

PS_SCRIPT='$ports=@('"${PORT_LIST}"'); foreach($port in $ports){ netsh interface portproxy delete v4tov4 listenport=$port listenaddress=0.0.0.0 | Out-Null; netsh interface portproxy add v4tov4 listenport=$port listenaddress=0.0.0.0 connectport=$port connectaddress=127.0.0.1 | Out-Null; $name=\"Dialtone Chrome DevTools WSL $port\"; if(Get-NetFirewallRule -DisplayName $name -ErrorAction SilentlyContinue){Remove-NetFirewallRule -DisplayName $name | Out-Null}; New-NetFirewallRule -DisplayName $name -Direction Inbound -Action Allow -Protocol TCP -LocalPort $port -Profile Any | Out-Null; Write-Output \"configured:$port\" }'
ENC="$(printf '%s' "${PS_SCRIPT}" | iconv -t UTF-16LE | base64 -w0)"

cd "${ROOT_DIR}"
./dialtone.sh ssh src_v1 run --node "${NODE}" --cmd "powershell -NoProfile -EncodedCommand ${ENC}"
