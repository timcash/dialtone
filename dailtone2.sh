#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
V3_DIR="$SCRIPT_DIR/src/mods/mesh/v3"
NIX_FLAGS=(--extra-experimental-features "nix-command flakes")
NIX_BIN=""

find_nix() {
  if command -v nix >/dev/null 2>&1; then
    command -v nix
    return 0
  fi

  local p=""
  for p in "/nix/var/nix/profiles/default/bin/nix" "$HOME/.nix-profile/bin/nix" "/run/current-system/sw/bin/nix"; do
    if [ -x "$p" ]; then
      echo "$p"
      return 0
    fi
  done

  p="$(ls -1d /nix/store/*-nix-*/bin/nix 2>/dev/null | sort | tail -n1 || true)"
  if [ -n "$p" ] && [ -x "$p" ]; then
    echo "$p"
    return 0
  fi

  return 1
}

if ! NIX_BIN="$(find_nix)"; then
  echo "nix is required" >&2
  exit 1
fi

if [ ! -d "$V3_DIR" ]; then
  echo "mesh/v3 not found: $V3_DIR" >&2
  exit 1
fi

run_in_shell() {
  local cmd="$1"
  exec "$NIX_BIN" "${NIX_FLAGS[@]}" develop "$V3_DIR" -c bash -c "cd \"$V3_DIR\" && command -v cargo >/dev/null 2>&1 || { echo 'cargo not found in nix dev shell' >&2; exit 127; }; $cmd"
}

if [ $# -eq 0 ]; then
  run_in_shell "cargo build --release"
fi

CMD=""
for arg in "$@"; do
  if [ -n "$CMD" ]; then
    CMD+=" "
  fi
  CMD+="$(printf '%q' "$arg")"
done

run_in_shell "$CMD"
