#!/usr/bin/env bash
# dialtone.sh — the one script for working on dialtone.
#
#   ./dialtone.sh check            lint, fmt, tests, release build, one-node test, three-node test
#   ./dialtone.sh lint             leak gate (no host names / NodeIds in git) and domain language
#   ./dialtone.sh e2e              three real nodes on this machine (about 30s)
#   ./dialtone.sh deploy [opts]    build, install on every node, wait for every probe to pass
#                                    --skip-check   do not run `check` first
#                                    --release      Windows nodes download the latest GitHub release
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
p = json.load(sys.stdin)["probe"]
c = {x["code"]: x for x in p["checks"]}
assert sorted(c) == ["BEAT", "MSH", "PRC", "PUL", "STO"] and c["STO"]["ok"] and c["PRC"]["ok"], c'
  echo "    Blobs share / pull, mesh-test, probe"

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

  # A pull needs the peer's previous probe blob, so probe until each node passes everything.
  probe_passes() { e2e_post "$1" /api/probe | python3 -c '
import sys, json
p = json.load(sys.stdin)["probe"]
sys.exit(not (p["passed_count"] == p["total_count"] == 5 and sum(x["ok"] for x in p["pulls"]) == 2))'; }
  for i in 0 1 2; do until_ok 60 "${names[$i]} probe 5/5 with 2 pulls" probe_passes "$i"; done
  echo "    2. Probe: 5/5 on every node, each pulled both peers' blobs over QUIC"

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
    for a in "${alt[@]}"; do
      # shellcheck disable=SC2086
      if ssh -o ConnectTimeout=3 -o BatchMode=yes $a "exit 0" >/dev/null 2>&1; then found="$a"; break; fi
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
  local started; started=$(date +%s)
  # Find SSH routes while the local build runs (an offline route costs a 3s timeout).
  local routes_file; routes_file=$(mktemp)
    pick_routes > "$routes_file" &
  local routes_pid=$!

  if [ "$skip_check" -eq 0 ]; then cmd_check; else say "Release build"; build; fi

  say "Install on $LOCAL_NODE"
  mkdir -p ~/.dialtone/bin
  [ -f ~/.dialtone/bin/dialtone ] && cp ~/.dialtone/bin/dialtone ~/.dialtone/bin/dialtone.bak
  cp target/release/dialtone ~/.dialtone/bin/dialtone.new && mv ~/.dialtone/bin/dialtone.new ~/.dialtone/bin/dialtone
  launchctl kickstart -k "gui/$(id -u)/$LOCAL_LAUNCHD"
  echo "    $LOCAL_NODE $(shasum -a 256 ~/.dialtone/bin/dialtone | cut -c1-16)"

  wait "$routes_pid" || fail "a Windows node is unreachable over SSH"
  # shellcheck disable=SC1090
  source "$routes_file"; rm -f "$routes_file"
  local builder rest node sha
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
      echo "    $node ${sha:0:16}"
    done
    rm -f /tmp/dialtone-src.tgz /tmp/dialtone-windows.tgz
  else
    local url="https://github.com/timcash/dialtone/releases/latest/download/dialtone-windows-x86_64.exe"
    [ "$tag" = latest ] || url="https://github.com/timcash/dialtone/releases/download/$tag/dialtone-windows-x86_64.exe"
    say "Install release $tag on $WINDOWS_NODES"
    for node in $WINDOWS_NODES; do
      sha=$(win_install "$node" "curl.exe -fsSL '$url' -o \"\$env:USERPROFILE\\.dialtone\\deploy-new\\dialtone.exe\"; if (\$LASTEXITCODE -ne 0) { throw 'download failed' };")
      echo "    $node ${sha:0:16}"
    done
  fi

  say "Waiting for every node's probe to pass 5/5 on the new build (up to 150s)"
  for _ in $(seq 1 30); do
    fleet_report "$started" > /tmp/dialtone-fleet.txt && { cat /tmp/dialtone-fleet.txt; say "DEPLOYED: every node passed its probe"; return 0; }
    sleep 5
  done
  cat /tmp/dialtone-fleet.txt
  fail "not every node passed its probe within 150s (FAIL lines above)"
}

# fleet_report [since]: every node's latest probe; succeeds when all nodes passed one run after `since`.
fleet_report() {
  curl -fsS "$LIVE_API/api/mesh" 2>/dev/null | python3 -c '
import sys, json
since, expected = int(sys.argv[1]), sys.argv[2].split()
nodes = {n["node"]: n for n in json.load(sys.stdin)["nodes"]}
ok = True
for name in expected:
    p = (nodes.get(name) or {}).get("probe") or {}
    fails = "; ".join("%s: %s" % (c["code"], c["detail"]) for c in p.get("checks", []) if not c["ok"])
    fresh = p.get("updated_at_unix", 0) >= since
    passed = name in nodes and fresh and p.get("passed_count") == p.get("total_count") and p.get("total_count")
    ok = ok and bool(passed)
    pulls = " ".join("%s %.0fms" % (x["peer"], x["ms"]) for x in p.get("pulls", []) if x["ok"])
    note = "missing from the mesh" if name not in nodes else ("" if fresh else "no probe since deploy yet")
    print("    %-8s probe #%-4s %s/%s  %s%s%s" % (name, p.get("cycle"), p.get("passed_count"), p.get("total_count"),
          "pulls: " + pulls if pulls else "", "  " + note if note else "", "  FAIL " + fails if fails else ""))
sys.exit(0 if ok else 1)' "${1:-0}" "$FLEET_NODES"
}

cmd_fleet() { load_fleet; fleet_report 0 || true; }

case "${1:-help}" in
  check) cmd_check ;;
  lint) cmd_lint ;;
  e2e) cmd_e2e ;;
  deploy) shift; cmd_deploy "$@" ;;
  fleet) cmd_fleet ;;
  help|-h|--help) awk 'NR > 1 && /^#/ { sub(/^# ?/, ""); print; next } NR > 1 { exit }' "$0" ;;
  *) fail "unknown command $1 (try ./dialtone.sh help)" ;;
esac
