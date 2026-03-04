#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALLER="$ROOT/scripts/tsnet-service-install.sh"
SSH_CONFIG="$ROOT/env/ssh_config"

if [[ ! -x "$INSTALLER" ]]; then
  chmod +x "$INSTALLER"
fi

install_local() {
  "$INSTALLER" "$ROOT"
}

install_remote() {
  local host="$1"
  local repo="$2"
  ssh -F "$SSH_CONFIG" "$host" "mkdir -p '$repo/scripts'"
  cat "$INSTALLER" | ssh -F "$SSH_CONFIG" "$host" "cat > '$repo/scripts/tsnet-service-install.sh' && chmod +x '$repo/scripts/tsnet-service-install.sh'"
  ssh -F "$SSH_CONFIG" "$host" "'$repo/scripts/tsnet-service-install.sh' '$repo'"
}

install_local
install_remote gold "/Users/user/dialtone"
install_remote grey "/Users/tim/dialtone"
install_remote rover "/home/tim/dialtone"

echo "tsnet service rollout complete"
