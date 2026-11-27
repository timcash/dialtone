# Channel-Scoped Plugins and Blobs: Architecture Review and Design

- Task ID: `t-1a21a7ada23-434d905d`
- Owner: Tim | Orchestrator: Grok Bot
- Requested agent spec: provider `openai`, model `astra`, effort `high`, node `legion`
- Actual runner: `agy` (`gemini-4-neon-high`, effort high) on `legion` (dispatched by `dialtone-agent` on `legion`)
- Audited commits:
  - `timcash/dialtone`: `origin/main` at `f8f1dcb1` (`v0.5.6`)
  - `timcash/dialtone-task`: `origin/main` at `9cd8e1c` (includes `v0.3.0` `2d6c8c9`, `origin/feat/scaling` `f653cf1`, and GB-34 `origin/feat/agent-spec` `9cd8e1c`)
  - Sibling plugins inspected read-only on `legion`: `dialtone-agent-build`, `dialtone-shell`
- Base branch and GB-34 coordination note:
  - At start of work, `origin/feat/agent-spec` (GB-34, `9cd8e1c`) had already been merged into `origin/main`. `feat/channel-scope` is therefore based on `origin/main` (`9cd8e1c`), while keeping all code changes isolated to config, key derivation, network bootstrap, data-dir resolution, and status reporting (avoiding GB-34's task-field, CLI `add`, log, and note-history code paths). If additional GB-34 commits land on `origin/feat/agent-spec`, `feat/channel-scope` must rebase onto GB-34 before merge.

---

## 1. Target Hierarchy

The target model makes the channel the single unit of network membership, authorization, plugin installation, and data isolation:

1. Anybody can install `dialtone`.
2. A node joins a channel with a link (invite or secret) or creates a new channel. **Channel = network.**
3. The channel declares which plugins are permitted (`agent`, `shell`, `task`, etc.).
4. A node installs permitted plugins onto that channel.
5. Every channel includes a **Blob store** and the **gossip/mesh** transport as built-ins. ALL data (tasks, logs, code, blobs) is visible only inside that channel. A peer outside the channel cannot list or fetch it, even if it knows a content hash.

---

## 2. Current State (Verified Codebase Audit)

### 2.1 `timcash/dialtone` (`v0.5.6`, `f8f1dcb1`)

#### Channel identity, KDF, and config (`src/mesh.rs`, `src/join.rs`, `src/main.rs`)
- **Single-channel config**: `MeshConfig` (`src/mesh.rs:808-830`) stores only ONE active channel per `~/.dialtone/config.json`: `channel`, `configured`, `derived_keys` (`kdf_version`, `topic_id_hex`, `auth_key_hex`, `enc_key_hex`), `channel_fingerprint`, `seeds`, `allowlist`, `used_invite_nonces`, `invites`, `processes`.
- **Key derivation**: `derive_channel_keys_v2(channel, secret)` (`src/mesh.rs:254-313`) derives a 32-byte master key via Argon2id (`m=19456 KiB, t=2, p=1`, salt = `SHA256("dialtone.kdf.salt.v2:" || channel)[..16]`), then expands three domain-separated 32-byte subkeys via SHA-256 (`topic_id`, `auth_key`, `enc_key`). The raw channel secret is never serialized to disk (`secret_key` has `#[serde(skip_serializing)]`, `src/mesh.rs:816-817`).
- **Channel fingerprint**: `ChannelKeys::fingerprint()` (`src/mesh.rs:241-244`) returns `ch-<first 12 hex chars of topic_id>`.
- **Gossip encryption & auth**: `seal_message` / `open_message` (`src/mesh.rs:1702-1770`) seal gossip messages with `XChaCha20-Poly1305` using `enc_key` and authenticate senders via Ed25519 signatures verified against the sender's `EndpointId`.
- **Invites and join**:
  - `OpenInvite` v2 (`src/mesh.rs:629-724`): carries `channel`, `expires_at_unix`, `nonce`, `seeds`, and a MAC keyed by `auth_key`. No keys are in the invite string.
  - `dialtone/join/1` (`src/join.rs:21-178`): `Joins::answer()` validates the `OpenInvite` MAC on open channels (empty `allowlist`) and replies with `JoinAnswer { channel, derived_keys, seeds }`.
  - `join_with_invite` (`src/main.rs:778-815`) and `join` (`src/main.rs:2434-2566`) overwrite the single global `~/.dialtone/config.json`.

#### Plugin system (`src/plugin.rs`, `src/main.rs`, `src/api.rs`)
- **Schema**: `PluginManifest` (`src/plugin.rs:49-68`, schema `"dialtone.plugin.v1"` at L20) stores `name`, `version`, `kind` (`builtin-protocol`, `blob-binary`, `blob-bundle`), `protocols`, `blob_digest` (SHA-256), `blake3_hash`, `bytes`, `entrypoint`, `ticket_uri`, `source_repo`, `installed_at_unix`.
- **Global install directory**: `default_plugins_dir()` (`src/plugin.rs:99-101`) resolves to `<state_dir>/plugins` (`~/.dialtone/plugins/<name>/`). `plugins_dir_for_store()` (`src/plugin.rs:103-108`) resolves to `<store_dir>/../plugins`. Neither is channel-scoped.
- **No channel plugin allowlist**: `install_builtin_plugin`, `install_plugin_from_file_with_repo`, `install_plugin_from_repo`, `install_plugin_from_ticket` (`src/plugin.rs:173-483`), and `POST /api/plugins/install` (`src/api.rs:1560-1623`) accept any valid plugin name without checking any channel policy.
- **Call-time CLI dispatch**: `build_plugin_command()` (`src/plugin.rs:882-910`) and `main.rs` `"plugin-dispatch"` (`src/main.rs:2603-2632`) strip `DIALTONE_SECRET`, `DIALTONE_SECRET_KEY`, `DIALTONE_GATEWAY_TOKEN`, and `MESH_ORIGIN_TOKEN`, and inject only:
  - `DIALTONE_HOME`
  - `DIALTONE_STATE_DIR`
  - `DIALTONE_API`
  - `DIALTONE_NODE`
  - `DIALTONE_PLUGIN_NAME`
  - `DIALTONE_PLUGIN_DIR`
  No channel ID, no derived plugin key, and no per-channel data dir are passed.

#### Blob store and access control (`src/blob.rs`, `src/node.rs`, `src/actors.rs`, `src/main.rs`, `src/api.rs`)
- **Single store per node**: `default_store_dir()` (`src/mesh.rs:904-906`) opens one `iroh_blobs::FsStore` at `~/.dialtone/blobs/blobs.db` (`src/blob.rs:116-161`).
- **Addressing**: Blobs are indexed by `iroh-blobs` BLAKE3 hash and tagged by SHA-256 (`sha256/<hex> -> blake3_hash`, `src/blob.rs:49-67`, L234-261). `Store::share_blob` (`src/blob.rs:571-588`) returns `iroh-blobs://ticket/<BlobTicket>#sha256:<hex>`.
- **Network serving**: `src/main.rs:1216,1237` mounts `iroh_blobs::BlobsProtocol::new(blobs.api(), None)` directly on the node's single `Router` under `iroh_blobs::ALPN`. `BlobsProtocol` itself has NO ACL and serves any hash present in `blobs.db`.
- **Handshake gating (`AllowlistHook`)**: `AllowlistHook::after_handshake` (`src/node.rs:396-449`) runs after QUIC TLS handshake:
  - If `allowlist` is non-empty: only listed `EndpointId`s are admitted for all protocols.
  - If `allowlist` is empty (open channel): `iroh_gossip::ALPN` and `dialtone/join/1` are open to any peer; all other ALPNs (including `iroh_blobs::ALPN`, `dialtone/fetch/1`, `dialtone/link/1`, `dialtone/exec/1`, `dialtone/shell/1`) require `self.members.contains(&remote)`.
  - `members` is populated in `Mesh::admit()` (`src/actors.rs:2225-2227`) whenever a peer sends a valid signed gossip message that decrypts with the active channel's `enc_key`.
- **Critical ACL gap on channel switch**: `Mesh::on_control(MeshControlCommand::JoinMesh)` (`src/actors.rs:1852-1896`) switches `self.channel`, `self.topic_id`, and `self.keys`, but **never clears `self.ctx.members` and never swaps the `Blobs` store**. Consequently:
  1. Peers from Channel A remain in `members` after switching to Channel B and retain QUIC access to Channel B's blobs, fetch, exec, shell, and links until restart.
  2. Blobs stored under Channel A remain in `~/.dialtone/blobs/blobs.db` and become fetchable by Channel B peers if they know or learn a BLAKE3 hash.
  3. On a multi-channel host (future target), a single `FsStore` mounted on a single global `iroh_blobs::ALPN` cannot separate Channel A blobs from Channel B blobs.
- **HTTP API**: `HEAD /api/blob`, `GET /api/blob`, `POST /api/blob`, `POST /api/blob/share`, `POST /api/blob/pull` (`src/api.rs:1841-1935`) operate on the single node-wide `st.blobs` store with no channel parameter or channel scoping.

#### Built-in `shell` plugin (`src/shell.rs`, `dialtone-shell/src/main.rs`)
- `ShellHandler` (`src/main.rs:1248-1251`) is mounted on `dialtone/shell/1` and checks `is_plugin_installed_and_verified` against the global `~/.dialtone/plugins/shell` directory.
- `dialtone-shell` (`dialtone-shell/src/main.rs:14-106`) is a stateless stdio JSON-RPC helper (`--serve --node <name>`) spawned by the daemon. It inherits channel gating from the daemon's `AllowlistHook`, but has no per-channel session table and does not isolate sessions when switching channels.

### 2.2 `timcash/dialtone-task` (`9cd8e1c`)

- **Separate network & random key**: `dialtone-task` runs its own independent `iroh::Endpoint` (`src/net.rs:1704-1710`) and generates its own random 32-byte `mesh.key` (`src/config.rs:474-492`) stored in `<data_dir>/mesh.key`.
- **Topic & frame crypto**: `derive_topic_id(&mesh_key)` (`src/config.rs:538-543`) computes `blake3("dialtone-task:topic:v1:" || mesh_key)`. Both `iroh-gossip` frames and `dialtone/task/1` sync/blob frames are sealed with `ChaCha20Poly1305` under `mesh_key` (`src/config.rs:546-570`, `src/net.rs:345-407`, `src/net.rs:805-847`).
- **Separate peer discovery & tickets**: Peers live in `<data_dir>/config.json` (`NodeConfig.peers`, `src/config.rs:245`) and propagate only via `dialtone-task ticket` / `dialtone-task join` (`JoinTicket` carrying raw hex `mesh_key` + `peers`, `src/config.rs:420-424`, `src/net.rs:3206-3253`).
- **Single global data dir**: `default_data_dir()` (`src/config.rs:22-34`) checks `DIALTONE_TASK_DIR` or defaults to `~/.dialtone/plugins/task`. It ignores `DIALTONE_CHANNEL_ID`, `DIALTONE_CHANNEL_DATA_DIR`, and `DIALTONE_CHANNEL_PLUGIN_KEY`.
- **Storage & RPC**:
  - `TaskStore` (`src/store.rs:11-55`) stores all CRDT entries in `<data_dir>/state.redb` (`entries` and `meta` tables) with no channel dimension.
  - `BlobStore` (`src/logs.rs:151-178`) stores gzipped task logs and large prompts under `<data_dir>/blobs/<blake3>.z`, served over `dialtone/task/1` `SyncFrame::FetchBlob` (`src/net.rs:419-437`, encrypted with `mesh_key`).
  - Loopback HTTP RPC (`src/rpc.rs:16-148`) binds to a single default `127.0.0.1:8794` (`DEFAULT_RPC_ADDR`, `src/config.rs:19`) authenticated by `<data_dir>/rpc.token`.
  - `NodeStatus` (`src/net.rs:67-85`) and `Commands::Status` (`src/main.rs:738-762`) do not report any `channel_id`.

### 2.3 `dialtone-agent` (`dialtone-agent-build`, read-only inspection)

- `src/config.rs:42-66`: defaults to `~/.dialtone/plugins/agent` (`DIALTONE_AGENT_DIR` / `DIALTONE_PLUGIN_DIR`) and hardcodes `default_task_dir()` to `DIALTONE_TASK_DIR` or `~/.dialtone/plugins/task`.
- `src/task_client.rs:140-178`: connects to a single loopback `dialtone-task` RPC (`127.0.0.1:8794` or `<task_dir>/config.json` `rpc_addr`) using `<task_dir>/rpc.token`.
- **Channel gap**: `dialtone-agent` assumes one global task mesh and one global workspace dir. In a channel-scoped world, an agent worker must run scoped to a channel (reading `DIALTONE_CHANNEL_ID` and `DIALTONE_CHANNEL_DATA_DIR`, discovering the matching channel's `task` data dir / RPC endpoint, and isolating its `work/` directory per channel).

---

## 3. Gaps vs. the 5-Step Hierarchy

| Component | Step 1-2: Channel = Network | Step 3: Channel Plugin Allowlist | Step 4: Per-Channel Plugin Install | Step 5: Channel-Only Data & Blobs |
|---|---|---|---|---|
| **dialtone daemon (gossip/mesh)** | Single channel per `config.json`; multi-channel join not supported yet. | None. No channel manifest or plugin allowlist. | Global `~/.dialtone/plugins/<name>/`. | `JoinMesh` does not clear `ctx.members`; single store dir across channel switches. |
| **Built-in Blob Store** | Single `~/.dialtone/blobs` per node. | N/A (built-in). | N/A (built-in). | `BlobsProtocol` has no per-channel store/ALPN; channel switch leaks old blobs and old `members`. |
| **`dialtone-task`** | Uses independent random `mesh.key` + `JoinTicket`, completely decoupled from dialtone channel. | Not checked. | Installed globally; single `state.redb` and single `127.0.0.1:8794`. | Tasks & log blobs isolated by `mesh.key`, not by dialtone channel; manual ticket distribution required. |
| **`dialtone-agent`** | Talks to global task RPC; unaware of channels. | Not checked. | Global `~/.dialtone/plugins/agent/` and `work/`. | Agent workdirs and task polling not separated per channel. |
| **`dialtone-shell`** | Inherits daemon's `AllowlistHook` on `dialtone/shell/1`. | Only checks global `plugins/shell` install, not channel policy. | Global `~/.dialtone/plugins/shell/`. | Sessions live in one process-wide `ShellManager`; not reset or namespaced per channel. |
| **Future / third-party plugins** | Receive no channel identity or key from daemon. | No enforcement at `install` or `dispatch`. | Global `~/.dialtone/plugins/<name>/`. | Every plugin would have to invent its own key & directory scheme. |

---

## 4. Proposed Design

### 4.1 Per-Channel Directory Layout

Replace the flat `~/.dialtone/plugins/` and `~/.dialtone/blobs/` layout with channel-rooted directories keyed by the channel fingerprint (`channel_id`, e.g. `ch-a1b2c3d4e5f6`):

```text
~/.dialtone/
  iroh-secret-key                 # Node Ed25519 identity (shared across channels)
  api-token                       # Local daemon HTTP API token (0600)
  config.json                     # Node settings + active_channel + channels map (or index)
  channels/
    <channel_id>/                 # e.g. ch-123456789abc (derived from channel topic_id)
      config.json                 # Channel name, derived_keys, allowlist, seeds, manifest
      manifest.json               # Signed channel policy manifest (dialtone.channel.manifest.v1)
      blobs/                      # Channel-private iroh-blobs FsStore (blobs.db + store.lock)
      plugins/
        <plugin_name>/            # Permitted & installed plugin binaries + manifest.json
          manifest.json
          bin/...
      data/
        task/                     # DIALTONE_CHANNEL_DATA_DIR for task plugin
          config.json
          state.redb
          rpc.token
          iroh-secret-key
          blobs/                  # Task log/prompt compressed blobs
        agent/                    # DIALTONE_CHANNEL_DATA_DIR for agent plugin
          config.json
          work/
        <other>/
```

**Mapping current `metal` in place**:
- During migration, `~/.dialtone/config.json` continues to work for single-channel nodes.
- The existing `metal` channel's fingerprint `ch-<metal_fp>` maps to `~/.dialtone/channels/<metal_fp>/`.
- Legacy paths (`~/.dialtone/blobs`, `~/.dialtone/plugins/<name>`) remain recognized as fallback when no `channels/<id>/` directory exists yet, so unmigrated nodes continue working without interruption.

### 4.2 Daemon-to-Plugin Key Derivation and Environment Contract

When `dialtone` dispatches or supervises a plugin for a channel with active `ChannelKeys`:

1. **Key derivation (never passes raw channel secret)**:
   Using RFC 5869 **HKDF-SHA256** (zero new dependencies in `dialtone`, already depends on `sha2`):
   - `salt = keys.topic_id.as_bytes()` (32 bytes, binds output to channel identity)
   - `ikm = keys.enc_key || keys.auth_key` (64 bytes of Argon2id-derived channel key material)
   - `prk = HMAC-SHA256(key = salt, msg = ikm)`
   - `info = "dialtone/plugin-key/v1/" || canonical_plugin_name`
   - `plugin_key = HMAC-SHA256(key = prk, msg = info || 0x01)` (32 bytes, hex-encoded as 64 lowercase hex chars)
   Properties:
   - Never touches or exposes the raw channel secret (`DIALTONE_SECRET` remains explicitly stripped).
   - Cryptographically independent per `(channel, plugin_name)`: compromising plugin `foo`'s key cannot recover `enc_key`, `auth_key`, or plugin `bar`'s key, nor any key on another channel.
   - Deterministic across all nodes on the same channel without extra out-of-band ticket exchange.

2. **Environment variables passed to plugin processes** (`src/plugin.rs` `build_plugin_command`):
   - Existing: `DIALTONE_HOME`, `DIALTONE_STATE_DIR`, `DIALTONE_API`, `DIALTONE_NODE`, `DIALTONE_PLUGIN_NAME`, `DIALTONE_PLUGIN_DIR`
   - New (set whenever the active channel has `ChannelKeys`):
     - `DIALTONE_CHANNEL_ID`: `keys.fingerprint()` (e.g. `ch-a1b2c3d4e5f6`)
     - `DIALTONE_CHANNEL_PLUGIN_KEY`: 64-char lowercase hex of the 32-byte derived plugin key
     - `DIALTONE_CHANNEL_DATA_DIR`: `<state_dir>/channels/<channel_id>/data/<plugin_name>`
   - Explicitly removed (defense in depth): `DIALTONE_SECRET`, `DIALTONE_SECRET_KEY`, `DIALTONE_GATEWAY_TOKEN`, `MESH_ORIGIN_TOKEN`, plus `DIALTONE_CHANNEL_PLUGIN_KEY` is removed first and only re-injected from the active `PluginDispatchContext` so stale parent env vars cannot leak across channels.

3. **How `dialtone-task` consumes the env contract**:
   - **Key & topic derivation**:
     - If `DIALTONE_CHANNEL_PLUGIN_KEY` is set (validated as 32-byte hex; never logged or printed):
       - `mesh_key = blake3::derive_key("dialtone-task/mesh/v1", &plugin_key_bytes)`
       - `topic_id = derive_topic_id(&mesh_key)` (`blake3("dialtone-task:topic:v1:" || mesh_key)`)
       - `mesh.key` on disk is NOT required and NOT overwritten with the derived key.
     - If `DIALTONE_CHANNEL_PLUGIN_KEY` is unset: fall back to existing `<data_dir>/mesh.key` (`load_or_create_mesh_key`) for 100% backwards compatibility.
   - **Data directory resolution**:
     1. Explicit `--data-dir` / `DIALTONE_TASK_DIR` wins if set.
     2. Else if `DIALTONE_CHANNEL_DATA_DIR` is set (non-empty), use it directly (e.g. `~/.dialtone/channels/<channel_id>/data/task`).
     3. Else if `DIALTONE_CHANNEL_ID` is set (non-empty, validated safe path component), use `<base>/channels/<channel_id>/` (where `<base>` is `~/.dialtone/plugins/task`, keeping `state.redb`, `mesh.key`, `rpc.token`, `config.json`, `blobs/` isolated per channel).
     4. Else legacy `~/.dialtone/plugins/task`.
   - **Status visibility**: `NodeStatus` includes `channel_id: Option<String>` (fingerprint only, NEVER the key), printed in `dialtone-task status` and `dialtone-task status --json`.

### 4.3 Per-Channel Blob Store Namespace and ACL

To guarantee Step 5 ("A peer outside the channel cannot list or fetch a blob, even if it knows a hash"):

1. **Storage isolation (per-channel `FsStore`)**:
   - Each channel owns its own `Blobs` store directory at `~/.dialtone/channels/<channel_id>/blobs/` (own `blobs.db` and `store.lock`).
   - Listing (`Blobs::list_digests()`), `has()`, `get()`, and `summary()` only see blobs stored in that channel's directory.
   - Blobs stored on Channel A never exist in Channel B's store.

2. **Transport & protocol ACL (`ChannelBlobsProtocol`)**:
   - Wrap `iroh_blobs::BlobsProtocol` in `ChannelBlobsProtocol { channel_id, inner, allowlist, members }`.
   - On every incoming connection in `ProtocolHandler::accept`:
     - Verify `remote_id = connection.remote_id()`.
     - If `allowlist` is non-empty, require `allowlist.contains(&remote_id)`.
     - If `allowlist` is empty (open channel), require `members.contains(&remote_id)` (meaning `remote_id` has already proven knowledge of this channel's `enc_key` by sending a valid signed gossip message on this channel).
     - Otherwise close the QUIC connection immediately with code `403` (`NOT_ALLOWLISTED_CLOSE_CODE`) without passing the connection to `iroh_blobs::BlobsProtocol`.
   - **Fix channel-switch leak**: `Mesh::on_control(JoinMesh)` clears `self.ctx.members` immediately when switching channels so previous-channel peers lose blob/fetch/exec/shell access right away.
   - **Multi-channel ALPN routing (P4 full)**: When a single daemon hosts multiple channels simultaneously on one `iroh::Endpoint`, register per-channel blob ALPNs `dialtone/blobs/v1/<channel_id>` (each backed by its own `ChannelBlobsProtocol` and channel-private `FsStore`). Legacy `iroh_blobs::ALPN` stays active only for the single default channel during rollout.

3. **Optional encryption at rest (sensitive channels)**:
   - Channels may enable `encrypt_blobs_at_rest: true` in channel policy, sealing blob payloads with `XChaCha20-Poly1305` using `HKDF-SHA256(enc_key, salt=topic_id, info="dialtone/blob-at-rest/v1")` before `FsStore::add_slice`. Because encryption happens before `iroh-blobs` ingestion, verified BLAKE3 streaming still works natively over ciphertext, and disk snapshots cannot expose plaintext.

4. **Task log and prompt blobs**:
   - `dialtone-task`'s own `BlobStore` (`src/logs.rs`) already lives inside `<data_dir>/blobs/` and serves `SyncFrame::FetchBlob` over `dialtone/task/1` sealed with `mesh_key`. With per-channel `data_dir` and channel-derived `mesh_key`, a peer outside the channel cannot decrypt or forge a `FetchBlob` frame and receives no response.

### 4.4 Channel Plugin Allowlist and Signed Channel Manifest

1. **Manifest schema (`dialtone.channel.manifest.v1`)**:
   ```json
   {
     "schema": "dialtone.channel.manifest.v1",
     "channel_id": "ch-a1b2c3d4e5f6",
     "channel_name": "metal",
     "seq": 3,
     "updated_at_unix": 1795740000,
     "admins": ["<ed25519_endpoint_id_1>", "<ed25519_endpoint_id_2>"],
     "allowed_plugins": [
       {
         "name": "task",
         "repo": "timcash/dialtone-task",
         "min_version": "0.4.0",
         "pinned_digests": {}
       },
       { "name": "shell" },
       { "name": "agent" }
     ],
     "blob_policy": {
       "encrypt_at_rest": false
     },
     "signatures": [
       { "signer": "<ed25519_endpoint_id_1>", "sig_hex": "..." }
     ]
   }
   ```
2. **Who can sign**:
   - The channel creator's `EndpointId` is the initial sole entry in `admins` (signed with the node's Ed25519 `iroh::SecretKey`).
   - Admins can add/remove other admin `EndpointId`s by publishing a manifest with `seq = prev_seq + 1` signed by a current admin (or threshold `k`-of-`n` for team channels).
   - For bootstrap on channels created via shared secret without a manifest yet, the first admin manifest is anchored by also carrying an HMAC with `auth_key` on `seq == 1`.
3. **Propagation**:
   - Manifests gossip on the channel's `events` topic (`events_topic_id(beats)`, already XChaCha20-Poly1305 encrypted + Ed25519 signed) and are included in `JoinAnswer` for new joiners, plus fetchable via `dialtone/fetch/1` (`Request::ChannelManifest`).
   - Nodes persist the highest valid `seq` manifest at `~/.dialtone/channels/<channel_id>/manifest.json` and reject rollbacks (`new.seq <= cur.seq`) or invalid signatures.
4. **Enforcement (fail-closed)**:
   - `dialtone install <plugin>` checks `manifest.allowed_plugins` for the target channel. If `<plugin>` is not permitted (or digest does not match `pinned_digests` when set), installation is refused with a clear error naming the channel and permitted plugin list.
   - `resolve_plugin_binary()` / `dispatch_plugin()` and `ShellHandler` re-verify against the channel manifest before execution. If an admin removes a plugin from `allowed_plugins` in `seq + 1`, all nodes immediately refuse further invocations of that plugin on that channel.

### 4.5 Channel Create and Join Flows

- **Create**: `dialtone channel create <name> [--plugins task,shell,agent] [--open|--allowlist]`
  1. Generates a high-entropy secret (`new_secret()`), derives `ChannelKeys` (`derive_channel_keys_v2`) and `channel_id = keys.fingerprint()`.
  2. Creates `~/.dialtone/channels/<channel_id>/{config.json, manifest.json, blobs/, plugins/, data/}`.
  3. Signs `seq = 1` `ChannelManifest` with this node's Ed25519 key (making this node the initial admin) and sets `allowed_plugins`.
  4. Starts gossip + `ChannelBlobsProtocol` for the new channel and prints the channel ID and an initial `OpenInvite` (or instructions to share secret / add allowlist peers).
- **Join by invite**: `dialtone join <dialtone-invite-v2:...>`
  1. Redeems invite over `dialtone/join/1`. `JoinAnswer` returns `channel`, `derived_keys`, `seeds`, and the signed `ChannelManifest`.
  2. Creates `~/.dialtone/channels/<channel_id>/`, saves `config.json` + verified `manifest.json`, and opens a channel-private `Blobs` store.
  3. Does NOT auto-install plugins without user action (or optionally prints `Channel permits: task, shell, agent; run 'dialtone install <plugin>'`).
- **Join by secret**: `dialtone join <secret> --channel <name>`
  1. Derives `ChannelKeys` locally, initializes `~/.dialtone/channels/<channel_id>/`, joins gossip, and pulls the latest signed `ChannelManifest` from peers over `events` / `dialtone/fetch/1`.

### 4.6 Migration Plan for the 41 Existing `metal` Tasks

The `metal` fleet (`rosie`, `legion`, `build-1`, `vm-us`, `vm-eu`, `vm-asia`, `steamdeck-2`, `grok-bot-metal`) currently runs `dialtone v0.5.6` and `dialtone-task v0.3.0` with 41 live tasks in `~/.dialtone/plugins/task/state.redb`, replicated under a shared random `mesh.key`. Zero task loss is mandatory.

**Exact Zero-Loss Migration Procedure (Dual-Topic / Dual-Key Transition Window)**:

1. **Pre-flight backup (before any binary upgrade)**:
   - On `rosie` and `legion`, snapshot `~/.dialtone/plugins/task/state.redb` and `config.json` to a timestamped backup directory (read-only copy; never print `mesh.key` or `rpc.token`). Record `dialtone task status --json` (`task_count`, `state_hash`).
2. **Stage 1: Deploy updated `dialtone-task` (compat mode, no env vars yet)**:
   - Because `dialtone-task` falls back to legacy `~/.dialtone/plugins/task` and `mesh.key` whenever `DIALTONE_CHANNEL_PLUGIN_KEY` and `DIALTONE_CHANNEL_ID` are unset, rolling out the new `dialtone-task` binary across all nodes changes zero wire behavior and keeps all 41 tasks syncing on the legacy topic.
3. **Stage 2: Preserve `state.redb` into `metal` channel dir + Dual-Key Bridge**:
   - Before flipping `DIALTONE_CHANNEL_PLUGIN_KEY` on, copy (not move, keeping legacy as rollback) `~/.dialtone/plugins/task/{state.redb, config.json, iroh-secret-key, blobs/}` into `~/.dialtone/channels/<metal_channel_id>/data/task/` (and preserve the legacy `mesh.key` as `legacy-mesh.key` inside that dir).
   - **Dual-key transition mode** (enabled only when `legacy-mesh.key` is present alongside `DIALTONE_CHANNEL_PLUGIN_KEY` and `channel_id == <metal_fp>`):
     - Primary outbound sealing and primary gossip topic use the channel-derived `mesh_key`.
     - During the transition window, the node also subscribes to `derive_topic_id(legacy_mesh_key)` and accepts inbound `dialtone/task/1` and gossip frames that decrypt with `legacy_mesh_key` (re-gossiping newly merged entries sealed under the new channel-derived `mesh_key`).
     - This guarantees mixed-version nodes (nodes still on `v0.3.0` + legacy `mesh.key`) and upgraded nodes remain in continuous sync with zero split-brain during rollout.
4. **Stage 3: Upgrade `dialtone` daemon and cut over `metal` nodes**:
   - Upgrade `dialtone` daemon across fleet nodes (passes `DIALTONE_CHANNEL_ID`, `DIALTONE_CHANNEL_PLUGIN_KEY`, `DIALTONE_CHANNEL_DATA_DIR`).
   - Restart `dialtone-task serve` under channel env on all nodes. Verify `dialtone task status` on every node reports `channel_id: ch-<metal_fp>`, `task_count >= 41`, and identical `state_hash`.
5. **Stage 4: Close transition window**:
   - Once every fleet node reports `channel_id: ch-<metal_fp>` and identical `state_hash`, remove `legacy-mesh.key` and restart `dialtone-task serve` (or reload) so legacy frames are no longer accepted. Old `dialtone-task` tickets stop working permanently.

### 4.7 Ticket and Key Rotation Story

- **Key rotation**: Rotating the channel secret (`dialtone secret` + re-join on new secret, or future `dialtone channel rotate`) produces new Argon2id `derived_keys` (`topic_id`, `auth_key`, `enc_key`), which deterministically yields a new `DIALTONE_CHANNEL_PLUGIN_KEY` for every plugin.
  - `dialtone-task` derives a new `mesh_key` and new `TopicId`.
  - Any peer still holding the old secret or an old `dialtone-task` `JoinTicket` fails ChaCha20-Poly1305 authentication and subscribes to a different gossip topic.
- **Deprecating raw `mesh_key` in `dialtone-task` tickets**:
  - Once running with `DIALTONE_CHANNEL_PLUGIN_KEY`, `dialtone-task` should refuse to export raw `mesh_key` in `JoinTicket` (schema v2 ticket carries only `channel_id` + peer hints, or `dialtone-task` pulls peer addresses directly from the local `dialtone` daemon HTTP API `GET /api/net` / `GET /api/status`, eliminating manual `dialtone-task ticket`/`join` altogether).

### 4.8 Risks and Mixed-Version Compatibility

- **Mixed-version partition risk**: A node running with `DIALTONE_CHANNEL_PLUGIN_KEY` derives a different `mesh_key` and `TopicId` than a `v0.3.0` node using `mesh.key`. Without the Stage 2 dual-key transition window (or simultaneous fleet cutover), the two sets of nodes will not sync with each other.
- **RPC port collision on multi-channel nodes**: `dialtone-task` defaults to `127.0.0.1:8794`. Running two `dialtone-task serve` instances for two channels on the same host requires per-channel RPC addresses (e.g. Unix domain sockets / named pipes in `<channel_data_dir>/rpc.sock`, or dynamic loopback port written to `<channel_data_dir>/rpc.addr`).
- **`dialtone-agent` discovery**: `dialtone-agent` currently reads `~/.dialtone/plugins/task/rpc.token`. When `task` moves to `<channel_data_dir>`, `dialtone-agent` must read `DIALTONE_CHANNEL_DATA_DIR` (or sibling `../task/`) and the channel's `rpc.addr` + `rpc.token`.
- **Direct binary invocation vs `dialtone <plugin>`**: Users or services (like `launchd` / `systemd` / Scheduled Tasks) running `dialtone-task serve` directly will not get `DIALTONE_CHANNEL_*` env vars unless launched by the `dialtone` supervisor or configured with the channel env / daemon HTTP lookup. Phase 2 preserves full legacy fallback when env vars are absent so existing service units do not break.

---

## 5. Phased Implementation Plan

| Phase | Scope | Status in this work |
|---|---|---|
| **P1: `dialtone-task` channel key & data dir** | Derive `mesh_key` + `TopicId` from `DIALTONE_CHANNEL_PLUGIN_KEY` (`blake3::derive_key("dialtone-task/mesh/v1", key)`); fallback to `mesh.key`; per-channel data dir (`DIALTONE_CHANNEL_DATA_DIR` / `DIALTONE_CHANNEL_ID`); expose `channel_id` in `status`; two-channel isolation, fallback, and data-dir tests. | **Implemented in Phase 2** (`feat/channel-scope`). |
| **P2: `dialtone` plugin key derivation & env** | HKDF-SHA256 `derive_channel_plugin_key(keys, plugin_name)` with label `"dialtone/plugin-key/v1/<plugin>"`; pass `DIALTONE_CHANNEL_ID`, `DIALTONE_CHANNEL_PLUGIN_KEY`, `DIALTONE_CHANNEL_DATA_DIR` in `build_plugin_command`; per-channel path helpers; unit tests proving per-plugin/per-channel uniqueness and zero raw-secret leakage. | **Implemented in Phase 2** (`feat/channel-plugin-keys`). |
| **P3: Channel plugin allowlist & signed manifest** | `dialtone.channel.manifest.v1` schema, Ed25519 admin signing, gossip propagation on `events` topic, enforcement in `dialtone install`, `dispatch_plugin`, and `ShellHandler`. | Designed in Section 4.4 (follow-up PR). |
| **P4: Channel-scoped blob store & ACL** | Minimal ACL + leak fix implemented now: `ChannelBlobsProtocol` enforcing channel membership on `iroh_blobs::ALPN`, `Blobs::list_digests()`, per-channel blob store dir helper, `JoinMesh` clearing `ctx.members`, and two-channel blob isolation test. Full multi-channel ALPN multiplexing (`dialtone/blobs/v1/<channel_id>`) and optional at-rest encryption follow in P4b. | **Minimal ACL + isolation test implemented in Phase 2** (`feat/channel-plugin-keys`); multi-channel ALPN & at-rest encryption designed. |
| **P5: Mac app / HUD channel listing** | Expose `GET /api/channels` on `dialtone` daemon returning joined channels (`channel_id`, `name`, `status`, `peers`, `allowed_plugins`, `installed_plugins`); update Mac app / HUD to list and switch joined channels from `dialtone`. | Designed (follow-up after P3/P4b). |
| **P6: Migration & fleet rollout (requires Tim's OK)** | Dual-key bridge for `metal` 41-task migration, service unit updates, coordinated fleet upgrade, and legacy `mesh.key` retirement. | Designed in Section 4.6; **gated on Tim's explicit OK** (no live changes made). |

---

## 6. Rollout Steps Requiring Tim's Explicit OK

1. Approve the HKDF-SHA256 plugin key derivation label (`"dialtone/plugin-key/v1/<plugin_name>"`) and `dialtone-task` subkey label (`"dialtone-task/mesh/v1"`) as frozen wire/KDF constants before any fleet deployment.
2. Approve merging `feat/channel-scope` (`timcash/dialtone-task`) and `feat/channel-plugin-keys` (`timcash/dialtone`), and rebasing if any additional GB-34 commits land.
3. Approve the `metal` 41-task migration plan (backup `state.redb` on `rosie`/`legion`, enable dual-key transition window, upgrade `dialtone` + `dialtone-task` across all 8 fleet nodes, verify `channel_id` and `state_hash`, then retire legacy `mesh.key`).
4. Approve whether `dialtone-task serve` on fleet nodes should move under `dialtone` daemon supervision (so `DIALTONE_CHANNEL_*` env vars are injected automatically) or remain standalone services reading channel env from a `0600` env file.
5. Approve the `dialtone.channel.manifest.v1` admin key model (creator `EndpointId` as initial admin, `seq`-monotonic gossip propagation) before P3 implementation.
6. Approve the P5 `GET /api/channels` schema for the Mac app.
