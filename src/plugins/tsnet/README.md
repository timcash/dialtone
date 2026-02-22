# Tsnet Plugin (`src/plugins/tsnet`)

Minimal tsnet plugin in `src_v1` layout.

## Commands

```bash
./dialtone.sh tsnet help
./dialtone.sh tsnet src_v1 config
./dialtone.sh tsnet src_v1 status
./dialtone.sh tsnet src_v1 up --dry-run
./dialtone.sh tsnet src_v1 up
./dialtone.sh tsnet src_v1 devices list --tailnet your-tailnet.ts.net --api-key tskey-api-...
./dialtone.sh tsnet src_v1 devices list --tailnet your-tailnet.ts.net --api-key tskey-api-... --format table
./dialtone.sh tsnet src_v1 devices prune --name-contains drone-1 --dry-run
./dialtone.sh tsnet src_v1 devices prune --name-contains drone-1 --yes
./dialtone.sh tsnet src_v1 computers list --tailnet your-tailnet.ts.net --api-key tskey-api-...
./dialtone.sh tsnet src_v1 list --tailnet your-tailnet.ts.net --api-key tskey-api-...
./dialtone.sh tsnet src_v1 keys provision --tailnet your-tailnet.ts.net --api-key tskey-api-... --description "dialtone robot key" --tags robot,prod --write-env env/.env
./dialtone.sh tsnet src_v1 keys list --tailnet your-tailnet.ts.net --api-key tskey-api-...
./dialtone.sh tsnet src_v1 keys usage --tailnet your-tailnet.ts.net --api-key tskey-api-...
./dialtone.sh tsnet src_v1 keys revoke <key-id> --tailnet your-tailnet.ts.net --api-key tskey-api-...
./dialtone.sh tsnet src_v1 test
```

## Layout

```text
src/plugins/tsnet/
  scaffold/main.go
  src_v1/cli/cli.go
  src_v1/go/tsnet.go
  src_v1/test/cmd/main.go
  src_v1/test/01_self_check/suite.go
  src_v1/test/02_example_library/suite.go
```

## Notes

- Uses `logs` library for plugin logging.
- CLI entrypoint matches the shared plugin pattern (`scaffold/main.go` delegates to `cli.Run`).
- Uses `test` library for self-check suite.
- Tests run in one process via `src_v1/test/cmd/main.go` and `testv1.StepContext`.
- `devices list` (and aliases `computers list`, `list`) lists all computers/devices on the tailnet.
  It uses control-plane API (`TS_API_KEY` + tailnet) when configured, and falls back to local tailscaled status if API creds are missing.
- `devices prune` removes matching devices by substring (`--name-contains`, default `drone-1`); safe by default with `--dry-run`, requires `--yes` to delete.
- If local tailscaled is unavailable (common in WSL), `devices list` can fall back to a temporary embedded ephemeral `tsnet` instance.
- Tailnet defaults to `TS_TAILNET`; if unset, tsnet auto-detects from local tailscaled status (LocalAPI first, then `tailscale status --json` fallback), then falls back to `shad-artichoke.ts.net`.
- `up` starts embedded `tsnet` in ephemeral mode and keeps running until Ctrl+C. If `TS_AUTHKEY` is missing, it auto-provisions one from `TS_API_KEY` and writes it to `env/.env`.
- Key lifecycle commands use Tailscale API v2.
- `keys usage` is an inferred mapping (tags/description/user overlap) because Tailscale does not provide a direct auth-key-to-device attribution field.
- Recommended env vars:
  - `TS_API_KEY` (or `TAILSCALE_API_KEY`)
  - `TS_TAILNET`
  - `TS_AUTHKEY` (set automatically by `keys provision` when `--write-env` is used)
