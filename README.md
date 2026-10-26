# Dialtone User Guide

`dialtone` is a single-binary Rust + **Iroh 1.3** peer-to-peer mesh control plane, 3D tactical HUD & terminal `top` monitor, content-addressed blob store (`iroh-blobs`), and process supervisor. It runs without SSH, Git, or Tailscale/VPN runtime dependencies.

This guide uses the project's [domain language](goal.md#domain-language): one word per concept (Node, NodeId, Peer, Mesh, Channel, Secret, Allowlist, Invite, Seed, Message, Beat, Inventory, Stats, Blob, Digest, Store, Ticket, Program, Process, Probe, HUD, API).

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
  probe [--all]            Run one probe now: the gossip checks (MSH, NBR, BEAT; nodes run them
                           every 30s), and with --all the store, pulls and programs (STO, PUL, PRC)
  devices [--json]         List the devices connected to this machine (cameras, mics, keyboards, ...)
  mesh-test                Check the running node's security, mesh convergence and blob transfer
  hud                      Serve the HUD and API only (no mesh)
  node                     Run the node without the HUD or API
  status                   Print this node's NodeId and transport report
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
  --os <macos|linux|windows> Target OS for service unit generation
  --once                   Run one cycle and exit
  --all                    'dialtone probe': also test the store, pulls and programs
```

HUD keys (at http://127.0.0.1:8788):

```text
  Up / Down Arrow                          Zoom camera in/out across Layers 1..5 (GLOBE -> PROCESS)
  Left / Right Arrow                       Step backward/forward through the 5-step tour (MSH BLB PRC NET DEV) and nodes
  Double-Click Node / Row                  Zoom into focused node symbol (resets to L1 GLOBE after L5)
  Click [1 MSH] .. [5 DEV] in Bottom CC    Jump directly to any capability step in the live Closed Captions
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

### 4. Optional: Install as a Background Login Service & Check the Mesh

```bash
# Register dialtone as a user-level OS service (launchd / systemd --user / Windows Scheduled Task)
./dialtone daemon install

# Check the running node: security, mesh convergence and a live blob transfer
./dialtone mesh-test --expect-peers 1
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

Running `./dialtone` (with no arguments, or `./dialtone up`) boots 5 subsystems in strict dependency order and starts the local Web HUD immediately:

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

Open **`http://127.0.0.1:8788`** to access the embedded tactical HUD:

- **Terminal `top` table**: mesh-wide rows (`MSH`, `NET`, `PRB`, `CPU`, `MEM`, `DSK`), where `PRB` counts each probe check passing across nodes (`MSH` `NBR` `BEAT` `PUL` `PRC`), then one row per node (`NODE`, `ARCH`, `CPU`, `MEM`, `DSK`, `IP/PATH`, `RX/TX`, `LNK`, `RTT`, `JIT`, `PROC`, `PRB`), then the focused node's devices.
- **Threshold colours**: values stay greyscale when nominal, turn amber at the warning level and red at the error level (CPU 75% / 92%, memory 78% / 92%, disk 82% / 94%).
- **Tour**: visits every node and steps through `MSH` (mesh), `BLB` (store and blob pulls), `PRC` (probe program), `NET` (beats and Rtt) and `DEV` (devices); the closed captions narrate that node's latest probe.
- **Interactive 3D Mesh Topology**: Drag to orbit, scroll to zoom, or click any peer node to inspect its live QUIC links and metadata.
- **Menu** (bottom-right button): tour and camera controls, zoom layers, display toggles, and — under
  *05 // Mesh setup (CLI)* — the commands for creating a mesh, inviting and allowing nodes. The page holds no API
  token, so mesh changes are made with the CLI.
- **Endpoints the HUD uses**: `GET /` (the page), `GET /hud/...` (its static files) and `GET /api/mesh` (polled
  every 1.5s).

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

Within a few seconds of starting, verify that all nodes are exchanging encrypted messages (`dialtone.iroh.envelope.v5`) and incrementing their beat `counter`. All nodes must run the same envelope version, so upgrade a mesh together:

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
- **Signed messages.** Nodes gossip three typed messages (see [Scaling the mesh](#scaling-the-mesh)): a **Beat** (liveness, stats, the node's probe summary and its neighbors' Rtts), a **Summary** (a beat with the 30-second stats summary, every sixth beat), and an **Inventory** (the node's processes, services, devices and details) when it changes. Each message is signed with the sender's Key and encrypted with the channel keys. A node drops messages whose signature does not match the NodeId they claim, messages from NodeIds not on its allowlist, and replays (each node's `sent_unix_ms` must increase and be within 5 minutes of local time — keep node clocks roughly in sync).
- **No process secrets on the wire.** An inventory lists each process without its `env` or `args`.
- **Deploys follow their origin.** A node that runs a copy of a peer's program never offers it further, and removes it as soon as that peer stops offering it, so removing a program on the node that deployed it removes it everywhere.
- **Deploys only from the allowlist.** A process offered for mesh deploy is pulled only from an allowlisted signer and an allowlisted blob provider, never a peer's probe program, and only once per (peer, digest). Blob pulls use the node's own endpoint, so the provider's allowlist decides.
- **HUD.** The page carries no API token; a per-response Content-Security-Policy nonce lets only its own scripts run. three.js and h3-js are compiled into the binary and served from `/hud/vendor/`, so the HUD loads nothing from the Internet.

### Unlock & Join the `"dialtone"` Channel (`join` & `secret`)

The `"dialtone"` channel (and any custom `--channel <name>`) is **secret-protected and end-to-end encrypted**:
- Booting `dialtone` without a configured secret places the node on an isolated per-node standby topic (`dialtone:locked-standby`) and does **not** dial mesh seeds until unlocked.
- **Minimum Secret Strength**: Secrets must be at least 16 characters long with multiple character classes and `>= 8` unique characters; weak or dictionary secrets (`"dialtone"`, `"dialtone2"`, `"secret123"`, `"default"`) are rejected. Run `./dialtone secret` to generate a CSPRNG 256-bit secret (`dt-...`).
- **Argon2id Key Derivation (`v2`) & XChaCha20-Poly1305 Gossip Encryption**:
  1. `dialtone` derives three domain-separated 32-byte subkeys (`topic_id`, `auth_key`, `enc_key`) from `(channel, secret)` using **Argon2id** (`m=19456 KiB, t=2, p=1`).
  2. Only the derived subkey bundle (`derived_keys`) is persisted in `~/.dialtone/config.json` (`0600` permissions) — the plaintext secret is **never** written to disk. Any legacy `~/.dialtone2/config.json` is automatically migrated and scrubbed on startup.
  3. Every message broadcast over `iroh-gossip` is signed with the sender's Key and encrypted with `enc_key` using **XChaCha20-Poly1305** (`dialtone.iroh.envelope.v5`: a binary frame, random 24-byte nonce). Unencrypted or unsigned frames are dropped.
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
| `--private-mesh` | Zero-internet private mesh: disables public relays, DNS/Pkarr and Mainline DHT; peers are dialed at the direct `ip:port` addresses from `allowlist add --addr` or invites. |
| `--quic-port <port>` | Binds the Iroh QUIC UDP socket to a fixed port (e.g. `19440`) so peers can dial it directly. |
| `--lan` / `--local-discovery` | Enables zero-conf mDNS local network discovery (`lan_only` profile). |
| `--dns` | Explicitly enables public Pkarr DNS address lookup/publishing (`dns.iroh.link`). |
| `--dht` | Explicitly enables Mainline BitTorrent DHT address publication and lookup. |
| `--no-dns` | Keeps DNS/Pkarr address lookup disabled (default). |
| `--no-relay` | Disables Iroh relay servers (direct UDP/QUIC only). |
| `--relay-only` | Strips direct IP addresses from tickets so traffic routes strictly via relay (`relay_only` profile). |
| `--relay-url <url>` | Adds a custom Iroh relay server URL (repeatable). |
| `--invite <t>` | Redeems a `dialtone-invite-v1:` invite or adds a seed list when joining (repeatable). |
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

---

## 6. Blob Store

The store is an iroh-blobs `FsStore` in `~/.dialtone/blobs/` (`blobs.db`, `data/`). Every blob is named by its digest through the tag `sha256/<hex>`, which points at its BLAKE3 hash; tagged blobs are kept and garbage collection removes anything untagged (such as an interrupted download). Uploads are checked against their SHA-256 before they are added, and a pulled blob is checked against the ticket's digest before it is tagged. Blobs left in the old one-file-per-digest layout are imported the first time the node starts. A store belongs to one process: `dialtone hud` and one-shot runs next to a running node use a private temporary store.

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

### Live Cross-Peer `iroh-blobs` QUIC Share & Pull (`blob share` / `blob pull`)

Every running `dialtone` daemon attaches `iroh_blobs::BlobsProtocol` to its live QUIC router so peers can share and pull content-addressed blobs directly over QUIC:

```bash
# On Node A: add a file (or text) to the store and get a ticket for it
./dialtone blob share ./process-script.sh
# -> {"digest": "sha256:...", "ticket_uri": "iroh-blobs://ticket/blob...#sha256:...", ...}

# On Node B: pull it over QUIC from Node A; its SHA-256 is checked against the ticket before it is kept
./dialtone blob pull "iroh-blobs://ticket/blob...#sha256:..." --force
```

---

### Self-test loop (Probe)

Every running node checks the gossip mesh every 30 seconds. These checks read what gossip already brought, so they cost no traffic of their own:

| Check | What it proves |
| :--- | :--- |
| `MSH` | Every allowlisted peer is in the mesh |
| `NBR` | The node has at least two gossip neighbors (one in a two-node mesh) |
| `BEAT` | Every peer's beats are fresh (within three of its beat intervals) and every peer hears as many nodes as this one |

A full probe also tests the node's other capabilities, which move data, so it runs only when asked (`dialtone probe --all`, or `POST /api/probe` with `{"all": true}`; `deploy` runs one on this machine):

| Check | What it proves |
| :--- | :--- |
| `STO` | A fresh blob is written to the store and read back intact |
| `PUL` | Up to 3 peers, chosen at random, each write a fresh blob on request; it downloads over QUIC through the node's downloader, SHA-256 verified |
| `PRC` | The probe program, staged from the store, is running under the supervisor |

A compact report (the checks, without per-peer results) travels in the node's beats, so each HUD shows every node's results (PRB row, tour steps). Failed checks are logged. Probe blobs are forgotten after use, so the store does not grow. `dialtone probe` (or `POST /api/probe`) runs one immediately; `GET /api/probe` returns the latest.

### Scaling the mesh

Gossip is routed the way [libp2p GossipSub](https://docs.libp2p.io/concepts/pubsub/overview/) routes it, on top of iroh-gossip's HyParView membership and Plumtree broadcast tree:

- **A few neighbors, not every node.** Each node keeps 2 to 5 gossip neighbors. Below two (GossipSub's `D_lo`) it joins up to three known peers chosen at random; nobody dials every other node. A message reaches the whole mesh along the broadcast tree, each node forwarding it once.
- **Small, fixed-size announcements.** A beat is about 1.2 KB sealed and does not grow with the mesh: no seed lists, no per-peer probe results, none of the node's details. The beat announces its node's `inventory_version`.
- **Fetch on demand.** The inventory and anything larger goes over a request–answer protocol of its own (`dialtone/fetch/1`, `fetch.rs`). A node that sees a beat announcing an inventory version it does not have (it missed the broadcast, or joined later) fetches the inventory from that node. Full probes fetch fresh blob tickets the same way.
- **Separate topics.** Beats go on the channel's beats topic; inventories are broadcast on its events topic when they change, so beats stay small and regular.
- **No echo broadcasts.** Rtt is timed with an empty request over a fetch connection to each gossip neighbor (the Links actor), not from replies broadcast to the whole mesh.
- **Beat interval grows with the mesh.** Every node hears every beat, so a node beats every `clamp(nodes / 10, 5, 60)` seconds: about 10 incoming beats per second per node at any size (5s up to 50 nodes, 30s at 300). Freshness checks use each peer's own interval.

`./dialtone.sh scale N [secs]` starts N nodes on this machine, each given 3 random seeds, and reports how long until every node sees all N and the traffic per node (`GET /api/net`: socket bytes from iroh's counters, gossip messages and neighbors). Measured on one Mac:

| Nodes | Before | After |
| ---: | :--- | :--- |
| 10 | converged in 4s; 56 Mbit/s in and out per node, 2001 connections opened | converged in 7s; 60 kbit/s per node, 4.6 neighbors |
| 30 | never converged (180s) | converged in 8s; 120 kbit/s per node |
| 50 | — | converged in 19s; 200 kbit/s per node |
| 100 | — | converged in 48s on one overloaded Mac |

### Devices

Every node lists the devices connected to it — cameras, microphones, speakers, keyboards, pointers (mice, trackpads), displays, printers, storage, Bluetooth devices and gamepads — using the OS's own view: `system_profiler` on macOS, `Get-PnpDevice` on Windows, `/sys` and `/proc` on Linux. Only each device's kind, name, bus (`built-in`, `usb`, `bluetooth`, `hid`, `display`, ...) and a short detail are kept; serial numbers and device IDs are dropped.

```bash
./dialtone devices          # table of this machine's devices
./dialtone devices --json   # the same as JSON
curl -s http://127.0.0.1:8788/api/devices | jq
```

A running node lists its devices every 60 seconds and gossips them in its inventory when they change, so every node's devices appear in `GET /api/mesh` (`nodes[].devices`). The HUD lists the focused node's devices under the node table, and the tour's `DEV` step describes them. Device names (for example a Bluetooth headset named after its owner) are visible to every node in the mesh.

---

## 7. Supervised Processes

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

# Remove it: stop it, forget its spec and delete its staged program
# (on the node that deployed it to the mesh, this removes every copy)
curl -X POST http://127.0.0.1:8788/api/processes \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "stats-blob-process", "desired_state": "removed"}'
```

- **Crash Auto-Respawn**: Every second (and on process status queries), the daemon checks all managed child PIDs via `try_wait()`. If a process with `"desired_state": "running"` exits or crashes, `dialtone` increments `restart_count`, re-stages the verified store blob, and spawns a fresh child PID automatically.
- **Environment Sanitization**: the node's own `DIALTONE_*` / `DAEMON_V1_*` variables (such as `DIALTONE_SECRET`) are removed from every process's environment, and a spec's `env` entries with those prefixes are ignored.
- **Restart Persistence**: process definitions are saved to `~/.dialtone/config.json` and automatically re-staged and started when `dialtone` boots.

## 8. HTTP API Quick Reference

All `POST` routes require `Authorization: Bearer <token>` (or `X-Dialtone-Token: <token>`) matching `~/.dialtone/api-token`.

Requests are parsed with `httparse`. The API refuses what it never needs: `Transfer-Encoding` (501), missing, duplicate or conflicting `Content-Length` values and bodies shorter than declared (400), bodies over 2 MiB (413), and request heads over 64 KiB or with more than 64 headers (431). Query values are percent-decoded.

| Method & Path | Description |
| :--- | :--- |
| `GET /` | The HUD (served with a per-response Content-Security-Policy nonce) |
| `GET /hud/...` | The HUD's static files (`app.js`, `style.css`, vendored three.js and h3-js) |
| `GET /health` | `{"status": "ok", "node": ..., "peers": ..., "boot_stages": ...}` |
| `GET /api/config` (`/api/boot-status`) | Boot stages, mesh config (`derived_keys` redacted), store path |
| `POST /api/mesh/join` | Join or switch the channel of the running node |
| `POST /api/daemon/install` | Install the daemon (OS login service) |
| `GET /api/state` | Full node state |
| `GET /api/mesh` | Every node's latest beat, the Rtt matrix and Services |
| `POST /api/blob` | Put a base64 blob in the store (`~/.dialtone/blobs`) |
| `GET /api/blob?digest=...` | Blob bytes and metadata (`HEAD` also supported) |
| `POST /api/blob/share` | Put a blob in the store and return its `iroh-blobs://ticket/...` |
| `POST /api/blob/pull` | Pull a blob by ticket over QUIC into the store |
| `GET /api/processes` | Supervised processes with live PID / status |
| `GET /api/devices` | Devices connected to this node (cameras, microphones, keyboards, ...) |
| `POST /api/processes/blob` | Stage a program blob (or ticket) and run / stop it as a process |
| `POST /api/processes` | Same, and `{"name": ..., "desired_state": "removed"}` stops a process and forgets it (mesh-deployed copies follow) |
| `POST /api/probe` | Run one probe now, `{"all": true}` for a full probe (`GET` returns the latest); nodes check the mesh every 30s |
| `GET /api/net` | Traffic since boot: socket bytes in and out, connections, gossip messages, neighbors |


---

## 9. Environment Variables & Local Paths

| Variable / Path | Default | Purpose |
| :--- | :--- | :--- |
| `DIALTONE_STATE_DIR` / `DIALTONE_HOME` | `~/.dialtone` | Root state directory for config, identity, blobs, and staged executables |
| `DIALTONE_IROH_SECRET_KEY_FILE` | `~/.dialtone/iroh-secret-key` | Persistent 32-byte Ed25519 Iroh endpoint secret key (`0600` permissions) |
| `DIALTONE_API_TOKEN` | `~/.dialtone/api-token` | Bearer token required for all `POST` control API routes (and required when `--listen` is non-loopback) |
| `DIALTONE_HUD_DIR` | *(unset)* | Serve the HUD from this directory (e.g. `src/hud`) instead of the copy in the binary, uncached: HUD edits show on a browser reload with no rebuild or restart |
| `DIALTONE_CHANNEL` | `dialtone` | Default gossip channel name when `--channel` is not passed |
| `DIALTONE_SECRET` (`DIALTONE_SECRET_KEY`) | *(unset)* | Channel secret used to derive Argon2id `v2` channel keys on boot |
| `~/.dialtone/config.json` | — | Host-local mesh config (`0600`): Argon2id `derived_keys` (no plaintext secret), `seeds`, `allowlist`, `used_invite_nonces`, and supervised process specs |
| `~/.dialtone/api-token` | — | Auto-generated 256-bit local HTTP control API bearer token (`0600`) |
| `~/.dialtone/blobs/` | — | Blob store (iroh-blobs `FsStore`; blobs named by `sha256/<hex>` tags) |
| `~/.dialtone/run/` | — | Staged `0755` executables spawned by the blob process supervisor |

---

## 10. Working on dialtone: `./dialtone.sh`

One script builds, tests and deploys. `cargo test` covers each capability with real iroh endpoints (signed gossip between two nodes, blob pulls through the node's downloader, the handshake allowlist, a two-node probe, the supervisor and the API); the script adds live tests on top.

```bash
./dialtone.sh check     # lint, fmt, tests, release build, one-node test, three-node test (about 35s)
./dialtone.sh lint      # leak gate (no host names / NodeIds in git) and domain language
./dialtone.sh e2e       # three real nodes on this machine: mesh, gossip probe 3/3, full probe 6/6
                        # with blob pulls between them, deploy and removal across the mesh, a
                        # stopped node caught (about 30s)
./dialtone.sh scale 30  # 30 nodes on this machine: convergence time and traffic per node
./dialtone.sh deploy    # check, build, install on every node, wait until every node's probe
                        # passes on the exact binary just installed, then a full probe here
./dialtone.sh deploy --skip-check   # ... without running check first
./dialtone.sh fleet     # each node's latest probe in the live mesh
```

`deploy` and `fleet` read your fleet's hosts from `fleet.local.env` next to the script. It is never committed (`*.local.env` is ignored), so host names and addresses stay on your machine:

```bash
FLEET_NODES="node-alpha node-beta node-gamma"   # every node; deploy waits for all of their probes
LOCAL_NODE="node-alpha"                          # this machine (macOS, launchd)
LOCAL_LAUNCHD="com.timcash.dialtone"
WINDOWS_NODES="node-beta node-gamma"             # the first compiles the Windows binary
SSH_node_beta="node-beta|-J node-gamma user@10.0.0.2"   # SSH alternatives; first that answers wins
SSH_node_gamma="node-gamma"
```

To check a single running node by hand, `./dialtone mesh-test --expect-peers 3` verifies its security posture, mesh convergence and a live blob transfer.

**Build profiles.** `cargo build --release` is the everyday optimized build (no LTO, parallel and incremental codegen): `dialtone.sh`, GitHub CI and fleet deploys use it, and a one-file change rebuilds in about a second. Published binaries are built with `cargo build --profile dist` (size-optimized, fat LTO), which takes about a minute and only runs when a version is tagged.
