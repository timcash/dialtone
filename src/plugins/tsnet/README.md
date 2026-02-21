# Tsnet Plugin (`src/plugins/tsnet`)

Minimal tsnet plugin in `src_v1` layout.

## Commands

```bash
./dialtone.sh tsnet help
./dialtone.sh tsnet config src_v1
./dialtone.sh tsnet status src_v1
./dialtone.sh tsnet up src_v1 --dry-run
./dialtone.sh tsnet keys provision src_v1 --tailnet your-tailnet.ts.net --api-key tskey-api-... --description "dialtone robot key" --tags robot,prod --write-env env/.env
./dialtone.sh tsnet keys list src_v1 --tailnet your-tailnet.ts.net --api-key tskey-api-...
./dialtone.sh tsnet keys usage src_v1 --tailnet your-tailnet.ts.net --api-key tskey-api-...
./dialtone.sh tsnet keys revoke src_v1 <key-id> --tailnet your-tailnet.ts.net --api-key tskey-api-...
./dialtone.sh tsnet test src_v1
```

## Layout

```text
src/plugins/tsnet/
  scaffold/main.go
  src_v1/go/tsnet.go
  src_v1/test/cmd/main.go
  src_v1/test/01_self_check/suite.go
  src_v1/test/02_example_library/suite.go
```

## Notes

- Uses `logs` library for plugin logging.
- Uses `test` library for self-check suite.
- Tests run in one process via `src_v1/test/cmd/main.go` and `testv1.StepContext`.
- `up` currently supports `--dry-run` only (safe config validation path).
- Key lifecycle commands use Tailscale API v2.
- `keys usage` is an inferred mapping (tags/description/user overlap) because Tailscale does not provide a direct auth-key-to-device attribution field.
- Recommended env vars:
  - `TS_API_KEY` (or `TAILSCALE_API_KEY`)
  - `TS_TAILNET` (or `TAILSCALE_TAILNET`)
  - `TS_AUTHKEY` (set automatically by `keys provision` when `--write-env` is used)
