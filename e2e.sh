#!/usr/bin/env bash
# e2e.sh — three real nodes on this machine, end to end:
#   1. they form a mesh (signed beats, allowlist at the handshake)
#   2. every node's probe passes: store, blob pulls from both peers, beats both ways, program
#   3. a program deployed on one node runs on all three, and removing it there removes it everywhere
#   4. a stopped node is caught by the others' probes
set -euo pipefail
cd "$(dirname "$0")"

BIN="${BIN:-./target/release/dialtone}"
[ -x "$BIN" ] || cargo build --release
ROOT=$(mktemp -d /tmp/dialtone-e2e-XXXXXX)
SECRET="dt-E2E_Local_Mesh_Secret_2026!"
NAMES=(e2e-a e2e-b e2e-c)
QUIC=(19511 19512 19513)
API=(18911 18912 18913)
PIDS=()

cleanup() {
  for pid in "${PIDS[@]:-}"; do kill "$pid" 2>/dev/null || true; done
  wait 2>/dev/null || true
  pkill -f "$ROOT/" 2>/dev/null || true
  rm -rf "$ROOT"
}
trap cleanup EXIT

fail() { echo "E2E FAIL: $*" >&2; exit 1; }
# until <seconds> <description> <command...>: retry the command every second until it succeeds.
until_ok() {
  local secs=$1 what=$2; shift 2
  for _ in $(seq 1 "$secs"); do "$@" >/dev/null 2>&1 && return 0; sleep 1; done
  fail "$what (gave up after ${secs}s)"
}
api() { curl -fsS -m 20 "http://127.0.0.1:${API[$1]}$2" "${@:3}"; }
post() { api "$1" "$2" -X POST -H "Authorization: Bearer $(cat "$ROOT/${NAMES[$1]}/api-token")" \
  -H "Content-Type: application/json" -d "${3:-{\}}"; }

# Identities, then everyone allowlists everyone at a fixed QUIC port.
IDS=()
for i in 0 1 2; do
  mkdir -p "$ROOT/${NAMES[$i]}"
  STATUS=$(DIALTONE_STATE_DIR="$ROOT/${NAMES[$i]}" "$BIN" status --node "${NAMES[$i]}" --private-mesh)
  IDS[$i]=$(printf '%s' "$STATUS" | python3 -c 'import sys,json; print(json.load(sys.stdin)["diagnostics"]["node_id"])')
  IP=$(printf '%s' "$STATUS" | python3 -c 'import sys,json; print(json.load(sys.stdin)["diagnostics"]["bound_non_tailnet_ipv4"] or "127.0.0.1")')
done
for i in 0 1 2; do
  for j in 0 1 2; do
    [ "$i" = "$j" ] && continue
    DIALTONE_STATE_DIR="$ROOT/${NAMES[$i]}" "$BIN" allowlist add "${IDS[$j]}" --addr "$IP:${QUIC[$j]}" >/dev/null
  done
done
for i in 0 1 2; do
  DIALTONE_STATE_DIR="$ROOT/${NAMES[$i]}" "$BIN" up --node "${NAMES[$i]}" --listen "127.0.0.1:${API[$i]}" \
    --private-mesh --quic-port "${QUIC[$i]}" --secret "$SECRET" > "$ROOT/${NAMES[$i]}.log" 2>&1 &
  PIDS[$i]=$!
done

# 1. Mesh: each node sees all three.
sees_all() { api "$1" /api/mesh | python3 -c 'import sys,json; sys.exit(0 if len(json.load(sys.stdin)["nodes"]) == 3 else 1)'; }
for i in 0 1 2; do until_ok 60 "${NAMES[$i]} sees 3 nodes" sees_all "$i"; done
echo "    1. Mesh: 3 nodes see each other"

# 2. Probe: run probes until each passes all five checks (a pull needs the peer's previous blob).
probe_passes() {
  post "$1" /api/probe | python3 -c '
import sys, json
p = json.load(sys.stdin)["probe"]
ok = p["passed_count"] == p["total_count"] == 5 and len([x for x in p["pulls"] if x["ok"]]) == 2
sys.exit(0 if ok else 1)'
}
for i in 0 1 2; do until_ok 60 "${NAMES[$i]} probe 5/5 with 2 pulls" probe_passes "$i"; done
echo "    2. Probe: 5/5 on every node, each pulled both peers' blobs over QUIC"

# 3. Deploy: run a program on a with deploy_to_mesh, then remove it there only.
PROGRAM=$(printf ':; while true; do sleep 5; done; exit 0;\r\n@echo off\r\nREM e2e %s\r\n:loop\r\nping -n 6 127.0.0.1 >NUL\r\ngoto loop\r\n' "$ROOT" | base64 | tr -d '\n')
post 0 /api/processes/blob "{\"name\":\"e2e-program\",\"bytes_base64\":\"$PROGRAM\",\"desired_state\":\"running\",\"deploy_to_mesh\":true}" >/dev/null
runs_program() { api "$1" /api/processes | python3 -c '
import sys, json
sys.exit(0 if any(p["name"] == "e2e-program" and p.get("status") == "running" for p in json.load(sys.stdin)["processes"]) else 1)'; }
lacks_program() { ! runs_program "$1"; }
for i in 1 2; do until_ok 60 "${NAMES[$i]} runs the deployed program" runs_program "$i"; done
post 0 /api/processes '{"name":"e2e-program","desired_state":"removed"}' >/dev/null
for i in 0 1 2; do until_ok 60 "${NAMES[$i]} removed the program" lacks_program "$i"; done
echo "    3. Deploy: ran on all 3 nodes, removing it on the origin removed every copy"

# 4. Failure: stop c; a's probe must report c's silence.
kill "${PIDS[2]}"
a_sees_c_silent() {
  post 0 /api/probe | python3 -c '
import sys, json
beat = next(c for c in json.load(sys.stdin)["probe"]["checks"] if c["code"] == "BEAT")
sys.exit(0 if not beat["ok"] and "e2e-c" in beat["detail"] else 1)'
}
until_ok 45 "a's probe reports e2e-c as silent" a_sees_c_silent
echo "    4. Failure: a's probe flagged e2e-c once it stopped"
echo "==> E2E PASSED (3 nodes: mesh, probe, deploy, removal, failure detection)"
