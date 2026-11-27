# Metal Fleet Migration Runbook: Channel-Scoped Plugins, Signed Manifest, and Channel Logs (P6)

**Status:** PLAN ONLY - DO NOT RUN WITHOUT TIM'S EXPLICIT APPROVAL AT EVERY GATE.
**Target Channel:** `metal` (`channel_id`: `ch-b39d38af727e`)
**Target Releases:**
- `timcash/dialtone`: `v0.6.0` (channel plugin keys, `ChannelBlobsProtocol` ACL, streamed >= 8 MB blobs, encrypted channel logs, signed `dialtone.channel.manifest.v1`, `GET /api/channels`)
- `timcash/dialtone-task`: `v0.5.0` (channel-derived mesh key via `blake3::derive_key("dialtone-task/mesh/v1", plugin_key)`, per-channel data dir, `logs --channel`)
- `timcash/dialtone-agent`: `v0.5.0` (opt-in `channels`, fail-closed manifest authorization for `agent`, per-channel task RPC and work dirs)

**Permanent Frozen KDF Labels:**
- `dialtone/plugin-key/v1/<plugin>` (HKDF-SHA256 in `dialtone`)
- `dialtone/channel-log/v1` (HKDF-SHA256 in `dialtone`)
- `dialtone-task/mesh/v1` (`blake3::derive_key` in `dialtone-task`)

**Fleet Inventory (7 Nodes) and Upgrade Order:**

| Wave | Node | OS / Arch | Role |
|---|---|---|---|
| 1 (Canary) | `rosie` | macOS arm64 | Tim's Mac, orchestrator, channel manifest creator/admin |
| 2 | `legion` | Windows x86_64 | Primary Windows worker |
| 2 | `build-1` | Linux | Build VM |
| 3 | `vm-us` | Linux | US VM |
| 3 | `vm-eu` | Linux | EU VM |
| 3 | `vm-asia` | Linux | Asia VM |
| 3 | `steamdeck-2` | Linux | Portable Linux node |

**Hard Safety Rules During Migration:**
- Never print, echo, log, or paste secrets, `mesh.key`, `iroh-secret-key`, `api-token`, `rpc.token`, `derived_keys`, blob tickets, or invite strings into chat, task notes, or terminal transcripts. Copy key files directly on disk only.
- Never delete dated backup directories until after full post-migration soak verification.
- Stop immediately and trigger rollback if any node diverges on `state_hash`, loses any pre-migration task ID, or fails manifest signature verification.

---

## Blocking Prerequisite: Dual-Key Bridge in `dialtone-task`

> **BLOCKING PREREQUISITE (DO NOT RUN ROLLING MIGRATION UNTIL RESOLVED):**
> As of `dialtone-task v0.5.0` (merged from `feat/channel-scope`), `dialtone-task` supports **either** the channel-derived key (`DIALTONE_CHANNEL_PLUGIN_KEY`) **or** legacy `<data_dir>/mesh.key` fallback, **not both topics simultaneously**.
>
> Upgrading nodes one by one without a dual-key bridge would partition `v0.5.0` nodes (channel-derived topic) from not-yet-upgraded `v0.4.0` nodes (legacy `mesh.key` topic) during the rollout window.

### Required Implementation Before Running Rolling Migration

Implement a dual-key transition mode in `dialtone-task` (enabled via `--dual-key-legacy <path-to-legacy-mesh.key>` or `DIALTONE_TASK_LEGACY_MESH_KEY=<path>`):
1. Subscribe to **both** the channel-derived gossip topic (`blake3::derive_key("dialtone-task/mesh/v1", plugin_key)`) and the legacy `mesh.key` topic simultaneously.
2. Accept and verify CRDT updates received on either topic, merging into the single channel `state.redb`.
3. Re-seal every newly accepted or locally written update onto the channel-derived topic (and optionally mirror to legacy topic during the window) so upgraded and un-upgraded peers converge on the exact same `state_hash` with zero task loss.
4. Once `DIALTONE_TASK_LEGACY_MESH_KEY` is unset and the legacy file is removed, unsubscribe from the legacy topic permanently.

### Required Proof Test (Must Be Green Before Gate 1)

Add and pass the following integration test in `timcash/dialtone-task` before executing this runbook:

- Test name: `dual_key_bridge_syncs_legacy_v0_4_peer_to_channel_scoped_v0_5_peer_and_stops_after_close`
- Must prove:
  1. Node A runs with legacy `mesh.key` only (simulating `v0.4.0`) holding 41+ seeded tasks.
  2. Node B runs with `DIALTONE_CHANNEL_PLUGIN_KEY` (for `ch-b39d38af727e`) + `DIALTONE_TASK_LEGACY_MESH_KEY` pointing to the legacy key file.
  3. Node C runs with `DIALTONE_CHANNEL_PLUGIN_KEY` only (no legacy key).
  4. Tasks created on Node A propagate through Node B to Node C and all 3 converge to the exact same `state_hash` and `task_count >= 41`.
  5. After closing the dual-key window on Node B (restarting without `DIALTONE_TASK_LEGACY_MESH_KEY`), a new task on Node A does NOT reach Node B or Node C, while tasks between Node B and Node C continue syncing cleanly.

*(Alternative only if Tim explicitly approves: a 5-minute maintenance window stopping `dialtone-task` on all 7 nodes, verifying identical `state_hash`, cutting all 7 over together, and starting simultaneously. Rolling zero-downtime migration requires the dual-key bridge above.)*

---

## Gate 0: Pre-Flight Audit (All 7 Nodes)

Record versions, peer counts, `task_count`, `state_hash`, and the full list of existing task IDs on every node before modifying anything. Confirm `task_count >= 41` and identical `state_hash` across all 7 nodes.

### macOS (`rosie`)
```bash
TS=$(date +%Y%m%d-%H%M)
PRE="$HOME/dialtone-preflight-$TS"
mkdir -p "$PRE"
"$HOME/.dialtone/bin/dialtone" --version 2>&1 || true
"$HOME/.dialtone/plugins/task/dialtone-task" --version 2>&1 | tee "$PRE/versions.txt"
"$HOME/.dialtone/plugins/agent/dialtone-agent" --version 2>&1 | tee -a "$PRE/versions.txt"
"$HOME/.dialtone/plugins/task/dialtone-task" status --json | tee "$PRE/task-status.json"
"$HOME/.dialtone/plugins/task/dialtone-task" list --all --json | tee "$PRE/task-list.json"
curl -s http://127.0.0.1:8788/api/mesh | tee "$PRE/mesh.json" > /dev/null
```

### Linux (`build-1`, `vm-us`, `vm-eu`, `vm-asia`, `steamdeck-2`)
```bash
TS=$(date +%Y%m%d-%H%M)
PRE="$HOME/dialtone-preflight-$TS"
mkdir -p "$PRE"
"$HOME/.dialtone/bin/dialtone" status > "$PRE/dialtone-status.json"
"$HOME/.dialtone/plugins/task/dialtone-task" --version 2>&1 | tee "$PRE/versions.txt"
"$HOME/.dialtone/plugins/agent/dialtone-agent" --version 2>&1 | tee -a "$PRE/versions.txt"
"$HOME/.dialtone/plugins/task/dialtone-task" status --json | tee "$PRE/task-status.json"
"$HOME/.dialtone/plugins/task/dialtone-task" list --all --json | tee "$PRE/task-list.json"
curl -s http://127.0.0.1:8788/api/mesh > "$PRE/mesh.json"
```

### Windows PowerShell (`legion`)
```powershell
$ts = Get-Date -Format "yyyyMMdd-HHmm"
$pre = "$env:USERPROFILE\dialtone-preflight-$ts"
New-Item -ItemType Directory -Force -Path $pre | Out-Null
& "$env:USERPROFILE\.dialtone\plugins\task\dialtone-task.exe" --version | Tee-Object "$pre\versions.txt"
& "$env:USERPROFILE\.dialtone\plugins\agent\dialtone-agent.exe" --version | Tee-Object -Append "$pre\versions.txt"
& "$env:USERPROFILE\.dialtone\plugins\task\dialtone-task.exe" status --json | Tee-Object "$pre\task-status.json"
& "$env:USERPROFILE\.dialtone\plugins\task\dialtone-task.exe" list --all --json | Tee-Object "$pre\task-list.json"
Invoke-RestMethod -Uri "http://127.0.0.1:8788/api/mesh" | ConvertTo-Json -Depth 10 | Set-Content "$pre\mesh.json"
```

**Pre-flight pass criteria:**
- Every node reports `task_count >= 41`.
- All 7 nodes report the exact same `state_hash`.
- All 7 nodes see `6` peers on `metal` (`channel_fingerprint == ch-b39d38af727e`).

---

## Gate 1: Full Backups and Checksum Verification (All 7 Nodes)

Copy `state.redb`, `config.json`, `mesh.key` (copy only, never print!), `iroh-secret-key` (copy only, never print!), plugin manifests, and existing binaries into a dated backup directory outside `~/.dialtone` (so clients never scan backups). Verify SHA-256 checksums for every copied file.

### macOS (`rosie`)
```bash
TS=$(date +%Y%m%d-%H%M)
BK="$HOME/dialtone-backup-$TS"
mkdir -p "$BK/bin" "$BK/dialtone" "$BK/task" "$BK/agent"
chmod 700 "$BK"

cp -p "$HOME/.dialtone/bin/dialtone" "$BK/bin/"
cp -p "$HOME/.dialtone/config.json" "$BK/dialtone/"
cp -p "$HOME/.dialtone/iroh-secret-key" "$BK/dialtone/"
[ -d "$HOME/.dialtone/channels" ] && cp -Rp "$HOME/.dialtone/channels" "$BK/dialtone/"

cp -p "$HOME/.dialtone/plugins/task/dialtone-task" "$BK/task/"
cp -p "$HOME/.dialtone/plugins/task/manifest.json" "$BK/task/" 2>/dev/null || true
cp -p "$HOME/.dialtone/plugins/task/config.json" "$BK/task/"
cp -p "$HOME/.dialtone/plugins/task/state.redb" "$BK/task/"
cp -p "$HOME/.dialtone/plugins/task/mesh.key" "$BK/task/"
cp -p "$HOME/.dialtone/plugins/task/iroh-secret-key" "$BK/task/" 2>/dev/null || true
[ -d "$HOME/.dialtone/plugins/task/blobs" ] && cp -Rp "$HOME/.dialtone/plugins/task/blobs" "$BK/task/"

cp -p "$HOME/.dialtone/plugins/agent/dialtone-agent" "$BK/agent/" 2>/dev/null || true
cp -p "$HOME/.dialtone/plugins/agent/manifest.json" "$BK/agent/" 2>/dev/null || true
cp -p "$HOME/.dialtone/plugins/agent/config.json" "$BK/agent/" 2>/dev/null || true
cp -p "$HOME/.dialtone/plugins/agent/policy.toml" "$BK/agent/" 2>/dev/null || true

(cd "$BK" && find . -type f ! -name SHA256SUMS -print0 | xargs -0 shasum -a 256 > SHA256SUMS)
(cd "$BK" && shasum -a 256 -c SHA256SUMS)
```

### Linux (`build-1`, `vm-us`, `vm-eu`, `vm-asia`, `steamdeck-2`)
```bash
TS=$(date +%Y%m%d-%H%M)
BK="$HOME/dialtone-backup-$TS"
mkdir -p "$BK/bin" "$BK/dialtone" "$BK/task" "$BK/agent"
chmod 700 "$BK"

cp -p "$HOME/.dialtone/bin/dialtone" "$BK/bin/"
cp -p "$HOME/.dialtone/config.json" "$BK/dialtone/"
cp -p "$HOME/.dialtone/iroh-secret-key" "$BK/dialtone/"
[ -d "$HOME/.dialtone/channels" ] && cp -a "$HOME/.dialtone/channels" "$BK/dialtone/"

cp -p "$HOME/.dialtone/plugins/task/dialtone-task" "$BK/task/"
cp -p "$HOME/.dialtone/plugins/task/manifest.json" "$BK/task/" 2>/dev/null || true
cp -p "$HOME/.dialtone/plugins/task/config.json" "$BK/task/"
cp -p "$HOME/.dialtone/plugins/task/state.redb" "$BK/task/"
cp -p "$HOME/.dialtone/plugins/task/mesh.key" "$BK/task/"
cp -p "$HOME/.dialtone/plugins/task/iroh-secret-key" "$BK/task/" 2>/dev/null || true
[ -d "$HOME/.dialtone/plugins/task/blobs" ] && cp -a "$HOME/.dialtone/plugins/task/blobs" "$BK/task/"

cp -p "$HOME/.dialtone/plugins/agent/dialtone-agent" "$BK/agent/" 2>/dev/null || true
cp -p "$HOME/.dialtone/plugins/agent/manifest.json" "$BK/agent/" 2>/dev/null || true
cp -p "$HOME/.dialtone/plugins/agent/config.json" "$BK/agent/" 2>/dev/null || true
cp -p "$HOME/.dialtone/plugins/agent/policy.toml" "$BK/agent/" 2>/dev/null || true

(cd "$BK" && find . -type f ! -name SHA256SUMS -print0 | xargs -0 sha256sum > SHA256SUMS)
(cd "$BK" && sha256sum -c SHA256SUMS)
```

### Windows PowerShell (`legion`)
```powershell
$ts = Get-Date -Format "yyyyMMdd-HHmm"
$bk = "$env:USERPROFILE\dialtone-backup-$ts"
New-Item -ItemType Directory -Force -Path "$bk\bin", "$bk\dialtone", "$bk\task", "$bk\agent" | Out-Null

Copy-Item "$env:USERPROFILE\.dialtone\bin\dialtone.exe" "$bk\bin\"
Copy-Item "$env:USERPROFILE\.dialtone\config.json" "$bk\dialtone\"
Copy-Item "$env:USERPROFILE\.dialtone\iroh-secret-key" "$bk\dialtone\"
if (Test-Path "$env:USERPROFILE\.dialtone\channels") { Copy-Item -Recurse "$env:USERPROFILE\.dialtone\channels" "$bk\dialtone\" }

Copy-Item "$env:USERPROFILE\.dialtone\plugins\task\dialtone-task.exe" "$bk\task\"
foreach ($f in @("manifest.json", "config.json", "state.redb", "mesh.key", "iroh-secret-key")) {
    $src = "$env:USERPROFILE\.dialtone\plugins\task\$f"
    if (Test-Path $src) { Copy-Item $src "$bk\task\" }
}
if (Test-Path "$env:USERPROFILE\.dialtone\plugins\task\blobs") { Copy-Item -Recurse "$env:USERPROFILE\.dialtone\plugins\task\blobs" "$bk\task\" }

foreach ($f in @("dialtone-agent.exe", "manifest.json", "config.json", "policy.toml")) {
    $src = "$env:USERPROFILE\.dialtone\plugins\agent\$f"
    if (Test-Path $src) { Copy-Item $src "$bk\agent\" }
}

$sums = Get-ChildItem -Path $bk -Recurse -File | Where-Object { $_.Name -ne "SHA256SUMS.txt" } | ForEach-Object {
    $h = Get-FileHash -Algorithm SHA256 -Path $_.FullName
    "$($h.Hash.ToLower())  $($_.FullName.Substring($bk.Length + 1))"
}
$sums | Set-Content "$bk\SHA256SUMS.txt"
# Re-verify every checksum against disk
$failed = 0
foreach ($line in Get-Content "$bk\SHA256SUMS.txt") {
    $parts = $line -split "  ", 2
    $obs = (Get-FileHash -Algorithm SHA256 -Path (Join-Path $bk $parts[1])).Hash.ToLower()
    if ($obs -ne $parts[0]) { Write-Error "Checksum mismatch: $($parts[1])"; $failed++ }
}
if ($failed -eq 0) { Write-Host "Backup verified: $bk ($($sums.Count) files)" }
```

---

## Gate 2: Canary Upgrade on `rosie` (macOS arm64) and Metal Manifest Initialization

Upgrade `rosie` FIRST and verify everything before touching any other node.

### 2.1 Stage `dialtone v0.6.0`, `dialtone-task v0.5.0`, `dialtone-agent v0.5.0` on `rosie`
```bash
CH_ID="ch-b39d38af727e"
CH_TASK_DIR="$HOME/.dialtone/channels/$CH_ID/data/task"
CH_AGENT_DIR="$HOME/.dialtone/channels/$CH_ID/data/agent"

# 1. Stop services cleanly so state.redb has no active writer
launchctl unload "$HOME/Library/LaunchAgents/earth.dialtone.agent.plist" 2>/dev/null || true
launchctl unload "$HOME/Library/LaunchAgents/earth.dialtone.task.plist" 2>/dev/null || true
launchctl unload "$HOME/Library/LaunchAgents/earth.dialtone.node.plist" 2>/dev/null || true

# 2. Seed per-channel task data directory from verified backup (copy only, never print keys)
mkdir -p "$CH_TASK_DIR" "$CH_AGENT_DIR"
cp -p "$HOME/.dialtone/plugins/task/state.redb" "$CH_TASK_DIR/"
cp -p "$HOME/.dialtone/plugins/task/config.json" "$CH_TASK_DIR/"
cp -p "$HOME/.dialtone/plugins/task/iroh-secret-key" "$CH_TASK_DIR/" 2>/dev/null || true
[ -d "$HOME/.dialtone/plugins/task/blobs" ] && cp -Rp "$HOME/.dialtone/plugins/task/blobs" "$CH_TASK_DIR/"
# Keep legacy mesh.key only for the dual-key transition bridge (file mode 0600)
cp -p "$HOME/.dialtone/plugins/task/mesh.key" "$CH_TASK_DIR/legacy-mesh.key"
chmod 600 "$CH_TASK_DIR/legacy-mesh.key"

# 3. Swap binaries (from verified release artifacts or local build)
install -m 0755 /path/to/dialtone-v0.6.0-macos-arm64 "$HOME/.dialtone/bin/dialtone"
install -m 0755 /path/to/dialtone-task-v0.5.0-macos-arm64 "$HOME/.dialtone/plugins/task/dialtone-task"
install -m 0755 /path/to/dialtone-agent-v0.5.0-macos-arm64 "$HOME/.dialtone/plugins/agent/dialtone-agent"

# Refresh plugin manifests so SHA-256 digests match the new binaries
"$HOME/.dialtone/plugins/task/dialtone-task" write-manifest --binary "$HOME/.dialtone/plugins/task/dialtone-task" > /dev/null
"$HOME/.dialtone/plugins/agent/dialtone-agent" write-manifest --binary "$HOME/.dialtone/plugins/agent/dialtone-agent" > /dev/null

# 4. Start dialtone v0.6.0 daemon first
launchctl load "$HOME/Library/LaunchAgents/earth.dialtone.node.plist"
sleep 3
curl -fsS http://127.0.0.1:8788/api/channels | head -c 500
```

### 2.2 Initialize the Signed `metal` Channel Manifest on `rosie` (Creator/Admin Node)
```bash
# On rosie (creator/admin node for metal):
"$HOME/.dialtone/bin/dialtone" channel manifest init --allow task,shell,agent --json

# Verify signature_valid == true and is_admin == true
"$HOME/.dialtone/bin/dialtone" channel manifest show --json
curl -fsS http://127.0.0.1:8788/api/channels
```

Expected:
- `channel_id`: `"ch-b39d38af727e"`
- `manifest.schema`: `"dialtone.channel.manifest.v1"`
- `manifest.seq`: `1`
- `manifest.allowed_plugins`: `["agent", "shell", "task"]`
- `manifest.signature_valid`: `true`
- `is_admin`: `true`

### 2.3 Enable Dual-Key Window and Start `dialtone-task v0.5.0` and `dialtone-agent v0.5.0` on `rosie`

Configure `dialtone-task` (either daemon-managed via `managed_plugins: ["task", "agent"]` in `~/.dialtone/config.json`, or launched by `launchd` with `DIALTONE_CHANNEL_ID=ch-b39d38af727e`, `DIALTONE_CHANNEL_DATA_DIR=$CH_TASK_DIR`, `DIALTONE_TASK_LEGACY_MESH_KEY=$CH_TASK_DIR/legacy-mesh.key`, and channel plugin key injected by `dialtone`):

```bash
launchctl load "$HOME/Library/LaunchAgents/earth.dialtone.task.plist"
launchctl load "$HOME/Library/LaunchAgents/earth.dialtone.agent.plist"
sleep 3
"$HOME/.dialtone/plugins/task/dialtone-task" status --json
```

**Canary (`rosie`) verification before Wave 2:**
- `GET http://127.0.0.1:8788/api/channels` returns `dialtone.channels.v1`, `signature_valid: true`, `peers: 6`.
- `dialtone-task status --json` shows `channel_id: "ch-b39d38af727e"`, `task_count >= 41`, and identical `state_hash` to pre-flight.
- `dialtone log --channel ch-b39d38af727e --limit 5` succeeds.

---

## Gate 3: Wave 2 (`legion`, `build-1`) and Wave 3 (`vm-us`, `vm-eu`, `vm-asia`, `steamdeck-2`)

Upgrade ONE node at a time. Verify each node before advancing to the next.

### Windows Procedure (`legion`)
```powershell
$cid = "ch-b39d38af727e"
$chTask = "$env:USERPROFILE\.dialtone\channels\$cid\data\task"
$chAgent = "$env:USERPROFILE\.dialtone\channels\$cid\data\agent"

# 1. Stop scheduled tasks / background processes cleanly
Stop-ScheduledTask -TaskName "DialtoneAgent" -ErrorAction SilentlyContinue
Stop-ScheduledTask -TaskName "DialtoneTask" -ErrorAction SilentlyContinue
Stop-ScheduledTask -TaskName "DialtoneNode" -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

# 2. Seed per-channel task data dir (copy only, never print keys)
New-Item -ItemType Directory -Force -Path $chTask, $chAgent | Out-Null
Copy-Item "$env:USERPROFILE\.dialtone\plugins\task\state.redb" "$chTask\"
Copy-Item "$env:USERPROFILE\.dialtone\plugins\task\config.json" "$chTask\"
if (Test-Path "$env:USERPROFILE\.dialtone\plugins\task\iroh-secret-key") { Copy-Item "$env:USERPROFILE\.dialtone\plugins\task\iroh-secret-key" "$chTask\" }
if (Test-Path "$env:USERPROFILE\.dialtone\plugins\task\blobs") { Copy-Item -Recurse -Force "$env:USERPROFILE\.dialtone\plugins\task\blobs" "$chTask\" }
Copy-Item "$env:USERPROFILE\.dialtone\plugins\task\mesh.key" "$chTask\legacy-mesh.key"

# 3. Swap binaries
Copy-Item -Force "C:\path\to\dialtone-v0.6.0-windows-x86_64.exe" "$env:USERPROFILE\.dialtone\bin\dialtone.exe"
Copy-Item -Force "C:\path\to\dialtone-task-v0.5.0-windows-x86_64.exe" "$env:USERPROFILE\.dialtone\plugins\task\dialtone-task.exe"
Copy-Item -Force "C:\path\to\dialtone-agent-v0.5.0-windows-x86_64.exe" "$env:USERPROFILE\.dialtone\plugins\agent\dialtone-agent.exe"

& "$env:USERPROFILE\.dialtone\plugins\task\dialtone-task.exe" write-manifest --binary "$env:USERPROFILE\.dialtone\plugins\task\dialtone-task.exe" | Out-Null
& "$env:USERPROFILE\.dialtone\plugins\agent\dialtone-agent.exe" write-manifest --binary "$env:USERPROFILE\.dialtone\plugins\agent\dialtone-agent.exe" | Out-Null

# 4. Restart services (with dual-key env enabled on dialtone-task during transition)
Start-ScheduledTask -TaskName "DialtoneNode"
Start-Sleep -Seconds 5
# Confirm signed manifest propagated via gossip from rosie
& "$env:USERPROFILE\.dialtone\bin\dialtone.exe" channel manifest show --json
Invoke-RestMethod -Uri "http://127.0.0.1:8788/api/channels" | ConvertTo-Json -Depth 8

Start-ScheduledTask -TaskName "DialtoneTask"
Start-ScheduledTask -TaskName "DialtoneAgent"
Start-Sleep -Seconds 3
& "$env:USERPROFILE\.dialtone\plugins\task\dialtone-task.exe" status --json
```

### Linux Procedure (`build-1`, then `vm-us`, `vm-eu`, `vm-asia`, `steamdeck-2`)
```bash
CH_ID="ch-b39d38af727e"
CH_TASK_DIR="$HOME/.dialtone/channels/$CH_ID/data/task"
CH_AGENT_DIR="$HOME/.dialtone/channels/$CH_ID/data/agent"

# 1. Stop user services cleanly
systemctl --user stop dialtone-agent.service 2>/dev/null || true
systemctl --user stop dialtone-task.service 2>/dev/null || true
systemctl --user stop dialtone.service 2>/dev/null || true
sleep 2

# 2. Seed per-channel task data dir (copy only, never print keys)
mkdir -p "$CH_TASK_DIR" "$CH_AGENT_DIR"
cp -p "$HOME/.dialtone/plugins/task/state.redb" "$CH_TASK_DIR/"
cp -p "$HOME/.dialtone/plugins/task/config.json" "$CH_TASK_DIR/"
cp -p "$HOME/.dialtone/plugins/task/iroh-secret-key" "$CH_TASK_DIR/" 2>/dev/null || true
[ -d "$HOME/.dialtone/plugins/task/blobs" ] && cp -a "$HOME/.dialtone/plugins/task/blobs" "$CH_TASK_DIR/"
cp -p "$HOME/.dialtone/plugins/task/mesh.key" "$CH_TASK_DIR/legacy-mesh.key"
chmod 600 "$CH_TASK_DIR/legacy-mesh.key"

# 3. Swap binaries (use linux-x86_64 or linux-arm64 matching host arch)
install -m 0755 /path/to/dialtone-v0.6.0-linux-arch "$HOME/.dialtone/bin/dialtone"
install -m 0755 /path/to/dialtone-task-v0.5.0-linux-arch "$HOME/.dialtone/plugins/task/dialtone-task"
install -m 0755 /path/to/dialtone-agent-v0.5.0-linux-arch "$HOME/.dialtone/plugins/agent/dialtone-agent"

"$HOME/.dialtone/plugins/task/dialtone-task" write-manifest --binary "$HOME/.dialtone/plugins/task/dialtone-task" > /dev/null
"$HOME/.dialtone/plugins/agent/dialtone-agent" write-manifest --binary "$HOME/.dialtone/plugins/agent/dialtone-agent" > /dev/null

# 4. Start daemon and verify manifest propagation from rosie
systemctl --user start dialtone.service
sleep 5
"$HOME/.dialtone/bin/dialtone" channel manifest show --json
curl -fsS http://127.0.0.1:8788/api/channels

# 5. Start task (with dual-key enabled) and agent
systemctl --user start dialtone-task.service
systemctl --user start dialtone-agent.service
sleep 3
"$HOME/.dialtone/plugins/task/dialtone-task" status --json
```

**Per-node verification before moving to the next node:**
- `dialtone channel manifest show --json` reports `signature_valid: true`, `admin == <rosie EndpointId>`, `allowed_plugins: ["agent", "shell", "task"]`.
- `dialtone-task status --json` reports `channel_id == "ch-b39d38af727e"`, `task_count >= 41`, and matching `state_hash`.

---

## Gate 4: Fleet-Wide Verification Gates (All 7 Nodes)

Run all 5 checks before closing the dual-key window:

1. **Identical `channel_id` and `state_hash` across all 7 nodes:**
   ```bash
   for n in rosie legion build-1 vm-us vm-eu vm-asia steamdeck-2; do
     echo "=== $n ==="
     dialtone exec --node "$n" -- "dialtone-task status --json"
   done
   ```
   Confirm all 7 report `channel_id: "ch-b39d38af727e"`, identical `state_hash`, and `task_count >= 41`.

2. **Every pre-migration task ID is preserved:**
   Compare task IDs from `$PRE/task-list.json` (Gate 0) against live `dialtone-task list --all --json` on `rosie` and `legion`:
   ```bash
   jq -r '.[].id' "$PRE/task-list.json" | sort > /tmp/pre_ids.txt
   "$HOME/.dialtone/plugins/task/dialtone-task" list --all --json | jq -r '.[].id' | sort > /tmp/post_ids.txt
   comm -23 /tmp/pre_ids.txt /tmp/post_ids.txt
   # Must output 0 lines (zero missing task IDs)
   ```

3. **New task round trip:**
   Create a smoke-test task on `rosie`, ping it from `legion`, verify it appears on `vm-asia` and `steamdeck-2`, then close it.

4. **Channel log visibility on all 7 nodes:**
   Post a test log entry via `POST http://127.0.0.1:8788/api/channel/log` on `rosie` (using local `api-token`), then run `dialtone log --channel ch-b39d38af727e --limit 10` on all 7 nodes and verify the signed entry is visible everywhere.

5. **Out-of-channel isolation check:**
   Start a disposable test node in a temporary directory (`DIALTONE_STATE_DIR=/tmp/dt-outsider-check`) on a different channel (`outsider` with a fresh secret):
   - Confirm it sees 0 `metal` peers, 0 `metal` tasks, 0 `metal` channel log entries, and no `metal` manifest.
   - Confirm attempting to pull a `metal` blob hash/ticket from `rosie` is refused by `ChannelBlobsProtocol`.
   - Stop and remove `/tmp/dt-outsider-check`.

---

## Gate 5: Close Dual-Key Window and Retire Legacy `mesh.key`

Only after Gate 4 is 100% green across all 7 nodes:

1. On every node (`rosie`, `legion`, `build-1`, `vm-us`, `vm-eu`, `vm-asia`, `steamdeck-2`):
   - Remove `DIALTONE_TASK_LEGACY_MESH_KEY` / `--dual-key-legacy` from the `dialtone-task` service configuration.
   - Remove active legacy key files (`~/.dialtone/channels/ch-b39d38af727e/data/task/legacy-mesh.key` and `~/.dialtone/plugins/task/mesh.key`). **Keep the dated backup directory intact.**
   - Restart `dialtone-task`.
2. Re-run Gate 4 check 1 (`state_hash` identical across all 7) and check 3 (new task round trip).
3. Verify an old legacy `mesh.key` peer cannot connect to or sync with the `metal` task mesh.

---

## Rollback Procedures

### Rollback Triggers (Abort and Roll Back Immediately If Any Occur)
- Any node loses a pre-migration task ID or `task_count < 41`.
- `state_hash` diverges across nodes for > 2 minutes after restart.
- `dialtone` fails to bind, crashes in a restart loop, or fails manifest signature verification on a valid admin manifest.
- `dialtone-agent` fails to claim authorized `metal` tasks or claims unauthorized channel tasks.

### Per-Node Rollback (Restore from Dated Backup `$BK`)

#### macOS (`rosie`)
```bash
BK="$HOME/dialtone-backup-<TS>"
launchctl unload "$HOME/Library/LaunchAgents/earth.dialtone.agent.plist" 2>/dev/null || true
launchctl unload "$HOME/Library/LaunchAgents/earth.dialtone.task.plist" 2>/dev/null || true
launchctl unload "$HOME/Library/LaunchAgents/earth.dialtone.node.plist" 2>/dev/null || true

cp -p "$BK/bin/dialtone" "$HOME/.dialtone/bin/dialtone"
cp -p "$BK/dialtone/config.json" "$HOME/.dialtone/config.json"
cp -p "$BK/task/dialtone-task" "$HOME/.dialtone/plugins/task/dialtone-task"
cp -p "$BK/task/config.json" "$HOME/.dialtone/plugins/task/config.json"
cp -p "$BK/task/state.redb" "$HOME/.dialtone/plugins/task/state.redb"
cp -p "$BK/task/mesh.key" "$HOME/.dialtone/plugins/task/mesh.key"
[ -f "$BK/agent/dialtone-agent" ] && cp -p "$BK/agent/dialtone-agent" "$HOME/.dialtone/plugins/agent/dialtone-agent"

launchctl load "$HOME/Library/LaunchAgents/earth.dialtone.node.plist"
launchctl load "$HOME/Library/LaunchAgents/earth.dialtone.task.plist"
launchctl load "$HOME/Library/LaunchAgents/earth.dialtone.agent.plist"
"$HOME/.dialtone/plugins/task/dialtone-task" status --json
```

#### Windows PowerShell (`legion`)
```powershell
$bk = "$env:USERPROFILE\dialtone-backup-<TS>"
Stop-ScheduledTask -TaskName "DialtoneAgent" -ErrorAction SilentlyContinue
Stop-ScheduledTask -TaskName "DialtoneTask" -ErrorAction SilentlyContinue
Stop-ScheduledTask -TaskName "DialtoneNode" -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

Copy-Item -Force "$bk\bin\dialtone.exe" "$env:USERPROFILE\.dialtone\bin\dialtone.exe"
Copy-Item -Force "$bk\dialtone\config.json" "$env:USERPROFILE\.dialtone\config.json"
Copy-Item -Force "$bk\task\dialtone-task.exe" "$env:USERPROFILE\.dialtone\plugins\task\dialtone-task.exe"
Copy-Item -Force "$bk\task\config.json" "$env:USERPROFILE\.dialtone\plugins\task\config.json"
Copy-Item -Force "$bk\task\state.redb" "$env:USERPROFILE\.dialtone\plugins\task\state.redb"
Copy-Item -Force "$bk\task\mesh.key" "$env:USERPROFILE\.dialtone\plugins\task\mesh.key"
if (Test-Path "$bk\agent\dialtone-agent.exe") { Copy-Item -Force "$bk\agent\dialtone-agent.exe" "$env:USERPROFILE\.dialtone\plugins\agent\dialtone-agent.exe" }

Start-ScheduledTask -TaskName "DialtoneNode"
Start-ScheduledTask -TaskName "DialtoneTask"
Start-ScheduledTask -TaskName "DialtoneAgent"
& "$env:USERPROFILE\.dialtone\plugins\task\dialtone-task.exe" status --json
```

#### Linux (`build-1`, `vm-us`, `vm-eu`, `vm-asia`, `steamdeck-2`)
```bash
BK="$HOME/dialtone-backup-<TS>"
systemctl --user stop dialtone-agent.service dialtone-task.service dialtone.service 2>/dev/null || true
sleep 2

cp -p "$BK/bin/dialtone" "$HOME/.dialtone/bin/dialtone"
cp -p "$BK/dialtone/config.json" "$HOME/.dialtone/config.json"
cp -p "$BK/task/dialtone-task" "$HOME/.dialtone/plugins/task/dialtone-task"
cp -p "$BK/task/config.json" "$HOME/.dialtone/plugins/task/config.json"
cp -p "$BK/task/state.redb" "$HOME/.dialtone/plugins/task/state.redb"
cp -p "$BK/task/mesh.key" "$HOME/.dialtone/plugins/task/mesh.key"
[ -f "$BK/agent/dialtone-agent" ] && cp -p "$BK/agent/dialtone-agent" "$HOME/.dialtone/plugins/agent/dialtone-agent"

systemctl --user start dialtone.service dialtone-task.service dialtone-agent.service
"$HOME/.dialtone/plugins/task/dialtone-task" status --json
```

### Fleet-Wide Rollback
1. Stop `dialtone-agent` and `dialtone-task` on all 7 nodes first so no split-brain writes occur.
2. Restore `$BK` binaries, `config.json`, `state.redb`, and `mesh.key` on all 7 nodes using the OS commands above.
3. Start `dialtone` (`v0.5.6`), then `dialtone-task` (`v0.4.0`), then `dialtone-agent` (`v0.4.0`) across all 7 nodes.
4. Confirm all 7 nodes match the pre-flight `state_hash` and `task_count >= 41`.

---

## Time Estimate and Tim Approval Gates

| Gate | Scope | Estimated Time | What Tim Must OK |
|---|---|---|---|
| **Pre-Gate** | Dual-key bridge PR/commit + proof test in `dialtone-task` | ~30-45 min (dev) | Confirm dual-key bridge test is green (or explicitly authorize a 5-min all-at-once cutover instead) |
| **Gate 0 & 1** | Pre-flight audit + checksum-verified backups on all 7 nodes | ~10 min | Confirm all 7 nodes show identical `state_hash`, `task_count >= 41`, and verified backups |
| **Gate 2** | Canary upgrade on `rosie` + `dialtone channel manifest init --allow task,shell,agent` | ~10 min | Confirm `rosie` canary healthy, manifest `signature_valid: true`, and `state_hash` intact |
| **Gate 3** | Wave 2 (`legion`, `build-1`) + Wave 3 (`vm-us`, `vm-eu`, `vm-asia`, `steamdeck-2`) | ~25 min (~4 min/node) | Confirm each wave green before advancing to the next |
| **Gate 4** | Fleet-wide verification (7-node `state_hash`, 41+ task IDs, task RT, channel log, outsider isolation) | ~10 min | Confirm all 5 verification gates pass |
| **Gate 5** | Close dual-key window, remove active legacy `mesh.key` (keep backups) | ~10 min | Explicit OK to permanently retire legacy `mesh.key` topic |

**Total execution time (once dual-key bridge prerequisite is ready):** ~65 minutes across all 7 nodes.
