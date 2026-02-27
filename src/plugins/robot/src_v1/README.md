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
./dialtone.sh robot src_v1 sleep
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
./dialtone.sh robot src_v1 deploy --service --relay
./dialtone.sh robot src_v1 deploy --service --relay --smoke-test
./dialtone.sh robot src_v1 wake
./dialtone.sh robot src_v1 deploy-test
./dialtone.sh robot src_v1 diagnostic
./dialtone.sh robot src_v1 vpn-test
```

## Deployment Verification

The `deploy` command performs several checks to ensure reliability:
- **Pre-deployment**: Checks for sufficient remote disk space (100MB+) and accessibility of the MAVLink serial device.
- **Service**: Verifies `dialtone-robot.service` is active and running.
- **Health**: Polls `http://127.0.0.1:8080/health` on the robot until it returns "ok".
- **Smoke Test**: (Optional via `--smoke-test`) Uses a headless browser to verify the UI loads at `drone-1.dialtone.earth`.

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

## LLM Dev Workflow

Use this exact loop when an LLM is iterating on robot code.

```sh
# 0) Start at repo root
cd /home/user/dialtone

# 1) Install local dependencies once (cached by plugin install state)
./dialtone.sh robot src_v1 install

# 2) Fast correctness gate before behavior changes
./dialtone.sh robot src_v1 fmt
./dialtone.sh robot src_v1 vet
./dialtone.sh robot src_v1 build
./dialtone.sh robot src_v1 test

# 3) Local UI/dev loop (WSL relay machine)
./dialtone.sh robot src_v1 dev

# 4) Remote robot loop without full deploy (preferred for rapid iteration)
./dialtone.sh robot src_v1 install --remote
./dialtone.sh robot src_v1 build --remote
./dialtone.sh robot src_v1 serve --remote

# 5) If binary/service swap is required on robot host
./dialtone.sh robot src_v1 deploy --service

# 6) Relay fallback page when robot is unplugged (run on relay/WSL host)
./dialtone.sh robot src_v1 sleep

# 7) Repoint public tunnel back to robot after wake/reconnect (no deploy)
./dialtone.sh robot src_v1 wake
```

LLM guardrails:
- Use `./dialtone.sh robot src_v1 <command>` argument order only.
- Use `sync-code` + `build --remote` + `serve --remote` for most remote validation; avoid `deploy` unless service/binary replacement is needed.
- Keep runtime/log validation through NATS topics; avoid adding file-log-only paths.
- For UI automation/tests: use ARIA selectors and wait for browser/NATS message confirmation after actions.

Sleep command behavior:
- `robot src_v1 sleep` auto-configures Cloudflare proxy service to target local relay sleep URL (`http://127.0.0.1:8080` by default) before starting sleep.
- `robot src_v1 sleep` starts/updates `dialtone-robot-sleep.service` (user systemd daemon) by default.
- Use `robot src_v1 sleep --foreground` to run sleep server in the current terminal instead of daemon mode.
- If system service setup fails (for example no sudo), it falls back to user-mode service setup.

Wake command behavior:
- `robot src_v1 wake` repoints Cloudflare relay tunnel back to robot URL from `ROBOT_HOST` (`http://<ROBOT_HOST>:8080`).
- Use `robot src_v1 wake --host <ip-or-host> --port <port>` to override target host/port.
- Use `robot src_v1 wake --url <full-url>` to set an explicit relay target URL.

## Environment

Set in `env/.env`:

```bash
# SSH used by --remote and deploy/sync-code
ROBOT_HOST=192.168.4.36
ROBOT_USER=tim
ROBOT_PASSWORD=...

# Robot network identity
DIALTONE_HOSTNAME=drone-1

# Optional: override relay sleep server tsnet hostname.
# Default is local machine hostname (recommended on relay/WSL host).
ROBOT_SLEEP_HOSTNAME=legion

# Optional: disable automatic Cloudflare proxy retarget when running
# `./dialtone.sh robot src_v1 sleep` (enabled by default).
ROBOT_SLEEP_CONFIGURE_PROXY=1

# Optional: override proxy target URL for sleep mode.
ROBOT_SLEEP_PROXY_URL=http://127.0.0.1:8080

# Optional: fail sleep command if proxy reconfiguration fails.
# Default behavior is warning + continue.
ROBOT_SLEEP_REQUIRE_PROXY=0

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
