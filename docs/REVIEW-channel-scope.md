# Channel-Scoped Plugins, Blobs, Agent Workers, and Logs: Architecture Review and Design

- Task ID: `t-1a21a7ada23-434d905d`
- Owner: Tim | Orchestrator: Grok Bot
- Requested agent spec: provider `openai`, model `astra`, effort `high`, node `legion`
- Actual runner: `agy` (`gemini-4-neon-high`, effort high) on `legion` (dispatched by `dialtone-agent` on `legion`)
- Audited commits:
  - `timcash/dialtone`: `origin/main` at `f8f1dcb1` (`v0.5.6`)
  - `timcash/dialtone-task`: `origin/main` at `9cd8e1c` (includes `v0.3.0` `2d6c8c9`, `origin/feat/scaling` `f653cf1`, and GB-34 `origin/feat/agent-spec` `9cd8e1c`)
  - `timcash/dialtone-agent`: `origin/main` at `10c4db7` (includes GB-34 `origin/feat/agent-spec` `10c4db7`)
  - Sibling plugin inspected read-only on `legion`: `dialtone-shell`
- Base branch and GB-34 coordination note:
  - GB-34 (`origin/feat/agent-spec`) is merged and pushed on `origin/main` in both `timcash/dialtone-task` (`9cd8e1c`, containing `v0.3.0` `2d6c8c9`) and `timcash/dialtone-agent` (`10c4db7`).
  - `dialtone-task` `feat/channel-scope` is based on `origin/main` (`9cd8e1c`). `dialtone-agent` `feat/channel-scope` is based on `origin/main` (`10c4db7`). `dialtone` `feat/channel-plugin-keys` is based on `origin/main` (`f8f1dcb1`).

---

## 1. Target Hierarchy

The target model makes the channel the single unit of network membership, authorization, plugin installation, and data isolation:

1. Anybody can install `dialtone`.
2. A node joins a channel with a link (invite or secret) or creates a new channel. **Channel = network.**
3. The channel declares which plugins are permitted (`agent`, `shell`, `task`, etc.).
4. A node installs permitted plugins onto that channel.
5. Every channel includes a **Blob store**, **Channel Logs**, and the **gossip/mesh** transport as built-ins. ALL data (tasks, logs, code, blobs) is visible only inside that channel. A peer outside the channel cannot list or fetch it, even if it knows a content hash.

---

## 2. Current State (Verified Codebase Audit)

### 2.1 `timcash/dialtone` (`v0.5.6`, `f8f1dcb1`)

#### Channel identity, KDF, and config (`src/mesh.rs`, `src/join.rs`, `src/main.rs`)
- **Single-channel config**: `MeshConfig` (`src/mesh.rs:880-902`) stores only ONE active channel per `~/.dialtone/config.json`: `channel`, `configured`, `derived_keys` (`kdf_version`, `topic_id_hex`, `auth_key_hex`, `enc_key_hex`), `channel_fingerprint`, `seeds`, `allowlist`, `used_invite_nonces`, `invites`, `processes`.
- **Key derivation**: `derive_channel_keys_v2(channel, secret)` (`src/mesh.rs:326-385`) derives a 32-byte master key via Argon2id (`m=19456 KiB, t=2, p=1`, salt = `SHA256("dialtone.kdf.salt.v2:" || channel)[..16]`), then expands three domain-separated 32-byte subkeys via SHA-256 (`topic_id`, `auth_key`, `enc_key`). The raw channel secret is never serialized to disk (`secret_key` has `#[serde(skip_serializing)]`, `src/mesh.rs:888-889`).
- **Channel fingerprint**: `ChannelKeys::fingerprint()` (`src/mesh.rs:241-244`) returns `ch-<first 12 hex chars of topic_id>`.
- **Gossip encryption & auth**: `seal_message` / `open_message` (`src/mesh.rs:1774-1848`) seal gossip messages with `XChaCha20-Poly1305` using `enc_key` and authenticate senders via Ed25519 signatures verified against the sender's `EndpointId`.
- **Invites and join**:
  - `OpenInvite` v2 (`src/mesh.rs:701-796`): carries `channel`, `expires_at_unix`, `nonce`, `seeds`, and a MAC keyed by `auth_key`. No keys are in the invite string.
  - `dialtone/join/1` (`src/join.rs:21-178`): `Joins::answer()` validates the `OpenInvite` MAC on open channels (empty `allowlist`) and replies with `JoinAnswer { channel, derived_keys, seeds }`.
  - `join_with_invite` (`src/main.rs:778-815`) and `join` (`src/main.rs:2443-2575`) overwrite the single global `~/.dialtone/config.json`.

#### Plugin system (`src/plugin.rs`, `src/main.rs`, `src/api.rs`)
- **Schema**: `PluginManifest` (`src/plugin.rs:49-68`, schema `"dialtone.plugin.v1"` at L20) stores `name`, `version`, `kind` (`builtin-protocol`, `blob-binary`, `blob-bundle`), `protocols`, `blob_digest` (SHA-256), `blake3_hash`, `bytes`, `entrypoint`, `ticket_uri`, `source_repo`, `installed_at_unix`.
- **Global install directory**: `default_plugins_dir()` (`src/plugin.rs:99-101`) resolves to `<state_dir>/plugins` (`~/.dialtone/plugins/<name>/`). `plugins_dir_for_store()` (`src/plugin.rs:103-108`) resolves to `<store_dir>/../plugins`. Neither is channel-scoped.
- **No channel plugin allowlist**: `install_builtin_plugin`, `install_plugin_from_file_with_repo`, `install_plugin_from_repo`, `install_plugin_from_ticket` (`src/plugin.rs:173-483`), and `POST /api/plugins/install` (`src/api.rs:1560-1623`) accept any valid plugin name without checking any channel policy.
- **Call-time CLI dispatch**: On `v0.5.6`, `build_plugin_command()` (`src/plugin.rs:885-929`) and `main.rs` `"plugin-dispatch"` (`src/main.rs:2612-2642`) stripped `DIALTONE_SECRET*` and injected only `DIALTONE_HOME`, `DIALTONE_STATE_DIR`, `DIALTONE_API`, `DIALTONE_NODE`, `DIALTONE_PLUGIN_NAME`, and `DIALTONE_PLUGIN_DIR`. (Implemented in P2 on `feat/channel-plugin-keys` to also inject `DIALTONE_CHANNEL_ID`, `DIALTONE_CHANNEL_PLUGIN_KEY`, and `DIALTONE_CHANNEL_DATA_DIR`.)

#### Blob store, access control, and >= 3 MB share bug (`src/blob.rs`, `src/node.rs`, `src/actors.rs`, `src/main.rs`, `src/api.rs`)
- **Single store per node**: `default_store_dir()` (`src/mesh.rs:904-906`) opens one `iroh_blobs::FsStore` at `~/.dialtone/blobs/blobs.db` (`src/blob.rs:116-161`).
- **Addressing**: Blobs are indexed by `iroh-blobs` BLAKE3 hash and tagged by SHA-256 (`sha256/<hex> -> blake3_hash`, `src/blob.rs:49-67`, L234-261). `Store::share_blob` (`src/blob.rs:601-618`) returns `iroh-blobs://ticket/<BlobTicket>#sha256:<hex>`.
- **Network serving and ACL gap**: On `v0.5.6`, `src/main.rs` mounted `iroh_blobs::BlobsProtocol` directly without per-channel ACL, and `Mesh::on_control(JoinMesh)` (`src/actors.rs:1854-1901`) did not clear `ctx.members` on channel switch. (Minimal ACL `ChannelBlobsProtocol` and `members.clear()` + `disconnect_unauthorized()` implemented in P4 on `feat/channel-plugin-keys`.)
- **Root cause of the `dialtone blob share` >= 3 MB bug (v0.3.0 rollout incident)**:
  1. **Whole-file read + Base64 JSON bloat**: `dialtone blob share <file>` (`src/main.rs:1716-1738`) calls `std::fs::read(&path)`, Base64-encodes the entire file (`+33.3%` expansion), wraps it in `{"bytes_base64": "..."}`, and POSTs it to `POST /api/blob/share` via `local_daemon_http_json` (`src/main.rs:703-748`).
  2. **Hard 2 MiB HTTP body cap**: `src/api.rs:30` sets `pub const MAX_HTTP_BODY_BYTES: usize = 2 * 1024 * 1024;` (2,097,152 bytes). Any raw file larger than ~1.50 MiB produces a Base64 JSON body exceeding 2 MiB, which `read_request()` (`src/api.rs:1035-1042`) rejects with `413 Payload Too Large` immediately upon reading headers and closes the socket without draining the body.
  3. **Write-side socket reset / dropped error + 2s I/O timeout**: Because the daemon closes the TCP socket immediately after writing `413` while `local_daemon_http_json` (`src/main.rs:730-732`) is still blocked in `stream.write_all(req.as_bytes())` pushing 3+ MB into the loopback socket buffer, the CLI hits `ConnectionReset` / `BrokenPipe` before reading the `413` JSON response (and under slow I/O, `HTTP_IO_TIMEOUT = Duration::from_secs(2)` at `src/api.rs:32` also aborts mid-body). That is why the v0.3.0 rollout had to chunk binaries at 1.5 MB and saw >= 3 MB shares fail or drop.
  4. **Whole-file memory buffers and short pull timeout in `src/blob.rs`**:
     - `Blobs::put` (`src/blob.rs:234-261`) takes `&[u8]` and uses `add_slice(bytes)` instead of streaming `add_path`.
     - `Blobs::adopt` (`src/blob.rs:264-289`) and `Blobs::get` (`src/blob.rs:292-312`) call `get_bytes(hash)`, pulling the entire blob into RAM to compute SHA-256 instead of streaming via `store.blobs().reader(hash)`.
     - `Store::pull_blob_from` (`src/blob.rs:684-685`) hardcodes a `15s` download timeout regardless of file size.

#### Built-in `shell` plugin and node logs (`src/shell.rs`, `src/api.rs`, `src/record.rs`)
- `ShellHandler` (`src/main.rs:1257-1259`) checks global `~/.dialtone/plugins/shell` rather than channel manifest allowlist and uses one process-wide `ShellManager`.
- `NodeState.logs` (`src/api.rs:175, 260-280`) and `NodeState.events` (`src/api.rs:174, 282-299`) are ephemeral, local-only in-memory ring buffers (256 items max). There is no durable, gossip-synced, encrypted, channel-scoped log (`dialtone log`).

### 2.2 `timcash/dialtone-task` (`9cd8e1c`, includes `v0.3.0` + GB-34)

- **Separate network & random key (before P1)**: `dialtone-task` ran its own `iroh::Endpoint` with a random `<data_dir>/mesh.key` (`src/config.rs`), `derive_topic_id(&mesh_key)`, and `JoinTicket` carrying raw hex `mesh_key`. (P1 on `feat/channel-scope` now derives `mesh_key` and `TopicId` from `DIALTONE_CHANNEL_PLUGIN_KEY` and isolates `<data_dir>` per channel while preserving legacy fallback.)
- **Task logs (GB-34 `src/logs.rs`, `src/main.rs`)**: `dialtone-task logs <task_id>` pulls per-task gzipped stdout/stderr chunks from `<data_dir>/blobs/<blake3>.z` over `dialtone/task/1`. It does not yet support `dialtone-task logs --channel <id>` to inspect task-related events from the built-in channel log.

### 2.3 `timcash/dialtone-agent` (`10c4db7`, includes GB-34)

- **Single global task dir & RPC (`src/config.rs:42-95`, `src/task_client.rs:221-298`, `src/main.rs:106-180`)**:
  - `default_agent_dir()` defaults to `DIALTONE_AGENT_DIR` / `DIALTONE_PLUGIN_DIR` / `~/.dialtone/plugins/agent`.
  - `default_task_dir()` hardcodes `DIALTONE_TASK_DIR` / `~/.dialtone/plugins/task`.
  - `AgentConfig` (`src/config.rs:120-144`) has NO `channels` opt-in list and NO per-channel task RPC or data-dir mapping.
- **No channel authorization check (`src/worker.rs:138-369`)**:
  - `AgentWorker::run_once()` polls a single `TaskClient` unconditionally and never checks whether the node has joined the channel or whether the channel's manifest authorizes the `"agent"` plugin.
- **Global workspace and logs (`src/worker.rs:526-527`)**:
  - Workspaces and logs go to `<agent_dir>/work/<task_id>` and `<agent_dir>/logs/<task_id>` rather than per-channel `<channel_data_dir>/agent/work/<task_id>`.
- **Subprocess environment (`src/runner.rs:440-468`)**:
  - Passes `DIALTONE_AGENT_DIR` and `DIALTONE_TASK_DIR` and strips `DIALTONE_SECRET*`, but does not forward `DIALTONE_CHANNEL_ID` / `DIALTONE_CHANNEL_DATA_DIR` or strip `DIALTONE_CHANNEL_PLUGIN_KEY` from spawned coding CLIs.

---

## 3. Gaps vs. the 5-Step Hierarchy

| Component | Step 1-2: Channel = Network | Step 3: Channel Plugin Allowlist | Step 4: Per-Channel Plugin Install | Step 5: Channel-Only Data, Blobs & Logs |
|---|---|---|---|---|
| **dialtone daemon (gossip/mesh)** | Single channel per `config.json`; multi-channel join designed. | None yet. Signed `dialtone.channel.manifest.v1` designed. | Global `~/.dialtone/plugins/<name>/` today; per-channel helpers added. | `JoinMesh` now clears `ctx.members` + disconnects unauthorized blob conns. |
| **Built-in Blob Store** | Single `~/.dialtone/blobs` on v0.5.6; per-channel dir helpers added. | N/A (built-in). | N/A (built-in). | v0.5.6 lacked ACL and failed on >= 1.5-3 MB via 2 MiB base64 JSON cap. Fixed via `ChannelBlobsProtocol` + streamed `file_path` import + streaming SHA-256 verification. |
| **Built-in Channel Logs** | Did not exist (only ephemeral local 256-entry `NodeState.logs`). | N/A (built-in). | N/A (built-in). | Needs encrypted, signed, gossip-synced append-only CRDT log per channel (`dialtone log`), redaction, and retention caps. |
| **`dialtone-task`** | Legacy random `mesh.key` replaced by `DIALTONE_CHANNEL_PLUGIN_KEY` derivation (with fallback). | Not checked directly (enforced by daemon). | Per-channel data dir implemented. | Tasks/log blobs isolated per channel; needs `dialtone-task logs --channel <id>` reader for channel log. |
| **`dialtone-agent`** | Polled single global task RPC; unaware of channels. | Never checked channel manifest authorization for `agent`. | Global `~/.dialtone/plugins/agent/` and `work/`. | Needs opt-in `channels` list, manifest allowlist enforcement, per-channel task RPC binding, and `<channel_data_dir>/agent/work/<task_id>`. |
| **`dialtone-shell`** | Inherits daemon `AllowlistHook` on `dialtone/shell/1`. | Only checks global `plugins/shell` install, not channel policy. | Global `~/.dialtone/plugins/shell/`. | Sessions live in one process-wide `ShellManager`; not reset per channel. |

---

## 4. Proposed Design

### 4.1 Per-Channel Directory Layout

Replace the flat `~/.dialtone/plugins/` and `~/.dialtone/blobs/` layout with channel-rooted directories keyed by the channel fingerprint (`channel_id`, e.g. `ch-a1b2c3d4e5f6`):

```text
~/.dialtone/
  iroh-secret-key                 # Node Ed25519 identity (shared across channels)
  api-token                       # Local daemon HTTP API token (0600)
  config.json                     # Node settings + active_channel + channels map
  channels/
    <channel_id>/                 # e.g. ch-123456789abc (derived from channel topic_id)
      config.json                 # Channel name, derived_keys, allowlist, seeds
      manifest.json               # Signed channel policy manifest (dialtone.channel.manifest.v1)
      channel-log.jsonl           # Durable append-only CRDT channel log (keyed by hlc, node)
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
          policy.toml
          state.json
          work/<task_id>/         # Per-channel isolated agent workspaces
          logs/<task_id>/         # Per-channel isolated agent run logs
        <other>/
```

**Mapping current `metal` in place**:
- During migration, `~/.dialtone/config.json` continues to work for single-channel nodes.
- The existing `metal` channel's fingerprint `ch-<metal_fp>` maps to `~/.dialtone/channels/<metal_fp>/`.
- Legacy paths (`~/.dialtone/blobs`, `~/.dialtone/plugins/<name>`) remain recognized as fallback when no channel env / `channels/<id>/` config is active.

### 4.2 Daemon-to-Plugin Key Derivation and Environment Contract

When `dialtone` dispatches or supervises a plugin for a channel with active `ChannelKeys`:

1. **Key derivation (never passes raw channel secret)**:
   Using RFC 5869 **HKDF-SHA256** (zero new dependencies in `dialtone`, already depends on `sha2`):
   - `salt = keys.topic_id.as_bytes()` (32 bytes, binds output to channel identity)
   - `ikm = keys.enc_key || keys.auth_key` (64 bytes of Argon2id-derived channel key material)
   - `prk = HMAC-SHA256(key = salt, msg = ikm)`
   - `info = "dialtone/plugin-key/v1/" || canonical_plugin_name`
   - `plugin_key = HMAC-SHA256(key = prk, msg = info || 0x01)` (32 bytes, hex-encoded as 64 lowercase hex chars)
2. **Environment variables passed to plugin processes** (`src/plugin.rs` `build_plugin_command`):
   - Existing: `DIALTONE_HOME`, `DIALTONE_STATE_DIR`, `DIALTONE_API`, `DIALTONE_NODE`, `DIALTONE_PLUGIN_NAME`, `DIALTONE_PLUGIN_DIR`
   - New (set whenever the active channel has `ChannelKeys`):
     - `DIALTONE_CHANNEL_ID`: `keys.fingerprint()` (e.g. `ch-a1b2c3d4e5f6`)
     - `DIALTONE_CHANNEL_PLUGIN_KEY`: 64-char lowercase hex of the 32-byte derived plugin key
     - `DIALTONE_CHANNEL_DATA_DIR`: `<state_dir>/channels/<channel_id>/data/<plugin_name>`
   - Explicitly removed (defense in depth): `DIALTONE_SECRET`, `DIALTONE_SECRET_KEY`, `DIALTONE_GATEWAY_TOKEN`, `MESH_ORIGIN_TOKEN`, plus stale `DIALTONE_CHANNEL_*` vars before re-injection.
3. **How `dialtone-task` consumes the env contract**:
   - If `DIALTONE_CHANNEL_PLUGIN_KEY` is set: `mesh_key = blake3::derive_key("dialtone-task/mesh/v1", &plugin_key_bytes)`, `topic_id = derive_topic_id(&mesh_key)`. Never logged or persisted into `mesh.key`.
   - Data dir precedence: `--data-dir` / `DIALTONE_TASK_DIR` > `DIALTONE_CHANNEL_DATA_DIR` > `<base>/channels/<DIALTONE_CHANNEL_ID>` > legacy `~/.dialtone/plugins/task`.
   - `NodeStatus` exposes `channel_id` (fingerprint only, never the key).

### 4.3 Per-Channel Blob Store Namespace, ACL, and Large-Blob Streaming

To guarantee Step 5 ("A peer outside the channel cannot list or fetch a blob, even if it knows a hash") and eliminate the >= 3 MB blob share failure:

1. **Storage isolation (per-channel `FsStore`)**:
   - Each channel owns its own `Blobs` store directory at `~/.dialtone/channels/<channel_id>/blobs/`.
   - `Blobs::list_digests()`, `has()`, `get()`, and `summary()` only see blobs in that channel's store.
2. **Transport & protocol ACL (`ChannelBlobsProtocol`)**:
   - Wraps `iroh_blobs::BlobsProtocol` and checks `admits(&remote_id)` on every incoming QUIC connection (allowlist when non-empty, else verified gossip `members`).
   - Tracks weak connection handles so `Mesh::on_control(JoinMesh)` calls `members.clear()` + `blobs_proto.disconnect_unauthorized()`, revoking pooled QUIC connections immediately on channel switch.
   - Multi-channel ALPN routing (P4b): register `dialtone/blobs/v1/<channel_id>` per channel when a single daemon hosts multiple channels concurrently.
   - Optional encryption at rest (P4b): `XChaCha20-Poly1305` keyed via `HKDF-SHA256(enc_key, salt=topic_id, info="dialtone/blob-at-rest/v1")` applied before `FsStore` ingestion so verified BLAKE3 streaming still works natively over ciphertext.
3. **Large-Blob Streaming (fix for the >= 3 MB share bug)**:
   - **Streamed file ingestion (`Blobs::put_file`, `Store::share_blob_file`)**:
     - Computes SHA-256 in 64 KiB streaming chunks via `digest_of_file(path)` (zero whole-file RAM copy).
     - Imports directly into `iroh_blobs::FsStore` via `store.blobs().add_path(path).with_named_tag(sha256_tag(&digest))` (`ImportMode::Copy`), streaming verified BLAKE3 outboard trees straight from disk.
   - **Streamed download verification (`Blobs::adopt_streaming`)**:
     - Replaces `store.blobs().get_bytes(hash)` in `Store::pull_blob_from` with `store.blobs().reader(hash)` (an `AsyncRead` stream hashed in 64 KiB chunks), verifying SHA-256 and tagging without allocating a whole-file `Vec<u8>`.
     - Also adds `Blobs::export_file(digest, dest_path)` backed by `store.blobs().export(hash, dest_path)` for zero-RAM export.
   - **HTTP API (`POST /api/blob/share`) and CLI (`dialtone blob share`)**:
     - `POST /api/blob/share` accepts `{"file_path": "<absolute path>", "digest": "<optional>"}` in addition to `bytes_base64` (kept for small inline payloads <= 1.5 MiB). Because the HTTP request is ~100 bytes of JSON regardless of file size (10 MB, 500 MB, etc.), it completely bypasses `MAX_HTTP_BODY_BYTES` (2 MiB) and Base64 bloat.
     - `dialtone blob share <arg>`: when `<arg>` is an existing file, resolves its absolute path and sends `{"file_path": ...}`.
     - `local_daemon_http_json`: if `write_all` hits `BrokenPipe`/`ConnectionReset` (e.g. a peer still sending oversized `bytes_base64` rejected with `413`), it still reads the server's HTTP response body so `413 Payload Too Large` is reported clearly rather than silently dropped.
     - `Store::pull_blob_from`: raises base download timeout from `15s` to `120s` so multi-megabyte transfers over WAN/relay complete reliably.

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
       { "name": "task", "repo": "timcash/dialtone-task", "min_version": "0.4.0", "pinned_digests": {} },
       { "name": "shell" },
       { "name": "agent" }
     ],
     "blob_policy": { "encrypt_at_rest": false },
     "signatures": [
       { "signer": "<ed25519_endpoint_id_1>", "sig_hex": "..." }
     ]
   }
   ```
2. **Who can sign & propagation**:
   - Creator's `EndpointId` is the initial sole admin (Ed25519 signature; `seq == 1` also anchored by `auth_key` HMAC for shared-secret channels).
   - Admins publish `seq + 1` updates over the channel's `events` topic and `JoinAnswer` / `dialtone/fetch/1`. Nodes persist the highest valid `seq` at `~/.dialtone/channels/<channel_id>/manifest.json` and reject rollbacks or bad signatures.
3. **Fail-closed enforcement**:
   - `dialtone install <plugin>`, `dispatch_plugin()`, `ShellHandler`, and `dialtone-agent` all check `manifest.allowed_plugins` for the target channel and refuse non-permitted plugins.

### 4.5 Built-in Channel Logs (`dialtone log`)

Every channel includes a built-in, append-only, channel-scoped activity log alongside the blob store and gossip mesh.

1. **Where it lives**:
   - Built-in daemon module in `timcash/dialtone` (`src/channel_log.rs`), persisted per channel at `~/.dialtone/channels/<channel_id>/channel-log.jsonl` (durable append-only WAL loaded into an in-memory CRDT `BTreeMap<(hlc_ms, hlc_seq, author_node_id), ChannelLogEntry>`). Using a compact WAL + `BTreeMap` avoids adding a `redb` dependency to the single-binary `dialtone` daemon while preserving exact `(hlc, node)` CRDT ordering and idempotent merge semantics; high-volume channels (>10k entries) can later swap the backing file for a `redb` table behind the same `ChannelLogStore` API.
2. **Key derivation, topic, encryption, and signatures**:
   - **Encryption key**: `log_key = HKDF-SHA256(ikm = enc_key || auth_key, salt = topic_id, info = "dialtone/channel-log/v1")` (32 bytes).
   - **Gossip topic**: `channel_log_topic_id(beats_topic) = SHA256("dialtone.channel.log.v1:" || beats_topic)`.
   - **Wire frame**: `b"dtl1"` (4B magic) + 24B random `XNonce` + `XChaCha20-Poly1305(key = log_key, aad = "dialtone.channel.log.v1:" || channel_id, plaintext = canonical_entry_json)`.
   - **Author signature**: Each `ChannelLogEntry` carries `author_node_id` (Ed25519 `PublicKey`) and `sig_hex` (64-byte Ed25519 signature over canonical fields `(channel_id, hlc_ms, hlc_seq, author_node_id, author_node, source, kind, task_id, level, message)`). Receivers decrypt with `log_key`, verify `entry.channel_id == active_channel_id`, verify the Ed25519 signature against `author_node_id`, and enforce the channel's peer allowlist before merging.
   - **Isolation**: A peer on Channel B derives a different `log_key` and different `TopicId`; even if fed raw Channel A wire frames, AEAD decryption fails immediately.
3. **Entry schema, size caps, retention, and secret redaction**:
   - Fields: `id`, `channel_id`, `hlc_ms`, `hlc_seq`, `author_node_id`, `author_node`, `source` (`daemon`, `task`, `agent`, `plugin`, `join`), `kind` (`task.event`, `agent.run`, `plugin.install`, `member.join`, `member.leave`, etc.), `task_id` (optional), `level`, `message`, `sig_hex`.
   - **Size caps**: Max single entry message = `4 KiB` (clamped on UTF-8 boundary, em-dashes stripped). Max wire frame = `16 KiB`.
   - **Retention**: Bounded ring-buffer retention of `10,000` entries / `16 MiB` per channel; oldest `(hlc_ms, hlc_seq, author_node_id)` entries are evicted and the WAL is compacted atomically on overflow.
   - **Redaction**: All messages are redacted before signing, sealing, or persisting: strips `dt-...` channel secrets, `dialtone-invite-v1/v2:...`, `iroh-blobs://ticket/...`, `dtk-...` API tokens, `Bearer ...`, GitHub/OpenAI/xAI/Google API keys, and `token/secret/key/password=...` assignments.
4. **Read/write interfaces and Mac app view**:
   - **Daemon HTTP API**:
     - `GET /api/channel/log?channel=<id>&source=<src>&task_id=<tid>&since_ms=<ms>&limit=<n>` -> returns ordered entries for the requested channel (rejects mismatched channel IDs).
     - `POST /api/channel/log` (local Bearer-token authenticated via `~/.dialtone/api-token`) -> plugins (`task`, `agent`, etc.) append entries (`source`, `kind`, `task_id`, `level`, `message`), which the daemon redacts, stamps with HLC, signs with the node's Ed25519 key, stores locally, and broadcasts sealed over the channel log gossip topic.
   - **CLI**:
     - `dialtone log [--channel <id>] [--follow] [--json] [--source <src>] [--task-id <tid>] [--limit <n>]` (queries local daemon HTTP API with offline fallback to `<state_dir>/channels/<channel_id>/channel-log.jsonl`).
     - `dialtone-task logs --channel <id> [--tail <n>] [--json]` reads task-related entries (`source == "task"` or `kind` starting with `"task."`) from the channel log via `DIALTONE_API` or local channel log file fallback, while keeping `dialtone-task logs <task_id>` intact for per-task stdout/stderr blobs.
   - **Mac app / HUD (design only)**:
     - Adds a "Channel Log" panel per selected channel in the Mac app, fed by `GET /api/channel/log?channel=<id>&since_ms=<cursor>` (or an SSE stream `/api/channel/log/stream`), with filters for `all | task | agent | plugin | membership`.

### 4.6 Channel-Aware `dialtone-agent`

`dialtone-agent` must only lease and execute tasks from channels that the node has joined AND whose channel manifest authorizes the `agent` plugin:

1. **Opt-in channel config (`AgentConfig`)**:
   - `channels: Vec<String>`: explicit list of `channel_id`s this node opts into (e.g. `["ch-a1b2c3d4e5f6"]`).
   - `channel_targets: BTreeMap<String, ChannelTargetConfig>`: optional per-channel override for `task_dir`, `rpc_addr`, and `agent_data_dir`.
   - **Resolution modes**:
     - **Single channel via env / flag** (`DIALTONE_CHANNEL_ID` or `--channel <id>`): scopes the worker strictly to that channel. If `config.channels` is non-empty, requires `<id>` to be listed in `config.channels`.
     - **Multi-channel coordinator** (`config.channels` non-empty, no single channel override): runs a `MultiChannelAgentCoordinator` polling ONLY the opted-in channels, each with its own isolated `TaskClient` and per-channel directories.
     - **Explicit empty `channels` in multi-channel mode**: claims nothing.
     - **Legacy compatibility mode**: when neither `DIALTONE_CHANNEL_ID` nor `config.channels` is set, retains existing single-task-dir (`~/.dialtone/plugins/task`) behavior so unmigrated nodes continue operating without disruption.
2. **Joined + channel manifest authorization (`ChannelAuthorizer`)**:
   - Before listing or claiming tasks on channel `C`, checks:
     1. The node has joined `C` (`<state_dir>/channels/<C>/config.json` or `manifest.json` exists).
     2. `<state_dir>/channels/<C>/manifest.json` is valid for `C` and includes `"agent"` in `allowed_plugins`.
   - Fail-closed: if not joined, manifest missing/invalid, or `"agent"` not authorized, returns `PollStepOutcome::ChannelNotAuthorized` and claims nothing.
3. **Per-channel data, RPC, and workdir isolation**:
   - Task RPC: resolves channel `C`'s task dir (`channel_targets[C].task_dir` > sibling `../task` of `DIALTONE_CHANNEL_DATA_DIR` > `<state_dir>/channels/<C>/data/task`), reading ONLY that channel's `config.json` (`rpc_addr`) and `rpc.token`.
   - Agent data & work dirs: `<channel_data_dir>/agent/work/<task_id>` and `<channel_data_dir>/agent/logs/<task_id>` (where `<channel_data_dir>` is `<state_dir>/channels/<C>/data`, or `<DIALTONE_CHANNEL_DATA_DIR>/work/<task_id>` when `DIALTONE_CHANNEL_DATA_DIR` already ends in `agent`).
   - Leases, progress notes, task logs, and results are written exclusively to channel `C`'s `TaskClient`.
   - Spawned coding CLI subprocesses receive `DIALTONE_CHANNEL_ID` (when set) and have `DIALTONE_SECRET*` and `DIALTONE_CHANNEL_PLUGIN_KEY` stripped.

### 4.7 Channel Create and Join Flows

- **Create**: `dialtone channel create <name> [--plugins task,shell,agent] [--open|--allowlist]`
  1. Generates secret (`new_secret()`), derives `ChannelKeys` and `channel_id = keys.fingerprint()`.
  2. Creates `~/.dialtone/channels/<channel_id>/{config.json, manifest.json, channel-log.jsonl, blobs/, plugins/, data/}`.
  3. Signs `seq = 1` `ChannelManifest` with this node's Ed25519 key and starts gossip + `ChannelBlobsProtocol` + channel log topic.
- **Join by invite**: `dialtone join <dialtone-invite-v2:...>`
  1. Redeems invite over `dialtone/join/1`, receiving `channel`, `derived_keys`, `seeds`, and signed `ChannelManifest`.
  2. Initializes `~/.dialtone/channels/<channel_id>/`, saves verified `manifest.json`, opens channel-private `Blobs` store and `ChannelLogStore`, and records a `member.join` log entry.
- **Join by secret**: `dialtone join <secret> --channel <name>`
  1. Derives `ChannelKeys`, initializes `~/.dialtone/channels/<channel_id>/`, joins gossip, and pulls the latest signed `ChannelManifest` from peers.

### 4.8 Migration Plan for the 41 Existing `metal` Tasks

The `metal` fleet (`rosie`, `legion`, `build-1`, `vm-us`, `vm-eu`, `vm-asia`, `steamdeck-2`, `grok-bot-metal`) runs `dialtone v0.5.6` and `dialtone-task v0.3.0` with 41 live tasks in `~/.dialtone/plugins/task/state.redb`. Zero task loss is mandatory.

1. **Pre-flight backup**: Snapshot `~/.dialtone/plugins/task/state.redb` and `config.json` on `rosie` and `legion`; record `task_count` and `state_hash`.
2. **Stage 1 (compat deploy)**: Deploy updated `dialtone-task` and `dialtone-agent` with no channel env vars set yet (100% legacy compat).
3. **Stage 2 (preserve state + dual-key bridge)**: Copy `~/.dialtone/plugins/task/{state.redb, config.json, iroh-secret-key, blobs/}` into `~/.dialtone/channels/<metal_fp>/data/task/` and keep `legacy-mesh.key` during transition. Dual-key transition mode seals outbound frames with the channel-derived `mesh_key` while accepting inbound frames decrypting under `legacy-mesh.key` and re-gossiping merged entries under the new key.
4. **Stage 3 (daemon + agent cutover)**: Upgrade `dialtone` daemon across all 8 nodes (injects `DIALTONE_CHANNEL_*`, enables streamed large blobs and channel logs), write signed `metal` manifest authorizing `["task", "shell", "agent"]`, configure `dialtone-agent` with `channels = ["ch-<metal_fp>"]`, and verify identical `state_hash` and `task_count >= 41`.
5. **Stage 4 (close transition window)**: Remove `legacy-mesh.key` on all nodes so old `dialtone-task` tickets stop working permanently.

### 4.9 Ticket and Key Rotation Story & Mixed-Version Risks

- **Key rotation**: Rotating the channel secret produces new Argon2id `derived_keys`, which deterministically rotates every `DIALTONE_CHANNEL_PLUGIN_KEY` and `derive_channel_log_key`. Old tickets and old-key peers immediately fail AEAD verification and move to disjoint gossip topics.
- **Deprecating raw `mesh_key` in `dialtone-task` tickets**: Once running with `DIALTONE_CHANNEL_PLUGIN_KEY`, `dialtone-task` should stop exporting raw `mesh_key` in `JoinTicket` and pull peer addresses from the local `dialtone` daemon API.
- **Mixed-version risks**:
  - Partition risk between `v0.3.0` (`mesh.key`) and channel-derived nodes is mitigated by Stage 2 dual-key bridge.
  - Multi-channel hosts require per-channel `dialtone-task` RPC addresses (dynamic loopback port in `<channel_data_dir>/config.json` or Unix domain socket/named pipe).
  - Direct binary invocation (`launchd`/`systemd`/Scheduled Tasks) keeps legacy fallback until migrated to daemon supervision or a `0600` channel env file.

---

## 5. Phased Implementation Plan

| Phase | Scope | Status in this work |
|---|---|---|
| **P1: `dialtone-task` channel key, data dir & log reader** | Derive `mesh_key` + `TopicId` from `DIALTONE_CHANNEL_PLUGIN_KEY` (`blake3::derive_key("dialtone-task/mesh/v1", key)`); fallback to `mesh.key`; per-channel data dir; `channel_id` in `status`; `dialtone-task logs --channel <id>` channel log reader. | **Implemented** (`dialtone-task` `feat/channel-scope`). |
| **P2: `dialtone` plugin key derivation & env** | HKDF-SHA256 `derive_channel_plugin_key(keys, plugin_name)` (`"dialtone/plugin-key/v1/<plugin>"`); inject `DIALTONE_CHANNEL_ID`, `DIALTONE_CHANNEL_PLUGIN_KEY`, `DIALTONE_CHANNEL_DATA_DIR`; strip secrets; per-channel path helpers. | **Implemented** (`dialtone` `feat/channel-plugin-keys`). |
| **P3: Channel plugin allowlist & signed manifest** | `dialtone.channel.manifest.v1` schema, Ed25519 admin signing, gossip propagation on `events` topic, fail-closed enforcement in `dialtone install`, `dispatch_plugin`, and `ShellHandler`. | Designed in Section 4.4 (manifest reader already enforced by `dialtone-agent` in P5a). |
| **P4: Channel-scoped blob store, ACL & large-blob streaming** | `ChannelBlobsProtocol` membership ACL, `JoinMesh` revocation, `Blobs::list_digests()`, AND streamed `Blobs::put_file` / `adopt_streaming` / `POST /api/blob/share` `file_path` fixing the >= 3 MB drop bug (>= 8 MB two-channel test). Multi-channel ALPN multiplexing (`dialtone/blobs/v1/<channel_id>`) and optional at-rest encryption follow in P4b. | **Minimal ACL + >= 8 MB streaming fix implemented** (`dialtone` `feat/channel-plugin-keys`); P4b designed. |
| **P5a: Channel-aware `dialtone-agent`** | Opt-in `channels` config, joined + channel manifest authorization for `"agent"`, per-channel task RPC binding, `<channel_data_dir>/agent/work/<task_id>` workspaces, `MultiChannelAgentCoordinator`, and cross-channel + unauthorized-channel isolation tests. | **Implemented** (`dialtone-agent` `feat/channel-scope`). |
| **P5b: Built-in Channel Logs** | Daemon module `src/channel_log.rs` (`HKDF-SHA256` `"dialtone/channel-log/v1"`, XChaCha20-Poly1305 + Ed25519 signed gossip, durable CRDT `(hlc, node)` store, redaction, retention caps), `GET/POST /api/channel/log`, `dialtone log`, gossip wiring, and two-channel crypto/gossip isolation tests. | **Implemented** (`dialtone` `feat/channel-plugin-keys`). |
| **P5c: Mac app / HUD channel listing & log view** | Expose `GET /api/channels` returning joined channels and allowed/installed plugins; update Mac app / HUD to list/switch channels and display the live channel log. | Designed (follow-up after P3/P4b). |
| **P6: Migration & fleet rollout (requires Tim's OK)** | Dual-key bridge for `metal` 41-task migration, signed `metal` manifest, service unit updates, coordinated 8-node fleet upgrade, and legacy `mesh.key` retirement. | Designed in Section 4.8; **gated on Tim's explicit OK** (no live changes made). |

---

## 6. Rollout Steps Requiring Tim's Explicit OK

1. Approve frozen KDF labels: `"dialtone/plugin-key/v1/<plugin_name>"` (HKDF-SHA256 in `dialtone`), `"dialtone/channel-log/v1"` (HKDF-SHA256 in `dialtone`), and `"dialtone-task/mesh/v1"` (`blake3::derive_key` in `dialtone-task`).
2. Approve merging all three feature branches (`timcash/dialtone` `feat/channel-plugin-keys`, `timcash/dialtone-task` `feat/channel-scope`, `timcash/dialtone-agent` `feat/channel-scope`).
3. Approve deploying the `dialtone` >= 3 MB blob streaming fix (`file_path` + `add_path` + streaming `adopt`) to unblock unchunked binary rollouts.
4. Approve the `metal` 41-task zero-loss migration plan (backup `state.redb` on `rosie`/`legion`, enable dual-key transition window, seed signed `metal` manifest authorizing `["task", "shell", "agent"]`, upgrade all 8 fleet nodes, verify `channel_id` and identical `state_hash`, then retire legacy `mesh.key`).
5. Approve whether `dialtone-task serve` and `dialtone-agent serve` on fleet nodes should run under `dialtone` daemon supervision (auto-injecting `DIALTONE_CHANNEL_*`) or stay standalone services with per-channel config / `0600` env files.
6. Approve the `dialtone.channel.manifest.v1` admin key model (creator `EndpointId` as initial admin, monotonic `seq`) before P3 daemon enforcement and P5c `GET /api/channels` Mac app work.
