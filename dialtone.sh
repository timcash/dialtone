#!/usr/bin/env bash
# dialtone.sh — the one script for working on dialtone.
#
#   ./dialtone.sh check            lint, fmt, tests, release build, one-node test, three-node test
#   ./dialtone.sh lint             leak gate (no host names / NodeIds in git) and domain language
#   ./dialtone.sh e2e              three real nodes on this machine (about 30s)
#   ./dialtone.sh scale [N] [secs] N nodes on this machine (default 20): gossip convergence and
#                                    bytes per node over secs (default 30); fails above
#                                    SCALE_BUDGET_KBPS (kbit/s in or out per node) when set
#   ./dialtone.sh scale-fleet [N] [secs]  N nodes on every fleet machine (default 10), one mesh
#                                    across them: convergence and bytes per node per machine
#   ./dialtone.sh deploy [opts]    build, install on every node, wait for every probe to pass
#                                    --skip-check   do not run `check` first
#                                    --release      every node downloads the latest GitHub release (SHA-256 checked)
#                                    --tag vX.Y.Z   ... or this release
#   ./dialtone.sh fleet            probe results of every node in the live mesh
#
# `deploy` and `fleet` read this fleet's hosts from fleet.local.env (never committed).
set -euo pipefail
cd "$(dirname "$0")"

BIN="${BIN:-./target/release/dialtone}"
FLEET_FILE="${FLEET_FILE:-fleet.local.env}"
LIVE_API="http://127.0.0.1:8788"

say() { echo "==> $*"; }
fail() { echo "FAIL: $*" >&2; exit 1; }
# until_ok <seconds> <what> <command...>: retry every second until the command succeeds.
until_ok() {
  local secs=$1 what=$2; shift 2
  for _ in $(seq 1 "$secs"); do "$@" >/dev/null 2>&1 && return 0; sleep 1; done
  fail "$what (gave up after ${secs}s)"
}
build() { cargo build --release; }
# One cleanup at a time (tests run one after another); it also runs if the script exits early.
CLEANUP=":"
trap 'eval "$CLEANUP"' EXIT
cleanup() { eval "$CLEANUP"; CLEANUP=":"; }

# -----------------------------------------------------------------------------
# lint
# -----------------------------------------------------------------------------
cmd_lint() {
  say "Leak gate: no host names or NodeIds in tracked files or history"
  local forbidden
  forbidden=$(python3 -c 'import base64; print(base64.b64decode("cm9zaWV8bGVnaW9ufGJ1aWxkLTF8amV0c29ufHN0ZWFtZGVja3w2NDQ2MzczOTc4NzJ8MDAxMmQ5NjdkMTY1fGZiZDViM2I3ZTRlMg==").decode())')
  # shellcheck disable=SC2046
  if git grep -qiE "$forbidden" -- $(git ls-files); then
    git grep -niE "$forbidden" -- $(git ls-files) | head -5 >&2
    fail "a host name or NodeId is in a tracked file"
  fi
  if [ -n "$(git log --all -i -G"$forbidden" --oneline 2>/dev/null)" ] \
    || [ -n "$(git log --all -i --grep="$forbidden" --oneline 2>/dev/null)" ]; then
    fail "a host name or NodeId is in git history"
  fi

  say "Domain language: one word per concept (goal.md), no banned synonyms in src/"
  # banned pattern (extended regex, case-insensitive) => word to use instead.
  # A line that must keep an old name (or names an iroh type) ends with `// compat`.
  local rules=(
    'heartbeat|(^|[^a-z0-9])hb([^a-z0-9]|$)|_hb([^a-z0-9]|$)|(^|[^a-z0-9])hb_=beat'
    'telemetry|host_?resources?=stats'
    'fleet|swarm=mesh'
    'artifact=blob'
    '(^|[^a-z0-9])cas([^a-z0-9]|$)=store'
    'blob_?cache|cache_dir|cache_root=store'
    'sub_?process|managed_?process|supervised_?process|(^|[^a-z])worker=process'
    '(^|[^0-9a-f])e2e([^0-9a-f]|$)|test[-_]tick|capability[-_ ]tick=probe'
    'dashboard=hud'
    'endpoint_ids?=node_id'
    'seed_peers?|bootstrap=seed / invite'
    'password=secret'
    'invite[_ ]?tickets?=invite'
  )
  local files status=0 rule hits
  files=$(ls src/*.rs src/hud/*.html src/hud/*.css src/hud/*.js | grep -v 'src/h3_land.rs')
  for rule in "${rules[@]}"; do
    # shellcheck disable=SC2086
    hits=$(grep -nEi "${rule%=*}" $files | grep -v '// compat' || true)
    if [ -n "$hits" ]; then
      echo "    say \"${rule##*=}\" (pattern: ${rule%=*}):"
      printf '%s\n' "$hits" | head -5 | cut -c1-160 | sed 's/^/      /'
      status=1
    fi
  done
  [ "$status" -eq 0 ] || fail "banned words in src/"
}

# -----------------------------------------------------------------------------
# check
# -----------------------------------------------------------------------------
cmd_check() {
  cmd_lint
  say "Format";  cargo fmt --check
  say "Tests";   cargo test
  say "Release build"; build
  node_test
  cmd_e2e
  say "ALL CHECKS PASSED"
}

# One node: CLI, secrets, allowlist, API auth, supervisor, blobs, probe, hud, routes.
node_test() {
  say "One-node test"
  local dir; dir=$(mktemp -d /tmp/dialtone-check-XXXXXX)
    local api="http://127.0.0.1:18794"
  # Stop the node and the processes its supervisor started, then remove its state.
  NODE_PID=""
  CLEANUP="kill \${NODE_PID:-} 2>/dev/null || true; pkill -f '$dir/run/' 2>/dev/null || true; rm -rf '$dir'"
  local d=(env DIALTONE_STATE_DIR="$dir" "$BIN")

  "${d[@]}" devices --json > /dev/null
  "${d[@]}" status --node node-alpha | grep -q '"steady_state_data_plane": "iroh-quic"'
  "${d[@]}" service --os macos --node node-alpha | grep -q 'com.timcash.dialtone'
  "${d[@]}" service --os linux --node node-alpha | grep -q 'EnvironmentFile=-/etc/dialtone/daemon.env'
  "${d[@]}" service --os windows --node node-beta | grep -q 'up --node node-beta'
  "${d[@]}" node --node node-alpha --no-dns --no-relay --once | grep -q '"schema": "dialtone.iroh.beat.v1"'
  local boot; boot=$("${d[@]}" --listen 127.0.0.1:0 --no-dns --no-relay --once)
  for stage in 1 2 3 4 5; do grep -q "\[BOOT $stage/5\]" <<<"$boot" || fail "boot stage $stage missing"; done
  echo "    CLI: devices, status, service units, beat, five boot stages"

  ! "${d[@]}" join dialtone --listen 127.0.0.1:19999 --once >/dev/null 2>&1 || fail "the channel name was accepted as a secret"
  ! "${d[@]}" join shortpass1 --listen 127.0.0.1:19999 --once >/dev/null 2>&1 || fail "a weak secret was accepted"
  local secret; secret=$("$BIN" secret)
  "${d[@]}" join "$secret" --listen 127.0.0.1:19999 --once | grep -q '"status": "saved_mesh_config"'
  ! grep -q "$secret" "$dir/config.json" || fail "the plaintext secret was written to config.json"
  grep -q '"derived_keys"' "$dir/config.json"
  echo "    Secrets: weak ones rejected, only derived keys stored"

  local peer="1111111111111111111111111111111111111111111111111111111111111111"
  "${d[@]}" allowlist add "$peer" --addr 127.0.0.1:19440 | grep -q '"status": "allowed"'
  "${d[@]}" allowlist list | grep -q "$peer"
  "${d[@]}" allowlist remove "$peer" | grep -q '"status": "revoked"'
  "${d[@]}" allowlist add "$peer" --addr 127.0.0.1:19440 > /dev/null
  "${d[@]}" invite --ttl 15m --no-dns --no-relay | grep -q '"invite": "dialtone-invite-v1:'
  echo "    Allowlist add / list / remove, invites"

    "${d[@]}" up --node check-node --listen 127.0.0.1:18794 --private-mesh > "$dir/node.log" 2>&1 &
  NODE_PID=$!
  until_ok 30 "node API up" curl -fsS "$api/api/processes"
  local token; token=$(tr -d '\r\n' < "$dir/api-token")
    node_post() { curl -fsS -X POST "$api$1" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$2"; }
  [ "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$api/api/blob" -d '{}')" = 401 ] || fail "unauthenticated POST was accepted"

  local program digest pid1 pid2
    # The program records its environment: the node's DIALTONE_* settings must not reach it.
  program=$(printf '#!/bin/sh\nenv > "%s/process.env"\nsleep 30\n' "$dir" | base64 | tr -d '\n')
  digest=$(node_post /api/blob "{\"bytes_base64\":\"$program\"}" | python3 -c 'import sys,json; print(json.load(sys.stdin)["digest"])')
  pid1=$(node_post /api/processes/blob "{\"name\":\"check-process\",\"digest\":\"$digest\",\"desired_state\":\"running\"}" \
    | python3 -c 'import sys,json; p=json.load(sys.stdin)["process"]; assert p["status"]=="running"; print(p["pid"])')
  kill -9 "$pid1"; sleep 0.3
  pid2=$(curl -fsS "$api/api/processes" | python3 -c 'import sys,json; p=[p for p in json.load(sys.stdin)["processes"] if p["name"]=="check-process"][0]; assert p["status"]=="running" and p["restart_count"]>=1; print(p["pid"])')
    [ "$pid1" != "$pid2" ] || fail "the supervisor did not restart the crashed process"
  grep -q '^APP_BIN=' "$dir/process.env" || fail "the process did not record its environment"
  ! grep -q '^DIALTONE_' "$dir/process.env" || fail "the node's DIALTONE_* environment reached a process"
  node_post /api/processes '{"name":"check-process","desired_state":"removed"}' | grep -q '"status":"removed"'
  ! ls "$dir"/run/check-process-* >/dev/null 2>&1 || fail "a removed process left its staged program"
    echo "    Supervisor: started from a blob without the node's env, restarted after kill -9, removed with its file"

  local ticket
  ticket=$(DIALTONE_STATE_DIR="$dir" "$BIN" blob share "check-blob" --listen 127.0.0.1:18794 | python3 -c 'import sys,json; print(json.load(sys.stdin)["ticket_uri"])')
  DIALTONE_STATE_DIR="$dir" "$BIN" blob pull "$ticket" --force --listen 127.0.0.1:18794 | grep -q '"method": "local-self"'
  DIALTONE_STATE_DIR="$dir" "$BIN" mesh-test --expect-peers 1 --timeout 5s --listen 127.0.0.1:18794 | grep -q '"status": "ok"'
  node_post /api/probe '{}' | python3 -c '
import sys, json
c = {x["code"]: x for x in json.load(sys.stdin)["probe"]["checks"]}
assert sorted(c) == ["BEAT", "MSH", "NBR"], c'
  node_post /api/probe '{"all":true}' | python3 -c '
import sys, json
p = json.load(sys.stdin)["probe"]
c = {x["code"]: x for x in p["checks"]}
assert sorted(c) == ["BEAT", "MSH", "NBR", "PRC", "PUL", "STO"] and c["STO"]["ok"] and c["PRC"]["ok"], c'
  echo "    Blobs share / pull, mesh-test, probe (gossip only, then full)"

  local before; before=$(shasum "$dir/config.json")
  DIALTONE_STATE_DIR="$dir" "$BIN" hud --node check-hud --listen 127.0.0.1:18795 --once > /dev/null 2>&1 &
  local hud=$!
  for _ in $(seq 1 50); do curl -fsS -o /dev/null http://127.0.0.1:18795/ 2>/dev/null && break; sleep 0.1; done
  wait "$hud" || true
  [ "$before" = "$(shasum "$dir/config.json")" ] || fail "dialtone hud rewrote the running node's config"
  for route in /api/deploy-plan /ws /api/gossip /api/artifacts/blob /api/e2e/status; do
    [ "$(curl -s -o /dev/null -w '%{http_code}' "$api$route")" = 404 ] || fail "removed route $route answered"
  done
    echo "    hud leaves the node's config alone; removed routes return 404"
  cleanup
}

# -----------------------------------------------------------------------------
# e2e: three real nodes on this machine
# -----------------------------------------------------------------------------
cmd_e2e() {
  say "Three-node test"
  [ -x "$BIN" ] || build
  local root; root=$(mktemp -d /tmp/dialtone-e2e-XXXXXX)
  local names=(e2e-a e2e-b e2e-c) quic=(19511 19512 19513) api=(18911 18912 18913)
    local ids=() ip="127.0.0.1" secret="dt-E2E_Local_Mesh_Secret_2026!" i j status
  E2E_PIDS=""
  CLEANUP="kill \${E2E_PIDS:-} 2>/dev/null || true; pkill -f '$root/' 2>/dev/null || true; rm -rf '$root'"
  get() { curl -fsS -m 20 "http://127.0.0.1:${api[$1]}$2"; }
  e2e_post() { curl -fsS -m 20 -X POST "http://127.0.0.1:${api[$1]}$2" -H "Content-Type: application/json" \
    -H "Authorization: Bearer $(cat "$root/${names[$1]}/api-token")" -d "${3:-{\}}"; }

  # Identities, then everyone allowlists everyone at a fixed QUIC port.
  for i in 0 1 2; do
    mkdir -p "$root/${names[$i]}"
    status=$(DIALTONE_STATE_DIR="$root/${names[$i]}" "$BIN" status --node "${names[$i]}" --private-mesh)
    ids[$i]=$(python3 -c 'import sys,json; print(json.loads(sys.argv[1])["diagnostics"]["node_id"])' "$status")
    ip=$(python3 -c 'import sys,json; print(json.loads(sys.argv[1])["diagnostics"]["bound_non_tailnet_ipv4"] or "127.0.0.1")' "$status")
  done
  for i in 0 1 2; do for j in 0 1 2; do
    [ "$i" = "$j" ] || DIALTONE_STATE_DIR="$root/${names[$i]}" "$BIN" allowlist add "${ids[$j]}" --addr "$ip:${quic[$j]}" >/dev/null
  done; done
  for i in 0 1 2; do
    DIALTONE_STATE_DIR="$root/${names[$i]}" "$BIN" up --node "${names[$i]}" --listen "127.0.0.1:${api[$i]}" \
            --private-mesh --quic-port "${quic[$i]}" --secret "$secret" > "$root/${names[$i]}.log" 2>&1 &
    E2E_PIDS="$E2E_PIDS $!"
  done

  sees_all() { get "$1" /api/mesh | python3 -c 'import sys,json; sys.exit(len(json.load(sys.stdin)["nodes"]) != 3)'; }
  for i in 0 1 2; do until_ok 60 "${names[$i]} sees 3 nodes" sees_all "$i"; done
  echo "    1. Mesh: 3 nodes see each other"

  # The gossip probe passes on every node, then a full probe pulls a fresh blob from each peer.
  gossip_passes() { e2e_post "$1" /api/probe | python3 -c '
import sys, json
p = json.load(sys.stdin)["probe"]
sys.exit(not (p["passed_count"] == p["total_count"] == 3))'; }
  full_passes() { e2e_post "$1" /api/probe '{"all":true}' | python3 -c '
import sys, json
p = json.load(sys.stdin)["probe"]
sys.exit(not (p["passed_count"] == p["total_count"] == 6 and sum(x["ok"] for x in p["pulls"]) == 2))'; }
  for i in 0 1 2; do until_ok 60 "${names[$i]} gossip probe 3/3" gossip_passes "$i"; done
  for i in 0 1 2; do until_ok 60 "${names[$i]} full probe 6/6 with 2 pulls" full_passes "$i"; done
  echo "    2. Probe: gossip checks 3/3 on every node; full probe 6/6, each pulled a fresh blob from both peers"
  get 0 "/api/details?node_id=${ids[1]}" | python3 -c '
import sys, json
d = json.load(sys.stdin)
assert d["node_id"] == sys.argv[1] and d["probe"]["checks"] and isinstance(d["devices"], list), d' "${ids[1]}"
  get 0 /api/net | python3 -c '
import sys, json
s = json.load(sys.stdin)["scores"]
assert s and not any(x["graylisted"] for x in s), s'
  echo "    Details: a fetched b's devices and probe from b; a scores its peers, none graylisted"

  local program
  program=$(printf ':; while true; do sleep 5; done; exit 0;\r\n@echo off\r\nREM e2e %s\r\n:loop\r\nping -n 6 127.0.0.1 >NUL\r\ngoto loop\r\n' "$root" | base64 | tr -d '\n')
  e2e_post 0 /api/processes/blob "{\"name\":\"e2e-program\",\"bytes_base64\":\"$program\",\"desired_state\":\"running\",\"deploy_to_mesh\":true}" >/dev/null
  runs_program() { get "$1" /api/processes | python3 -c '
import sys, json
sys.exit(not any(p["name"] == "e2e-program" and p.get("status") == "running" for p in json.load(sys.stdin)["processes"]))'; }
  lacks_program() { ! runs_program "$1"; }
  for i in 1 2; do until_ok 60 "${names[$i]} runs the deployed program" runs_program "$i"; done
  e2e_post 0 /api/processes '{"name":"e2e-program","desired_state":"removed"}' >/dev/null
  for i in 0 1 2; do until_ok 60 "${names[$i]} removed the program" lacks_program "$i"; done
  echo "    3. Deploy: ran on all 3 nodes, removing it on the origin removed every copy"

    kill "${E2E_PIDS##* }"
  a_sees_c_silent() { e2e_post 0 /api/probe | python3 -c '
import sys, json
beat = next(c for c in json.load(sys.stdin)["probe"]["checks"] if c["code"] == "BEAT")
sys.exit(beat["ok"] or "e2e-c" not in beat["detail"])'; }
  until_ok 45 "a's probe reports e2e-c as silent" a_sees_c_silent
    echo "    4. Failure: a's probe flagged e2e-c once it stopped"
  cleanup
}

# -----------------------------------------------------------------------------
# scale: many nodes on this machine, gossip only, bytes per node
# -----------------------------------------------------------------------------
cmd_scale() {
  local n=${1:-20} secs=${2:-30} budget=${SCALE_BUDGET_KBPS:-0}
  say "Scale test: $n nodes, measuring for ${secs}s"
  [ -x "$BIN" ] || build
  local root; root=$(mktemp -d /tmp/dialtone-scale-XXXXXX)
  local secret="dt-Scale_Local_Mesh_Secret_2026!" i ip="127.0.0.1"
  SCALE_PIDS=""
  # Nodes left by a run that was killed (no EXIT trap) would hold the ports.
  pkill -9 -f -- "--secret $secret" 2>/dev/null && sleep 1
  CLEANUP="kill \${SCALE_PIDS:-} 2>/dev/null || true; sleep 2; kill -9 \${SCALE_PIDS:-} 2>/dev/null || true; wait 2>/dev/null || true; rm -rf '$root'"
  name() { echo "s$1"; }

  # Identities (in parallel), then each node allowlists everyone but dials only 3 seeds.
  for i in $(seq 1 "$n"); do
    mkdir -p "$root/$(name "$i")"
    DIALTONE_STATE_DIR="$root/$(name "$i")" "$BIN" status --node "$(name "$i")" --private-mesh > "$root/$(name "$i").status" &
  done
  wait
  for i in $(seq 1 "$n"); do
    ip=$(python3 -c 'import sys,json; print(json.load(open(sys.argv[1]))["diagnostics"]["bound_non_tailnet_ipv4"] or "127.0.0.1")' "$root/$(name "$i").status")
  done
  python3 - "$root" "$n" "$ip" <<'PY'
import json, os, random, sys
root, n, ip = sys.argv[1], int(sys.argv[2]), sys.argv[3]
ids = {i: json.load(open(f"{root}/s{i}.status"))["diagnostics"]["node_id"] for i in range(1, n + 1)}
random.seed(n)
for i in range(1, n + 1):
    others = [j for j in ids if j != i]
    seeds = random.sample(others, min(3, len(others)))
    cfg = {
        "allowlist": [ids[j] for j in others],
        "seeds": [{"node_id": ids[j], "relay_url": "none", "ip_addrs": [f"{ip}:{20000 + j}"]} for j in seeds],
    }
    with open(f"{root}/s{i}/config.json", "w") as f:
        json.dump(cfg, f)
PY
  for i in $(seq 1 "$n"); do
    DIALTONE_STATE_DIR="$root/$(name "$i")" "$BIN" up --node "$(name "$i")" --listen "127.0.0.1:$((21000 + i))" \
      --private-mesh --quic-port "$((20000 + i))" --secret "$secret" > "$root/$(name "$i").log" 2>&1 &
    SCALE_PIDS="$SCALE_PIDS $!"
  done

  # Convergence: every node lists every node.
  local started; started=$(date +%s)
  converged() {
    local k
    for k in $(seq 1 "$n"); do
      curl -fsS -m 5 "http://127.0.0.1:$((21000 + k))/api/net" 2>/dev/null \
        | python3 -c 'import sys,json; sys.exit(json.load(sys.stdin)["nodes_known"] < int(sys.argv[1]))' "$n" 2>/dev/null || return 1
    done
  }
  report() {
    local k
    for k in $(seq 1 "$n"); do
      printf '    s%-4s ' "$k"
      curl -fsS -m 5 "http://127.0.0.1:$((21000 + k))/api/net" 2>/dev/null \
        | python3 -c 'import sys,json; d=json.load(sys.stdin); print("knows", d["nodes_known"], "neighbors", d["gossip"]["neighbors"])' \
        || echo "no answer"
    done
    echo "    last lines of s1's log:"; tail -n 15 "$root/s1.log" | sed 's/^/      /'
  }
  for _ in $(seq 1 180); do converged && break; sleep 1; done
  converged || { report; fail "all $n nodes see each other (gave up after 180s)"; }
  echo "    converged: every node sees all $n in $(( $(date +%s) - started ))s"

  snapshot() {
    local k
    for k in $(seq 1 "$n"); do
      curl -fsS -m 5 "http://127.0.0.1:$((21000 + k))/api/net" 2>/dev/null || echo '{}'
      echo
    done > "$1"
  }
  snapshot "$root/net-start.json"
  sleep "$secs"
  snapshot "$root/net-end.json"
  python3 - "$root" "$secs" "$budget" <<'PY'
import json, sys
root, secs, budget = sys.argv[1], float(sys.argv[2]), float(sys.argv[3])
def load(p): return [json.loads(l) for l in open(p) if l.strip()]
start, end = load(f"{root}/net-start.json"), load(f"{root}/net-end.json")
rows = []
for a, b in zip(start, end):
    if not a.get("socket") or not b.get("socket"):
        continue
    kin = (b["socket"]["bytes_in"] - a["socket"]["bytes_in"]) * 8 / secs / 1000
    kout = (b["socket"]["bytes_out"] - a["socket"]["bytes_out"]) * 8 / secs / 1000
    g0, g1 = a["gossip"], b["gossip"]
    msgs_in = (g1["gossip_msgs_in"] - g0["gossip_msgs_in"]) / secs
    out_n = g1["gossip_msgs_out"] - g0["gossip_msgs_out"]
    out_b = g1["gossip_bytes_out"] - g0["gossip_bytes_out"]
    w0, w1 = a.get("gossip_wire") or {}, b.get("gossip_wire") or {}
    wd = lambda k: (w1.get(k, 0) - w0.get(k, 0))
    rows.append((kin, kout, msgs_in, out_b / out_n if out_n else 0, g1["neighbors"], b["socket"]["conns_opened"],
                 wd("data_recv") / max(1, msgs_in * secs), wd("data_recv_bytes") * 8 / secs / 1000,
                 wd("ctrl_recv_bytes") * 8 / secs / 1000, (g1["gossip_bytes_in"] - g0["gossip_bytes_in"]) * 8 / secs / 1000))
avg = lambda xs: sum(xs) / len(xs)
cols = list(zip(*rows))
print("    per node (avg / max): in %.1f / %.1f kbit/s, out %.1f / %.1f kbit/s" % (avg(cols[0]), max(cols[0]), avg(cols[1]), max(cols[1])))
print("    gossip: %.1f msgs/s received per node, %.0f bytes per message sent, %.1f neighbors, %.1f connections opened since boot"
      % (avg(cols[2]), avg(cols[3]), avg(cols[4]), avg(cols[5])))
print("    gossip wire: payload %.1f kbit/s; %.2f data frames per message delivered, data %.1f kbit/s, control %.1f kbit/s"
      % (avg(cols[9]), avg(cols[6]), avg(cols[7]), avg(cols[8])))
worst = max(max(cols[0]), max(cols[1]))
if budget and worst > budget:
    print("    FAIL: %.1f kbit/s is over the budget of %.0f kbit/s" % (worst, budget))
    sys.exit(1)
PY
  cleanup
}

# -----------------------------------------------------------------------------
# fleet: hosts from fleet.local.env
# -----------------------------------------------------------------------------
load_fleet() {
  [ -f "$FLEET_FILE" ] || fail "$FLEET_FILE not found: it lists this fleet's hosts (see the deploy section of README.md)"
  # shellcheck disable=SC1090
  source "$FLEET_FILE"
  [ -n "${FLEET_PATH:-}" ] && export PATH="$FLEET_PATH:$PATH"
  return 0
}

# pick_routes: for each Windows node, print ROUTE_<node>=<first SSH alternative in SSH_<node>
# that answers> (SSH options followed by the destination).
pick_routes() {
  local node var alts alt a found
  for node in $WINDOWS_NODES; do
    var="SSH_${node//-/_}"
    alts="${!var:-$node}"
    found=""
    IFS='|' read -ra alt <<<"$alts"
    # Two rounds: a jump host that is slow to answer once should not fail a deploy.
    for a in "${alt[@]}" "${alt[@]}"; do
      # shellcheck disable=SC2086
      if ssh -o ConnectTimeout=5 -o BatchMode=yes $a "exit 0" >/dev/null 2>&1; then found="$a"; break; fi
    done
    [ -n "$found" ] || { echo "echo 'cannot reach $node over SSH (tried: $alts)' >&2; exit 1"; return 1; }
    printf 'ROUTE_%s=%q\n' "${node//-/_}" "$found"
  done
}
route() { local v="ROUTE_${1//-/_}"; echo "${!v}"; }
# win <node> <powershell>: run PowerShell on a Windows node.
win() {
  local b64; b64=$(printf '%s' "\$ProgressPreference = 'SilentlyContinue'; $2" | iconv -f UTF-8 -t UTF-16LE | base64 | tr -d '\r\n')
  # shellcheck disable=SC2086
    ssh $(route "$1") "powershell -NoProfile -NonInteractive -EncodedCommand $b64"
}
# copy_to <node> <local> <remote path under the user's home>, copy_from <node> <remote> <local>
scp_args() { local w=($(route "$1")); echo "${w[*]:0:${#w[@]}-1}"; }
scp_dest() { local w=($(route "$1")); echo "${w[${#w[@]}-1]}"; }
# shellcheck disable=SC2046
copy_to() { scp -q $(scp_args "$1") "$2" "$(scp_dest "$1"):$3"; }
# shellcheck disable=SC2046
copy_from() { scp -q $(scp_args "$1") "$(scp_dest "$1"):$2" "$3"; }

# Stop the node, install .dialtone\deploy-new\dialtone.exe, restart it at logon, print its SHA-256.
WIN_INSTALL='
  $d = "$env:USERPROFILE\.dialtone"
  cmd.exe /c "schtasks /End /TN DialtoneDaemon >nul 2>&1"
  cmd.exe /c "taskkill /IM dialtone.exe /T /F >nul 2>&1"
  Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -like "*start-dialtone.cmd*" -or $_.CommandLine -like "*\.dialtone\run\*" } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
  Start-Sleep -Milliseconds 800
  if (Test-Path "$d\bin\dialtone.exe") { Move-Item "$d\bin\dialtone.exe" "$d\bin\dialtone.exe.bak" -Force }
  Move-Item "$d\deploy-new\dialtone.exe" "$d\bin\dialtone.exe" -Force
  $cmd = "$d\start-dialtone.cmd"
  Set-Content -Path $cmd -Encoding Ascii -Value ("@echo off`r`n:loop`r`n`"%USERPROFILE%\.dialtone\bin\dialtone.exe`" up --node NODE --listen 127.0.0.1:8788 >> `"%USERPROFILE%\.dialtone\daemon.stdout.log`" 2>> `"%USERPROFILE%\.dialtone\daemon.stderr.log`"`r`ntimeout /t 2 /nobreak >nul`r`ngoto loop`r`n")
  schtasks /Create /F /SC ONLOGON /TN DialtoneDaemon /TR "conhost.exe --headless cmd.exe /c `"$cmd`"" | Out-Null
  schtasks /Run /TN DialtoneDaemon | Out-Null
  Write-Output ("SHA256=" + (Get-FileHash "$d\bin\dialtone.exe" -Algorithm SHA256).Hash.ToLower())
'
win_install() { win "$1" "\$ErrorActionPreference = 'Stop'; New-Item -ItemType Directory -Force -Path \"\$env:USERPROFILE\\.dialtone\\deploy-new\" | Out-Null; $2 ${WIN_INSTALL//NODE/$1}" | grep -o 'SHA256=[0-9a-f]*' | cut -d= -f2; }

cmd_deploy() {
  local skip_check=0 tag=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --skip-check) skip_check=1 ;;
      --release) tag="latest" ;;
      --tag) tag="$2"; shift ;;
      *) fail "unknown deploy option $1" ;;
    esac
    shift
  done
      load_fleet
  # node=sha256 of the binary installed on it; the gate waits for probes from exactly these.
  local expect="" sha
  # Published binaries: every node downloads its own and checks the published SHA-256.
  local releases="https://github.com/timcash/dialtone/releases/latest/download"
  [ -z "$tag" ] || [ "$tag" = latest ] || releases="https://github.com/timcash/dialtone/releases/download/$tag"
  # Find SSH routes while the local build or download runs (an offline route costs 3s).
  local routes_file; routes_file=$(mktemp)
  pick_routes > "$routes_file" &
  local routes_pid=$!

  local binary=target/release/dialtone
  if [ -n "$tag" ]; then
    say "Download release $tag for $LOCAL_NODE"
    [ "$(uname -sm)" = "Darwin arm64" ] || fail "$LOCAL_NODE: only macOS arm64 has a published local binary"
    binary=/tmp/dialtone-release
    curl -fsSL "$releases/dialtone-macos-arm64" -o "$binary"
    [ "$(shasum -a 256 "$binary" | cut -d' ' -f1)" = "$(curl -fsSL "$releases/dialtone-macos-arm64.sha256" | cut -d' ' -f1)" ] \
      || fail "the downloaded macOS binary does not match its published SHA-256"
    chmod +x "$binary"
    echo "    $("$binary" --help | head -1)  (SHA-256 matches the release)"
  elif [ "$skip_check" -eq 0 ]; then
    cmd_check
  else
    say "Release build"; build
  fi

  say "Install on $LOCAL_NODE"
  mkdir -p ~/.dialtone/bin
  [ -f ~/.dialtone/bin/dialtone ] && cp ~/.dialtone/bin/dialtone ~/.dialtone/bin/dialtone.bak
  cp "$binary" ~/.dialtone/bin/dialtone.new && mv ~/.dialtone/bin/dialtone.new ~/.dialtone/bin/dialtone
  launchctl kickstart -k "gui/$(id -u)/$LOCAL_LAUNCHD"
  sha=$(shasum -a 256 ~/.dialtone/bin/dialtone | cut -d' ' -f1)
  expect="$expect $LOCAL_NODE=$sha"
  echo "    $LOCAL_NODE ${sha:0:16}"

  wait "$routes_pid" || fail "a Windows node is unreachable over SSH"
  # shellcheck disable=SC1090
  source "$routes_file"; rm -f "$routes_file"
    local builder rest node
  read -r builder rest <<<"$WINDOWS_NODES"
  if [ -z "$tag" ]; then
    say "Build on $builder, install on $WINDOWS_NODES"
    COPYFILE_DISABLE=1 tar czf /tmp/dialtone-src.tgz Cargo.toml Cargo.lock .cargo src
    copy_to "$builder" /tmp/dialtone-src.tgz .dialtone/dialtone-src.tgz
    win "$builder" '
      $ErrorActionPreference = "Stop"
      $b = "$env:USERPROFILE\.dialtone\fleet-build"
      New-Item -ItemType Directory -Force -Path $b | Out-Null
      tar.exe -xzf "$env:USERPROFILE\.dialtone\dialtone-src.tgz" -C $b
      Set-Location $b
      & "$env:USERPROFILE\.cargo\bin\cargo.exe" build --release
      if ($LASTEXITCODE -ne 0) { throw "cargo build failed" }
      tar.exe -czf "$env:USERPROFILE\.dialtone\dialtone-windows.tgz" -C "$b\target\release" dialtone.exe'
    copy_from "$builder" .dialtone/dialtone-windows.tgz /tmp/dialtone-windows.tgz
    for node in $WINDOWS_NODES; do
      [ "$node" = "$builder" ] || copy_to "$node" /tmp/dialtone-windows.tgz .dialtone/dialtone-windows.tgz
            sha=$(win_install "$node" 'tar.exe -xzf "$env:USERPROFILE\.dialtone\dialtone-windows.tgz" -C "$env:USERPROFILE\.dialtone\deploy-new";')
      [ -n "$sha" ] || fail "install on $node did not report its binary"
      expect="$expect $node=$sha"
      echo "    $node ${sha:0:16}"
    done
    rm -f /tmp/dialtone-src.tgz /tmp/dialtone-windows.tgz
  else
    say "Download release $tag on $WINDOWS_NODES"
    # Download the published .exe on the node and refuse it unless its SHA-256 matches.
    local download
    download=$(cat <<'PS'
$n = "$env:USERPROFILE\.dialtone\deploy-new\dialtone.exe"
curl.exe -fsSL "URL" -o $n
if ($LASTEXITCODE -ne 0) { throw "download failed" }
$want = ((curl.exe -fsSL "URL.sha256") -split '\s+')[0].ToLower()
if ($want -ne (Get-FileHash $n -Algorithm SHA256).Hash.ToLower()) { throw "the download does not match its published SHA-256" }
PS
)
    download=${download//URL/$releases/dialtone-windows-x86_64.exe}
    for node in $WINDOWS_NODES; do
      sha=$(win_install "$node" "$download")
      [ -n "$sha" ] || fail "install on $node failed (download, SHA-256 check or restart)"
      expect="$expect $node=$sha"
      echo "    $node ${sha:0:16}  (SHA-256 matches the release)"
    done
  fi

  say "Waiting for every node's gossip probe to pass on the new build (up to 150s)"
  local passed=""
  for _ in $(seq 1 30); do
    fleet_report "$expect" > /tmp/dialtone-fleet.txt && { passed=1; break; }
    sleep 5
  done
  cat /tmp/dialtone-fleet.txt
  [ -n "$passed" ] || fail "not every node passed its probe within 150s (FAIL lines above)"
  # The store, pulls (a fresh blob from each peer) and programs, from this node.
  say "Full probe on $LOCAL_NODE"
  "$HOME/.dialtone/bin/dialtone" probe --all | python3 -c '
import sys, json
p = json.load(sys.stdin)["probe"]
for c in p["checks"]:
    print("    %-4s %s %s" % (c["code"], "ok  " if c["ok"] else "FAIL", c.get("detail", "")))
sys.exit(p["passed_count"] != p["total_count"])' || fail "the full probe on $LOCAL_NODE failed"
  say "DEPLOYED: every node passed its probe"
}

# fleet_report ["node=sha256 ..."]: every node's latest probe. Succeeds when every node's
# probe passed and, for each node given, came from the binary with that digest.
fleet_report() {
  curl -fsS "$LIVE_API/api/mesh" 2>/dev/null | python3 -c '
import sys, json
builds = dict(x.split("=", 1) for x in sys.argv[1].split())
nodes = {n["node"]: n for n in json.load(sys.stdin)["nodes"]}
ok = True
for name in sys.argv[2].split():
    p = (nodes.get(name) or {}).get("probe") or {}
    build = p.get("build", "").replace("sha256:", "")
    fails = "; ".join("%s: %s" % (c["code"], c["detail"]) for c in p.get("checks", []) if not c["ok"])
    on_build = name not in builds or build == builds[name]
    passed = name in nodes and on_build and p.get("total_count") and p.get("passed_count") == p.get("total_count")
    ok = ok and bool(passed)
    pulls = " ".join("%s %.0fms" % (x["peer"], x["ms"]) for x in p.get("pulls", []) if x["ok"])
    note = "missing from the mesh" if name not in nodes else ("" if on_build else "no probe from the new build yet")
    print("    %-8s build %-12s probe #%-4s %s/%s  %s%s%s" % (name, build[:12] or "?", p.get("cycle"), p.get("passed_count"),
          p.get("total_count"), "pulls: " + pulls if pulls else "", "  " + note if note else "", "  FAIL " + fails if fails else ""))
sys.exit(0 if ok else 1)' "${1:-}" "$FLEET_NODES"
}

cmd_fleet() { load_fleet; fleet_report "" || true; }

# -----------------------------------------------------------------------------
# scale-fleet: N nodes on every fleet machine, one mesh across them
# -----------------------------------------------------------------------------
# PowerShell on a Windows node. __PREFIX__, __N__, __URL__ and __SECRET__ are filled in per node.
# Scale nodes carry the scale secret on their command line (their state directory is only
# in their environment); the node's own daemon never does.
WIN_SCALE_STOP='
  Get-CimInstance Win32_Process | Where-Object { $_.Name -eq "dialtone.exe" -and $_.CommandLine -like "*Scale_Fleet_Mesh_Secret*" } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
  Start-Sleep -Milliseconds 800
  Remove-Item -Recurse -Force (Join-Path $env:TEMP "dialtone-scale") -ErrorAction SilentlyContinue
'
WIN_SCALE_IDS='
  $root = Join-Path $env:TEMP "dialtone-scale"; $exe = "$env:USERPROFILE\.dialtone\bin\dialtone.exe"
  1..__N__ | ForEach-Object {
    $dir = Join-Path $root "s$_"; New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $env:DIALTONE_STATE_DIR = $dir
    $st = (& $exe status --node "__PREFIX__-s$_" | Out-String) | ConvertFrom-Json
    Write-Output ("NODE __PREFIX__-s$_ " + $st.diagnostics.node_id + " " + $st.diagnostics.bound_non_tailnet_ipv4 + " " + $_)
  }
'
# Configs come from scale-configs.json (copied over first); nodes start through WMI so they
# outlive this SSH session.
WIN_SCALE_START='
  $root = Join-Path $env:TEMP "dialtone-scale"; $exe = "$env:USERPROFILE\.dialtone\bin\dialtone.exe"
  $configs = Get-Content -Raw "$env:USERPROFILE\.dialtone\scale-configs.json" | ConvertFrom-Json
  1..__N__ | ForEach-Object {
    $dir = Join-Path $root "s$_"
    [IO.File]::WriteAllText((Join-Path $dir "config.json"), $configs."s$_")
    $line = "cmd.exe /c set DIALTONE_STATE_DIR=$dir&& `"$exe`" up --node __PREFIX__-s$_ --listen 127.0.0.1:$(21000 + $_) --quic-port $(20000 + $_) --relay-url __URL__ --secret __SECRET__ > `"$dir\node.log`" 2>&1"
    Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = $line } | Out-Null
  }
'
WIN_SCALE_NET='
  1..__N__ | ForEach-Object {
    try { (Invoke-WebRequest -UseBasicParsing -TimeoutSec 5 "http://127.0.0.1:$(21000 + $_)/api/net").Content } catch { "{}" }
  }
'

scale_fleet_stop() {
  local node
  kill ${SCALE_PIDS:-} 2>/dev/null || true; sleep 2; kill -9 ${SCALE_PIDS:-} 2>/dev/null || true
  wait 2>/dev/null || true
  [ -n "${SCALE_ROOT:-}" ] && rm -rf "$SCALE_ROOT"
  for node in $WINDOWS_NODES; do win "$node" "$WIN_SCALE_STOP" >/dev/null 2>&1 || true; done
}

# scale_fleet_net <file>: every node's /api/net, one JSON line per node, "<machine> <json>".
scale_fleet_net() {
  local k node
  {
    for k in $(seq 1 "$SCALE_N"); do
      printf '%s ' "$LOCAL_NODE"; curl -fsS -m 5 "http://127.0.0.1:$((21000 + k))/api/net" 2>/dev/null || printf '{}'; echo
    done
    for node in $WINDOWS_NODES; do
      win "$node" "${WIN_SCALE_NET//__N__/$SCALE_N}" 2>/dev/null | tr -d '\r' | sed "/^$/d; s/^/$node /"
    done
  } > "$1"
}

cmd_scale_fleet() {
  local n=${1:-10} secs=${2:-60} node k
  local url="${SCALE_RELAY_URL:-https://usw1-1.relay.n0.iroh.link./}"
  local secret="dt-Scale_Fleet_Mesh_Secret_2026!"
  load_fleet
  [ -x "$BIN" ] || build
  say "Fleet scale test: $n nodes on each of: $FLEET_NODES (relay $url), measuring for ${secs}s"
  eval "$(pick_routes)"
  SCALE_N=$n SCALE_PIDS="" SCALE_ROOT=$(mktemp -d /tmp/dialtone-scale-XXXXXX)
  CLEANUP="scale_fleet_stop"
  pkill -9 -f -- "--secret $secret" 2>/dev/null || true
  for node in $WINDOWS_NODES; do win "$node" "$WIN_SCALE_STOP" >/dev/null 2>&1 || true; done

  # Identities on every machine: "NODE <name> <node_id> <ip> <index>" lines.
  local ids="$SCALE_ROOT/ids.txt" ip
  : > "$ids"
  for k in $(seq 1 "$n"); do
    mkdir -p "$SCALE_ROOT/s$k"
    DIALTONE_STATE_DIR="$SCALE_ROOT/s$k" "$BIN" status --node "$LOCAL_NODE-s$k" > "$SCALE_ROOT/s$k.status" &
  done
  wait
  for k in $(seq 1 "$n"); do
    python3 -c 'import sys,json; d=json.load(open(sys.argv[1]))["diagnostics"]; print("NODE", sys.argv[2], d["node_id"], d["bound_non_tailnet_ipv4"] or "127.0.0.1", sys.argv[3], sys.argv[4])' \
      "$SCALE_ROOT/s$k.status" "$LOCAL_NODE-s$k" "$k" "$LOCAL_NODE" >> "$ids"
  done
  for node in $WINDOWS_NODES; do
    local script="${WIN_SCALE_IDS//__PREFIX__/$node}"
    win "$node" "${script//__N__/$n}" | tr -d '\r' | grep '^NODE ' | sed "s/\$/ $node/" >> "$ids"
  done
  local total; total=$(grep -c '^NODE ' "$ids")
  [ "$total" -eq $((n * $(echo $FLEET_NODES | wc -w))) ] || fail "expected $((n * $(echo $FLEET_NODES | wc -w))) identities, got $total"

  # Configs: everyone allowlists everyone; 3 random seeds anywhere in the fleet, reached
  # directly (LAN address) or through the relay.
  python3 - "$SCALE_ROOT" "$ids" "$url" <<'PY'
import json, random, sys
root, ids_file, url = sys.argv[1:4]
nodes = [l.split() for l in open(ids_file) if l.startswith("NODE ")]  # NODE name id ip index machine
random.seed(len(nodes))
by_machine = {}
for _, name, nid, ip, idx, machine in nodes:
    others = [o for o in nodes if o[2] != nid]
    seeds = random.sample(others, min(3, len(others)))
    cfg = {
        "allowlist": [o[2] for o in others],
        "seeds": [{"node_id": o[2], "relay_url": url, "ip_addrs": [f"{o[3]}:{20000 + int(o[4])}"]} for o in seeds],
    }
    by_machine.setdefault(machine, {})[f"s{idx}"] = json.dumps(cfg)
for machine, cfgs in by_machine.items():
    json.dump(cfgs, open(f"{root}/configs-{machine}.json", "w"))
PY
  python3 -c 'import json,sys; [open(f"{sys.argv[1]}/{k}/config.json","w").write(v) for k,v in json.load(open(sys.argv[2])).items()]' \
    "$SCALE_ROOT" "$SCALE_ROOT/configs-$LOCAL_NODE.json"
  for node in $WINDOWS_NODES; do
    copy_to "$node" "$SCALE_ROOT/configs-$node.json" ".dialtone/scale-configs.json"
  done

  # Start every node.
  for k in $(seq 1 "$n"); do
    DIALTONE_STATE_DIR="$SCALE_ROOT/s$k" "$BIN" up --node "$LOCAL_NODE-s$k" --listen "127.0.0.1:$((21000 + k))" \
      --quic-port "$((20000 + k))" --relay-url "$url" --secret "$secret" > "$SCALE_ROOT/s$k.log" 2>&1 &
    SCALE_PIDS="$SCALE_PIDS $!"
  done
  for node in $WINDOWS_NODES; do
    local script="${WIN_SCALE_START//__PREFIX__/$node}"
    script="${script//__N__/$n}"
    script="${script//__URL__/$url}"
    win "$node" "${script//__SECRET__/$secret}" > /dev/null
  done
  echo "    started $total nodes"

  # Convergence: every node lists every node.
  local started; started=$(date +%s)
  local snap="$SCALE_ROOT/net.txt"
  fleet_converged() {
    scale_fleet_net "$snap"
    python3 - "$snap" "$total" <<'PY'
import json, sys
rows = [l.split(" ", 1) for l in open(sys.argv[1]) if l.strip()]
known = [json.loads(j).get("nodes_known", 0) for _, j in rows]
print("    %d/%d nodes answer, nodes known: min %d, max %d" % (sum(1 for k in known if k), int(sys.argv[2]), min(known or [0]), max(known or [0])))
sys.exit(not (len(known) == int(sys.argv[2]) and min(known) >= int(sys.argv[2])))
PY
  }
  local ok=""
  for _ in $(seq 1 30); do
    fleet_converged > "$SCALE_ROOT/conv.txt" && { ok=1; break; }
    sleep 5
  done
  cat "$SCALE_ROOT/conv.txt"
  [ -n "$ok" ] || fail "the fleet mesh of $total nodes did not converge within about 4 minutes"
  echo "    converged: every node sees all $total in $(( $(date +%s) - started ))s"

  scale_fleet_net "$SCALE_ROOT/net-start.txt"
  sleep "$secs"
  scale_fleet_net "$SCALE_ROOT/net-end.txt"
  python3 - "$SCALE_ROOT" "$secs" <<'PY'
import json, sys
root, secs = sys.argv[1], float(sys.argv[2])
def load(p):
    out = {}
    for i, l in enumerate(open(p)):
        if l.strip():
            machine, js = l.rstrip("\n").split(" ", 1)
            d = json.loads(js)
            if d.get("node"):
                out[d["node"]] = (machine, d)
    return out
a, b = load(f"{root}/net-start.txt"), load(f"{root}/net-end.txt")
per = {}
for name, (machine, y) in b.items():
    if name not in a: continue
    x = a[name][1]
    f = lambda k, s="socket": y[s][k] - x[s][k]
    per.setdefault(machine, []).append((f("bytes_in") * 8 / secs / 1000, f("bytes_out") * 8 / secs / 1000,
        f("gossip_msgs_in", "gossip") / secs, y["gossip"]["neighbors"],
        sum(1 for s in y.get("scores", []) if s.get("graylisted")),
        f("gossip_bytes_in", "gossip") * 8 / secs / 1000,
        f("data_recv", "gossip_wire") / max(1, f("gossip_msgs_in", "gossip")),
        f("data_recv_bytes", "gossip_wire") * 8 / secs / 1000,
        f("ctrl_recv_bytes", "gossip_wire") * 8 / secs / 1000))
allrows = [r for rows in per.values() for r in rows]
avg = lambda xs: sum(xs) / len(xs)
for machine, rows in sorted(per.items()) + [("all", allrows)]:
    c = list(zip(*rows))
    print("    %-8s %3d nodes: in %.1f / %.1f kbit/s (avg / max; gossip payload %.1f), out %.1f / %.1f, %.1f msgs/s, %.1f neighbors, %d graylisted"
          % (machine, len(rows), avg(c[0]), max(c[0]), avg(c[5]), avg(c[1]), max(c[1]), avg(c[2]), avg(c[3]), sum(c[4])))
    print("    %-8s            gossip wire: %.2f data frames per message delivered, data %.1f kbit/s, control %.1f kbit/s"
          % ("", avg(c[6]), avg(c[7]), avg(c[8])))
PY
  cleanup
}


case "${1:-help}" in
  check) cmd_check ;;
  lint) cmd_lint ;;
  e2e) cmd_e2e ;;
  scale) shift; cmd_scale "$@" ;;
  scale-fleet) shift; cmd_scale_fleet "$@" ;;
  deploy) shift; cmd_deploy "$@" ;;
  fleet) cmd_fleet ;;
  help|-h|--help) awk 'NR > 1 && /^#/ { sub(/^# ?/, ""); print; next } NR > 1 { exit }' "$0" ;;
  *) fail "unknown command $1 (try ./dialtone.sh help)" ;;
esac
