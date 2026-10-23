# Dialtone: Goals, Domain Language & Architecture (`goal.md`)

## Domain Language

Every concept has **one word**. Code, CLI, API, HUD, docs and commit messages use that word and nothing else.
`ci.sh` fails the build if a banned synonym appears in `src/` (lines that must keep an old name for wire / CLI
compatibility, or that name an Iroh library type, carry a `// compat` marker).

| Word | Means | Code form | Never say |
|------|-------|-----------|-----------|
| **Node** | One running dialtone instance | `Node`, `node` | host, hub, site, fleet member |
| **NodeId** | A node's public Ed25519 key (iroh's `EndpointId`) | `NodeId`, `node_id` | endpoint id, peer id |
| **Key** | A node's private Ed25519 key, stored at `~/.dialtone/iroh-secret-key` | `Key`, `key_path` | secret key, identity key |
| **Peer** | Any node other than this one | `peer`, `peers` | remote host, neighbour (except iroh-gossip `NeighborUp/Down`) |
| **Mesh** | All nodes in one channel | `mesh` | fleet, swarm, cluster |
| **Channel** | A named gossip room unlocked by a secret | `Channel`, `channel` | room; *topic* only inside transport code |
| **Secret** | A channel's password (`dt-…`) | `secret` | password, passphrase |
| **Allowlist** | NodeIds permitted in the mesh | `allowlist` | allowed endpoint ids, ACL |
| **Invite** | Signed, expiring, single-use join token | `Invite`, `invite` | bootstrap ticket, gossip ticket |
| **Seed** | A peer address used to dial into the mesh | `Seed`, `seeds` | bootstrap peer |
| **Beat** | The signed, encrypted status message a node gossips; `kind` is `beat` (5s), `summary` (30s) or `echo` (RTT reply) | `Beat`, `beat` | heartbeat, hb, ping, pulse |
| **Stats** | Measured machine numbers: cpu, mem, disk, net | `Stats`, `stats` | telemetry, host resources, metrics |
| **Rtt** | Round-trip time between two nodes, in ms | `rtt_ms` | latency |
| **Blob** | Immutable bytes named by a digest | `Blob`, `blob` | artifact, payload, sha blob |
| **Digest** | A blob's name: `sha256:<hex>` | `digest` | sha, checksum; *hash* only for iroh's BLAKE3 `Hash` |
| **Store** | A node's local blob storage (`~/.dialtone/blobs`) | `Store`, `store_dir` | cache, CAS, blob cache |
| **Ticket** | An iroh-blobs link to fetch a blob from a node | `ticket` | blob uri |
| **Program** | A blob that can be executed | `Program`, `program` | binary, executable |
| **Process** | A running program owned by the supervisor | `Process`, `ProcessSpec` | job, worker, subprocess, child, managed process |
| **Supervisor** | Keeps every process at its desired state | `Supervisor` | — |
| **Service** | A network endpoint a process exposes to the mesh | `Service` | service advertisement |
| **Deploy** | An order to run a program digest on nodes | `Deploy`, `deploy` | rollout, manifest apply |
| **Slot** | The A or B install location of a program; *rollback* flips the slot | `Slot` | — |
| **Probe** | One run of the five checks: `BLB` `KVS` `PRC` `MSH` `A/B` | `Probe`, `probe` | e2e tick, test tick, capability tick |
| **Record** | A stored event, alert, log line or KV entry | `Event`, `Alert`, `Log`, `Kv` | evidence |
| **Daemon** | The OS login service (launchd / systemd / Scheduled Task) that starts the node | `daemon` | — |
| **HUD** | The node's local web UI | `hud` | dashboard, index page |
| **API** | The node's local HTTP control surface on `127.0.0.1:8788` | `api` | control plane routes |

### Code layout follows the words

| Module | Owns |
|--------|------|
| `node.rs` | Key, NodeId, transport config (was `identity.rs`) |
| `mesh.rs` | Channel, Secret, Allowlist, Invite, Seed, Beat, Rtt, geo (was `gossip.rs`) |
| `stats.rs` | Stats sampling and summaries (was the first half of `evidence.rs`) |
| `record.rs` | Event, Alert, Log, Kv records (was the second half of `evidence.rs`) |
| `blob.rs` | Blob, Digest, Store, Ticket (was `blobs.rs`) |
| `supervisor.rs` | Program, Process, ProcessSpec, Supervisor |
| `deploy.rs` | Deploy, Slot, Daemon install |
| `service.rs` | Service and service references (was `metadata.rs`) |
| `api.rs` / `hud.rs` | API routes and node state / HUD page (was `http.rs`) |

### Commands and routes use the words

| Old name (still accepted for one release) | Name |
|-------|------------|
| `dialtone gen-password` | `dialtone secret` |
| `dialtone peers list / allow / revoke` | `dialtone allowlist list / add / remove` |
| `dialtone blobs share / pull` | `dialtone blob share / pull` |
| `dialtone test-tick` | `dialtone probe` |
| `dialtone fleet-test` | `dialtone mesh-test` |
| `dialtone index` | `dialtone hud` |
| `--password`, `--bootstrap-ticket` | `--secret`, `--invite` |
| `GET /api/gossip` | `GET /api/mesh` |
| Beat fields `host_resources`, `managed_processes`, `e2e_test`, `blob_store`, `endpoint_id` | `stats`, `processes`, `probe`, `store`, `node_id` (old names accepted on read) |

## Current Work

1. **Now — Phase 3 cleanup** (see roadmap): cleanup tiers B (fixtures, seeded state, preview planners, uncalled
   routes) and C (hand-rolled hex / base64 / SHA-1), and build or remove the HUD's `/api/channel/*` actions.
2. **Done — 2026-10-23: Phase 2 reliability.** API never holds the node-state lock while writing to a socket;
   SIGTERM / SIGINT / SIGHUP (ctrl-c on Windows) stop every process tree and close the endpoint; processes run as
   their own process group (Unix) or kill-on-close Job Object (Windows), so stopping one stops what it started;
   trees left by a killed node are reaped at the next boot (`run/children.json`); a failed spawn is reported in
   the process status; gossip re-subscribes with 1s→30s backoff and logs `Lagged`; no `?` can end the run loop;
   a corrupt `config.json` is fatal at load (exit 78) and never overwritten, and a running node keeps its last good
   allowlist; unknown stats are `null` (no 16 GiB / 8-core / 1 Gbit / `tailnet=off` guesses, and a peer without
   stats no longer shows this machine's numbers); nodes are keyed by NodeId and silent peers are pruned every 5s.
3. **Done — 2026-10-23:** Phase 1 security, domain language, cleanup tier A. **2026-10-22:** authentic stats,
   Rtt, 5s / 30s beats, H3 globe, blob / process FX (see §0).

## Main Features (System Map)

| # | Feature | What it does | Where it lives |
|---|---------|--------------|----------------|
| F1 | **Node & transport** | Persistent Key → NodeId, iroh endpoint presets (N0 / minimal / private mesh), relay / Pkarr DNS / DHT / mDNS policy, tailnet-address filtering, transport diagnostics | `node.rs` |
| F2 | **Mesh membership** | Secret → Argon2id channel keys (topic, auth, enc), XChaCha20-Poly1305 beat envelopes, deny-by-default Allowlist, signed single-use Invites, Seeds, `~/.dialtone/config.json` | `mesh.rs`, run loop in `main.rs` |
| F3 | **Beats, stats & Rtt** | 5s beats / 30s summaries, real cpu / mem / disk / net stats on macOS, Linux and Windows, spread + thrashing detection, echo Rtt → mesh Rtt matrix, public-IP / Rtt-cluster geo placement | `stats.rs`, `mesh.rs` |
| F4 | **Blob store & tickets** | Store under `~/.dialtone/blobs` named by Digest, `blob share` / `blob pull` tickets over QUIC with digest re-check, per-node `store` stats in beats | `blob.rs` |
| F5 | **Supervisor & probe** | Stage programs from blobs, start / restart / stop processes, default `probe` program, the five-check Probe (`BLB` `KVS` `PRC` `MSH` `A/B`) | `supervisor.rs`, node state in `api.rs` |
| F6 | **Deploy, slots & daemon install** | Deploy contract, A/B slot rollback, launchd / systemd `--user` / Scheduled Task daemon install | `deploy.rs` |
| F7 | **Node runtime & API** | 5-stage boot, shared node state, API + WebSocket on `127.0.0.1:8788`, bearer-token POST auth, Host / Origin checks, live channel join | `main.rs` (`run_node`), `api.rs` |
| F8 | **HUD** | Embedded Three.js + H3 globe, `top`-style overlay, 6-step tour with captions, blob / process FX | `hud.rs`, `h3_land.rs` |
| F9 | **Verification** | `self-test`, `mesh-test`, `probe`, unit + live iroh tests, `ci.sh` (zero-leak gate, domain-language gate, fmt, tests, live acceptance), GitHub CI matrix | `lib.rs`, `ci.sh`, `.github/workflows/` |
| F10 | **Services & records** | Service refs, Services, key leases, policies, Event / Alert / Log / Kv records with cursor pagination, proof validation | `service.rs`, `record.rs` |
| F11 | **CLI & fixtures** | Argument parsing, `help`, and the `*-fixture` preview commands | `main.rs` |

## Code Review (2026-10-23): Findings, Cleanup & Target Iroh Architecture

### Critical findings (all fixed 2026-10-23: 1-5 in Phase 1, 6 in Phase 2)

1. **The allowlist trusts a self-reported identity.** Received beats are checked against `rx_beat.node_id`, a field inside the payload (`main.rs` ~1683). iroh-gossip's `delivered_from` is the relaying neighbour, not the author, so any holder of the channel secret can claim an allowlisted node's id. The `Router` also accepts the gossip and blobs ALPNs from *any* `NodeId` (`main.rs` ~1005), and custom channels have no allowlist at all. — **Fixed:** an iroh `after_handshake` hook (`node::AllowlistHook`) closes any connection from a NodeId not on the live allowlist, for every protocol and channel; beats are signed with the sender's Key and verified against the `node_id` they claim.
2. **Channel secret ⇒ mesh-wide code execution.** A beat whose `processes[].env` contains `DIALTONE_MESH_DEPLOY=1` + `DIALTONE_BLOB_TICKET=…` makes every receiver pull, stage and run that binary (`main.rs` ~1800-1937). Combined with (1), the secret is the only security boundary. The guard `!already_running || insert(..)` is also inverted, so a non-running deploy is re-pulled on a new OS thread + new endpoint on *every* beat. — **Fixed:** mesh deploy now needs a signed beat from an allowlisted node, an explicit `deploy_ticket` whose provider is allowlisted, never a peer's probe program, and pulls each (peer, digest) once.
3. **Beats broadcast every process `env`** (incl. blob tickets and any secret a process is given) to all peers. — **Fixed:** `seal_beat` sends `mesh::process_advert` (no `env`, no `args`).
4. **HUD token theft via XSS.** The bearer token is templated into the unauthenticated `GET /` page, state JSON is injected into `<script>` without `<` escaping, and peer-supplied node names go into `innerHTML`. A channel member can name a node with markup and steal the token. — **Fixed:** the page carries no token; a per-response CSP nonce lets only its own scripts run; embedded JSON is `<`-escaped; peer strings are scrubbed of markup characters; peer node names must be plain names.
5. **No replay protection.** `counter` is never checked and `updated_at_unix` is overwritten with local time, so captured frames replay as fresh. — **Fixed:** every beat carries a strictly increasing `sent_unix_ms`; `mesh::ReplayGuard` drops anything not newer than the last beat from that node or more than 5 min from local time.
6. **The mesh stalls behind HTTP.** One `std::sync::Mutex<NodeState>` is held while rendering/writing HTTP responses (2s write timeout), spawning processes, and pulling blobs (up to 15s); the gossip loop blocks on the same lock. — **Fixed:** API routes write their response into a buffer under the lock and the socket write happens after it is released; blob shares / pulls in `/api/processes/blob` run with the lock released.

### Per-feature findings

- **F1 Transport.** Sound core (persistent key, 0600). Issues: seed addresses are re-injected into `MemoryLookup` every 5s unfiltered (tailnet leak); bind silently falls back to all interfaces; `RelayOnly` only filters what is advertised; public n0 relay URL hardcoded in 5 places; `build_diagnostics` does a UDP probe per beat.
- **F2 Gossip mesh.** MAC (keyed SHA-256, not HMAC) is redundant with the AEAD and uses the same secret; beats are signed twice; KDF cache keeps plaintext secrets for the process lifetime; `Debug` derives print keys; corrupt `config.json` silently becomes `Default` and the next save wipes keys + allowlist; allowlist snapshotted at boot (`allowlist add/remove` needs a restart); re-subscribe has no backoff and can spin; JoinMesh swaps keys before `subscribe` succeeds; invite "single-use" is only enforced on the redeemer. Dead: `derive_channel_auth_key`, `derive_channel_enc_key`, `is_allowed_main_mesh_peer`, `parse_secret_or_ticket`, `compute_beat_mac`, `sign_beat_with_secret`, `verify_beat_secret`, `IrohTransportConfig::lan_only`; legacy `.dialtone2` migration + `dialtone-gossip-v1:` parsing.
- **F3 Stats & RTT.** macOS sampler forks `vm_stat` / `df` / `netstat` every second on the async runtime under a global mutex; remaining fake values in live mode (`link_speed_mbps = 1000`, `tailnet=off`, 16 GiB / 8-core fallbacks, failures reported as 0%); `Stats::default()` samples *this* host, so a peer beat missing the field shows local numbers; NIC rate can be computed across two different interfaces; echo RTT is a full-mesh broadcast (N×(N−1) per 15s) that measures gossip-overlay hops, not link RTT; `merge_beat` never expires `peer_rtt_ms` / `probe`; peers keyed by node name, not `NodeId`; beat built in 6 places; unconditional third-party `ipinfo.io` lookup with ~1 km coordinates gossiped to all peers and table guesses labelled `public_ip_geo`.
- **F4 Blobs.** Two stores (iroh-blobs `MemStore` + store) with every blob tagged into RAM forever (no GC); provider state lost on restart so stored tickets go stale; each pull binds a throwaway endpoint (new identity, fresh NAT traversal, temp-file name collides within one second); `BlobsProtocol` has no connection filter. Fixture-only: `evaluate_tiered_blob_fetch` (reports `transferred_bytes` for a plan that transfers nothing), `transfer_blob_via_iroh_ticket`.
- **F5 Supervision.** No signal handling, so children (and grandchildren — no process group / Job Object) orphan on SIGTERM; no restart backoff; spawn errors swallowed (stale `running` / pid); `reconcile` polled from 4 places incl. GET handlers; probe tick re-stages blobs every 5s and `mesh_connectivity_ok` is true by construction. Fixture-only: rtcore UDP loopback, `RobotCommandIntentRecord` dispatch, `CursorJobRecord`, systemd drop-in reconcile.
- **F6 Deploy.** Live: A/B rollback (fed synthetic slot bytes) and `install_persistent_os_daemon`. Preview-only: `generate_deploy_plan` (4 fixed synthetic peers), `evaluate_deploy_build_decision` (fixed timings/hash), `probe_build_cache` (cache list never populated), `validate_manifest_contract` (accepted manifests go nowhere), `render_install_v1_sh`. Service templates are duplicated between `render_os_service_unit` and the installer and have drifted.
- **F7 Runtime & API.** `run_node` is one ~1,300-line future (no `tokio::spawn`, no shutdown phase); serialized awaits in the 5s tick (~0.6s + N×350ms + 0.6s); `?` inside the loop can kill the daemon; one accept error silently ends the HTTP server; peers pruned only inside HTTP requests; GET routes mutate state. Hand-rolled HTTP lacks chunked bodies, truncation checks, percent-decoding; `/ws` is a one-frame stub. ~25 routes are called by nothing (HUD uses only `/` and `/api/gossip`).
- **F8 HUD.** ~3,500 lines of HTML/JS in a Rust string; three.js / h3-js loaded from a CDN without SRI (fails on air-gapped meshes, contradicts "UI compiled into the binary"); **Init / Invite / Join buttons call `/api/channel/*` routes that do not exist**; polls `/api/gossip` every 1.5s.
- **F9 Verification.** ci.sh lines ~54-89 assert on seeded fixture strings; self-test steps 4/6/7 validate lookup tables and synthetic bytes rather than behaviour.
- **F10 Metadata & evidence.** Dead now: `StatsTemplate`, `NodeRoleConfig`, `SessionBlobPolicy`, `CursorPolicy`, `ServiceStatusProbe` / `hydrated_for_node`, `DialtoneMetadata::load_from_file` / `roles_for_node` and the unread metadata fields. Self-test-only: `PeerRef`, `resolve_service`, `KeyLease*`, `ProofValidationReport`. `dialtone index` serves fabricated peers, PID `4242`, NATS service, alerts (goal §0 forbids this).
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
   Transport ─ Mesh ─ Stats ─ Supervisor ─ BlobService ─ ApiServer(hyper/httparse) ─ HUD(static files)
```

- **Identity = authorization.** Enforce the allowlist at the QUIC handshake for every ALPN and channel, fed from an `Arc<RwLock<HashSet<NodeId>>>` that `allowlist add/remove` update live.
- **Signed, typed messages.** Every gossip payload is `SignedMessage { from: NodeId, sig, body }` (iroh-gossip chat example pattern), verified against the node key and the allowlist; AEAD stays for confidentiality; the SHA-256 MAC goes. `body` is an enum: `Beat` (small, every 5s, monotonic counter checked), `Summary` (30s), `Inventory` (processes without `env`, services, blob digests — on change), `Deploy` (see below).
- **Let iroh find peers.** Rely on the endpoint's address lookup (pkarr/DHT or one-time `MemoryLookup` seeding) instead of gossiping all-peer tickets and re-joining every 5s; use `neighbors()` / `joined()` for membership; RTT from iroh connection paths instead of broadcast echoes.
- **One blob store.** `FsStore` (feature `fs-store`) as the only store, SHA-256 kept as a verified tag (`digest/<hex>` → BLAKE3 hash), GC via tags; pulls through the daemon's own endpoint (`store.downloader(&endpoint)`), no temp endpoints or temp files.
- **Deploy is explicit and signed.** Replace the env-flag auto-deploy with a `Deploy` message signed by an operator key (or an allowlisted node with a deploy capability), naming digest + target nodes; receivers verify signature, provider ∈ allowlist and sha before staging.
- **Supervisor owns children.** `tokio::process` with `kill_on_drop`, process groups (Unix) / Job Objects (Windows), exponential restart backoff, spawn errors recorded in status, `shutdown_all` on SIGTERM / ctrl-c.
- **API & HUD.** API task reads `watch` snapshots and never holds the mesh lock; HUD moves to `src/hud/*` via `include_str!` with vendored three.js / h3-js; token fetched by an authenticated seed call, never templated; one live channel (`/ws` push or polling — not both).

### Cleanup plan

| Tier | What | Size |
|---|---|---|
| A — delete now (no callers) | Dead F2 functions listed above, `lan_only`, dead F10 metadata types + fields + their tests | ~400 lines |
| B — delete with ci.sh / README / test edits | All `*-fixture` commands + `build_seeded_state` + `new_seeded`; preview planners (deploy plan, build decision, build cache, proof validation, alert ack, tiered fetch report, install.sh, catalogs, manifests); rtcore UDP / robot intents / cursor jobs / systemd drop-in; ~25 uncalled routes; self-test steps 4/6/7 | ~1,500 lines Rust + ~35 ci.sh + ~120 README |
| C — replace | hex / base64 / SHA-1 → `hex`, `base64`, `sha1_smol` (already in lockfile) | ~150 lines |

### Roadmap

1. **Phase 1 — Security (critical 1-5) — done 2026-10-23:** handshake allowlist hook + live allowlist, Ed25519-signed payloads with counter check, remove env-flag auto-deploy (gate behind signed `Deploy`), stop gossiping `env`, HUD escaping / `textContent` / no templated token.
2. **Phase 2 — Reliability — done 2026-10-23:** release the state lock before IO, ctrl-c shutdown + child process groups, re-subscribe backoff, no `?` in the loop, config parse errors fail loudly, stats unknowns as `None`, peers keyed by `NodeId` with TTL pruning.
3. **Phase 3 — Cleanup:** tiers A → B → C, fix or remove the HUD `/api/channel/*` buttons.
4. **Phase 4 — Architecture:** actor split of `run_node`, FsStore + downloader, typed message enum + inventory-on-change, hyper/httparse server, HUD as static files with vendored JS.

## 0. Current Priority Goal: Authentic Stats, Inter-Host RTT, & 5s / 30s Summary Variability Beats

1. **100% Authentic Stats, Logs & Beats (Zero Fake Numbers)**:
   - Replace all synthetic/hardcoded stats values (such as fake `23.4%` / `120 GiB / 512 GiB` disk usage, fake `50.0%` / `8 GiB / 16 GiB` RAM, and synthetic counter-based CPU/network waves) with real OS-level measurements across macOS, Linux, and Windows:
     - **CPU**: Real logical core count and real CPU utilization percentage computed from OS kernel tick deltas (`host_statistics(HOST_CPU_LOAD_INFO)` on macOS, `GetSystemTimes` on Windows, `/proc/stat` on Linux).
     - **Memory**: Real physical RAM total, used bytes, and percentage (`sysctl hw.memsize` + `vm_stat` on macOS, `GlobalMemoryStatusEx` on Windows, `/proc/meminfo` on Linux).
     - **Disk**: Real filesystem total bytes, used bytes, and percentage for the state volume (`df -Pk` on macOS/Linux, `GetDiskFreeSpaceExW` on Windows).
     - **Network**: Real physical interface name, link status, cumulative RX/TX bytes, and live RX/TX bytes/sec throughput (`netstat -ibn` on macOS, `netstat -e` on Windows, `/proc/net/dev` on Linux).
   - Ensure live daemon state (`dialtone up`) records real runtime `logs`, `events`, `alerts`, `installed_programs` (actual running executable SHA-256), and `processes` (never fake PIDs like `4242` in live mode).
2. **Inter-Host Round-Trip Time (RTT) Calculation & Mesh Propagation**:
   - Periodically calculate and update the round-trip latency (`peer_rtt_ms` and pairwise `mesh_rtt_matrix`) between every pair of hosts in the mesh using monotonic timestamp echoes over encrypted gossip frames (zero wall-clock skew dependency).
3. **5-Second Regular Beats & 30-Second Summary Beats with Thrashing / Variability Statistics**:
   - Publish regular lightweight beats every **5 seconds** (`beat_interval_secs = 5`, `kind = "regular"`).
   - Every **30 seconds** (`summary_interval_secs = 30`, `kind = "summary"`), publish a richer summary beat containing rolling window variability statistics (`min`, `max`, `mean`, `stddev`, `delta_range`, `jitter_mean_abs`) for **CPU %**, **Memory %**, **Network throughput**, and **Peer RTT**, plus `process_restarts_delta`, `thrashing_score`, `thrashing_detected`, and `thrashing_reasons` so operators can detect thrashing nodes directly from summary statistics without high-frequency raw stats floods.
4. **H3 Three.js Globe & Public IP / Latency Clustering Node Placement**:
   - Replace the 2D canvas projection in the Web HUD (`render_hud`) with the **H3 Three.js globe** (`three@0.179.1` + `h3-js@4.4.0` + 11,216 resolution-3 H3 land cells from `deployrobotic`, with occluder ocean, shader limb outline, graticule, H3 land hex surface & outlines, procedural cloud/atmosphere shaders, active node H3 cell + 1-ring highlights, and great-circle QUIC arcs).
   - Compute approximate node coordinates (`geo_location`) from **public IP** (`source = "public_ip_geo"`) when a globally routable IPv4 is present or detected, and otherwise place nodes via **latency clustering** (`source = "latency_cluster"`) using pairwise inter-host RTT (`mesh_rtt_ms` / `peer_rtt_ms`).

---

## 1. Core Security Objective

Keep the public `dialtone` repository (`https://github.com/timcash/dialtone`) **100% generic and free of any host-specific metadata**, while locking the live mesh mesh so **only explicitly provisioned nodes** can discover, authenticate, and communicate with each other.

Specifically:
1. **Zero Hostnames, Keys, or IPs in Git**: No real computer names, Ed25519 `NodeId`s, IP addresses, or secrets may ever appear in source code, fixtures, tests, documentation, or git-tracked files.
2. **Out-of-Band Host Configuration via Tailnet / SSH**: Real node names, channel secrets, seed `NodeId`s, and allowlisted `NodeId`s live **exclusively** in each machine's local `~/.dialtone/config.json` (`0600` permissions), outside the git repository, and are provisioned/updated directly over SSH/Tailnet.
3. **Defense-in-Depth Mesh & Local Control Plane Security**: Combine channel secret topic derivation, per-beat authentication, Ed25519 identity allowlisting, command-line secret elimination, and localhost HTTP CSRF/DNS-rebinding protection.

---

## 2. Threat Model: Why Public Repo Metadata Matters

When a repository is public, any hardcoded operational defaults become public intelligence:
- **Public `NodeId`s Leak WAN/LAN IPs via Pkarr/DHT**: In Iroh 1.3 (`presets::N0`), endpoints publish their current UDP socket addresses (public WAN IP + local LAN IP) to Pkarr DNS (`dns.iroh.link`) and Mainline DHT under their 32-byte Ed25519 `NodeId`. If a node's `NodeId` is hardcoded in `src/gossip.rs`, anyone reading the repo can query Pkarr DNS to resolve that computer's home/office WAN IP and internal LAN IP, or dial its `GOSSIP_ALPN` endpoint via public relays.
- **Public Hostnames Leak Mesh Topology**: Hardcoding real workstation, build server, or edge device names in fixtures, tests, or README examples reveals the mesh inventory, operating systems, and service roles.
- **Process Command-Line Flags Leak Secrets Locally**: Passing `--secret <secret>` (old name `--password`) or `--secret-key <secret>` in `LaunchAgents` / Windows Scheduled Task command lines exposes the channel secret to any local process running `ps aux` or `Get-CimInstance Win32_Process`.
- **Predictable Localhost HTTP Port (`127.0.0.1:8788`) Enables Browser CSRF / DNS Rebinding**: Because the public repo documents `POST /api/blob` and `POST /api/processes/blob` on `127.0.0.1:8788`, an untrusted webpage opened in a browser on a mesh machine could attempt cross-origin `fetch()` or DNS-rebinding requests against `127.0.0.1:8788` unless `Host` and `Origin` headers are strictly validated.

---

## 3. Architecture & Design Decisions

### Pillar A — Zero-Leak Public Repository & Automated CI Gate
- **Synthetic Identifiers Only**: All source code (`src/*.rs`), unit tests, CLI fixtures, `ci.sh`, and `README.md` use generic synthetic node names (`node-alpha`, `node-beta`, `node-gamma`, `edge-1`, `edge-2`, `pendant-1`).
- **No Hardcoded Seed Keys in Source**: `default_seeds()` returns an empty list by default unless populated from the host's local `~/.dialtone/config.json` (`seeds`).
- **Hardened `.gitignore`**: Explicitly ignore all local state, config, and secret patterns:
  - `/target`, `/bin`, `.dialtone/`, `.dialtone2/`, `*config.json`, `*mesh*.json`, `*.key`, `*.pem`, `*.secret`, `.env`, `.env.*`.
- **Automated Leak Gate in `ci.sh`**: Step 0 of `ci.sh` scans all git-tracked files (`git ls-files`) and fails the build if any forbidden machine name or real `NodeId` appears anywhere in the repository.

### Pillar B — Out-of-Band Host Provisioning over SSH / Tailnet (`~/.dialtone/config.json`)
- **Host-Local State Directory**: All runtime identity and mesh membership config is stored in `~/.dialtone/` (`C:\Users\<user>\.dialtone\` on Windows), never inside any git checkout directory.
- **Self-Contained `MeshConfig` (`~/.dialtone/config.json`, mode `0600`)**:
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
    "seeds": [
      {
        "node_id": "<32-byte-hex-ed25519-id>",
        "relay_url": "https://usw1-1.relay.n0.iroh.link./"
      }
    ],
    "allowlist": [
      "<peer-1-ed25519-id>",
      "<peer-2-ed25519-id>",
      "<peer-3-ed25519-id>"
    ],
    "used_invite_nonces": [],
    "invites": [],
    "processes": []
  }
  ```
- **Provisioning Workflow via SSH / Tailnet**:
  - Each host generates and keeps its own `~/.dialtone/iroh-secret-key` (`0600` permissions) locally.
  - An operator (or provisioning script running over Tailscale SSH) queries `dialtone status` over SSH on each authorized machine to collect its `node_id`, then runs `dialtone join "<CHANNEL_PW>" --once` and `dialtone allowlist add <node_id>` on each host over SSH (or redeems a single-use `dialtone-invite-v1:` ticket).
  - Plaintext channel secrets are never written to disk; only the Argon2id `derived_keys` bundle is stored in `~/.dialtone/config.json` (`0600`), and real `NodeId`s, hostnames, and channel secrets never touch the git working tree.

### Pillar C — Multi-Layer Gossip Channel & Peer Authentication
1. **Argon2id `v2` Key Derivation + Isolated Standby**:
   - Without valid channel keys in `~/.dialtone/config.json`, a node binds a node-local standby topic (`derive_standby_topic_id(&node_id)`) and dials zero peers.
   - With a valid secret, `derive_channel_keys_v2(channel, secret)` derives three domain-separated 256-bit keys (`topic_id`, `auth_key`, `enc_key`) using **Argon2id** (`m=19456 KiB, t=2, p=1`).
2. **XChaCha20-Poly1305 Wire Encryption + Per-Beat MAC + Ed25519 Allowlist**:
   - Every outbound `Beat` is signed with `channel_mac` (`auth_key`) and encrypted with **XChaCha20-Poly1305** (`dialtone.iroh.envelope.v2`, random 24-byte nonce).
   - On the `"dialtone"` channel, inbound peers are **denied by default** unless their `node_id` is listed in `allowlist` in `~/.dialtone/config.json` (manageable via `dialtone peers [allow|revoke|list]` or single-use `dialtone invite` tickets).

### Pillar D — Local Daemon & HTTP Control Plane Hardening
1. **Eliminate Command-Line Secrets (`ps aux` / `wmic` hygiene)**:
   - `dialtone daemon install` and `render_os_service_unit` store only `derived_keys` in `~/.dialtone/config.json` (`0600`) and register the OS service unit with `dialtone up --node <name> --listen <addr>` (no `--secret-key`, `--secret` or old `--password` flag in process arguments).
2. **Localhost HTTP Bearer Token Auth + `Host` & `Origin` Enforcement (`127.0.0.1:8788`)**:
   - All state-changing `POST` routes require `Authorization: Bearer <token>` matching `~/.dialtone/api-token` (`0600`). Non-loopback `--listen` bindings are refused unless an explicit API token is configured.
   - Reject HTTP requests whose `Host` header is not `127.0.0.1`, `localhost`, or `[::1]` (prevents DNS rebinding).
   - Reject state-mutating `POST` requests that carry a cross-origin browser `Origin` header (prevents drive-by browser CSRF).

---

## 4. Implementation & Verification Checklist

- [x] **Step 1**: Secret-protect the `"dialtone"` channel with Argon2id `v2` key derivation, XChaCha20-Poly1305 payload encryption (`dialtone.iroh.envelope.v2`), and constant-time `channel_mac` verification.
- [x] **Step 2**: Extend `MeshConfig` in [`src/gossip.rs`](src/gossip.rs) with `derived_keys`, `seeds`, `allowlist`, and single-use `used_invite_nonces` loaded from `~/.dialtone/config.json`, removing all hardcoded `NodeId`s from source code.
- [x] **Step 3**: Replace all occurrences of real computer names across `src/`, `README.md`, and `ci.sh` with generic synthetic names (`node-alpha`, `node-beta`, `node-gamma`, `edge-1`, `edge-2`, `pendant-1`).
- [x] **Step 4**: Remove `--secret-key` / `--secret` (old name `--password`) from generated OS service command-line arguments in [`src/deploy.rs`](src/deploy.rs) so channel keys are read exclusively from `~/.dialtone/config.json` (`0600`) and never appear in `ps aux`.
- [x] **Step 5**: Add Bearer token authentication (`~/.dialtone/api-token`), bounded process concurrency, and `Host`/`Origin` header validation to [`src/http.rs`](src/http.rs) to block unauthenticated access, browser CSRF, and DNS rebinding against `127.0.0.1:8788`.
- [x] **Step 6**: Harden `.gitignore` and add an automated git-tracked file and commit history leak scanner to [`ci.sh`](ci.sh).
- [x] **Step 7**: Provision `~/.dialtone/config.json` (`seeds` + `allowlist` + `derived_keys`) onto the production mesh hosts out-of-band over SSH/Tailnet and verify multi-node gossip sync with zero secrets or hostnames in git.
