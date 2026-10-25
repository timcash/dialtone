#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "==> [0/5] Verifying zero hostnames or NodeIds in git-tracked files and git history"
FORBIDDEN_RE=$(python3 -c 'import base64; print(base64.b64decode("cm9zaWV8bGVnaW9ufGJ1aWxkLTF8amV0c29ufHN0ZWFtZGVja3w2NDQ2MzczOTc4NzJ8MDAxMmQ5NjdkMTY1fGZiZDViM2I3ZTRlMg==").decode())')
if git grep -iE "$FORBIDDEN_RE" -- $(git ls-files); then
  echo "ERROR: Forbidden hostname or NodeId found in git-tracked files!" >&2
  exit 1
fi
if [ -n "$(git log --all -i -G"$FORBIDDEN_RE" --oneline 2>/dev/null)" ] || [ -n "$(git log --all -i --grep="$FORBIDDEN_RE" --oneline 2>/dev/null)" ]; then
  echo "ERROR: Forbidden hostname or NodeId found in git commit history!" >&2
  exit 1
fi

echo "==> [0b/5] Domain-language gate (goal.md vocabulary, banned synonyms in src/)"
./domain-lint.sh

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
# Stop the CI node *and* the processes its supervisor started: killing the node
# alone re-parents its processes to init, leaving one orphan per CI run.
ci_cleanup() {
  if [ -n "${CI_DAEMON_PID:-}" ]; then
    kill "$CI_DAEMON_PID" 2>/dev/null || true
    wait "$CI_DAEMON_PID" 2>/dev/null || true
  fi
  pkill -f "$CI_BLOB_DIR/run/" 2>/dev/null || true
  rm -rf "$CI_BLOB_DIR" /tmp/dialtone-ci-*.json /tmp/dialtone-ci-*.log
}
trap ci_cleanup EXIT
export DIALTONE_STATE_DIR="$CI_BLOB_DIR"

echo "==> [4/5] Running end-to-end single-binary CLI checks"
"$BIN" devices --json > /tmp/dialtone-ci-devices.json

"$BIN" status --node node-alpha > /tmp/dialtone-ci-status.json
grep -q '"steady_state_data_plane": "iroh-quic"' /tmp/dialtone-ci-status.json

"$BIN" service --os macos --node node-alpha | grep -q 'com.timcash.dialtone'
"$BIN" service --os linux --node node-alpha | grep -q 'EnvironmentFile=-/etc/dialtone/daemon.env'
"$BIN" service --os windows --node node-beta | grep -q 'up --node node-beta'

DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" node --node node-alpha --no-dns --no-relay --once > /tmp/dialtone-ci-node.json
grep -q '"schema": "dialtone.iroh.beat.v1"' /tmp/dialtone-ci-node.json

DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" --listen 127.0.0.1:0 --no-dns --no-relay --once > /tmp/dialtone-ci-up.log
grep -q '\[BOOT 1/5\]' /tmp/dialtone-ci-up.log
grep -q '\[BOOT 2/5\] Local Web HUD & Config Server' /tmp/dialtone-ci-up.log
grep -q '\[BOOT 3/5\] Content-Addressed Blob Store' /tmp/dialtone-ci-up.log
grep -q '\[BOOT 4/5\] Iroh 1.3 QUIC Endpoint' /tmp/dialtone-ci-up.log
grep -q '\[BOOT 5/5\] Gossip Mesh & Channel Secret' /tmp/dialtone-ci-up.log

# Issue #1: Reject weak or default channel secrets, accept a generated secret
if DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" join dialtone --listen 127.0.0.1:19999 --once >/dev/null 2>&1; then
  echo "ERROR: 'dialtone join dialtone' should be rejected (default channel name is not a valid secret)"
  exit 1
fi
if DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" join shortpass1 --listen 127.0.0.1:19999 --once >/dev/null 2>&1; then
  echo "ERROR: weak <16 char secret should be rejected"
  exit 1
fi

GEN_SECRET=$("$BIN" secret)
echo "$GEN_SECRET" | grep -q '^dt-'
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" join "$GEN_SECRET" --listen 127.0.0.1:19999 --once > /tmp/dialtone-ci-join.json
grep -q '"channel": "dialtone"' /tmp/dialtone-ci-join.json
grep -q '"status": "saved_mesh_config"' /tmp/dialtone-ci-join.json

# Issue #2: Ensure the plaintext secret is never persisted in config.json (only derived_keys)
if grep -q "$GEN_SECRET" "$CI_BLOB_DIR/config.json"; then
  echo "ERROR: plaintext secret was written to config.json!" >&2
  exit 1
fi
grep -q '"derived_keys"' "$CI_BLOB_DIR/config.json"

# Issue #3 & #6: Verify allowlist add/list/remove CLI and expiring single-use invites
FAKE_PEER_ID="1111111111111111111111111111111111111111111111111111111111111111"
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" allowlist add "$FAKE_PEER_ID" --addr 127.0.0.1:19440 > /tmp/dialtone-ci-peers-allow.json
grep -q '"status": "allowed"' /tmp/dialtone-ci-peers-allow.json
grep -q '"127.0.0.1:19440"' /tmp/dialtone-ci-peers-allow.json
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" allowlist list > /tmp/dialtone-ci-peers-list.json
grep -q "$FAKE_PEER_ID" /tmp/dialtone-ci-peers-list.json
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" allowlist remove "$FAKE_PEER_ID" > /tmp/dialtone-ci-peers-revoke.json
grep -q '"status": "revoked"' /tmp/dialtone-ci-peers-revoke.json
# Re-add the peer so mesh-test sees a non-empty allowlist
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" allowlist add "$FAKE_PEER_ID" --addr 127.0.0.1:19440 > /dev/null

DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" invite --ttl 15m --no-dns --no-relay > /tmp/dialtone-ci-invite.json
grep -q '"invite": "dialtone-invite-v1:' /tmp/dialtone-ci-invite.json

echo "==> [5/5] Verifying store, supervisor, API bearer auth & crash restart on a live node"
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" up \
  --node ci-blob-sup \
  --listen 127.0.0.1:18794 \
  --private-mesh > "$CI_BLOB_DIR/daemon.log" 2>&1 &
CI_DAEMON_PID=$!

for _ in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:18794/api/processes >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

API_TOKEN=$(tr -d '\r\n' < "$CI_BLOB_DIR/api-token")

# Issue #7: Unauthenticated POST must return HTTP 401 Unauthorized
UNAUTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://127.0.0.1:18794/api/blob \
  -H "Content-Type: application/json" \
  -d '{"bytes_base64":"ZGlhbHRvbmU="}')
if [ "$UNAUTH_CODE" != "401" ]; then
  echo "ERROR: expected HTTP 401 on unauthenticated POST, got $UNAUTH_CODE" >&2
  exit 1
fi

# 1. Put a program blob in the node's store
BLOB_B64=$(printf '#!/bin/sh\nsleep 30\n' | base64 | tr -d '\n')
BLOB_DIGEST=$(curl -fsS -X POST http://127.0.0.1:18794/api/blob \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"bytes_base64\":\"$BLOB_B64\"}" | python3 -c 'import sys,json; print(json.load(sys.stdin)["digest"])')
echo "    1. Put program blob in the store: $BLOB_DIGEST"

# 2. Ask the supervisor to keep the program running as a process
PROC_RESP=$(curl -fsS -X POST http://127.0.0.1:18794/api/processes/blob \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"ci-process\",\"digest\":\"$BLOB_DIGEST\",\"desired_state\":\"running\",\"env\":[\"PROCESS_MODE=live\"]}")
PID_1=$(printf '%s' "$PROC_RESP" | python3 -c 'import sys,json; d=json.load(sys.stdin)["process"]; assert d["status"]=="running"; assert d["digest"]=="'"$BLOB_DIGEST"'"; print(d["pid"])')
kill -0 "$PID_1"
echo "    2. Process 'ci-process' started by the supervisor (PID=$PID_1)"

# 3. Kill the process (simulate a crash) and verify the supervisor restarts it
kill -9 "$PID_1" 2>/dev/null || true
sleep 0.3
PID_2=$(curl -fsS http://127.0.0.1:18794/api/processes | python3 -c 'import sys,json; procs=[p for p in json.load(sys.stdin)["processes"] if p["name"]=="ci-process"]; assert len(procs)==1; assert procs[0]["status"]=="running"; assert procs[0]["restart_count"]>=1; print(procs[0]["pid"])')
if [ "$PID_1" = "$PID_2" ]; then
  echo "ERROR: expected the restarted process to have a new PID (still $PID_1)" >&2
  exit 1
fi
kill -0 "$PID_2"
echo "    3. Simulated crash (kill -9 $PID_1) -> supervisor restarted the process (new PID=$PID_2)"

# 4. Stop the process cleanly
STOP_STATUS=$(curl -fsS -X POST http://127.0.0.1:18794/api/processes/blob \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"ci-process\",\"digest\":\"$BLOB_DIGEST\",\"desired_state\":\"stopped\"}" | python3 -c 'import sys,json; d=json.load(sys.stdin)["process"]; assert d["pid"] is None; print(d["status"])')
echo "    4. Stopped the process cleanly (status=$STOP_STATUS)"
curl -fsS -X POST http://127.0.0.1:18794/api/processes \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"ci-process","desired_state":"removed"}' | grep -q '"status":"removed"'
curl -fsS http://127.0.0.1:18794/api/processes | python3 -c 'import sys,json; assert not [p for p in json.load(sys.stdin)["processes"] if p["name"]=="ci-process"]'
if ls "$CI_BLOB_DIR"/run/ci-process-* >/dev/null 2>&1; then
  echo "ERROR: removed process left its staged program behind" >&2
  exit 1
fi
echo "       Removed it: gone from /api/processes and its staged program deleted"

# 5. Verify blob share & pull and mesh-test against the running node
SHARE_JSON=$(DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" blob share "ci-live-blob-payload" --listen 127.0.0.1:18794)
TICKET_URI=$(printf '%s' "$SHARE_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["ticket_uri"])')
PULL_JSON=$(DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" blob pull "$TICKET_URI" --force --listen 127.0.0.1:18794)
printf '%s' "$PULL_JSON" | grep -q '"method": "local-self"'
echo "    5. Verified blob share & pull via CLI ($TICKET_URI)"

MESH_TEST_JSON=$(DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" mesh-test --expect-peers 1 --timeout 5s --listen 127.0.0.1:18794)
printf '%s' "$MESH_TEST_JSON" | grep -q '"status": "ok"'
printf '%s' "$MESH_TEST_JSON" | grep -q '"public_directory_publishing": false'
printf '%s' "$MESH_TEST_JSON" | grep -q '"real_stats_verified": true'
printf '%s' "$MESH_TEST_JSON" | grep -q '"summary_stats_verified": true'
echo "    6. Verified dialtone mesh-test --expect-peers 1 passed (real stats + summary_stats)"

curl -fsS http://127.0.0.1:18794/api/state | python3 -c '
import sys, json
st = json.load(sys.stdin)
nodes = st["nodes"]
assert len(nodes) >= 1
n = nodes[0]
beat_int = n.get("beat_interval_secs")
sum_int = n.get("summary_interval_secs")
assert beat_int == 5, f"expected 5s beat interval, got {beat_int}"
assert sum_int == 30, f"expected 30s summary interval, got {sum_int}"
hr = n["stats"]
assert hr["disk_used_bytes"] != 120 * 1024 * 1024 * 1024, "fake 120 GiB disk detected"
assert abs(hr["disk_percent"] - 23.4) > 0.01, "fake 23.4% disk detected"
assert n.get("summary_stats") is not None, "missing summary_stats on live beat"
assert n["summary_stats"]["window_secs"] == 30
logs_txt = json.dumps(st.get("logs", []))
assert "nats://" not in logs_txt, "fake seeded NATS log found in live node state"
'
echo "    7. Verified live /api/state reports real stats, 5s/30s summaries, and non-synthetic logs"

# 8. `dialtone hud` next to a running node serves the HUD but never rewrites its config
CFG_BEFORE=$(shasum "$CI_BLOB_DIR/config.json" | awk '{print $1}')
DIALTONE_STATE_DIR="$CI_BLOB_DIR" "$BIN" hud --node ci-hud --listen 127.0.0.1:18795 --once > /tmp/dialtone-ci-hud.log 2>&1 &
HUD_PID=$!
for _ in $(seq 1 50); do curl -fsS -o /dev/null http://127.0.0.1:18795/ 2>/dev/null && break; sleep 0.1; done
wait "$HUD_PID" || true
if [ "$CFG_BEFORE" != "$(shasum "$CI_BLOB_DIR/config.json" | awk '{print $1}')" ]; then
  echo "ERROR: dialtone hud rewrote the running node's config.json" >&2
  exit 1
fi
echo "    8. Verified dialtone hud leaves the running node's config.json untouched"

# 9. Removed preview / fixture routes stay removed
for route in /api/deploy-plan /api/catalogs /api/process-intent /install/v1/install.sh /ws /api/gossip /api/artifacts/blob /api/e2e/status; do
  CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:18794$route")
  if [ "$CODE" != "404" ]; then
    echo "ERROR: expected 404 for removed route $route, got $CODE" >&2
    exit 1
  fi
done
echo "    9. Verified removed and renamed routes return 404"

ci_cleanup
trap - EXIT
echo "==> ALL LOCAL CI/CD CHECKS PASSED!"
