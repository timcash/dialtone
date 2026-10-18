# Dialtone Security, Authentic Telemetry & Zero-Leak Fleet Architecture (`goal.md`)

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
