#!/usr/bin/env bash
set -euo pipefail

NIX_BIN=""
NIX_FLAGS=(--extra-experimental-features "nix-command")
NIXPKGS_URL="${NIXPKGS_URL:-https://channels.nixos.org/nixpkgs-unstable/nixexprs.tar.xz}"
NIX_PKGS=(bashInteractive openssh tmux go bun)

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

if [ $# -eq 0 ]; then
  exec "$NIX_BIN" "${NIX_FLAGS[@]}" shell -f "$NIXPKGS_URL" "${NIX_PKGS[@]}" -c env IN_NIX_SHELL=1 bash -lc 'TMUX_BIN="$(command -v tmux)"; if [ -n "${TMUX:-}" ]; then exec bash -i; else exec "$TMUX_BIN" new -A -s dialtone; fi'
fi

exec "$NIX_BIN" "${NIX_FLAGS[@]}" shell -f "$NIXPKGS_URL" "${NIX_PKGS[@]}" -c env IN_NIX_SHELL=1 "$@"
