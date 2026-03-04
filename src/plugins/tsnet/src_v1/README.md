# Tsnet Plugin (`src/plugins/tsnet/src_v1`)

```bash
# Generic plugin workflow
./dialtone.sh tsnet src_v1 install
./dialtone.sh tsnet src_v1 format
./dialtone.sh tsnet src_v1 lint
./dialtone.sh tsnet src_v1 build
./dialtone.sh tsnet src_v1 test

# Help + introspection
./dialtone.sh tsnet src_v1 help
./dialtone.sh tsnet src_v1 config
./dialtone.sh tsnet src_v1 status

# Embedded tsnet node
./dialtone.sh tsnet src_v1 up --dry-run
./dialtone.sh tsnet src_v1 up

# Devices/computers (same list path)
./dialtone.sh tsnet src_v1 devices list --tailnet <tailnet> --api-key <ts_api_key>
./dialtone.sh tsnet src_v1 devices list --tailnet <tailnet> --api-key <ts_api_key> --all
./dialtone.sh tsnet src_v1 devices list --tailnet <tailnet> --api-key <ts_api_key> --format json
./dialtone.sh tsnet src_v1 computers list --tailnet <tailnet> --api-key <ts_api_key>
./dialtone.sh tsnet src_v1 list --tailnet <tailnet> --api-key <ts_api_key>

# Device cleanup (safe by default)
./dialtone.sh tsnet src_v1 devices prune --name-contains rover --dry-run
./dialtone.sh tsnet src_v1 devices prune --name-contains rover --yes

# Auth key lifecycle
./dialtone.sh tsnet src_v1 keys provision --tailnet <tailnet> --api-key <ts_api_key> --description "dialtone key" --tags dialtone,robot --write-env env/.env
./dialtone.sh tsnet src_v1 keys list --tailnet <tailnet> --api-key <ts_api_key>
./dialtone.sh tsnet src_v1 keys usage --tailnet <tailnet> --api-key <ts_api_key>
./dialtone.sh tsnet src_v1 keys revoke <key_id> --tailnet <tailnet> --api-key <ts_api_key>

# ACL policy
./dialtone.sh tsnet src_v1 acl get --tailnet <tailnet> --api-key <ts_api_key>
./dialtone.sh tsnet src_v1 acl ensure --tailnet <tailnet> --api-key <ts_api_key> --hostname <dialtone-hostname>
```

## What this plugin does

- Runs an embedded `tsnet` node (`up`) for local/agent workflows.
- Lists or prunes devices in your tailnet.
- Provisions/revokes/list auth keys through Tailscale API v2.
- Fetches tailnet ACL policy via Tailscale API v2.
- Falls back to local tailscale status (and then embedded tsnet) when control-plane credentials are missing.

## Environment

- `TS_API_KEY` or `TAILSCALE_API_KEY`: Tailscale API key (needed for control-plane operations).
- `TS_TAILNET`: Tailnet name like `example.ts.net`.
- `TS_AUTHKEY` or `TAILSCALE_AUTHKEY`: auth key used by embedded `tsnet up`.
- `DIALTONE_HOSTNAME`: optional hostname override for embedded node.
- `DIALTONE_TSNET_STATE_DIR`: optional state directory override.
- `DIALTONE_ENV_FILE`: optional env file path used by `keys provision` and auto-provision flow.

Defaults:

- If `TS_TAILNET` is unset, plugin attempts auto-detection from local tailscale status.
- If not detected, tailnet falls back to `shad-artichoke.ts.net`.
- If `TS_AUTHKEY` is missing and `TS_API_KEY` exists, `up` auto-provisions an ephemeral auth key and writes it to `env/.env` (or `DIALTONE_ENV_FILE`).

## Command Reference

### `config`
Prints resolved runtime config.

```bash
./dialtone.sh tsnet src_v1 config
```

### `status`
Prints config plus whether `tailscale` CLI is available.

```bash
./dialtone.sh tsnet src_v1 status
```

### `up [--dry-run]`
Starts embedded tsnet in ephemeral mode and blocks until Ctrl+C.

```bash
./dialtone.sh tsnet src_v1 up --dry-run
./dialtone.sh tsnet src_v1 up
```

### `devices list`
Flags:

- `--tailnet <name>`
- `--api-key <key>`
- `--format report|json` (default `report`)
- `--all` (include inactive)
- `--active-only` (default behavior)

```bash
./dialtone.sh tsnet src_v1 devices list --tailnet shad-artichoke.ts.net --api-key "$TS_API_KEY"
./dialtone.sh tsnet src_v1 devices list --format json --all
```

Aliases:

- `computers list`
- `list`

### `devices prune`
Safeguards:

- defaults to dry-run behavior
- requires `--yes` to actually delete

Flags:

- `--name-contains <substring>` (default `drone-1`)
- `--tailnet <name>`
- `--api-key <key>`
- `--dry-run`
- `--yes`

```bash
./dialtone.sh tsnet src_v1 devices prune --name-contains rover --dry-run
./dialtone.sh tsnet src_v1 devices prune --name-contains rover --yes
```

### `acl get`
Prints the current ACL policy JSON for the target tailnet.

Flags:

- `--tailnet <name>`
- `--api-key <key>`

```bash
./dialtone.sh tsnet src_v1 acl get --tailnet shad-artichoke.ts.net --api-key "$TS_API_KEY"
```

### `acl ensure`
Fetches the current ACL policy and ensures a mosh-access rule exists for the current host.

Flags:

- `--tailnet <name>`
- `--api-key <key>`
- `--hostname <dialtone-hostname>` (default: `DIALTONE_HOSTNAME` or auto-detected host)

```bash
./dialtone.sh tsnet src_v1 acl ensure --tailnet shad-artichoke.ts.net --api-key "$TS_API_KEY" --hostname gold
```

### `keys provision`
Creates auth key and writes it to env file.

Flags:

- `--tailnet <name>`
- `--api-key <key>`
- `--description <text>`
- `--tags t1,t2` (auto-prefixed as `tag:<name>`)
- `--ephemeral` / `--no-ephemeral`
- `--reusable`
- `--preauthorized`
- `--expiry-hours <int>`
- `--write-env <path>`
- `--env-key <name>` (default `TS_AUTHKEY`)

```bash
./dialtone.sh tsnet src_v1 keys provision \
  --tailnet shad-artichoke.ts.net \
  --api-key "$TS_API_KEY" \
  --description "dialtone-tsnet" \
  --tags dialtone,robot \
  --expiry-hours 24 \
  --write-env env/.env
```

### `keys list`

```bash
./dialtone.sh tsnet src_v1 keys list --tailnet shad-artichoke.ts.net --api-key "$TS_API_KEY"
```

### `keys usage`
Returns inferred key-to-device usage (best effort; Tailscale API does not expose direct attribution).

```bash
./dialtone.sh tsnet src_v1 keys usage --tailnet shad-artichoke.ts.net --api-key "$TS_API_KEY"
```

### `keys revoke <key-id>`

```bash
./dialtone.sh tsnet src_v1 keys revoke tskey-abc123 --tailnet shad-artichoke.ts.net --api-key "$TS_API_KEY"
```

## Testing

```bash
./dialtone.sh tsnet src_v1 test
```

## Notes

- `devices list` prefers control-plane API when credentials are present.
- Without API credentials, it falls back to local tailscaled status.
- In environments where local tailscaled is unavailable (common in WSL), fallback can spin up embedded ephemeral tsnet to inspect peers.
