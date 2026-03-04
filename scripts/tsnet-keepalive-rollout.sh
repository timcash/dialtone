#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SSH_CONFIG="$ROOT/env/ssh_config"
INSTALLER="$ROOT/scripts/tsnet-service-install.sh"
SERVICE_NAME="dialtone_tsnet_keepalive"
RUN_CMD="./bin/tsnet-keepalive"

TAILNET="${TS_TAILNET:-shad-artichoke.ts.net}"
API_KEY="${TS_API_KEY:-}"

if [[ -z "$API_KEY" && -f "$ROOT/env/.env" ]]; then
  API_KEY="$(awk -F= '/^TS_API_KEY=/{print $2}' "$ROOT/env/.env" | tail -n1)"
fi
if [[ -z "$API_KEY" ]]; then
  echo "missing TS_API_KEY (env or env/.env)" >&2
  exit 1
fi

provision_key() {
  local host="$1"
  local tmp
  tmp="$(mktemp /tmp/tskey-${host}.XXXXXX.env)"
  (
    cd "$ROOT"
    ./dialtone.sh tsnet src_v1 keys provision \
      --tailnet "$TAILNET" \
      --api-key "$API_KEY" \
      --description "dialtone-tsnet-${host}" \
      --expiry-hours 720 \
      --write-env "$tmp" >/dev/null
  )
  awk -F= '/^TS_AUTHKEY=/{print $2}' "$tmp" | tail -n1
  rm -f "$tmp"
}

upsert_local_env() {
  local file="$1"
  local key="$2"
  local value="$3"
  touch "$file"
  if grep -q "^${key}=" "$file"; then
    sed -i "s|^${key}=.*|${key}=${value}|" "$file"
  else
    printf '%s=%s\n' "$key" "$value" >>"$file"
  fi
}

upsert_remote_env() {
  local host="$1"
  local file="$2"
  local key="$3"
  local value="$4"
  ssh -F "$SSH_CONFIG" "$host" "mkdir -p \"\$(dirname '$file')\"; touch '$file'; if grep -q '^${key}=' '$file'; then sed -i.bak \"s|^${key}=.*|${key}=${value}|\" '$file'; else printf '%s=%s\n' '${key}' '${value}' >> '$file'; fi"
}

build_local_artifacts() {
  (
    cd "$ROOT"
    mkdir -p bin
    nix --extra-experimental-features 'nix-command flakes' develop . -c bash -lc '
      cd src
      GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ../bin/tsnet-keepalive_linux_amd64 ./cmd/tsnet_keepalive
      GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o ../bin/tsnet-keepalive_linux_arm64 ./cmd/tsnet_keepalive
      GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o ../bin/tsnet-keepalive_darwin_arm64 ./cmd/tsnet_keepalive
    '
  )
}

deploy_remote_binary() {
  local host="$1"
  local repo="$2"
  local os arch srcbin
  os="$(ssh -F "$SSH_CONFIG" "$host" "uname -s | tr '[:upper:]' '[:lower:]'")"
  arch="$(ssh -F "$SSH_CONFIG" "$host" "uname -m")"
  case "$os/$arch" in
    linux/x86_64) srcbin="$ROOT/bin/tsnet-keepalive_linux_amd64" ;;
    linux/aarch64|linux/arm64) srcbin="$ROOT/bin/tsnet-keepalive_linux_arm64" ;;
    darwin/arm64) srcbin="$ROOT/bin/tsnet-keepalive_darwin_arm64" ;;
    *)
      echo "unsupported target for $host: $os/$arch" >&2
      return 1
      ;;
  esac
  ssh -F "$SSH_CONFIG" "$host" "mkdir -p '$repo/bin'"
  cat "$srcbin" | ssh -F "$SSH_CONFIG" "$host" "cat > '$repo/bin/tsnet-keepalive' && chmod +x '$repo/bin/tsnet-keepalive'"
}

install_local_service() {
  SERVICE_NAME="$SERVICE_NAME" RUN_CMD="$RUN_CMD" "$INSTALLER" "$ROOT"
}

install_remote_service() {
  local host="$1"
  local repo="$2"
  ssh -F "$SSH_CONFIG" "$host" "mkdir -p '$repo/scripts'"
  cat "$INSTALLER" | ssh -F "$SSH_CONFIG" "$host" "cat > '$repo/scripts/tsnet-service-install.sh' && chmod +x '$repo/scripts/tsnet-service-install.sh'"
  ssh -F "$SSH_CONFIG" "$host" "SERVICE_NAME='$SERVICE_NAME' RUN_CMD='$RUN_CMD' '$repo/scripts/tsnet-service-install.sh' '$repo'"
}

rollout_host() {
  local host="$1"
  local repo="$2"
  local hostname="$3"
  local key="$4"

  upsert_remote_env "$host" "$repo/env/.env" "TS_AUTHKEY" "$key"
  upsert_remote_env "$host" "$repo/env/.env" "DIALTONE_HOSTNAME" "$hostname"
  upsert_remote_env "$host" "$repo/env/.env" "TS_TAILNET" "$TAILNET"
  deploy_remote_binary "$host" "$repo"
  install_remote_service "$host" "$repo"
}

main() {
  local key_wsl key_gold key_grey key_rover
  key_wsl="$(provision_key wsl)"
  key_gold="$(provision_key gold)"
  key_grey="$(provision_key grey)"
  key_rover="$(provision_key rover)"

  upsert_local_env "$ROOT/env/.env" "TS_AUTHKEY" "$key_wsl"
  upsert_local_env "$ROOT/env/.env" "DIALTONE_HOSTNAME" "wsl"
  upsert_local_env "$ROOT/env/.env" "TS_TAILNET" "$TAILNET"

  build_local_artifacts
  cp -f "$ROOT/bin/tsnet-keepalive_linux_amd64" "$ROOT/bin/tsnet-keepalive"
  chmod +x "$ROOT/bin/tsnet-keepalive"
  install_local_service

  rollout_host gold "/Users/user/dialtone" "gold" "$key_gold"
  rollout_host grey "/Users/tim/dialtone" "grey" "$key_grey"
  rollout_host rover "/home/tim/dialtone" "rover" "$key_rover"

  echo "tsnet keepalive rollout complete (wsl, gold, grey, rover)"
}

main "$@"
