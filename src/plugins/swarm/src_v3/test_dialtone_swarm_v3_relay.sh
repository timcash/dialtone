#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../../" && pwd)"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) BIN="$SCRIPT_DIR/dialtone_swarm_v3_x86_64" ;;
  aarch64|arm64) BIN="$SCRIPT_DIR/dialtone_swarm_v3_arm64" ;;
  *) echo "error: unsupported architecture: $ARCH"; exit 1 ;;
esac
RENDEZVOUS_URL="${RENDEZVOUS_URL:-${RELAY_URL:-https://relay.dialtone.earth}}"
TOPIC="${TOPIC:-swarm-v3-relay-test}"
A_PORT="${A_PORT:-19401}"
B_PORT="${B_PORT:-19402}"
MSG="${MSG:-relay-discovery-ok}"
START_LOCAL_RENDEZVOUS="${START_LOCAL_RENDEZVOUS:-0}"

if [ ! -x "$BIN" ]; then
  echo "error: missing binary $BIN"
  exit 1
fi

if ! curl -fsS "$RENDEZVOUS_URL/health" >/dev/null 2>&1; then
  if [ "$START_LOCAL_RENDEZVOUS" = "1" ]; then
    echo "[test] rendezvous not running at $RENDEZVOUS_URL; starting local rendezvous server"
    (
      cd "$REPO_ROOT"
      nohup env RELAY_LISTEN=":8080" go run ./src/plugins/swarm/src_v3/relay_web/main.go \
        >/tmp/dialtone_swarm_v3_relay.log 2>&1 &
    )
    sleep 1
  else
    echo "error: rendezvous unavailable at $RENDEZVOUS_URL"
    echo "hint: set START_LOCAL_RENDEZVOUS=1 only for local dev fallback"
    exit 1
  fi
fi

curl -fsS "$RENDEZVOUS_URL/health" >/dev/null
echo "[test] rendezvous health OK at $RENDEZVOUS_URL"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

register() {
  local who="$1"
  local port="$2"
  curl -fsS "$RENDEZVOUS_URL/api/register" \
    -H "content-type: application/json" \
    -d "{\"topic\":\"$TOPIC\",\"who\":\"$who\",\"port\":$port}"
}

echo "[test] register node-a"
register "node-a" "$A_PORT" >"$tmpdir/a1.json"

echo "[test] register node-b"
register "node-b" "$B_PORT" >"$tmpdir/b1.json"

echo "[test] refresh node-a (should now discover node-b)"
register "node-a" "$A_PORT" >"$tmpdir/a2.json"

A_PEER_IP="$(python3 - <<'PY' "$tmpdir/a2.json"
import json,sys
data=json.load(open(sys.argv[1]))
peers=data.get("peers",[])
print(peers[0]["ip"] if peers else "")
PY
)"
A_PEER_PORT="$(python3 - <<'PY' "$tmpdir/a2.json"
import json,sys
data=json.load(open(sys.argv[1]))
peers=data.get("peers",[])
print(peers[0]["port"] if peers else "")
PY
)"

B_PEER_IP="$(python3 - <<'PY' "$tmpdir/b1.json"
import json,sys
data=json.load(open(sys.argv[1]))
peers=data.get("peers",[])
print(peers[0]["ip"] if peers else "")
PY
)"
B_PEER_PORT="$(python3 - <<'PY' "$tmpdir/b1.json"
import json,sys
data=json.load(open(sys.argv[1]))
peers=data.get("peers",[])
print(peers[0]["port"] if peers else "")
PY
)"

if [ -z "$A_PEER_IP" ] || [ -z "$A_PEER_PORT" ] || [ -z "$B_PEER_IP" ] || [ -z "$B_PEER_PORT" ]; then
  echo "error: rendezvous discovery did not return peer endpoints"
  echo "--- node-a register ---"
  cat "$tmpdir/a2.json"
  echo "--- node-b register ---"
  cat "$tmpdir/b1.json"
  exit 1
fi

format_hostport() {
  local ip="$1"
  local port="$2"
  if [[ "$ip" == *:* ]]; then
    echo "[$ip]:$port"
  else
    echo "$ip:$port"
  fi
}

BIND_IP_A="0.0.0.0"
BIND_IP_B="0.0.0.0"

# Local single-host test mode: preserve rendezvous discovery but use loopback for transport.
if [ "$A_PEER_IP" = "$B_PEER_IP" ]; then
  if [[ "$A_PEER_IP" == *:* ]]; then
    BIND_IP_A="::1"
    BIND_IP_B="::1"
    A_PEER_IP="::1"
    B_PEER_IP="::1"
  else
    BIND_IP_A="127.0.0.1"
    BIND_IP_B="127.0.0.1"
    A_PEER_IP="127.0.0.1"
    B_PEER_IP="127.0.0.1"
  fi
  echo "[test] single-host mode: using loopback transport while keeping rendezvous domain discovery"
fi

echo "[test] discovered via rendezvous:"
echo "  node-a peer -> $(format_hostport "$A_PEER_IP" "$A_PEER_PORT")"
echo "  node-b peer -> $(format_hostport "$B_PEER_IP" "$B_PEER_PORT")"

"$BIN" \
  --bind-ip "$BIND_IP_A" --bind-port "$A_PORT" \
  --peer-ip "$A_PEER_IP" --peer-port "$A_PEER_PORT" \
  --local-id 2 --peer-id 1 \
  --no-send \
  --exit-after-ms 2600 \
  >"$tmpdir/receiver.log" 2>&1 &
receiver_pid=$!
sleep 0.3

"$BIN" \
  --bind-ip "$BIND_IP_B" --bind-port "$B_PORT" \
  --peer-ip "$B_PEER_IP" --peer-port "$B_PEER_PORT" \
  --local-id 1 --peer-id 2 \
  --message "$MSG" --count 2 --interval-ms 200 \
  --exit-after-ms 1200 \
  >"$tmpdir/sender.log" 2>&1

wait "$receiver_pid"

echo "[test] sender log"
cat "$tmpdir/sender.log"
echo "[test] receiver log"
cat "$tmpdir/receiver.log"

grep -q "sent\\[1/2\\]" "$tmpdir/sender.log"
grep -q "received\\[" "$tmpdir/receiver.log"
grep -q "$MSG" "$tmpdir/receiver.log"

echo "[ok] relay discovery test passed"
