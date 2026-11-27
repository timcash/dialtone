# Channel HTTP API Contract (`GET /api/channels`, `/api/channel/manifest`, `/api/channel/log`)

This document specifies the local daemon HTTP routes exposed by `dialtone` (`v0.6.0+`) on `http://127.0.0.1:8788` for inspecting joined channels, signed channel policy manifests (`dialtone.channel.manifest.v1`), and encrypted channel-scoped logs.

All `GET` routes follow the daemon's standard local HTTP rules (read-only). All `POST` routes require `Authorization: Bearer <token>` from `~/.dialtone/api-token` (`0600`).

---

## 1. `GET /api/channels`

Read-only endpoint returning all joined channels known to this node, their manifest summary, installed plugins, and per-plugin local RPC/data directory info.

**Security invariant:** Only the channel fingerprint (`ch-<12 hex chars>`) is returned. Channel secrets, derived `enc_key` / `auth_key`, plugin keys, and admin private keys are never included.

### Response (`200 OK`)

```json
{
  "schema": "dialtone.channels.v1",
  "node": "rosie",
  "channels": [
    {
      "channel_id": "ch-b39d38af727e",
      "name": "metal",
      "active": true,
      "joined_at_unix": 1795000000,
      "peers": 6,
      "is_admin": true,
      "manifest": {
        "schema": "dialtone.channel.manifest.v1",
        "seq": 3,
        "admin": "<ed25519 endpoint id hex>",
        "allowed_plugins": ["agent", "shell", "task"],
        "signature_valid": true
      },
      "installed_plugins": ["agent", "task"],
      "plugins": {
        "task": {
          "rpc_addr": "127.0.0.1:8794",
          "data_dir": "/Users/x/.dialtone/channels/ch-b39d38af727e/data/task"
        }
      }
    }
  ]
}
```

### Field Notes

- `manifest`: `null` for legacy channels that do not yet have `~/.dialtone/channels/<channel_id>/manifest.json`.
- `is_admin`: `true` when this node holds the Ed25519 key matching `manifest.admin`.
- `peers`: current member count seen on the active channel (excluding this node).
- `plugins`: per-plugin local runtime info (`rpc_addr` when configured/applicable, `data_dir`, plus `pid` and `running` when managed by the daemon via `MeshConfig.managed_plugins`).

### Legacy Fallback (Daemon `v0.5.6`)

Daemons prior to `v0.6.0` return `404` for `/api/channels` and `/api/channel/log`. Clients should fall back to the single active channel in `~/.dialtone/config.json` (`channel` + `channel_fingerprint`) with task RPC `127.0.0.1:8794` and no channel log view. Clients must never scan backup folders.

---

## 2. `GET /api/channel/manifest` and `POST /api/channel/manifest`

### `GET /api/channel/manifest[?channel=<channel_id>]`

Returns the full signed `dialtone.channel.manifest.v1` document (or `null` for legacy channels), along with `is_admin` and `signature_valid`.

```json
{
  "channel_id": "ch-b39d38af727e",
  "is_admin": true,
  "signature_valid": true,
  "manifest": {
    "schema": "dialtone.channel.manifest.v1",
    "channel_id": "ch-b39d38af727e",
    "channel_name": "metal",
    "admin": "<ed25519 endpoint id hex>",
    "seq": 3,
    "allowed_plugins": ["agent", "shell", "task"],
    "issued_at_unix": 1795000000,
    "signature": "<64-byte ed25519 hex>"
  }
}
```

### `POST /api/channel/manifest` (Bearer auth required)

Accepts `{ "manifest": { ... } }` (or a top-level `ChannelManifest`), validates the Ed25519 signature over canonical JSON (excluding `signature`), enforces matching recorded `admin` and strictly increasing `seq`, persists atomically to `~/.dialtone/channels/<channel_id>/manifest.json`, and broadcasts the manifest over the channel's `events` gossip topic. Returns `403` on invalid signature, wrong admin, or stale `seq`.

---

## 3. `GET /api/channel/log` and `POST /api/channel/log`

### `GET /api/channel/log`

Query parameters:

- `channel` (optional): channel ID (`ch-...`) or channel name. Returns `404` if it does not match the active channel.
- `limit` (optional): maximum number of most-recent entries to return.
- `since_hlc` (optional): return entries strictly newer than this HLC timestamp (accepts `<ms>` or `<ms>:<seq>`).
- `since_ms` (optional): alias for millisecond cursor.
- `source` (optional): filter by entry source (`daemon`, `task`, `agent`, `plugin`, `join`).
- `task_id` (optional): filter by `task_id`.

### Response (`200 OK`)

```json
{
  "channel_id": "ch-b39d38af727e",
  "entries": [
    {
      "id": "1795000001000-0-<author_prefix>",
      "channel_id": "ch-b39d38af727e",
      "hlc_ms": 1795000001000,
      "hlc_seq": 0,
      "author_node_id": "<ed25519 endpoint id hex>",
      "author_node": "rosie",
      "source": "daemon",
      "kind": "manifest.legacy_warning",
      "level": "warn",
      "message": "legacy channel 'ch-b39d38af727e' has no signed manifest; ...",
      "sig_hex": "<64-byte ed25519 hex>"
    }
  ]
}
```

### `POST /api/channel/log` (Bearer auth required)

Request body:

```json
{
  "channel_id": "ch-b39d38af727e",
  "source": "task",
  "kind": "task.updated",
  "task_id": "t-...",
  "level": "info",
  "message": "human-readable summary"
}
```

The daemon redacts secrets/tokens/tickets/invites, clamps message length, stamps HLC, signs with the node's Ed25519 identity key, persists to `~/.dialtone/channels/<channel_id>/channel-log.jsonl`, and broadcasts XChaCha20-Poly1305 encrypted over the channel log gossip topic (`dialtone/channel-log/v1`).

---

## 4. Daemon-Managed Plugins (`MeshConfig.managed_plugins`)

By default, `managed_plugins` in `~/.dialtone/config.json` is empty (off) so existing standalone OS services are unaffected during upgrades. When configured (for example `"managed_plugins": ["task", "agent"]`), the daemon starts each listed plugin per joined channel with:

- `DIALTONE_CHANNEL_ID`: channel fingerprint (`ch-...`)
- `DIALTONE_CHANNEL_PLUGIN_KEY`: HKDF-SHA256 derived 32-byte key (`dialtone/plugin-key/v1/<plugin>`)
- `DIALTONE_CHANNEL_DATA_DIR`: `~/.dialtone/channels/<channel_id>/data/<plugin>`

Start and supervision are enforced fail-closed against `manifest.json`: any plugin not listed in `allowed_plugins` is refused (and stopped if already running when a higher-`seq` manifest denies it).
