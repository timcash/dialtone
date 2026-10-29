# Dialtone: Goals, Domain Language & Architecture (`goal.md`)

## Domain Language

Every concept has **one word**. Code, CLI, API, HUD, docs and commit messages use that word and nothing else.
`./dialtone.sh test --only lint` (part of every `test`) fails if a banned synonym appears in `src/` (lines that must keep an old name for wire / CLI
compatibility, or that name an Iroh library type, carry a `// compat` marker).

| Word | Means | Code form | Never say |
|------|-------|-----------|-----------|
| **Node** | One running dialtone instance | `Node`, `node` | host, hub, site, fleet member |
| **NodeId** | A node's public Ed25519 key (iroh's `EndpointId`) | `NodeId`, `node_id` | endpoint id, peer id |
| **Key** | A node's private Ed25519 key, stored at `~/.dialtone/iroh-secret-key` | `Key`, `key_path` | secret key, identity key |
| **Peer** | Any node other than this one | `peer`, `peers` | remote host |
| **Neighbor** | A peer this node is linked to directly in gossip (iroh-gossip's active view, 2 to 5 per node); messages reach other peers through neighbors | `neighbors`, `NBR` | neighbour, direct peer |
| **Mesh** | All nodes in one channel | `mesh` | fleet, swarm, cluster |
| **Channel** | A named gossip room unlocked by a secret | `Channel`, `channel` | room; *topic* only inside transport code |
| **Secret** | A channel's password (`dt-…`) | `secret` | password, passphrase |
| **Allowlist** | NodeIds permitted in the mesh; empty means an open channel (any holder of the channel keys) | `allowlist` | allowed endpoint ids, ACL |
| **Invite** | Expiring link to join a channel: presented to the member it names, which answers with the channel keys (`OpenInvite`, v2); v1 was single-use with the secret | `OpenInvite`, `Invite`, `invite` | bootstrap ticket, gossip ticket, join link |
| **Seed** | A peer address used to dial into the mesh | `Seed`, `seeds` | bootstrap peer |
| **Message** | What nodes gossip on a channel, signed and encrypted: a Beat or an Inventory | `Message` | frame, packet |
| **Beat** | A node's status (load, probe checks, gossip neighbors with their Rtt), a compact binary record of about 255 bytes sealed, gossiped every 5 to 120s (the interval grows with the mesh) | `Beat`, `beat` | heartbeat, hb, ping, pulse |
| **Inventory** | A node's processes, services and details, broadcast on the events topic when they change | `Inventory` | manifest, catalog |
| **Fetch** | Asking one node for one thing (its details, a fresh probe blob) over `dialtone/fetch/1`, only when someone asks | `fetch` | rpc, query |
| **Score** | How a node rates a peer from what it delivered (GossipSub's peer score); below −20 the peer is *graylisted* | `PeerScore`, `scores` | reputation, trust |
| **Device** | Hardware connected to a node: camera, microphone, speaker, keyboard, pointer, display, printer, storage, bluetooth, gamepad | `Device`, `devices` | peripheral, gadget |
| **Stats** | Measured machine numbers: cpu, mem, disk, net | `Stats`, `stats` | telemetry, host resources, metrics |
| **Rtt** | Round-trip time between two nodes, in ms | `rtt_ms` | latency |
| **Blob** | Immutable bytes named by a digest | `Blob`, `blob` | artifact, payload, sha blob |
| **Digest** | A blob's name: `sha256:<hex>` | `digest` | sha, checksum; *hash* only for iroh's BLAKE3 `Hash` |
| **Store** | A node's local blob storage (`~/.dialtone/blobs`, an iroh-blobs `FsStore`; each blob tagged `sha256/<hex>`) | `Blobs`, `Store`, `store_dir` | cache, CAS, blob cache |
| **Ticket** | An iroh-blobs link to fetch a blob from a node | `ticket` | blob uri |
| **Program** | A blob that can be executed | `Program`, `program` | binary, executable |
| **Process** | A running program owned by the supervisor | `Process`, `ProcessSpec` | job, worker, subprocess, child, managed process |
| **Supervisor** | Keeps every process at its desired state | `Supervisor` | — |
| **Service** | A network endpoint a process exposes to the mesh | `Service` | service advertisement |
| **Deploy** | An order to run a program digest on nodes | `Deploy`, `deploy` | rollout, manifest apply |
| **Probe** | The node testing itself: every 30s the gossip checks `MSH` peers in the mesh, `NBR` enough neighbors, `BEAT` beats both ways; a full probe (on request) adds `STO` store round trip, `PUL` blob pulls from sampled peers, `PRC` probe program running | `Probe`, `probe` | e2e tick, test tick, capability tick |
| **Record** | A stored event, alert, log line or KV entry | `Event`, `Alert`, `Log`, `Kv` | evidence |
| **Daemon** | The OS login service (launchd / systemd / Scheduled Task) that starts the node | `daemon` | — |
| **HUD** | The node's local web UI | `hud` | dashboard, index page |
| **API** | The node's local HTTP control surface on `127.0.0.1:8788` | `api` | control plane routes |
| **Actor** | One task of the running node: Mesh, Stats, Supervisor, Devices, Paths, Allowlist, Probe, Deploy | `actors.rs` | worker, loop |

### Code layout follows the words

| Module | Owns |
|--------|------|
| `node.rs` | Key, NodeId, transport config (was `identity.rs`) |
| `mesh.rs` | Channel, Secret, Allowlist, Invite, Seed, Beat, Rtt, geo (was `gossip.rs`) |
| `wire.rs` | The compact form a Beat takes on the wire (postcard) |
| `stats.rs` | Stats sampling and summaries (was the first half of `evidence.rs`) |
| `record.rs` | Event, Alert, Log, Kv records (was the second half of `evidence.rs`) |
| `blob.rs` | Blob, Digest, Store, Ticket (was `blobs.rs`) |
| `supervisor.rs` | Program, Process, ProcessSpec, Supervisor |
| `deploy.rs` | Deploy, Slot, Daemon install |
| `service.rs` | Service and service references (was `metadata.rs`) |
| `api.rs` / `hud.rs` | API routes and node state / HUD page (was `http.rs`) |
| `actors.rs` | The running node's actors and the channels between them |
| `devices.rs` | Device listing per OS (`system_profiler`, `Get-PnpDevice`, `/sys`) |
| `fetch.rs` | Fetch: the `dialtone/fetch/1` request–answer protocol |
| `link.rs` | Links: direct QUIC connections for live media (`dialtone/link/1`) |
| `join.rs` | Join: open invites and the `dialtone/join/1` protocol that answers them with the channel keys |
| `probe.rs` | Probe checks and full probes |
| `hud/` | The HUD's static files and vendored three.js / h3-js |

### Commands and routes use the words

The old names (`gen-password`, `peers`, `blobs`, `test-tick`, `fleet-test`, `index`, `--password`,
`--bootstrap-ticket`, `/api/gossip`, `/api/artifacts/blob`, `/api/blobs/*`, `/api/e2e/*`, and the pre-rename beat
and config field names) were accepted for a while after the rename and were removed on 2026-10-25.

## Current Work

1. **Next — candidates:** rerun the scale test (`test --only scale`) to see whether the
   longer `message_id_retention` removed the extra messages; cut the remaining duplicate gossip frames (IDONTWANT as
   in GossipSub v1.2); the API reading `watch` snapshots instead of locking
   node state.
2. **Done — 2026-10-29: Every test on the VMs.** `./dialtone.sh test` runs every node and test end to end on
   the three Linux test VMs (only the lint checks run on the machine that drives it): the build VM runs the whole
   `cargo test` suite and the release build, every VM installs that build and passes its probes (gossip probe on
   the exact binary, then a full probe), then scale nodes on every VM as one mesh. All testing goes through that one
   command: `--here` runs on one machine (GitHub CI), `--only lint|suite|scale|probes` picks a part, `--scale`,
   `--secs` and `-- ARGS` tune it; the old `check`, `lint`, `scale`, `scale-fleet` and `fleet` tasks are gone and
   `deploy` no longer tests. `./dialtone.sh dev` sets any node up for development (C toolchain, pkg-config, git,
   curl, Rust with rustfmt and clippy). The desktop machines left the test fleet.
   **Rejoin test** (`--only rejoin`, also in every full `test`): a node on the previous signed release joins the
   live channel by invite, goes away, rejoins with its updater on and must update itself to the newest release
   after hearing of it in a peer's beat (not from GitHub); first run: v0.5.0 to v0.5.1 kept 72s after rejoining.
   A departed node fails peers' `BEAT` probes until pruned (issue #15), so the VMs' nodes restart after it.
2. **Done — 2026-10-28: Self-update.** Beats carry the version a node runs; a node checks GitHub hourly and
   follows peers whose beats show a newer release, verifies the CI-signed `dialtone-release.json` (release key in
   `src/update.rs`), fetches the binary from same-platform peers (iroh-blobs) or GitHub, installs it A/B
   (`dialtone.prev`) and restarts through its service; a trial binary that is not healthy in 3 minutes or fails to
   start 3 times is rolled back and marked bad. `tests/update.rs`.
2. **Done — 2026-10-27: Direct links for video (phase 3).** `dialtone link export <name> --udp|--tcp <addr>` and
   `dialtone link import <node>/<name> --udp|--tcp <addr>` carry a local source to one peer over a direct QUIC
   connection (`dialtone/link/1`): UDP as QUIC datagrams (split and rejoined when too large; late packets dropped,
   never queued), TCP as one stream per connection. `GET /api/links`, `dialtone link list` and the HUD show each
   link's path (DIRECT, or RELAY when hole punching fails) and QUIC's Rtt. Checked by e2e (UDP with a split packet,
   TCP) on macOS and Windows. Live, between two machines on a direct path: a 720p30 H.264 stream
   at 4 Mbit/s crossed with every one of 11,539 packets delivered and none dropped, while 64-byte probes echoed
   through the same link took 13 ms p50 / 29 ms p99 round trip. Capturing a USB camera with ffmpeg on the sending
   machine ran at only 0.17x real time (5 fps): the capture and scaling, not the link, limit live camera video there.
3. **Done — 2026-10-27: Deploys announced over gossip (phase 2).** A signed `Deploy` message on the events topic
   (program advert, blob ticket, target NodeIds) replaced inventory-driven auto-deploy. Only targeted nodes pull,
   first from peers that already run the program, then the origin, after a random delay of up to one beat. Copies
   are removed when the origin removes the program, retargets it, or stops announcing it for 30 minutes; the origin
   re-announces every 10 minutes and soon after a node joins, so missed deploys are repaired through gossip. The
   probe program is no longer offered. Checked by e2e (deploy to all, deploy to one) and on the fleet.
4. **Done — 2026-10-27: Gossip only, minimal by default.** Once joined, a node talks to the mesh only through
   gossip: no inventory fetches, no catch-ups, no Rtt requests. Beats are a compact postcard record (envelope v6),
   about 255 bytes sealed instead of 1.2 KB, with up to five gossip neighbors and QUIC's own Rtt to each (read from
   the open gossip connection by the Paths actor, which replaced Links); summaries are no longer gossiped. One-sided
   links are repaired from the neighbor lists in beats. The beat interval comes from a 4 KB/s budget
   (`clamp(nodes × 500 B / 4 KB/s, 5, 120)` s), iroh-gossip remembers message ids for 6 minutes, and silent peers
   stop being join candidates. What beats leave out is fetched for the HUD's focused node (`/api/details`).
   50 nodes on one laptop: converged in 12s at 42.5 kbit/s per node (was 19s at 200); the live fleet takes in
   0.76 kbit/s of gossip payload per node and opens no connection in steady state.
5. **Done — 2026-10-25: Scaling phase B.** Peer scores after GossipSub (delivered, invalid; failed fetches only
   steer fetches elsewhere — counting them toward the graylist cut a node off from the live mesh during a rolling
   deploy, caught by the deploy gate). Scuttlebutt-style catch-up from one neighbor on joining and every 12 beats,
   with frames still signed by their authors; inventory fetches return the signed frame too. Devices and full
   probe reports are fetched for the HUD's focused node (`GET /api/details`) instead of gossiped.
   `./dialtone.sh scale-fleet N` runs N nodes on every fleet machine as one mesh: 60 nodes on three machines (before the move to VMs)
   converge in 30 to 50s and mostly run at about 420 kbit/s per node; some runs settle at about 850 kbit/s with
   50% more messages delivered (open; suspected: old beats coming round after iroh-gossip forgets their ids). A longer Plumtree graft timeout looked like
   a fix after one run each and made no difference over two more. `GET /api/net` adds iroh-gossip's data /
   control frame counts and each peer's last fetch error.
6. **Done — 2026-10-25: Gossip that scales (Phase A, after libp2p GossipSub).** Each node keeps 2 to 5 neighbors
   and joins three random known peers only when it has fewer than two, instead of joining every known peer every
   beat. Beats are a fixed ~1.2 KB (binary envelope v5, no seed lists, no per-peer probe results, no details); they
   announce the node's inventory version, and inventories go on a separate events topic when they change or are
   fetched on demand over `dialtone/fetch/1`. Rtt echoes broadcast to the whole mesh were replaced by timing a
   request to each neighbor. The beat interval grows with the mesh (`clamp(nodes / 10, 5, 60)` s). The default
   probe checks gossip only (`MSH` `NBR` `BEAT`); `dialtone probe --all` adds the store, pulls from 3 sampled
   peers and the probe program. `./dialtone.sh scale N` measures it: 10 local nodes went from 56 Mbit/s per node
   (2001 connections opened) to 60 kbit/s; 30 nodes, which never converged, converge in 8s at 120 kbit/s; 50 in 19s
   at 200 kbit/s.
7. **Done — 2026-10-25: One script.** `./dialtone.sh check | lint | e2e | deploy | fleet` replaced `ci.sh`, `e2e.sh`,
   `domain-lint.sh` and the local fleet deploy / test scripts. Fleet hosts live in the uncommitted `fleet.local.env`;
   `deploy` ends by waiting until every node's probe passes on the new build. **2026-10-28:** the script is now one line,
   `cargo run -p xtask`: the dev tasks are Rust (`xtask/`), the one-node and three-node tests are `cargo test`
   integration tests (`tests/node.rs`, `tests/e2e.rs`) that run on every OS, and no Python, jq or curl is needed.
8. **Done — 2026-10-25: Self-testing mesh and removals.** Every node runs a Probe every 30s against the live mesh:
   it writes a fresh blob, pulls every peer's latest probe blob over QUIC (SHA-256 verified), checks beats both ways,
   mesh membership and the probe program; results travel in beats and show in the HUD. It immediately found that
   the downloader's 1s connect timeout broke pulls over relayed / high-Rtt paths (now 10s) and that mesh-deployed
   programs could not be removed (copies now follow their origin). Removed: the old-name compatibility layer,
   the fake A/B slot and KV checks, the Rust node placement the HUD duplicated. Also fixed: `dialtone hud` next to
   a running node rewrote its config, hung on the store lock and could reap its processes.
9. **Done — 2026-10-25: Faster change → test loop.** The everyday `--release` build no longer uses fat LTO
   (`dist` profile keeps it for tagged releases): a one-file change rebuilds in ~1.5s instead of ~52s, `check` and
   fleet deploys reuse that build. Removed the in-process `self-test` command (`run_self_test`, the MemStore blob
   transfer and the gossip fixture in production code), which duplicated `cargo test`; GitHub CI no longer builds a
   second LTO binary just to run it, and `cargo test` builds one test binary instead of three.
10. **Done — 2026-10-25: Devices.** `dialtone devices` and a Devices actor list each node's cameras, microphones,
   speakers, keyboards, pointers, displays, printers, storage and Bluetooth devices (no serials or IDs); they
   travel in the Inventory, appear in `GET /api/devices` and `/api/mesh`, and the HUD lists the focused node's
   devices with a 7th tour step, `DEV`.
11. **Done — 2026-10-25: Phase 4 architecture.** The HUD is `src/hud/` static files served from `/hud/` with
   vendored three.js 0.179.1 and h3-js 4.4.0 (no CDN). The API parses requests with `httparse` and rejects
   `Transfer-Encoding`, ambiguous `Content-Length` and oversize heads. One persistent iroh-blobs `FsStore` replaces
   the SHA-256 file tree and the in-memory store: blobs are named by `sha256/<hex>` tags, GC removes untagged data,
   the node's one downloader pulls and SHA-256-checks peers' blobs, and old-layout blobs are imported at start.
   `run_node` boots the node and starts six actors (`actors.rs`: Mesh, Stats, Supervisor, Links, Allowlist,
   Deploy) stopped by one `CancellationToken`. Gossip carries a typed `Message` (envelope v4 then: Beat, Summary,
   Echo, Inventory); processes and services travel only in inventories, sent on change.
12. **Done — 2026-10-25: Phase 3 cleanup.** Removed the 11 `*-fixture` / preview commands and the fabricated
   seeded node state (`dialtone hud` now serves live state); the deploy-plan, build-cache, build-decision, manifest,
   proof-validation, alert-ack, tiered-fetch, process-intent, install.sh, catalog, command-journal / rtcore UDP,
   cursor-job, key-lease, service-ref, systemd drop-in and peer-cache code; 22 API routes no caller used, including
   the one-frame `/ws` stub; the hand-written SHA-1 (now unneeded) and hex / base64 codecs (now the `hex` and
   `base64` crates, same accepted inputs). The self-test exercises only real behaviour. The HUD's non-working
   Init / Invite / Join actions became a panel of the CLI commands (the page carries no API token by design).
14. **Done — 2026-10-23:** Phase 2 reliability, Phase 1 security, domain language, cleanup tier A.
   **2026-10-22:** authentic stats, Rtt, 5s / 30s beats, H3 globe, blob / process FX (see §0).

## Main Features (System Map)

| # | Feature | What it does | Where it lives |
|---|---------|--------------|----------------|
| F1 | **Node & transport** | Persistent Key → NodeId, iroh endpoint presets (N0 / minimal / private mesh), relay / Pkarr DNS / DHT / mDNS policy, tailnet-address filtering, transport diagnostics | `node.rs` |
| F2 | **Mesh membership** | Secret → Argon2id channel keys (topic, auth, enc), signed typed messages in XChaCha20-Poly1305 envelopes, deny-by-default Allowlist, signed single-use Invites, Seeds, `~/.dialtone/config.json` | `mesh.rs`, Mesh actor in `actors.rs` |
| F3 | **Beats, stats & Rtt** | Compact beats (about 255 bytes, `wire.rs`) every 5 to 120s within a 4 KB/s budget, real cpu / mem / disk / net stats on macOS, Linux and Windows, the 30s summary with spread + thrashing detection (fetched for the focused node), neighbor Rtt from QUIC on the gossip connection (Paths actor) → mesh Rtt matrix, public-IP location (ipinfo); the HUD places the rest | `stats.rs`, `mesh.rs`, `wire.rs` |
| F4 | **Blob store & tickets** | iroh-blobs `FsStore` under `~/.dialtone/blobs` with `sha256/<hex>` tags and GC, `blob share` / `blob pull` tickets through the node's downloader with digest re-check, per-node `store` stats in beats | `blob.rs` |
| F5 | **Supervisor & probe** | Stage programs from blobs, start / restart / stop / remove processes, boot-time sweep of unused staged programs, the `probe` program, and the Probe actor checking the live mesh every 30s (`MSH` `NBR` `BEAT`) and running full probes on request (`STO` `PUL` `PRC`) | `supervisor.rs`, `probe.rs` |
| F6 | **Deploy & daemon install** | Mesh deploy by signed `Deploy` announcements to all or named nodes (copies follow their origin and are removed with it), launchd / systemd `--user` / Scheduled Task daemon install | `deploy.rs`, Deploy actor |
| F7 | **Node runtime & API** | 5-stage boot, six actors with one shutdown token, shared node state, `httparse` API on `127.0.0.1:8788`, bearer-token POST auth, Host / Origin checks, live channel join | `main.rs` (`run_node`), `actors.rs`, `api.rs` |
| F8 | **HUD** | Three.js + H3 globe (vendored, compiled into the binary), `top`-style overlay, focused node's device list, 5-step tour with captions (mesh, blobs and pulls, process, beats, devices), blob / process FX | `hud.rs`, `hud/`, `h3_land.rs` |
| F12 | **Devices** | Per-OS listing of connected cameras, microphones, speakers, keyboards, pointers, displays, printers, storage, Bluetooth devices; gossiped in the Inventory | `devices.rs`, Devices actor |
| F9 | **Verification** | `probe` (`--all` adds the store, pulls, programs and security), unit + live iroh tests, `./dialtone.sh test` (zero-leak gate, domain-language gate, fmt, clippy, the cargo test suite, probes and a scale test on the VMs; `--here` on one machine), GitHub CI matrix | `lib.rs`, `tests/`, `xtask/`, `.github/workflows/` |
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
- **F6 Deploy.** Live: A/B rollback (fed synthetic slot bytes) and `install_persistent_os_daemon`. Preview-only: `generate_deploy_plan` (4 fixed synthetic peers), `evaluate_deploy_build_decision` (fixed timings/hash), `probe_build_cache` (cache list never populated), `validate_manifest_contract` (accepted manifests go nowhere), `render_install_v1_sh`. The service units are written only by `dialtone daemon install`.
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

### Cleanup plan (done 2026-10-23 / 2026-10-25)

| Tier | What | Size |
|---|---|---|
| A — delete now (no callers) | Dead F2 functions listed above, `lan_only`, dead F10 metadata types + fields + their tests | ~400 lines |
| B — delete with ci.sh / README / test edits | All `*-fixture` commands + `build_seeded_state` + `new_seeded`; preview planners (deploy plan, build decision, build cache, proof validation, alert ack, tiered fetch report, install.sh, catalogs, manifests); rtcore UDP / robot intents / cursor jobs / systemd drop-in; ~25 uncalled routes; self-test steps 4/6/7 | ~1,500 lines Rust + ~35 ci.sh + ~120 README |
| C — replace | hex / base64 / SHA-1 → `hex`, `base64`, `sha1_smol` (already in lockfile) | ~150 lines |

### Roadmap

1. **Phase 1 — Security (critical 1-5) — done 2026-10-23:** handshake allowlist hook + live allowlist, Ed25519-signed payloads with counter check, remove env-flag auto-deploy (gate behind signed `Deploy`), stop gossiping `env`, HUD escaping / `textContent` / no templated token.
2. **Phase 2 — Reliability — done 2026-10-23:** release the state lock before IO, ctrl-c shutdown + child process groups, re-subscribe backoff, no `?` in the loop, config parse errors fail loudly, stats unknowns as `None`, peers keyed by `NodeId` with TTL pruning.
3. **Phase 3 — Cleanup — done 2026-10-25:** tiers A → B → C, fix or remove the HUD `/api/channel/*` buttons.
4. **Phase 4 — Architecture — done 2026-10-25:** actor split of `run_node`, FsStore + downloader, typed message enum + inventory-on-change, httparse server, HUD as static files with vendored JS.

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
- **Synthetic Identifiers Only**: All source code (`src/*.rs`), unit tests, CLI fixtures, `dialtone.sh`, and `README.md` use generic synthetic node names (`node-alpha`, `node-beta`, `node-gamma`, `edge-1`, `edge-2`, `pendant-1`).
- **No Hardcoded Seed Keys in Source**: `default_seeds()` returns an empty list by default unless populated from the host's local `~/.dialtone/config.json` (`seeds`).
- **Hardened `.gitignore`**: Explicitly ignore all local state, config, and secret patterns:
  - `/target`, `/bin`, `.dialtone/`, `.dialtone2/`, `*config.json`, `*mesh*.json`, `*.key`, `*.pem`, `*.secret`, `.env`, `.env.*`.
- **Automated Leak Gate in `./dialtone.sh test --only lint`**: it scans all git-tracked files and fails the build if any forbidden machine name or real `NodeId` appears anywhere in the repository.

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
   - `dialtone daemon install` stores only `derived_keys` in `~/.dialtone/config.json` (`0600`) and register the OS service unit with `dialtone up --node <name> --listen <addr>` (no `--secret-key`, `--secret` or old `--password` flag in process arguments).
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
- [x] **Step 6**: Harden `.gitignore` and add an automated git-tracked file and commit history leak scanner to [`dialtone.sh`](dialtone.sh) (`lint`).
- [x] **Step 7**: Provision `~/.dialtone/config.json` (`seeds` + `allowlist` + `derived_keys`) onto the production mesh hosts out-of-band over SSH/Tailnet and verify multi-node gossip sync with zero secrets or hostnames in git.
