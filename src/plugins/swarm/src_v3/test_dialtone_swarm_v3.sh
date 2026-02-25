#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) BIN="$SCRIPT_DIR/dialtone_swarm_v3_x86_64" ;;
  aarch64|arm64) BIN="$SCRIPT_DIR/dialtone_swarm_v3_arm64" ;;
  *) echo "error: unsupported architecture: $ARCH"; exit 1 ;;
esac

if [ ! -x "$BIN" ]; then
  echo "error: missing binary $BIN"
  exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

echo "[test] help output"
"$BIN" --help >"$tmpdir/help.txt" 2>&1
rg -q "Usage:" "$tmpdir/help.txt"

echo "[test] required arg validation"
if "$BIN" --bind-ip 127.0.0.1 >"$tmpdir/invalid.log" 2>&1; then
  echo "error: expected failure when --bind-port is missing"
  exit 1
fi
rg -q -- "--bind-port is required" "$tmpdir/invalid.log"

echo "[test] local loopback exchange"
"$BIN" \
  --bind-ip 127.0.0.1 --bind-port 19002 \
  --peer-ip 127.0.0.1 --peer-port 19001 \
  --local-id 2 --peer-id 1 \
  --no-send \
  --exit-after-ms 2200 \
  >"$tmpdir/receiver.log" 2>&1 &
receiver_pid=$!
sleep 0.3

"$BIN" \
  --bind-ip 127.0.0.1 --bind-port 19001 \
  --peer-ip 127.0.0.1 --peer-port 19002 \
  --local-id 1 --peer-id 2 \
  --message "test-payload" \
  --count 2 --interval-ms 200 \
  --exit-after-ms 1200 \
  >"$tmpdir/sender.log" 2>&1

wait "$receiver_pid"
rg -q "received\\[" "$tmpdir/receiver.log"
rg -q "test-payload" "$tmpdir/receiver.log"

echo "[ok] all tests passed"
