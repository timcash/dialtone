# Dialtone Security, Authentic Telemetry & Zero-Leak Fleet Architecture (`goal.md`)

## Main Features (System Map)

| # | Feature | What it does | Where it lives |
|---|---------|--------------|----------------|
| F1 | **Iroh identity & QUIC transport** | Persistent Ed25519 `EndpointId` (`~/.dialtone/iroh-secret-key`), endpoint presets (N0 / minimal / private mesh), relay / Pkarr DNS / DHT / mDNS policy, tailnet-address filtering, transport diagnostics | `src/identity.rs` |
| F2 | **Encrypted gossip mesh & membership** | Channel password → Argon2id subkeys (topic, auth, enc), XChaCha20-Poly1305 gossip envelopes, heartbeat MAC, deny-by-default `EndpointId` allowlist, signed single-use invite tickets, seed peers, `~/.dialtone/config.json` persistence | `src/gossip.rs`, run loop in `src/main.rs` |
| F3 | **Heartbeats, telemetry & RTT** | 5s regular / 30s summary heartbeats, real OS CPU / mem / disk / net sampling (macOS, Linux, Windows), rolling variability + thrashing detection, monotonic RTT echoes → mesh RTT matrix, public-IP / latency-cluster geo placement | `src/evidence.rs`, `src/gossip.rs` |
| F4 | **SHA-256 blob store & iroh-blobs transfer** | Content-addressed cache under `~/.dialtone/blobs`, `blobs share` / `blobs pull` tickets over QUIC with SHA-256 re-verification, tiered hydration plan (cache → iroh-blobs → guarded HTTP fallback), per-node `blob_store` stats in heartbeats | `src/blobs.rs` |
| F5 | **Process supervision & E2E worker** | Stage executables from verified blobs, spawn / respawn-on-crash / stop managed processes, default cross-platform `mesh-blob-worker`, 5-capability E2E tick (BLB, KVS, PRC, MSH, A/B) | `src/supervisor.rs`, `DaemonRuntimeState` in `src/http.rs` |
| F6 | **Deploy, A/B rollback & OS daemon install** | Deploy manifest contract, build-cache probe & build decision, A/B slot rollback planning / verified execution, launchd / systemd `--user` / Windows Scheduled Task install | `src/deploy.rs` |
| F7 | **Daemon runtime & local control API** | 5-stage boot, shared runtime state, local HTTP API + WebSocket on `127.0.0.1:8788`, bearer-token POST auth, Host / Origin (CSRF, DNS-rebinding) checks, live channel join | `src/main.rs` (`run_node_or_hub`), `src/http.rs` |
| F8 | **3D tactical HUD** | Embedded Three.js + H3 globe, terminal `top` overlay, 6-step guided tour with closed captions, live blob-download / subprocess-start FX | `render_dashboard_html` in `src/http.rs`, `src/h3_land.rs` |
| F9 | **Verification suite** | `self-test`, `fleet-test`, `test-tick`, unit + live Iroh tests, `ci.sh` (zero-leak gate, fmt, tests, live acceptance), GitHub CI matrix | `src/lib.rs`, `ci.sh`, `.github/workflows/` |
| F10 | **Metadata, service discovery & evidence records** | Peer / service refs, service advertisements, key leases, telemetry templates, policies, events / alerts / logs, KV records with cursor pagination, proof validation | `src/metadata.rs`, `src/evidence.rs` |
| F11 | **CLI & diagnostic fixtures** | Argument parsing, `help`, and the `*-fixture` / planner preview commands | `src/main.rs` |

## Code Review (2026-10-23): Findings, Cleanup & Target Iroh Architecture

### Critical findings (fix before anything else)

1. **The allowlist trusts a self-reported identity.** Received heartbeats are checked against `rx_hb.endpoint_id`, a field inside the payload (`main.rs` ~1683). iroh-gossip's `delivered_from` is the relaying neighbour, not the author, so any holder of the channel password can claim an allowlisted node's id. The `Router` also accepts the gossip and blobs ALPNs from *any* `EndpointId` (`main.rs` ~1005), and custom channels have no allowlist at all.
2. **Channel password ⇒ fleet-wide code execution.** A heartbeat whose `managed_processes[].env` contains `DIALTONE_MESH_DEPLOY=1` + `DIALTONE_BLOB_TICKET=…` makes every receiver pull, stage and run that binary (`main.rs` ~1800-1937). Combined with (1), the password is the only security boundary. The guard `!already_running || insert(..)` is also inverted, so a non-running deploy is re-pulled on a new OS thread + new endpoint on *every* heartbeat.
3. **Heartbeats broadcast every process `env`** (incl. blob tickets and any secret a process is given) to all peers.
4. **HUD token theft via XSS.** The bearer token is templated into the unauthenticated `GET /` page, state JSON is injected into `<script>` without `<` escaping, and peer-supplied node names go into `innerHTML`. A channel member can name a node with markup and steal the token.
5. **No replay protection.** `counter` is never checked and `updated_at_unix` is overwritten with local time, so captured frames replay as fresh.
6. **The mesh stalls behind HTTP.** One `std::sync::Mutex<DaemonRuntimeState>` is held while rendering/writing HTTP responses (2s write timeout), spawning processes, and pulling blobs (up to 15s); the gossip loop blocks on the same lock.

### Per-feature findings

- **F1 Transport.** Sound core (persistent key, 0600). Issues: seed addresses are re-injected into `MemoryLookup` every 5s unfiltered (tailnet leak); bind silently falls back to all interfaces; `RelayOnly` only filters what is advertised; public n0 relay URL hardcoded in 5 places; `build_diagnostics` does a UDP probe per heartbeat.
- **F2 Gossip mesh.** MAC (keyed SHA-256, not HMAC) is redundant with the AEAD and uses the same secret; heartbeats are signed twice; KDF cache keeps plaintext passwords for the process lifetime; `Debug` derives print keys; corrupt `config.json` silently becomes `Default` and the next save wipes keys + allowlist; allowlist snapshotted at boot (`peers allow/revoke` needs a restart); re-subscribe has no backoff and can spin; JoinMesh swaps keys before `subscribe` succeeds; invite "single-use" is only enforced on the redeemer. Dead: `derive_channel_auth_key`, `derive_channel_enc_key`, `is_allowed_main_fleet_peer`, `parse_secret_or_ticket`, `compute_heartbeat_mac`, `sign_heartbeat_with_password`, `verify_heartbeat_password`, `IrohTransportConfig::lan_only`; legacy `.dialtone2` migration + `dialtone-gossip-v1:` parsing.
- **F3 Telemetry & RTT.** macOS sampler forks `vm_stat` / `df` / `netstat` every second on the async runtime under a global mutex; remaining fake values in live mode (`link_speed_mbps = 1000`, `tailnet=off`, 16 GiB / 8-core fallbacks, failures reported as 0%); `HostResourceTelemetry::default()` samples *this* host, so a peer heartbeat missing the field shows local numbers; NIC rate can be computed across two different interfaces; echo RTT is a full-mesh broadcast (N×(N−1) per 15s) that measures gossip-overlay hops, not link RTT; `upsert_heartbeat` never expires `peer_rtt_ms` / `e2e_test`; peers keyed by node name, not `EndpointId`; heartbeat built in 6 places; unconditional third-party `ipinfo.io` lookup with ~1 km coordinates gossiped to all peers and table guesses labelled `public_ip_geo`.
- **F4 Blobs.** Two stores (iroh-blobs `MemStore` + SHA-256 file CAS) with every blob tagged into RAM forever (no GC); provider state lost on restart so stored tickets go stale; each pull binds a throwaway endpoint (new identity, fresh NAT traversal, temp-file name collides within one second); `BlobsProtocol` has no connection filter. Fixture-only: `evaluate_tiered_blob_fetch` (reports `transferred_bytes` for a plan that transfers nothing), `transfer_blob_via_iroh_ticket`.
- **F5 Supervision.** No signal handling, so children (and grandchildren — no process group / Job Object) orphan on SIGTERM; no restart backoff; spawn errors swallowed (stale `running` / pid); `reconcile` polled from 4 places incl. GET handlers; E2E tick re-stages blobs every 5s and `mesh_connectivity_ok` is true by construction. Fixture-only: rtcore UDP loopback, `RobotCommandIntentRecord` dispatch, `CursorJobRecord`, systemd drop-in reconcile.
- **F6 Deploy.** Live: A/B rollback (fed synthetic slot bytes) and `install_persistent_os_daemon`. Preview-only: `generate_deploy_plan` (4 fixed synthetic peers), `evaluate_deploy_build_decision` (fixed timings/hash), `probe_build_cache` (cache list never populated), `validate_manifest_contract` (accepted manifests go nowhere), `render_install_v1_sh`. Service templates are duplicated between `render_os_service_unit` and the installer and have drifted.
- **F7 Runtime & API.** `run_node_or_hub` is one ~1,300-line future (no `tokio::spawn`, no shutdown phase); serialized awaits in the 5s tick (~0.6s + N×350ms + 0.6s); `?` inside the loop can kill the daemon; one accept error silently ends the HTTP server; peers pruned only inside HTTP requests; GET routes mutate state. Hand-rolled HTTP lacks chunked bodies, truncation checks, percent-decoding; `/ws` is a one-frame stub. ~25 routes are called by nothing (HUD uses only `/` and `/api/gossip`).
- **F8 HUD.** ~3,500 lines of HTML/JS in a Rust string; three.js / h3-js loaded from a CDN without SRI (fails on air-gapped meshes, contradicts "UI compiled into the binary"); **Init / Invite / Join buttons call `/api/channel/*` routes that do not exist**; polls `/api/gossip` every 1.5s.
- **F9 Verification.** ci.sh lines ~54-89 assert on seeded fixture strings; self-test steps 4/6/7 validate lookup tables and synthetic bytes rather than behaviour.
- **F10 Metadata & evidence.** Dead now: `TelemetryTemplate`, `NodeRoleConfig`, `SessionArtifactPolicy`, `CursorPolicy`, `ServiceStatusProbe` / `hydrated_for_node`, `DialtoneMetadata::load_from_file` / `roles_for_node` and the unread metadata fields. Self-test-only: `PeerRef`, `resolve_service`, `KeyLease*`, `ProofValidationReport`. `dialtone index` serves fabricated peers, PID `4242`, NATS service, alerts (goal §0 forbids this).
- **F11 CLI & utilities.** 11 preview fixtures + `build_seeded_state`; `main` ~700 lines, `parse_cli` ~360; hand-rolled hex / base64 / SHA-1 (WebSocket handshake only) duplicate `hex`, `base64`, `sha1_smol` already in `Cargo.lock`.

### Target Iroh architecture

```text
                 ┌────────────── one iroh Endpoint (persistent Ed25519 key) ──────────────┐
                 │ hooks: after_handshake → Reject unless remote_id ∈ live allowlist      │
                 │ Router ── GOSSIP_ALPN ── iroh-gossip (one long-lived subscription)      │
                 │        └─ iroh_blobs::ALPN ── BlobsProtocol(FsStore, EventSender:      │
                 │                                 intercept ClientConnected → allowlist) │
                 └─────────────────────────────────────────────────────────────────────────┘
 Actors (tokio tasks, mpsc commands + watch snapshots, CancellationToken shutdown):
   Transport ─ Mesh ─ Telemetry ─ Supervisor ─ BlobService ─ ApiServer(hyper/httparse) ─ HUD(static files)
```

- **Identity = authorization.** Enforce the allowlist at the QUIC handshake for every ALPN and channel, fed from an `Arc<RwLock<HashSet<EndpointId>>>` that `peers allow/revoke` update live.
- **Signed, typed messages.** Every gossip payload is `SignedMessage { from: EndpointId, sig, body }` (iroh-gossip chat example pattern), verified against the node key and the allowlist; AEAD stays for confidentiality; the SHA-256 MAC goes. `body` is an enum: `Beat` (small, every 5s, monotonic counter checked), `Summary` (30s), `Inventory` (processes without `env`, services, blob digests — on change), `Deploy` (see below).
- **Let iroh find peers.** Rely on the endpoint's address lookup (pkarr/DHT or one-time `MemoryLookup` seeding) instead of gossiping all-peer tickets and re-joining every 5s; use `neighbors()` / `joined()` for membership; RTT from iroh connection paths instead of broadcast echoes.
- **One blob store.** `FsStore` (feature `fs-store`) as the only store, SHA-256 kept as a verified tag (`sha256/<hex>` → BLAKE3 hash), GC via tags; pulls through the daemon's own endpoint (`store.downloader(&endpoint)`), no temp endpoints or temp files.
- **Deploy is explicit and signed.** Replace the env-flag auto-deploy with a `Deploy` message signed by an operator key (or an allowlisted node with a deploy capability), naming sha256 + target nodes; receivers verify signature, provider ∈ allowlist and sha before staging.
- **Supervisor owns children.** `tokio::process` with `kill_on_drop`, process groups (Unix) / Job Objects (Windows), exponential restart backoff, spawn errors recorded in status, `shutdown_all` on SIGTERM / ctrl-c.
- **API & HUD.** API task reads `watch` snapshots and never holds the mesh lock; HUD moves to `src/hud/*` via `include_str!` with vendored three.js / h3-js; token fetched by an authenticated bootstrap call, never templated; one live channel (`/ws` push or polling — not both).

### Cleanup plan

| Tier | What | Size |
|---|---|---|
| A — delete now (no callers) | Dead F2 functions listed above, `lan_only`, dead F10 metadata types + fields + their tests | ~400 lines |
| B — delete with ci.sh / README / test edits | All `*-fixture` commands + `build_seeded_state` + `new_seeded`; preview planners (deploy plan, build decision, build cache, proof validation, alert ack, tiered fetch report, install.sh, catalogs, manifests); rtcore UDP / robot intents / cursor jobs / systemd drop-in; ~25 uncalled routes; self-test steps 4/6/7 | ~1,500 lines Rust + ~35 ci.sh + ~120 README |
| C — replace | hex / base64 / SHA-1 → `hex`, `base64`, `sha1_smol` (already in lockfile) | ~150 lines |

### Roadmap

1. **Phase 1 — Security (critical 1-5):** handshake allowlist hook + live allowlist, Ed25519-signed payloads with counter check, remove env-flag auto-deploy (gate behind signed `Deploy`), stop gossiping `env`, HUD escaping / `textContent` / no templated token.
2. **Phase 2 — Reliability:** release the state lock before IO, ctrl-c shutdown + child process groups, re-subscribe backoff, no `?` in the loop, config parse errors fail loudly, telemetry unknowns as `None`, peers keyed by `EndpointId` with TTL pruning.
3. **Phase 3 — Cleanup:** tiers A → B → C, fix or remove the HUD `/api/channel/*` buttons.
4. **Phase 4 — Architecture:** actor split of `run_node_or_hub`, FsStore + downloader, typed message enum + inventory-on-change, hyper/httparse server, HUD as static files with vendored JS.

## 0. Current Priority Goal: Authentic Telemetry, Inter-Host RTT, & 5s / 30s Summary Variability Heartbeats

1. **100% Authentic Telemetry, Logs & Heartbeats (Zero Fake Numbers)**:
   - Replace all synthetic/hardcoded telemetry values (such as fake `23.4%` / `120 GiB / 512 GiB` disk usage, fake `50.0%` / `8 GiB / 16 GiB` RAM, and synthetic counter-based CPU/network waves) with real OS-level measurements across macOS, Linux, and Windows:
     - **CPU**: Real logical core count and real CPU utilization percentage computed from OS kernel tick deltas (`host_statistics(HOST_CPU_LOAD_INFO)` on macOS, `GetSystemTimes` on Windows, `/proc/stat` on Linux).
     - **Memory**: Real physical RAM total, used bytes, and percentage (`sysctl hw.memsize` + `vm_stat` on macOS, `GlobalMemoryStatusEx` on Windows, `/proc/meminfo` on Linux).
     - **Disk**: Real filesystem total bytes, used bytes, and percentage for the state volume (`df -Pk` on macOS/Linux, `GetDiskFreeSpaceExW` on Windows).
     - **Network**: Real physical interface name, link status, cumulative RX/TX bytes, and live RX/TX bytes/sec throughput (`netstat -ibn` on macOS, `netstat -e` on Windows, `/proc/net/dev` on Linux).
   - Ensure live daemon state (`dialtone up`) records real runtime `logs`, `events`, `alerts`, `installed_binaries` (actual running executable SHA-256), and `managed_processes` (never fake PIDs like `4242` in live mode).
2. **Inter-Host Round-Trip Time (RTT) Calculation & Mesh Propagation**:
   - Periodically calculate and update the round-trip latency (`peer_rtt_ms` and pairwise `mesh_rtt_matrix`) between every pair of hosts in the mesh using monotonic timestamp echoes over encrypted gossip frames (zero wall-clock skew dependency).
3. **5-Second Regular Heartbeats & 30-Second Summary Heartbeats with Thrashing / Variability Statistics**:
   - Publish regular lightweight heartbeats every **5 seconds** (`heartbeat_interval_secs = 5`, `heartbeat_kind = "regular"`).
   - Every **30 seconds** (`summary_interval_secs = 30`, `heartbeat_kind = "summary"`), publish a richer summary heartbeat containing rolling window variability statistics (`min`, `max`, `mean`, `stddev`, `delta_range`, `jitter_mean_abs`) for **CPU %**, **Memory %**, **Network throughput**, and **Peer RTT**, plus `process_restarts_delta`, `thrashing_score`, `thrashing_detected`, and `thrashing_reasons` so operators can detect thrashing nodes directly from summary statistics without high-frequency raw telemetry floods.
4. **H3 Three.js Globe & Public IP / Latency Clustering Node Placement**:
   - Replace the 2D canvas projection in the Web HUD (`render_dashboard_html`) with the **H3 Three.js globe** (`three@0.179.1` + `h3-js@4.4.0` + 11,216 resolution-3 H3 land cells from `deployrobotic`, with occluder ocean, shader limb outline, graticule, H3 land hex surface & outlines, procedural cloud/atmosphere shaders, active node H3 cell + 1-ring highlights, and great-circle QUIC arcs).
   - Compute approximate node coordinates (`geo_location`) from **public IP** (`source = "public_ip_geo"`) when a globally routable IPv4 is present or detected, and otherwise place nodes via **latency clustering** (`source = "latency_cluster"`) using pairwise inter-host RTT (`mesh_rtt_ms` / `peer_rtt_ms`).

---

## 1. Core Security Objective

Keep the public `dialtone` repository (`https://github.com/timcash/dialtone`) **100% generic and free of any host-specific metadata**, while locking the live fleet mesh so **only explicitly provisioned nodes** can discover, authenticate, and communicate with each other.

Specifically:
1. **Zero Hostnames, Keys, or IPs in Git**: No real computer names, Ed25519 `EndpointId`s, IP addresses, or passwords may ever appear in source code, fixtures, tests, documentation, or git-tracked files.
2. **Out-of-Band Host Configuration via Tailnet / SSH**: Real node names, channel passwords, seed peer `EndpointId`s, and allowlisted `EndpointId`s live **exclusively** in each machine's local `~/.dialtone/config.json` (`0600` permissions), outside the git repository, and are provisioned/updated directly over SSH/Tailnet.
3. **Defense-in-Depth Mesh & Local Control Plane Security**: Combine channel password topic derivation, per-heartbeat authentication, Ed25519 identity allowlisting, command-line secret elimination, and localhost HTTP CSRF/DNS-rebinding protection.

---

## 2. Threat Model: Why Public Repo Metadata Matters

When a repository is public, any hardcoded operational defaults become public intelligence:
- **Public `EndpointId`s Leak WAN/LAN IPs via Pkarr/DHT**: In Iroh 1.3 (`presets::N0`), endpoints publish their current UDP socket addresses (public WAN IP + local LAN IP) to Pkarr DNS (`dns.iroh.link`) and Mainline DHT under their 32-byte Ed25519 `EndpointId`. If a node's `EndpointId` is hardcoded in `src/gossip.rs`, anyone reading the repo can query Pkarr DNS to resolve that computer's home/office WAN IP and internal LAN IP, or dial its `GOSSIP_ALPN` endpoint via public relays.
- **Public Hostnames Leak Fleet Topology**: Hardcoding real workstation, build server, or edge device names in fixtures, tests, or README examples reveals the fleet inventory, operating systems, and service roles.
- **Process Command-Line Flags Leak Secrets Locally**: Passing `--password <secret>` or `--secret-key <secret>` in `LaunchAgents` / Windows Scheduled Task command lines exposes the channel password to any local process running `ps aux` or `Get-CimInstance Win32_Process`.
- **Predictable Localhost HTTP Port (`127.0.0.1:8788`) Enables Browser CSRF / DNS Rebinding**: Because the public repo documents `POST /api/artifacts/blob` and `POST /api/processes/blob` on `127.0.0.1:8788`, an untrusted webpage opened in a browser on a fleet machine could attempt cross-origin `fetch()` or DNS-rebinding requests against `127.0.0.1:8788` unless `Host` and `Origin` headers are strictly validated.

---

## 3. Architecture & Design Decisions

### Pillar A — Zero-Leak Public Repository & Automated CI Gate
- **Synthetic Identifiers Only**: All source code (`src/*.rs`), unit tests, CLI fixtures, `ci.sh`, and `README.md` use generic synthetic node names (`node-alpha`, `node-beta`, `node-gamma`, `edge-1`, `edge-2`, `pendant-1`).
- **No Hardcoded Seed Keys in Source**: `default_fleet_seed_peers()` returns an empty list by default unless populated from the host's local `~/.dialtone/config.json` (`seed_peers`).
- **Hardened `.gitignore`**: Explicitly ignore all local state, config, and secret patterns:
  - `/target`, `/bin`, `.dialtone/`, `.dialtone2/`, `*config.json`, `*fleet*.json`, `*.key`, `*.pem`, `*.secret`, `.env`, `.env.*`.
- **Automated Leak Gate in `ci.sh`**: Step 0 of `ci.sh` scans all git-tracked files (`git ls-files`) and fails the build if any forbidden machine name or real `EndpointId` appears anywhere in the repository.

### Pillar B — Out-of-Band Host Provisioning over SSH / Tailnet (`~/.dialtone/config.json`)
- **Host-Local State Directory**: All runtime identity and mesh membership config is stored in `~/.dialtone/` (`C:\Users\<user>\.dialtone\` on Windows), never inside any git checkout directory.
- **Self-Contained `PersistentMeshConfig` (`~/.dialtone/config.json`, mode `0600`)**:
  ```json
  {
    "node_name": "<host-specific-name>",
    "listen_addr": "127.0.0.1:8788",
    "channel": "dialtone",
    "configured": true,
    "derived_keys": {
      "kdf_version": "argon2id-v2",
      "topic_id_hex": "<64-hex-topic-key>",
      "auth_key_hex": "<64-hex-auth-key>",
      "enc_key_hex": "<64-hex-enc-key>"
    },
    "channel_fingerprint": "ch-...",
    "seed_peers": [
      {
        "endpoint_id": "<32-byte-hex-ed25519-id>",
        "relay_url": "https://usw1-1.relay.n0.iroh.link./"
      }
    ],
    "allowed_endpoint_ids": [
      "<peer-1-ed25519-id>",
      "<peer-2-ed25519-id>",
      "<peer-3-ed25519-id>"
    ],
    "used_invite_nonces": [],
    "bootstrap_tickets": [],
    "managed_processes": []
  }
  ```
- **Provisioning Workflow via SSH / Tailnet**:
  - Each host generates and keeps its own `~/.dialtone/iroh-secret-key` (`0600` permissions) locally.
  - An operator (or provisioning script running over Tailscale SSH) queries `dialtone status` over SSH on each authorized machine to collect its `endpoint_id`, then runs `dialtone join "<CHANNEL_PW>" --once` and `dialtone peers allow <endpoint_id>` on each host over SSH (or redeems a single-use `dialtone-invite-v1:` ticket).
  - Plaintext channel passwords are never written to disk; only the Argon2id `derived_keys` bundle is stored in `~/.dialtone/config.json` (`0600`), and real `EndpointId`s, hostnames, and channel passwords never touch the git working tree.

### Pillar C — Multi-Layer Gossip Channel & Peer Authentication
1. **Argon2id `v2` Key Derivation + Isolated Standby**:
   - Without valid channel keys in `~/.dialtone/config.json`, a node binds a node-local standby topic (`derive_standby_topic_id(&endpoint_id)`) and dials zero peers.
   - With a valid password, `derive_channel_keys_v2(channel, password)` derives three domain-separated 256-bit keys (`topic_id`, `auth_key`, `enc_key`) using **Argon2id** (`m=19456 KiB, t=2, p=1`).
2. **XChaCha20-Poly1305 Wire Encryption + Per-Heartbeat MAC + Ed25519 Allowlist**:
   - Every outbound `DialtoneHeartbeat` is signed with `channel_mac` (`auth_key`) and encrypted with **XChaCha20-Poly1305** (`dialtone.iroh.envelope.v2`, random 24-byte nonce).
   - On the `"dialtone"` channel, inbound peers are **denied by default** unless their `endpoint_id` is listed in `allowed_endpoint_ids` in `~/.dialtone/config.json` (manageable via `dialtone peers [allow|revoke|list]` or single-use `dialtone invite` tickets).

### Pillar D — Local Daemon & HTTP Control Plane Hardening
1. **Eliminate Command-Line Secrets (`ps aux` / `wmic` hygiene)**:
   - `dialtone daemon install` and `render_os_service_unit` store only `derived_keys` in `~/.dialtone/config.json` (`0600`) and register the OS service unit with `dialtone up --node <name> --listen <addr>` (no `--secret-key` or `--password` flag in process arguments).
2. **Localhost HTTP Bearer Token Auth + `Host` & `Origin` Enforcement (`127.0.0.1:8788`)**:
   - All state-changing `POST` routes require `Authorization: Bearer <token>` matching `~/.dialtone/api-token` (`0600`). Non-loopback `--listen` bindings are refused unless an explicit API token is configured.
   - Reject HTTP requests whose `Host` header is not `127.0.0.1`, `localhost`, or `[::1]` (prevents DNS rebinding).
   - Reject state-mutating `POST` requests that carry a cross-origin browser `Origin` header (prevents drive-by browser CSRF).

---

## 4. Implementation & Verification Checklist

- [x] **Step 1**: Password-protect the `"dialtone"` channel with Argon2id `v2` key derivation, XChaCha20-Poly1305 payload encryption (`dialtone.iroh.envelope.v2`), and constant-time `channel_mac` verification.
- [x] **Step 2**: Extend `PersistentMeshConfig` in [`src/gossip.rs`](src/gossip.rs) with `derived_keys`, `seed_peers`, `allowed_endpoint_ids`, and single-use `used_invite_nonces` loaded from `~/.dialtone/config.json`, removing all hardcoded `EndpointId`s from source code.
- [x] **Step 3**: Replace all occurrences of real computer names across `src/`, `README.md`, and `ci.sh` with generic synthetic names (`node-alpha`, `node-beta`, `node-gamma`, `edge-1`, `edge-2`, `pendant-1`).
- [x] **Step 4**: Remove `--secret-key` / `--password` from generated OS service command-line arguments in [`src/deploy.rs`](src/deploy.rs) so channel keys are read exclusively from `~/.dialtone/config.json` (`0600`) and never appear in `ps aux`.
- [x] **Step 5**: Add Bearer token authentication (`~/.dialtone/api-token`), bounded worker concurrency, and `Host`/`Origin` header validation to [`src/http.rs`](src/http.rs) to block unauthenticated access, browser CSRF, and DNS rebinding against `127.0.0.1:8788`.
- [x] **Step 6**: Harden `.gitignore` and add an automated git-tracked file and commit history leak scanner to [`ci.sh`](ci.sh).
- [x] **Step 7**: Provision `~/.dialtone/config.json` (`seed_peers` + `allowed_endpoint_ids` + `derived_keys`) onto the production fleet hosts out-of-band over SSH/Tailnet and verify multi-node gossip sync with zero secrets or hostnames in git.
