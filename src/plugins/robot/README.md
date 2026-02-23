# Robot Plugin

The `robot` plugin runs the robot web UI, MAVLink bridge, and embedded NATS runtime.

All commands use the current CLI order:

```bash
./dialtone.sh robot src_v1 <command> [args]
```

## Commands

```bash
# Setup
./dialtone.sh robot src_v1 install
./dialtone.sh robot src_v1 install --remote

# Build
./dialtone.sh robot src_v1 build
./dialtone.sh robot src_v1 build --remote

# Run
./dialtone.sh robot src_v1 serve
./dialtone.sh robot src_v1 serve --remote
./dialtone.sh robot src_v1 dev
./dialtone.sh robot src_v1 dev --robot

# Quality
./dialtone.sh robot src_v1 test
./dialtone.sh robot src_v1 lint
./dialtone.sh robot src_v1 format
./dialtone.sh robot src_v1 fmt
./dialtone.sh robot src_v1 vet
./dialtone.sh robot src_v1 go-build

# Remote/deploy tools
./dialtone.sh robot src_v1 sync-code
./dialtone.sh robot src_v1 deploy
./dialtone.sh robot src_v1 deploy --service
./dialtone.sh robot src_v1 deploy --service --proxy
./dialtone.sh robot src_v1 deploy-test
./dialtone.sh robot src_v1 diagnostic
./dialtone.sh robot src_v1 vpn-test
```

## Remote Native Workflow (recommended)

Use this for fast robot iteration without full deploy:

```bash
./dialtone.sh robot src_v1 install --remote
./dialtone.sh robot src_v1 build --remote
./dialtone.sh robot src_v1 serve --remote
```

What it does:
- Syncs source-only tree to the robot (no `node_modules`/tool cache sync).
- Bootstraps Go/Bun on robot under `DIALTONE_ENV` if missing.
- Builds UI + server on robot.
- Starts remote binary from `plugins/robot/src_v1` (so `/` serves UI correctly).

## Environment

Set in `env/.env`:

```bash
# SSH used by --remote and deploy/sync-code
ROBOT_HOST=192.168.4.36
ROBOT_USER=tim
ROBOT_PASSWORD=...

# Robot network identity
DIALTONE_HOSTNAME=drone-1

# Tailscale auth (robot/tsnet)
ROBOT_TS_AUTHKEY=tskey-auth-...
# fallback if ROBOT_TS_AUTHKEY is unset
TS_AUTHKEY=tskey-auth-...

# MAVLink ingress endpoint
ROBOT_MAVLINK_ENDPOINT=serial:/dev/ttyAMA0:57600
# fallback if ROBOT_MAVLINK_ENDPOINT is unset
MAVLINK_ENDPOINT=serial:/dev/ttyAMA0:57600
```

## Runtime Architecture

- Browser uses NATS WebSocket on same HTTP origin via `/natsws`.
- Robot server exposes:
  - `GET /` UI
  - `GET /health`
  - `GET /api/init`
  - `GET /stream` (camera MJPEG)
  - `POST /api/bookmark`
- Embedded NATS runs locally on robot server process.
- MAVLink is ingested by server and published to `mavlink.>` topics.

## TSNet / Tailnet Behavior

When `ROBOT_TSNET=1` and auth key is present, robot starts embedded tsnet listeners and can serve on tailnet hostnames (for example `http://drone-1`).

If stale `drone-1*` devices accumulate, use tsnet prune:

```bash
./dialtone.sh tsnet src_v1 devices prune --name-contains drone-1 --yes
```

## Version / Update Behavior

UI update checks are automatic and build-based:
- UI runtime version derives from built asset identity.
- Server `api/init.version` derives from built UI assets (or `APP_VERSION` override).
- This avoids false update prompts from `dev` sentinel versions.

## Notes

- `diagnostic` checks LAN first and then tailnet hostname reachability.
- `serve --remote` forwards TSNet and MAVLink environment values from local env into remote startup.
