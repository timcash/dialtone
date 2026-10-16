#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "==> [0/5] Verifying zero hostnames or EndpointIds in git-tracked files and git history"
FORBIDDEN_RE=$(python3 -c 'import base64; print(base64.b64decode("cm9zaWV8bGVnaW9ufGJ1aWxkLTF8amV0c29ufHN0ZWFtZGVja3w2NDQ2MzczOTc4NzJ8MDAxMmQ5NjdkMTY1fGZiZDViM2I3ZTRlMg==").decode())')
if git grep -iE "$FORBIDDEN_RE" -- $(git ls-files); then
  echo "ERROR: Forbidden hostname or EndpointId found in git-tracked files!" >&2
  exit 1
fi
if [ -n "$(git log --all -i -G"$FORBIDDEN_RE" --oneline 2>/dev/null)" ] || [ -n "$(git log --all -i --grep="$FORBIDDEN_RE" --oneline 2>/dev/null)" ]; then
  echo "ERROR: Forbidden hostname or EndpointId found in git commit history!" >&2
  exit 1
fi

echo "==> [1/5] Formatting check (cargo fmt --check)"
cargo fmt --check

echo "==> [2/5] Running unit & live Iroh 1.3 integration test suite (cargo test)"
cargo test

echo "==> [3/5] Building single release binary (cargo build --release)"
cargo build --release
mkdir -p bin
rm -f bin/dialtone
cp target/release/dialtone bin/dialtone
shasum -a 256 bin/dialtone > bin/dialtone.sha256
echo "    Staged single binary: $(cat bin/dialtone.sha256)"

BIN="./bin/dialtone"
CI_BLOB_DIR=$(mktemp -d /tmp/dialtone-ci-blob-sup-XXXXXX)
trap 'kill ${CI_DAEMON_PID:-} 2>/dev/null || true; rm -rf "$CI_BLOB_DIR" /tmp/dialtone-ci-*.json /tmp/dialtone-ci-*.log' EXIT
export DIALTONE_STATE_DIR="$CI_BLOB_DIR"

echo "==> [4/5] Running end-to-end single-binary CLI & 7-capability acceptance suite"
"$BIN" self-test > /tmp/dialtone-ci-self-test.json
grep -q '"status": "passed"' /tmp/dialtone-ci-self-test.json
grep -q '"iroh_version": "1.3.0"' /tmp/dialtone-ci-self-test.json

"$BIN" status --node node-alpha > /tmp/dialtone-ci-status.json
grep -q '"steady_state_data_plane": "iroh-quic"' /tmp/dialtone-ci-status.json

"$BIN" state-fixture --node node-alpha > /tmp/dialtone-ci-state.json
grep -q '"robot-edge-motion-server"' /tmp/dialtone-ci-state.json

"$BIN" gossip-fixture --node node-alpha > /tmp/dialtone-ci-gossip.json
grep -q '"mesh": "dialtone-iroh-gossip"' /tmp/dialtone-ci-gossip.json

"$BIN" deploy-plan-fixture --deploy-target robot-v4-default > /tmp/dialtone-ci-deploy.json
grep -q '"includes_daemon_self_update": false' /tmp/dialtone-ci-deploy.json

"$BIN" deploy-plan-fixture --build-case peer-local-async > /tmp/dialtone-ci-build.json
grep -q '"async_non_blocking": true' /tmp/dialtone-ci-build.json

"$BIN" blob-fetch-report > /tmp/dialtone-ci-blob.json
grep -q '"method": "iroh-blobs-ticket"' /tmp/dialtone-ci-blob.json

"$BIN" process-intent-fixture --node edge-1 --intent-action restart > /tmp/dialtone-ci-proc.json
grep -q '"restart_requested": true' /tmp/dialtone-ci-proc.json

"$BIN" rollback-plan-fixture --node edge-1 > /tmp/dialtone-ci-rb.json
grep -q '"side_effect_free_preview": true' /tmp/dialtone-ci-rb.json

"$BIN" alert-ack-fixture --actor operator-ci --note "ci verified" > /tmp/dialtone-ci-ack.json
grep -q '"planned_status": "acknowledged"' /tmp/dialtone-ci-ack.json

"$BIN" proof-validation-fixture --proof-case valid > /tmp/dialtone-ci-proof-valid.json
grep -q '"accepted": true' /tmp/dialtone-ci-proof-valid.json

"$BIN" proof-validation-fixture --proof-case non-daemon-fallback > /tmp/dialtone-ci-proof-bad.json
grep -q '"accepted": false' /tmp/dialtone-ci-proof-bad.json

"$BIN" program-catalog-fixture > /tmp/dialtone-ci-cat.json
grep -q '"row-05-calibration"' /tmp/dialtone-ci-cat.json

"$BIN" service --os macos --node node-alpha | grep -q 'com.timcash.dialtone'
"$BIN" service --os linux --node node-alpha | grep -q 'EnvironmentFile=-/etc/dialtone/daemon.env'
"$BIN" service --os windows --node node-beta | grep -q 'up --node node-beta'

DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" node --node node-alpha --no-dns --no-relay --once > /tmp/dialtone-ci-node.json
grep -q '"schema": "dialtone.iroh.heartbeat.v1"' /tmp/dialtone-ci-node.json

DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" --listen 127.0.0.1:0 --no-dns --no-relay --once > /tmp/dialtone-ci-up.log
grep -q '\[BOOT 1/5\]' /tmp/dialtone-ci-up.log
grep -q '\[BOOT 2/5\] Local Web HUD & Config Server' /tmp/dialtone-ci-up.log
grep -q '\[BOOT 3/5\] Content-Addressed Blob Store' /tmp/dialtone-ci-up.log
grep -q '\[BOOT 4/5\] Iroh 1.3 QUIC Endpoint' /tmp/dialtone-ci-up.log
grep -q '\[BOOT 5/5\] Gossip Mesh & Channel Password' /tmp/dialtone-ci-up.log

# Issue #1: Reject weak or default channel passwords, accept CSPRNG gen-password
if DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" join dialtone --listen 127.0.0.1:19999 --once >/dev/null 2>&1; then
  echo "ERROR: 'dialtone join dialtone' should be rejected (default channel name is not a valid password)"
  exit 1
fi
if DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" join shortpass1 --listen 127.0.0.1:19999 --once >/dev/null 2>&1; then
  echo "ERROR: weak <16 char password should be rejected"
  exit 1
fi

GEN_PW=$("$BIN" gen-password)
echo "$GEN_PW" | grep -q '^dt-'
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" join "$GEN_PW" --listen 127.0.0.1:19999 --once > /tmp/dialtone-ci-join.json
grep -q '"channel": "dialtone"' /tmp/dialtone-ci-join.json
grep -q '"status": "saved_mesh_config"' /tmp/dialtone-ci-join.json

# Issue #2: Ensure plaintext password is never persisted in config.json (only derived_keys)
if grep -q "$GEN_PW" "$CI_BLOB_DIR/config.json"; then
  echo "ERROR: plaintext password was written to config.json!" >&2
  exit 1
fi
grep -q '"derived_keys"' "$CI_BLOB_DIR/config.json"

# Issue #3 & #6: Verify peers allow/list/revoke CLI and expiring single-use invite tickets
FAKE_PEER_ID="1111111111111111111111111111111111111111111111111111111111111111"
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" peers allow "$FAKE_PEER_ID" > /tmp/dialtone-ci-peers-allow.json
grep -q '"status": "allowed"' /tmp/dialtone-ci-peers-allow.json
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" peers list > /tmp/dialtone-ci-peers-list.json
grep -q "$FAKE_PEER_ID" /tmp/dialtone-ci-peers-list.json
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" peers revoke "$FAKE_PEER_ID" > /tmp/dialtone-ci-peers-revoke.json
grep -q '"status": "revoked"' /tmp/dialtone-ci-peers-revoke.json

DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" invite --ttl 15m --no-dns --no-relay > /tmp/dialtone-ci-invite.json
grep -q '"invite_ticket": "dialtone-invite-v1:' /tmp/dialtone-ci-invite.json

echo "==> [5/5] Verifying live CAS blob subprocess supervision, API bearer auth & crash auto-respawn"
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" up \
  --node ci-blob-sup \
  --listen 127.0.0.1:18794 \
  --no-dns \
  --no-relay > "$CI_BLOB_DIR/daemon.log" 2>&1 &
CI_DAEMON_PID=$!

for _ in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:18794/api/processes >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

API_TOKEN=$(tr -d '\r\n' < "$CI_BLOB_DIR/api-token")

# Issue #7: Unauthenticated POST must return HTTP 401 Unauthorized
UNAUTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://127.0.0.1:18794/api/artifacts/blob \
  -H "Content-Type: application/json" \
  -d '{"bytes_base64":"ZGlhbHRvbmU="}')
if [ "$UNAUTH_CODE" != "401" ]; then
  echo "ERROR: expected HTTP 401 on unauthenticated POST, got $UNAUTH_CODE" >&2
  exit 1
fi

# 1. Stage an executable payload into the node's Content-Addressed Blob Store
BLOB_B64=$(printf '#!/bin/sh\nsleep 30\n' | base64 | tr -d '\n')
BLOB_SHA=$(curl -fsS -X POST http://127.0.0.1:18794/api/artifacts/blob \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"bytes_base64\":\"$BLOB_B64\"}" | python3 -c 'import sys,json; print(json.load(sys.stdin)["sha256"])')
echo "    1. Staged executable blob into CAS: $BLOB_SHA"

# 2. Configure the node to keep the blob running as a supervised daemon subprocess
PROC_RESP=$(curl -fsS -X POST http://127.0.0.1:18794/api/processes/blob \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"telemetry-blob-worker\",\"sha256\":\"$BLOB_SHA\",\"desired_state\":\"running\",\"env\":[\"WORKER_MODE=live\"]}")
PID_1=$(printf '%s' "$PROC_RESP" | python3 -c 'import sys,json; d=json.load(sys.stdin)["process"]; assert d["status"]=="running"; assert d["sha256"]=="'"$BLOB_SHA"'"; print(d["pid"])')
kill -0 "$PID_1"
echo "    2. Blob subprocess 'telemetry-blob-worker' spawned & running under daemon (PID=$PID_1)"

# 3. Kill the blob subprocess (simulate crash) and verify daemon automatically respawns it
kill -9 "$PID_1" 2>/dev/null || true
sleep 0.3
PID_2=$(curl -fsS http://127.0.0.1:18794/api/processes | python3 -c 'import sys,json; procs=[p for p in json.load(sys.stdin)["managed_processes"] if p["name"]=="telemetry-blob-worker"]; assert len(procs)==1; assert procs[0]["status"]=="running"; print(procs[0]["pid"])')
if [ "$PID_1" = "$PID_2" ]; then
  echo "ERROR: expected respawned blob subprocess to have a new PID (still $PID_1)" >&2
  exit 1
fi
kill -0 "$PID_2"
echo "    3. Simulated crash (kill -9 $PID_1) -> daemon auto-respawned blob subprocess (new PID=$PID_2)"

# 4. Stop the supervised blob subprocess cleanly
STOP_STATUS=$(curl -fsS -X POST http://127.0.0.1:18794/api/processes/blob \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"telemetry-blob-worker\",\"sha256\":\"$BLOB_SHA\",\"desired_state\":\"stopped\"}" | python3 -c 'import sys,json; d=json.load(sys.stdin)["process"]; assert d["pid"] is None; print(d["status"])')
echo "    4. Cleanly stopped supervised blob subprocess (status=$STOP_STATUS)"

kill $CI_DAEMON_PID 2>/dev/null || true
wait $CI_DAEMON_PID 2>/dev/null || true
rm -rf "$CI_BLOB_DIR" /tmp/dialtone-ci-*.json /tmp/dialtone-ci-*.log
trap - EXIT
echo "==> ALL LOCAL CI/CD CHECKS PASSED!"
