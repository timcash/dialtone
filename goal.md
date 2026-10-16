# Dialtone Security & Zero-Leak Fleet Architecture (`goal.md`)

## 1. Core Objective

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
    "secret_key": "<channel-password>",
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
    "bootstrap_tickets": [],
    "managed_processes": []
  }
  ```
- **Provisioning Workflow via SSH / Tailnet**:
  - Each host generates and keeps its own `~/.dialtone/iroh-secret-key` (`0600` permissions) locally.
  - An operator (or provisioning script running over Tailscale SSH) queries `dialtone status` over SSH on each authorized machine to collect its `endpoint_id`, then writes the shared channel password, `seed_peers`, and `allowed_endpoint_ids` directly to `~/.dialtone/config.json` on each host over SSH.
  - Because this happens over SSH into `~/.dialtone/config.json`, the real `EndpointId`s, hostnames, and channel password never touch the git working tree.

### Pillar C — Multi-Layer Gossip Channel & Peer Authentication
1. **Password-Derived Topic ID + Isolated Standby**:
   - Without a valid password in `~/.dialtone/config.json`, a node binds a node-local standby topic (`derive_standby_topic_id(&endpoint_id)`) and dials zero peers.
   - With a valid password, the gossip `TopicId` is derived via `SHA-256("dialtone.channel.topic.v1:<channel>:<password>")`.
2. **Per-Heartbeat Channel MAC + Ed25519 Identity Allowlist**:
   - Every outbound `DialtoneHeartbeat` is authenticated with `channel_mac` derived from `SHA-256("dialtone.channel.auth.v1:<channel>:<password>")`.
   - When `allowed_endpoint_ids` is non-empty in `~/.dialtone/config.json`, `dialtone` drops any inbound heartbeat whose `endpoint_id` (or QUIC transport `delivered_from` peer ID) is not in `allowed_endpoint_ids`.

### Pillar D — Local Daemon & HTTP Control Plane Hardening
1. **Eliminate Command-Line Secrets (`ps aux` / `wmic` hygiene)**:
   - `dialtone daemon install` and `render_os_service_unit` write the password to `~/.dialtone/config.json` (`0600`) and register the OS service unit with `dialtone up --node <name> --listen <addr>` (no `--secret-key` or `--password` flag in process arguments).
2. **Localhost HTTP `Host` & `Origin` Enforcement (`127.0.0.1:8788`)**:
   - Reject HTTP requests whose `Host` header is not `127.0.0.1`, `localhost`, or `[::1]` (prevents DNS rebinding).
   - Reject state-mutating `POST` requests that carry a cross-origin browser `Origin` header (prevents drive-by browser CSRF against `POST /api/processes/blob` and `POST /api/mesh/join`).

---

## 4. Implementation & Verification Checklist

- [x] **Step 1**: Password-protect the `"dialtone"` channel and authenticate heartbeats in constant time (`channel_mac`).
- [x] **Step 2**: Extend `PersistentMeshConfig` in [`src/gossip.rs`](file:///Users/tim.c/dialtone2/src/gossip.rs) with `seed_peers` and `allowed_endpoint_ids` loaded from `~/.dialtone/config.json`, and remove all hardcoded `EndpointId`s from source code.
- [x] **Step 3**: Replace all occurrences of real computer names across `src/`, `README.md`, and `ci.sh` with generic synthetic names (`node-alpha`, `node-beta`, `node-gamma`, `edge-1`, `edge-2`, `pendant-1`).
- [x] **Step 4**: Remove `--secret-key` / `--password` from generated OS service command-line arguments in [`src/deploy.rs`](file:///Users/tim.c/dialtone2/src/deploy.rs) so secrets are read exclusively from `~/.dialtone/config.json` (`0600`) and never appear in `ps aux`.
- [x] **Step 5**: Add `Host` and `Origin` header validation to [`src/http.rs`](file:///Users/tim.c/dialtone2/src/http.rs) to block browser CSRF and DNS rebinding against `127.0.0.1:8788`.
- [x] **Step 6**: Harden `.gitignore` and add an automated git-tracked file leak scanner to [`ci.sh`](file:///Users/tim.c/dialtone2/ci.sh).
- [x] **Step 7**: Provision `~/.dialtone/config.json` (`seed_peers` + `allowed_endpoint_ids` + `secret_key`) onto the three production hosts over SSH/Tailnet and verify 3-node gossip sync with zero secrets or hostnames in git.
