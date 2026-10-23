# Dialtone User Guide

`dialtone` is a single-binary Rust + **Iroh 1.3** peer-to-peer mesh control plane, 3D tactical HUD & terminal `top` monitor, content-addressed blob store (`iroh-blobs`), and process supervisor. It runs without SSH, Git, or Tailscale/VPN runtime dependencies.

This guide uses the project's [domain language](goal.md#domain-language): one word per concept (Node, NodeId, Peer, Mesh, Channel, Secret, Allowlist, Invite, Seed, Beat, Stats, Blob, Digest, Store, Ticket, Program, Process, Probe, HUD, API).

```text
$ dialtone --help
usage: dialtone [action] [flags]

Single-binary Iroh 1.3 + Rust mesh control plane, 3D tactical HUD, blob store & daemon.
Running `dialtone` with no arguments boots in dependency order and prints the local Web HUD URL.

actions:
  up                       Start the node: HUD -> store -> Iroh QUIC -> mesh
  join <secret>            Unlock and join a channel with its secret
  secret                   Generate a new channel secret (dt-...)
  allowlist [list|add|remove] [<node_id>] Manage the allowlist in ~/.dialtone/config.json
  blob [share|pull] <arg>  Share a file or text as a blob, or pull a blob by ticket
  invite [--ttl 15m]       Mint a signed, expiring, single-use invite
  daemon [install|status]  Install or inspect the daemon that starts the node at login
  probe                    Run one probe on the local node (BLB, KVS, PRC, MSH, A/B)
  mesh-test                Check the running node's security, mesh convergence and blob transfer
  hud                      Serve the HUD and API only (no mesh)
  node                     Run the node without the HUD or API
  status                   Print this node's NodeId and transport report
  self-test | ci           Run full 7-capability live verification suite
  state-fixture            Print representative daemon state JSON
  mesh-fixture             Print global mesh peer index JSON
  gossip-fixture           Print Iroh gossip & service discovery JSON
  iroh-runtime-fixture     Print Iroh 1.3 runtime identity & transport diagnostics
  deploy-plan-fixture      Print deploy target plan or build-decision evidence
  blob-fetch-report        Print tiered blob hydration plan (cache / iroh-blobs / fallback)
  process-intent-fixture   Print start/stop/restart process intent plan & diff
  rollback-plan-fixture    Print side-effect-free guarded rollback plan
  alert-ack-fixture        Print side-effect-free alert acknowledgement preview
  proof-validation-fixture Print session proof acceptance/rejection report
  program-catalog-fixture  Print program catalog & named dataset evidence
  service                  Render OS login service unit (macos | linux | windows)
  help                     Show this help

flags:
  --node <name>            Node name (default: saved config or hostname)
  --listen <addr>          HUD and API listen address (default: 127.0.0.1:8788)
  --api-token <token>      Explicit API bearer token for POST routes (default: ~/.dialtone/api-token)
  --channel <name>         Channel name (default: dialtone)
  --secret <secret>        Channel secret (or set DIALTONE_SECRET)
  --invite <invite>        Invite to redeem when joining (repeatable)
  --ttl <duration>         TTL for 'dialtone invite' (e.g. 15m, 1h, 300s; default: 15m)
  --peer <node_id>         Target NodeId for 'dialtone invite' or 'dialtone allowlist'
  --addr <ip:port>         Direct address for 'dialtone allowlist add' (repeatable)
  --expect-peers <count>   Expected peer count for 'dialtone mesh-test'
  --timeout <duration>     Timeout for 'dialtone mesh-test' (default: 15s)
  --force                  Pull over QUIC even if the blob is already in the store
  --install-daemon         Install the daemon so the node starts at login
  --no-tailnet             Exclude Tailscale (100.64.0.0/10 & fd7a:115c:a1e0::/48) and bind physical IPv4
  --allow-tailnet          Allow Tailscale interface addresses
  --private-mesh           Disable relays, Pkarr DNS, and Mainline DHT (direct static peer QUIC only)
  --quic-port <port>       Explicit UDP port to bind for the Iroh QUIC endpoint
  --lan                    Enable mDNS local discovery (lan_only profile)
  --dns                    Enable public Pkarr DNS lookup/publish (disabled by default)
  --dht                    Enable Mainline DHT address lookup (disabled by default)
  --no-dns                 Disable DNS/Pkarr lookup
  --no-relay               Disable Iroh relay servers
  --relay-only             Advertise relay-only gossip tickets
  --relay-url <url>        Custom Iroh relay URL (repeatable)
  --deploy-target <t>      robot-v4-default | dialtone | all-daemon | all-with-daemons
  --build-case <c>         unchanged-skip | cache-reused | peer-local-async
  --intent-action <a>      start | stop | restart
  --proof-case <c>         valid | missing | stale | cross-session | non-daemon-fallback |
                           no-renderer-fallback | peer-build-deps-missing
  --digest <digest>        SHA-256 digest for blob-fetch-report
  --uri <uri>              Blob URI for blob-fetch-report
  --http-fallback-enabled  Allow HTTP peer-bridge fallback for blob-fetch-report
  --actor <name>           Actor name for alert-ack-fixture
  --note <text>            Note for alert-ack-fixture
  --os <macos|linux|windows> Target OS for service unit generation
  --once                   Run one cycle and exit
```

HUD keys (at http://127.0.0.1:8788):

```text
  Up / Down Arrow                          Zoom camera in/out across Layers 1..5 (GLOBE -> PROCESS)
  Left / Right Arrow                       Step backward/forward through the 6-Step System Tour & Nodes
  Double-Click Node / Row                  Zoom into focused node symbol (resets to L1 GLOBE after L5)
  Click [1 MSH] .. [6 A/B] in Bottom CC    Jump directly to any capability step in the live Closed Captions
```

---

## Quick Start (60 Seconds)

### 1. Download (or Build) the Single Binary

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/timcash/dialtone/releases/latest/download/dialtone-macos-arm64 -o dialtone && chmod +x dialtone

# Linux (x86_64 static musl)
curl -fsSL https://github.com/timcash/dialtone/releases/latest/download/dialtone-linux-x86_64 -o dialtone && chmod +x dialtone

# Or build from source with Rust
cargo build --release && cp target/release/dialtone ./dialtone
```

```powershell
# Windows (PowerShell)
Invoke-WebRequest -Uri https://github.com/timcash/dialtone/releases/latest/download/dialtone-windows-x86_64.exe -OutFile dialtone.exe
```

### 2. Boot a Node & Open the 3D Tactical HUD

```bash
./dialtone up --node node-alpha
```

Open **`http://127.0.0.1:8788`** in your browser to view the live 3D mesh topology and terminal `top` stats table.

### 3. Link Two Nodes into an Encrypted Gossip Mesh

```bash
# On Node A (node-alpha): generate a 256-bit secret, unlock the channel, and mint a 15m invite
CHANNEL_PW=$(./dialtone secret)
echo "Channel secret: $CHANNEL_PW"
./dialtone join "$CHANNEL_PW" --node node-alpha --once
./dialtone invite --ttl 15m
```

```bash
# On Node B (node-beta): inspect NodeId and join using the secret + single-use invite
./dialtone status --node node-beta
./dialtone join "<CHANNEL_PW>" --node node-beta --invite "dialtone-invite-v1:<ticket>" --once
./dialtone up --node node-beta
```

```bash
# Back on Node A (node-alpha): allow Node B's NodeId and start
./dialtone allowlist add <node-beta-node-id>
./dialtone up --node node-alpha
```

### 4. Optional: Install as a Background Login Service & Run Self-Test

```bash
# Register dialtone as a user-level OS service (launchd / systemd --user / Windows Scheduled Task)
./dialtone daemon install

# Verify all 7 capabilities (live Iroh 1.3 QUIC gossip, store, and process supervision)
./dialtone self-test
```

---

## 1. Installation & Boot Pipeline

### Pre-Compiled Release Assets

Every GitHub Release publishes standalone, size-optimized single-file executables with all runtime assets and the Web UI compiled directly into the binary:

| Platform | Release Asset |
| :--- | :--- |
| **macOS (Apple Silicon)** | `dialtone-macos-arm64` |
| **Linux (x86_64, static musl)** | `dialtone-linux-x86_64` |
| **Linux (ARM64 / Edge)** | `dialtone-linux-arm64` |
| **Windows (x86_64)** | `dialtone-windows-x86_64.exe` |

### 5-Stage Boot Sequence

Running `./dialtone` (with no arguments, or `./dialtone up` / `start` / `hub`) boots 5 subsystems in strict dependency order and starts the local Web HUD immediately:

```text
[BOOT 1/5] Local Runtime & Stdout Logger ................. [READY] (node=node-alpha, os=macos/aarch64)
[BOOT 2/5] Local Web HUD & Config Server ................. [READY] http://127.0.0.1:8788
[BOOT 3/5] Content-Addressed Blob Store (iroh-blobs) ..... [READY] ~/.dialtone/blobs
[BOOT 4/5] Iroh 1.3 QUIC Endpoint (No-Tailnet NIC) ....... [READY] id=a67ff536b0f9.. ip=192.168.1.40
[BOOT 5/5] Gossip Mesh & Channel Secret ................ [READY] channel=ch-7981c39da07e peers=2
```

- **Port Auto-Fallback**: Defaults to `127.0.0.1:8788` (override with `--listen <ip:port>`). If `8788` is in use and `--listen` was not explicitly forced, `dialtone` automatically scans `127.0.0.1:8788..=8799` (falling back to an OS-assigned port).
- **HUD-only mode**: Run `./dialtone hud --listen 127.0.0.1:8788` to run only the HTTP HUD and state API server without spawning the live gossip loop.

---

## 2. Local Web HUD, Terminal `top` UI & Stats

Open **`http://127.0.0.1:8788`** (also served at `/nodes`, `/peer`, and `/hud`) to access the embedded tactical HUD:

- **Terminal `top` Process & Resource Table**: Displays every discovered peer (`NODE`, `OS/ARCH`, `BIND IP`, `HB`, `CORES`, `CPU%`, `MEM%`, `DISK%`, `NET RX/TX`, `STATUS / WARNINGS`).
- **Automatic Threshold Warnings**: Rows remain monochrome greyscale when nominal and highlight in red (`#ff2e3b`) only when limits are breached:
  - `CPU >= 80%`
  - `MEM >= 80%`
  - `DISK >= 85%`
  - `VPN PATH` (`tailnet_used: true` detected on transport path)
- **Interactive 3D Mesh Topology**: Drag to orbit, scroll to zoom, or click any peer node to inspect its live QUIC links and metadata.
- **Menu** (bottom-right button):
  - `RESET ORBIT` — Return the camera to the tour view.
  - `INIT NEW PRIVATE MESH`, `GENERATE INVITE`, `JOIN MESH` — Call `POST /api/channel/{init,invite,join}`, which the API does not implement yet (see goal.md roadmap, Phase 3); use the CLI (`dialtone secret`, `dialtone invite`, `dialtone join`) meanwhile.
- **Live WebSocket & Health Endpoints**:
  - `GET /ws` — RFC 6455 WebSocket upgrade streaming real-time mesh state snapshots.
  - `GET /health` — Lightweight JSON health check (`{"ok": true, "node": "..."}`).
  - `GET /api/boot-status` (alias `GET /api/config`) — Returns the 5 boot stages, active `mesh_config` (with `derived_keys` redacted), and `blob_cache_dir`.

---

## 3. Starting a Gossip Mesh from Scratch

Because `dialtone` ships with **zero hardcoded mesh peers** and enforces a **deny-by-default peer allowlist** + **Argon2id / XChaCha20-Poly1305 encrypted channel**, a brand-new node boots into an isolated `dialtone:locked-standby` topic until you configure a channel secret and authorize your peers.

Below are the two ways to seed a new multi-node mesh from scratch (using generic example nodes `node-alpha` and `node-beta`).

### Option A: Join with an Expiring Single-Use invite (Fastest)

#### Step 1 — Generate a secret & initialize `node-alpha`
On **Host A** (`node-alpha`), generate a 256-bit channel secret, derive and save the Argon2id channel keys into `~/.dialtone/config.json` (`0600`), and mint a 15-minute single-use invite:

```bash
# 1. Generate a high-entropy channel secret (save this to share with your other nodes)
CHANNEL_PW=$(./dialtone secret)
echo "Channel secret: $CHANNEL_PW"

# 2. Derive Argon2id subkeys and save to ~/.dialtone/config.json without starting the long-running loop yet
./dialtone join "$CHANNEL_PW" --node node-alpha --once

# 3. Mint a signed, single-use invite valid for 15 minutes
./dialtone invite --ttl 15m
```

Copy the `"invite": "dialtone-invite-v1:..."` string from the JSON output.

#### Step 2 — Join `node-beta` using the invite
On **Host B** (`node-beta`), redeem the invite with the same channel secret. Redeeming a valid `dialtone-invite-v1:` ticket automatically verifies the issuer signature under `auth_key`, records the single-use nonce, adds `node-alpha` to `node-beta`'s `allowlist` and `seeds`, and prints `node-beta`'s `NodeId`:

```bash
# 1. Inspect node-beta's persistent NodeId (created at ~/.dialtone/iroh-secret-key)
./dialtone status --node node-beta

# 2. Join the channel and redeem the single-use invite from node-alpha
./dialtone join "<CHANNEL_PW>" \
  --node node-beta \
  --invite "dialtone-invite-v1:<ticket-from-node-alpha>" \
  --once
```

#### Step 3 — Allow `node-beta` on `node-alpha` & start both nodes
On **Host A** (`node-alpha`), authorize `node-beta`'s `NodeId` (so both sides permit each other and include each other in `seeds`) and start the daemons:

```bash
# On Host A (node-alpha): allow node-beta's 64-hex NodeId and start
./dialtone allowlist add <node-beta-node-id>
./dialtone up --node node-alpha
```

```bash
# On Host B (node-beta): start the daemon
./dialtone up --node node-beta
```

---

### Option B: Explicit Allowlist Provisioning Across N Nodes (Deterministic / Automated)

If you are provisioning 2, 3, or more hosts via a setup script or SSH, you can pre-populate each node's allowlist directly using `dialtone status`, `dialtone join --once`, and `dialtone allowlist add`:

```bash
# 1. On EVERY host, initialize its persistent Ed25519 identity (~/.dialtone/iroh-secret-key)
#    and record its 64-character hex "node_id":
./dialtone status --node <node-name>

# 2. On ONE host, generate the shared channel secret:
./dialtone secret
# -> e.g. dt-R9xK4mP7vQ2wL8nJ5tB3yF6hC1zX0sW9_AbCdEfGhIj

# 3. On EVERY host, derive the Argon2id channel keys into ~/.dialtone/config.json:
./dialtone join "<CHANNEL_PW>" --node <node-name> --once

# 4. On EVERY host, authorize every peer's NodeId (adds to allowlist & seeds):
./dialtone allowlist add <node-id-of-node-alpha>
./dialtone allowlist add <node-id-of-node-beta>
./dialtone allowlist add <node-id-of-node-gamma>

# Verify the local allowlist and seeds:
./dialtone allowlist list

# 5. Start dialtone in the foreground (or install as a persistent background service):
./dialtone up --node <node-name>
# or:
./dialtone daemon install
```

### Step 4 — Verify Mesh Convergence

Within a few seconds of starting, verify that all nodes are exchanging encrypted beats (`dialtone.iroh.envelope.v2`) and incrementing their `counter`:

```bash
# Check discovered peers and beat counters via the local HTTP API
curl -s http://127.0.0.1:8788/api/state | jq '.nodes[] | {node, role, node_id, counter}'

# Check the 5 boot stages and active channel fingerprint (ch-...)
curl -s http://127.0.0.1:8788/api/config | jq '{node, channel_fingerprint: .mesh_config.channel_fingerprint, boot_stages}'
```

Or open **`http://127.0.0.1:8788`** in your browser to view the live 3D topology and terminal `top` table.

---

## 4. Channel Security, Peer Allowlist & Control API Reference

**How the mesh is protected**

- **Allowlist at the handshake.** Every QUIC connection (gossip and blobs, in and out) is closed right after the TLS handshake unless the remote NodeId is on the allowlist. `dialtone allowlist add/remove` edits `~/.dialtone/config.json`; a running node picks the change up within 5 seconds, no restart.
- **Signed beats.** Each beat is signed with the sender's Key and encrypted with the channel keys. A node drops beats whose signature does not match the NodeId they claim, beats from NodeIds not on its allowlist, and replays (each node's `sent_unix_ms` must increase and be within 5 minutes of local time — keep node clocks roughly in sync).
- **No process secrets on the wire.** Beats list each process without its `env` or `args`.
- **Deploys only from the allowlist.** A process offered for mesh deploy is pulled only from an allowlisted signer and an allowlisted blob provider, never a peer's probe program, and only once per (peer, digest). Blob pulls use the node's own endpoint, so the provider's allowlist decides.
- **HUD.** The page carries no API token; a per-response Content-Security-Policy nonce lets only its own scripts run.

### Unlock & Join the `"dialtone"` Channel (`join` & `secret`)

The `"dialtone"` channel (and any custom `--channel <name>`) is **secret-protected and end-to-end encrypted**:
- Booting `dialtone` without a configured secret places the node on an isolated per-node standby topic (`dialtone:locked-standby`) and does **not** dial mesh seeds until unlocked.
- **Minimum Secret Strength**: Secrets must be at least 16 characters long with multiple character classes and `>= 8` unique characters; weak or dictionary secrets (`"dialtone"`, `"dialtone2"`, `"secret123"`, `"default"`) are rejected. Run `./dialtone secret` to generate a CSPRNG 256-bit secret (`dt-...`).
- **Argon2id Key Derivation (`v2`) & XChaCha20-Poly1305 Gossip Encryption**:
  1. `dialtone` derives three domain-separated 32-byte subkeys (`topic_id`, `auth_key`, `enc_key`) from `(channel, secret)` using **Argon2id** (`m=19456 KiB, t=2, p=1`).
  2. Only the derived subkey bundle (`derived_keys`) is persisted in `~/.dialtone/config.json` (`0600` permissions) — the plaintext secret is **never** written to disk. Any legacy `~/.dialtone2/config.json` is automatically migrated and scrubbed on startup.
  3. Every `DialtoneBeat` broadcast over `iroh-gossip` is signed with `auth_key` (`channel_mac`) and encrypted with **XChaCha20-Poly1305** (`dialtone.iroh.envelope.v2`, random 24-byte nonce). Unencrypted or unauthenticated frames are dropped.
- **Deny-by-Default Peer Allowlist (`allowlist`)**:
  - On the main `"dialtone"` channel, inbound peers are **denied by default** unless their `NodeId` is listed in the host's local `~/.dialtone/config.json` (`allowlist`), even if `--invite` is passed.
  - Manage the allowlist via CLI:
    ```bash
    ./dialtone allowlist list
    ./dialtone allowlist add <node_id>
    ./dialtone allowlist add <node_id> --addr 192.168.1.50:19440
    ./dialtone allowlist remove <node_id>
    ```
- **Expiring, Single-Use invites (`invite`)**:
  - Mint a signed, time-limited, single-use invite (`dialtone-invite-v1:...`) that automatically authorizes the peer upon redemption:
    ```bash
    ./dialtone invite --ttl 15m
    ./dialtone invite --ttl 15m --peer <target_node_id>
    ```

```bash
# Generate a high-entropy 256-bit channel secret
./dialtone secret

# Unlock the default 'dialtone' channel with a secret (updates a running local daemon live, or boots a new one)
./dialtone join <secret>

# Redeem a single-use invite while joining
./dialtone join <secret> --invite dialtone-invite-v1:<ticket>

# Save derived channel keys to ~/.dialtone/config.json (0600 permissions) and exit immediately
./dialtone join <secret> --once
```

### Local HTTP Control API Bearer Authentication (`~/.dialtone/api-token`)

All state-changing `POST` routes on the local HTTP control plane require bearer token authentication (`Authorization: Bearer <token>` or `X-Dialtone-Token: <token>`):
- On first boot, `dialtone` generates a 32-byte random token in `~/.dialtone/api-token` (`0600` permissions). Override with `--api-token <token>` or `DIALTONE_API_TOKEN`.
- Binding `--listen` to a non-loopback address (e.g. `0.0.0.0:8788`) is refused unless an explicit `--api-token` or `DIALTONE_API_TOKEN` is configured.
- The HTTP server processes requests concurrently on bounded process threads (`MAX_CONCURRENT_HTTP_CONNECTIONS = 32`) with a 2-second socket read/write timeout and 2 MiB body size limit.

```bash
API_TOKEN=$(cat ~/.dialtone/api-token)
curl -X POST http://127.0.0.1:8788/api/mesh/join \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"channel":"dialtone","secret":"dt-...","node":"node-alpha"}'
```

### Headless Peer Mode (`node`) & Transport Inspection (`status`)

```bash
# Run a headless Iroh 1.3 gossip + blob peer (no HTTP server)
./dialtone node --node node-alpha --channel dialtone --secret <secret>

# Emit one beat JSON payload and exit
./dialtone node --node node-alpha --no-dns --no-relay --once

# Bind persistent Ed25519 identity (~/.dialtone/iroh-secret-key) and print transport diagnostics
./dialtone status --node node-alpha
```

### Transport & Discovery Flags

By default, `dialtone` uses **zero public directory publishing** (`dns-pkarr` and `mainline-dht` are disabled unless `--dns` or `--dht` is explicitly passed), so node `NodeId`s and IP addresses are never published to public DNS or DHT directories.

| Flag | Behavior |
| :--- | :--- |
| `--no-tailnet` | **(Default)** Strips Tailscale IPv4 (`100.64.0.0/10`) and IPv6 (`fd7a:115c:a1e0::/48`) addresses from advertised tickets and binds the QUIC socket to the primary physical non-Tailnet IPv4 interface. |
| `--allow-tailnet` | Permits Tailscale interface addresses in advertised endpoints. |
| `--private-mesh` | Zero-internet private LAN mesh mode: disables public relays, DNS/Pkarr, and Mainline DHT, and enables mDNS + direct IP:port peer dialing. |
| `--quic-port <port>` | Binds the Iroh QUIC UDP socket to a deterministic port (e.g. `19440`) for direct LAN peer dialing (`DIALTONE_QUIC_PORT`). |
| `--lan` / `--local-discovery` | Enables zero-conf mDNS local network discovery (`lan_only` profile). |
| `--dns` | Explicitly enables public Pkarr DNS address lookup/publishing (`dns.iroh.link`). |
| `--dht` | Explicitly enables Mainline BitTorrent DHT address publication and lookup. |
| `--no-dns` | Keeps DNS/Pkarr address lookup disabled (default). |
| `--no-relay` | Disables Iroh relay servers (direct UDP/QUIC only). |
| `--relay-only` | Strips direct IP addresses from tickets so traffic routes strictly via relay (`relay_only` profile). |
| `--relay-url <url>` | Adds a custom Iroh relay server URL (repeatable). |
| `--invite <t>` | Adds an explicit seed gossip or `dialtone-invite-v1:` ticket (repeatable; alias `--join-ticket`). |
| `--api-token <token>` | Sets an explicit bearer token for `POST` routes on the HTTP control API. |

---

## 5. Persistent OS Daemon & Service Units

`dialtone` can install itself into your user account so it starts automatically on boot/login without requiring `root` or `sudo`. Derived Argon2id channel keys are saved to `~/.dialtone/config.json` (`0600`) and raw secrets are never written to disk or passed on the service command line:

```bash
# Copy binary to ~/.dialtone/bin/ and register OS login service
./dialtone daemon install

# Or install inline while booting
./dialtone up --secret "<CHANNEL_PW>" --install-daemon

# Check installation status and saved configuration
./dialtone daemon status
```

| Operating System | Installed Service Blob |
| :--- | :--- |
| **macOS** | `~/Library/LaunchAgents/com.timcash.dialtone.plist` (`launchctl load -w`) |
| **Linux** | `~/.config/systemd/user/dialtone.service` (`systemctl --user enable --now dialtone.service`) |
| **Windows** | `~/.dialtone/start-dialtone.cmd` registered in `HKCU\...\Run\DialtoneDaemon` + Scheduled Task `DialtoneDaemon` |

### Render Service Unit Templates (`service`)

To preview or pipe service definitions for system-wide deployment:

```bash
./dialtone service --os macos --node node-alpha
./dialtone service --os linux --node node-alpha
./dialtone service --os windows --node node-beta
```

Seed script and release manifest endpoints are also served over HTTP at `GET /install/v1/install.sh` and `GET /install/v1/manifest.json`.

---

## 6. Blob Store & Tiered Hydration

All blobs and executable payloads are stored by SHA-256 digest under `~/.dialtone/blobs/<2-hex-prefix>/<64-hex-digest>`. Uploads are staged to a temporary file, verified against SHA-256, and atomically renamed into the store.

### Store, Fetch & Verify Blobs via HTTP

```bash
API_TOKEN=$(cat ~/.dialtone/api-token)

# 1. Upload base64 payload into the verified store
BLOB_B64=$(printf '#!/bin/sh\nsleep 30\n' | base64 | tr -d '\n')
curl -X POST http://127.0.0.1:8788/api/blob \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"bytes_base64\":\"$BLOB_B64\"}"

# 2. Check blob presence and byte length (HEAD) or download bytes/metadata (GET)
curl -I "http://127.0.0.1:8788/api/blob?sha256=<64-hex-digest>"
curl -s "http://127.0.0.1:8788/api/blob?sha256=<64-hex-digest>"
```

### Tiered Blob Fetch Planning (`blob-fetch-report`)

When hydrating an blob across the mesh, `dialtone` evaluates a 3-tier resolution order:
1. **`local-cache`** (`status: "cache_hit"`) — `0` network bytes transferred if the verified SHA-256 already exists in `~/.dialtone/blobs`.
2. **`iroh-blobs-ticket`** — Native peer-to-peer QUIC transfer when the URI is `iroh-blobs://ticket/<ticket>#sha256:<hex>`.
3. **`http-peer-fallback`** — Guarded HTTP fallback (`http://<peer>/...`) used only when `--http-fallback-enabled` (`http_fallback_enabled: true`) is set.

```bash
# CLI evaluation
./dialtone blob-fetch-report \
  --sha256 sha256:5b9134298b0802a1f4c1201d61bee747a5e3fcca2219cbcf00cec335a3365c7d \
  --uri "iroh-blobs://ticket/blob123#sha256:5b9134298b0802a1f4c1201d61bee747a5e3fcca2219cbcf00cec335a3365c7d"

# HTTP evaluation
curl -X POST http://127.0.0.1:8788/api/blob-fetch-report \
  -H "Authorization: Bearer $(cat ~/.dialtone/api-token)" \
  -H "Content-Type: application/json" \
  -d '{"digest":"sha256:5b9134298b0802a1f4c1201d61bee747a5e3fcca2219cbcf00cec335a3365c7d","uri":"iroh-blobs://ticket/blob123","provider_node":"node-gamma","expected_bytes":4096}'
```

### Live Cross-Peer `iroh-blobs` QUIC Share & Pull (`blob share` / `blob pull`)

Every running `dialtone` daemon attaches `iroh_blobs::BlobsProtocol` to its live QUIC router so peers can share and pull content-addressed blobs directly over QUIC:

```bash
# On Node A: import a file or payload into store + live iroh-blobs store and mint an iroh-blob:// ticket URI
./dialtone blob share ./process-script.sh
# -> returns {"digest": "...", "ticket_uri": "iroh-blob://<sha256>?peer=<node_id>&relay=...", ...}

# On Node B: pull the blob over live Iroh QUIC from Node A and verify its SHA-256 into ~/.dialtone/blobs
./dialtone blob pull "iroh-blob://<sha256>?peer=<node_id>&relay=..." --force
```

---

## 7. Supervised Blob Processes & Process Intent

### Keep a store Blob Running as a Daemon Process

Once an executable binary or script is stored in the local store (`~/.dialtone/blobs`), you can configure `dialtone` to stage it into `~/.dialtone/run/<name>-<short_sha>` (`chmod 0755`) and supervise it as a child process of the daemon:

```bash
API_TOKEN=$(cat ~/.dialtone/api-token)

# Spawn and supervise a store blob as a child process of dialtone
curl -X POST http://127.0.0.1:8788/api/processes/blob \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "stats-blob-process",
    "digest": "<64-hex-digest>",
    "desired_state": "running",
    "args": ["--interval", "5"],
    "env": ["PROCESS_MODE=live"]
  }'

# Inspect live supervised processes (status, PID, restart_count, sha256)
curl -s http://127.0.0.1:8788/api/processes

# Stop a supervised blob process cleanly
curl -X POST http://127.0.0.1:8788/api/processes/blob \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "stats-blob-process",
    "digest": "<64-hex-digest>",
    "desired_state": "stopped"
  }'
```

- **Crash Auto-Respawn**: Every 2 seconds (and on process status queries), the daemon checks all managed child PIDs via `try_wait()`. If a process with `"desired_state": "running"` exits or crashes, `dialtone` increments `restart_count`, re-stages the verified store blob, and spawns a fresh child PID automatically.
- **Environment Sanitization**: Internal daemon environment variables (`DIALTONE_*`, `DAEMON_V1_*`) are stripped before spawning child processes.
- **Restart Persistence**: process definitions are saved to `~/.dialtone/config.json` and automatically re-staged and started when `dialtone` boots.

### Process Intent Planning & Dependency Auditing (`process-intent-fixture`)

Preview `start`, `stop`, or `restart` convergence diffs, systemd drop-in drift repairs (`25-daemon-intent.conf`), and runtime dependency audits (which reject `.ts.net` hostnames, hardcoded LAN IPs, or SSH/Git dependencies):

```bash
./dialtone process-intent-fixture --node edge-1 --intent-action restart
```

Or via HTTP: `GET /api/process-intent?node=edge-1&action=restart` and `POST /api/process-intent`.

---

## 8. Manifest-Driven Deployment, Build Cache & Rollback

### Deploy Target Plans (`deploy-plan-fixture`)

`dialtone` separates regular workload rollouts from daemon self-updates:

```bash
# Default application rollout (includes_daemon_self_update: false)
./dialtone deploy-plan-fixture --deploy-target robot-v4-default

# Explicit daemon self-update targets (includes_daemon_self_update: true)
./dialtone deploy-plan-fixture --deploy-target dialtone
./dialtone deploy-plan-fixture --deploy-target all-daemon
./dialtone deploy-plan-fixture --deploy-target all-with-daemons
```

### Blob Manifest Validation & Guarded Apply

Submit `dialtone.blob_manifest.v1` manifests to `POST /api/deploy/gossip-manifest` or `POST /api/deploy/apply-manifest`:
- Rejects `whole_repo_sync: true`, `ssh_required: true`, `git_required: true`, or empty blob lists.
- Rejects source bundles containing `.git`, `target`, `node_modules`, `.venv`, `dist`, `build`, or `__pycache__`.
- Enforces allowed staging roots (`/opt/dialtone/`, `/var/lib/dialtone/`, `/tmp/dialtone-staging/`, configurable via `DIALTONE_DEPLOY_ALLOWED_ROOTS`).

### Distributed Build Cache Decisions (`--build-case`)

Evaluate build cache reuse and non-blocking async peer builds:

```bash
./dialtone deploy-plan-fixture --build-case unchanged-skip
./dialtone deploy-plan-fixture --build-case cache-reused
./dialtone deploy-plan-fixture --build-case peer-local-async
```

Or via HTTP: `GET /api/build-decision?case=peer-local-async` and `POST /api/deploy/build-request`.

### Dual-Slot Verified Rollback (`rollback-plan-fixture`)

Preview or execute a rollback from `slot-b` (`current_bad_sha256`) to `slot-a` (`previous_good_sha256`) with SHA-256 verification before cutover:

```bash
./dialtone rollback-plan-fixture --node edge-1
```

Or via HTTP: `GET /api/rollback-plan?node=edge-1`, `POST /api/rollback-plan`, and `POST /api/deploy/rollback`.

---

## 9. Service Discovery, Command Journals, Catalogs & Evidence

### Canonical `dialtone://` Service Discovery

Service endpoints use the 3-segment peer-qualified URI format:

```text
dialtone://<peer>/<role>/<name>
```

Examples: `dialtone://node-alpha/control/hub-api`, `dialtone://edge-1/rover/rtcore-cmd`, `dialtone://node-gamma/builder/blob-cache`. Ambiguous role-only URIs (such as `dialtone://rover-1/rtcore-cmd`) are rejected.

### Deduplicated Command Journal & `rtcore-udp` Dispatch

Submit command intents with an `id` to `POST /api/commands/intent`. Terminal or duplicate intents return `"merged": false` without re-executing, while `rtcore-udp` commands encode into deterministic 64-byte binary UDP packets (`RTCORE_ARM_MAGIC: 0x41524d44` / `"ARMD"`, mode-bound v2).

### Program Catalogs & Named Datasets (`program-catalog-fixture`)

Inspect the built-in row calibration catalog or query named datasets (`programs`, `jobs`):

```bash
./dialtone program-catalog-fixture
```

HTTP endpoints: `GET /api/program-catalog`, `GET /api/catalogs`, and `GET /api/catalogs/<dataset_name>`.

### State Pagination, Alert Acks & Session Proofs

- **Paginated State & KV**: `./dialtone state-fixture --node node-alpha` or `GET /api/state?include_kv=true&kv_offset=0&kv_limit=50`
- **Cursor Job Pagination**: `GET /api/cursor/jobs?offset=0&limit=50`
- **Alert Acknowledgement Preview**:
  ```bash
  ./dialtone alert-ack-fixture --actor operator-a --note "verified nominal"
  ```
  (HTTP: `GET /api/alert-ack?actor=operator-a&note=ok` or `POST /api/alert-ack`)
- **Session Proof Validation**:
  ```bash
  ./dialtone proof-validation-fixture --proof-case valid
  ./dialtone proof-validation-fixture --proof-case non-daemon-fallback
  ```
  Supported `--proof-case` values: `valid`, `missing`, `stale`, `cross-session`, `non-daemon-fallback`, `no-renderer-fallback`, `peer-build-deps-missing`.

---

## 10. HTTP API Quick Reference

All `POST` routes require `Authorization: Bearer <token>` (or `X-Dialtone-Token: <token>`) matching `~/.dialtone/api-token`.

| Method & Path | Description |
| :--- | :--- |
| `GET /` (`/nodes`, `/peer`, `/hud`) | Embedded 3D Tactical HUD & Terminal `top` Web UI |
| `GET /ws` | Real-time WebSocket stats stream |
| `GET /health` | Node health status JSON |
| `GET /api/boot-status` (`/api/config`) | 5-stage boot progress, mesh config (`derived_keys` redacted), and blob store path |
| `POST /api/mesh/join` (`/api/config/join`) | Join or switch encrypted gossip channel live |
| `POST /api/daemon/install` | Install persistent OS user daemon |
| `GET /api/state` | Full daemon runtime state (supports `include_kv`, `kv_offset`, `kv_limit`) |
| `GET /api/mesh` (`/api/mesh`) | Active mesh peer beats and `dialtone://` service endpoints |
| `GET /api/iroh-runtime` | Live Iroh 1.3 NodeIdentity, relay, and NIC binding diagnostics |
| `POST /api/blob` | Store base64 payload in SHA-256 verified store (`~/.dialtone/blobs`) |
| `GET /api/blob?sha256=...` | Fetch verified blob bytes & metadata from store (`HEAD` also supported) |
| `POST /api/blob-fetch-report` | Evaluate tiered blob hydration plan (`GET /api/blob-fetch-report`) |
| `POST /api/blob/share` | Share a payload or store SHA-256 via live `iroh-blobs` and return its `iroh-blob://` ticket URI |
| `POST /api/blob/pull` | Pull a remote `iroh-blob://` ticket URI over live QUIC into the local store |
| `GET /api/processes` | List supervised child processes with live PID reconciliation |
| `POST /api/processes/blob` (`/api/processes`) | Stage a store blob (or `ticket_uri`) and supervise it as a daemon process (`running`/`stopped`) |
| `GET/POST /api/process-intent` | Preview start/stop/restart convergence plan and dependency audit |
| `GET/POST /api/deploy-plan` | Generate target deployment plan (`target=robot-v4-default`, `dialtone`, etc.) |
| `POST /api/deploy/gossip-manifest` | Validate a `dialtone.blob_manifest.v1` payload |
| `POST /api/deploy/apply-manifest` | Validate and stage a deployment manifest into allowed roots |
| `POST /api/deploy/build-request` | Evaluate distributed build cache & async peer build (`GET /api/build-decision`) |
| `GET/POST /api/rollback-plan` | Preview side-effect-free dual-slot SHA-256 rollback plan (`POST /api/deploy/rollback` to execute) |
| `GET/POST /api/alert-ack` | Preview side-effect-free alert acknowledgement |
| `POST /api/commands/intent` | Record deduplicated command intent in the command journal |
| `GET /api/cursor/jobs` | Paginated Cursor job log (`offset`, `limit`) |
| `GET /api/program-catalog` (`/api/catalogs/*`) | Program catalog and named datasets |
| `GET /install/v1/install.sh` | Cross-platform shell seed script (`/install/v1/manifest.json`) |

---

## 11. Environment Variables & Local Paths

| Variable / Path | Default | Purpose |
| :--- | :--- | :--- |
| `DIALTONE_STATE_DIR` / `DIALTONE_HOME` | `~/.dialtone` | Root state directory for config, identity, blobs, and staged executables |
| `DIALTONE_IROH_SECRET_KEY_FILE` | `~/.dialtone/iroh-secret-key` | Persistent 32-byte Ed25519 Iroh endpoint secret key (`0600` permissions) |
| `DIALTONE_API_TOKEN` | `~/.dialtone/api-token` | Bearer token required for all `POST` control API routes (and required when `--listen` is non-loopback) |
| `DIALTONE_CHANNEL` | `dialtone` | Default gossip channel name when `--channel` is not passed |
| `DIALTONE_SECRET` (`DIALTONE_SECRET_KEY`) | *(unset)* | Secret used to derive Argon2id `v2` channel keys on boot |
| `DIALTONE_NODE` | System hostname | Override default node name when `--node` is not passed |
| `DIALTONE_QUIC_PORT` | *(OS dynamic)* | Optional deterministic UDP port for the Iroh QUIC endpoint (`--quic-port`) |
| `DIALTONE_DEPLOY_ALLOWED_ROOTS` | `/opt/dialtone/,/var/lib/dialtone/,/tmp/dialtone-staging/` | Comma-separated allowed filesystem prefixes for manifest staging |
| `~/.dialtone/config.json` | — | Host-local mesh config (`0600`): Argon2id `derived_keys` (no plaintext secret), `seeds`, `allowlist`, `used_invite_nonces`, and supervised process specs |
| `~/.dialtone/api-token` | — | Auto-generated 256-bit local HTTP control API bearer token (`0600`) |
| `~/.dialtone/blobs/` | — | SHA-256 Content-Addressed Blob Store (`<2-hex>/<64-hex>`) |
| `~/.dialtone/run/` | — | Staged `0755` executables spawned by the blob process supervisor |

---

## 12. Self-Test, Live Mesh Verification & Local CI

Run the built-in 7-capability live verification suite, the live multi-node mesh verification command, or the full local CI pipeline at any time:

```bash
# Built-in 7-capability live self-test (spawns ephemeral local Iroh 1.3 gossip & store checks)
./dialtone self-test

# Verify the running local daemon's security posture, zero public directory exposure,
# N-peer encrypted gossip convergence, and live QUIC blob transfer
./dialtone mesh-test --expect-peers 3 --timeout 15s

# Full local CI suite (fmt, cargo test, release build, CLI fixtures, and live blob process crash auto-respawn)
./ci.sh
```
