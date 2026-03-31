# Robot src_v2 Workflow

This document is the end-to-end operating workflow for `robot src_v2`:
- edit code
- test locally
- publish artifacts
- run/update on robot through autoswap + published release artifacts
- verify with diagnostics
- expose UI through local WSL relay
- run headed UI on `legion` while rover serves APIs and telemetry

Validated WSL no-UI release path:
- `./dialtone.sh robot src_v2 install`
- `./dialtone.sh robot src_v2 build`
- `./dialtone.sh robot src_v2 publish --repo timcash/dialtone`
- `./dialtone.sh autoswap src_v1 update --host rover`
- `./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui --public-check=false`

Validated result from the current WSL workflow:
- all `./dialtone.sh ...` commands above were run through the REPL path from the WSL host
- `robot src_v2 build` produced the local runtime binaries under `bin/plugins/<plugin>/<src_version>/...`
- `robot src_v2 diagnostic --skip-ui --public-check=false` passed against `rover`

## 1) CLI & REPL Interaction Guide

`robot src_v2` embraces a task-first REPL execution model. When you run `./dialtone.sh robot src_v2 <command>`, the CLI routes the request through the local REPL leader and returns a managed task identity instead of streaming the full worker output in the foreground.

**The Default Operator Path:**
For 99% of your daily workflow, just use plain `dialtone.sh` commands. The framework handles the REPL routing automatically.

```bash
# Good: the command is automatically routed through the default REPL leader
./dialtone.sh robot src_v2 dev
```

**What to expect in the terminal:**
1. A brief `dialtone>` receipt acknowledging the request.
2. A queued `task-id`.
3. The task topic and permanent task log path.
4. Short operator-facing status lines when appropriate.
5. Detailed worker output in the task log instead of the shell transcript.

**Advanced REPL Inspection:**
Because your commands run as REPL-managed tasks, you can inspect their lifecycle and logs with the `repl src_v3` tools:

```bash
# List recent completed or running tasks
./dialtone.sh repl src_v3 task list --count 20

# Inspect one queued/running task
./dialtone.sh repl src_v3 task show --task-id <task-id>

# View the detailed task log
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200

# Watch the raw underlying NATS traffic for the REPL topic space
./dialtone.sh repl src_v3 watch --subject 'repl.>'
```

**Explicit Remote Injection:**
By default, commands are routed using the local environment settings in `env/dialtone.json`. If you intentionally need to target a remote robot's REPL leader directly, you can bypass the automatic local routing and inject explicitly:

```bash
./dialtone.sh repl src_v3 inject --nats-url nats://rover-1.shad-artichoke.ts.net:4222 --user llm-codex robot src_v2 diagnostic
```

*(Note: Most `robot` commands support a `--host` flag for targeting the remote node via SSH/Mesh, so explicit injection is rarely needed unless testing REPL core behavior).*

---

## 2) Architecture Contract

```sh
# From repo root
cd /home/user/dialtone

# 1. Build and run the local dev UI, but use rover for API + telemetry
./dialtone.sh robot src_v2 build
./dialtone.sh robot src_v2 dev \
  --browser-node legion \
  --public-url http://127.0.0.1:3000 \
  --backend-url http://rover-1:18086

# 2. Drive the long-lived chrome src_v3 session on legion
./dialtone.sh chrome src_v3 status --host legion --role robot-test
./dialtone.sh chrome src_v3 goto --host legion --role robot-test --url http://127.0.0.1:3000/#robot-three-stage
./dialtone.sh chrome src_v3 click-aria --host legion --role robot-test --label "Three Mode"
./dialtone.sh chrome src_v3 click-aria --host legion --role robot-test --label "Three Thumb 1"
./dialtone.sh chrome src_v3 wait-log --host legion --role robot-test --contains "Publishing rover.command cmd=arm" --timeout-ms 5000

# 3. Run the integrated robot src_v2 test suite
./dialtone.sh robot src_v2 test

# 4. Run focused section-by-section browser steps through the Go test/src_v1 suite
./dialtone.sh robot src_v2 test --filter ui-three-buttons
./dialtone.sh robot src_v2 test --filter ui-terminal-routing-and-buttons
./dialtone.sh robot src_v2 test --filter ui-video-buttons

# 6. Inspect generated report + screenshots
sed -n '1,220p' src/plugins/robot/src_v2/TEST.md
ls -l src/plugins/robot/src_v2/test/screenshots
```

```bash
# From repo root: /home/user/dialtone

# Core local workflow
./dialtone.sh robot src_v2 install
./dialtone.sh robot src_v2 format
./dialtone.sh robot src_v2 lint
./dialtone.sh robot src_v2 build
./dialtone.sh robot src_v2 test

# Dev server (local)
./dialtone.sh robot src_v2 dev

# Dev server + headed browser on mesh node + rover backend proxy routes
./dialtone.sh robot src_v2 dev --browser-node chroma --backend-url http://rover-1:18086

# Dev server on this WSL node, headed browser on legion, real rover backend
./dialtone.sh robot src_v2 dev --browser-node legion --public-url http://127.0.0.1:3000 --backend-url https://rover-1.dialtone.earth

# Walk the live robot UI menu in the headed browser at one action per second
src/plugins/robot/src_v2/ui/demo_menu_walkthrough.sh --host legion --url http://127.0.0.1:3000 --apm 60

# Use the long-lived chrome src_v3 service on legion directly
./dialtone.sh chrome src_v3 status --host legion --role robot-test
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role robot-test --label "Xterm Terminal" --attr data-last-command-ack-result
```

Notes:
- `robot src_v2 install` uses plugin dependencies instead of duplicating tool setup:
  - `camera src_v1 install`
  - `github src_v1 install`
  - `bun src_v1 install`
- `camera src_v1 install` provisions managed Go and managed Zig from the shared caches configured in `env/dialtone.json`.
- `robot src_v2 build` now produces both the UI dist and the local binary set used by `robot src_v2 diagnostic`.
- On WSL, the camera Linux ARM64 publish artifact is built from the main WSL instance with the managed Zig toolchain; podman is not required for the normal robot target.

Binary layout contract:
- local host binaries now live under `bin/plugins/<plugin>/<src_version>/<binary>`
- current robot build outputs are:
  - `bin/plugins/autoswap/src_v1/dialtone_autoswap_v1`
  - `bin/plugins/robot/src_v2/dialtone_robot_v2`
  - `bin/plugins/camera/src_v1/dialtone_camera_v1`
  - `bin/plugins/mavlink/src_v1/dialtone_mavlink_v1`
  - `bin/plugins/repl/src_v1/dialtone_repl_v1`
- treat those paths as the canonical repo-local runtime artifacts for local testing and diagnostics

```bash
# Default REPL-routed publish/update/diagnostic workflow from WSL

# 0. Optional: watch raw REPL traffic while the plain commands run
./dialtone.sh repl src_v3 watch --subject 'repl.>'

# 1. Publish release artifacts to GitHub
./dialtone.sh robot src_v2 publish --repo timcash/dialtone

# 2. Install/update autoswap runtime on robot (one-time/bootstrap path)
./dialtone.sh autoswap src_v1 deploy --host rover --service --repo timcash/dialtone

# 3. Force immediate rover poll instead of waiting for autoswap's interval
./dialtone.sh autoswap src_v1 update --host rover

# 4. Validate runtime and UI integration
./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui --public-check=false

# 5. Public relay from WSL to robot UI
./dialtone.sh robot src_v2 relay --subdomain rover-1 --robot-ui-url http://rover-1:18086 --service

# 6. Cleanup/reset robot host
./dialtone.sh robot src_v2 clean --host rover
```

When you need to pin a specific leader explicitly:

```bash
./dialtone.sh repl src_v3 inject --nats-url nats://127.0.0.1:47222 --user llm-codex robot src_v2 publish --repo timcash/dialtone
./dialtone.sh repl src_v3 inject --nats-url nats://127.0.0.1:47222 --user llm-codex autoswap src_v1 update --host rover
./dialtone.sh repl src_v3 inject --nats-url nats://127.0.0.1:47222 --user llm-codex robot src_v2 diagnostic --host rover --skip-ui --public-check=false
```

## 1) Architecture Contract

`robot src_v2` runtime is autoswap-managed and artifact-executed:
- `dialtone_autoswap_v1` (only OS service)
- `dialtone_robot_v2`
- `dialtone_camera_v1`
- `dialtone_mavlink_v1`
- `dialtone_repl_v1`
- `robot_src_v2_ui_dist` (directory artifact extracted from release tarball)

Primary manifest:
- `src/plugins/robot/src_v2/config/composition.manifest.json`

Important rule:
- Robot runtime comes from release artifacts downloaded by autoswap. The rover does not need to build the runtime from source during normal updates.
- Headed UI tests should prefer one long-lived `chrome src_v3` session on `legion` and one managed tab. Recreate the tab only as a recovery path.

## 2) Prerequisites

Run from repo root on WSL node:
- `/home/user/dialtone`

Required tools:
- `go`, `bun`
- GitHub release auth via `GH_TOKEN` or `GITHUB_TOKEN` in `env/dialtone.json`
- `gh` will be auto-installed into the managed dependency home when needed
- SSH mesh connectivity to robot (`rover` alias) via `env/dialtone.json`
- Robot host autoswap service must have `GITHUB_TOKEN` set (to avoid GitHub API rate limits when pulling release artifacts)

Useful mesh host aliases are documented in:
- `plan/SSH_MESH_NODES.md`

## 3) Dev Server (`robot src_v2 dev`)

Use `dev` to run the Vite UI on this WSL node, optionally open a headed browser on a mesh node, and optionally proxy backend routes to rover.

Command shape:
```bash
./dialtone.sh robot src_v2 dev \
  [--host 0.0.0.0] \
  [--port 3000] \
  [--browser-node chroma] \
  [--public-url http://legion-wsl-1.shad-artichoke.ts.net:3000] \
  [--backend-url http://rover-1:18086] \
  [--live]
```

Flags:
- `--host`: Vite bind host (default `0.0.0.0`)
- `--port`: Vite bind port (default `3000`)
- `--browser-node`: mesh node for headed browser (example `chroma`)
- `--public-url`: URL opened by remote browser (auto-inferred if omitted)
- `--backend-url`: shared proxy target for `/api`, `/stream`, `/natsws`, `/ws`
- `--live`: automatically pull the `rover` node IP from `env/dialtone.json` and proxy backend routes to it.

Common dev flows:
```bash
# Local dev only
./dialtone.sh robot src_v2 dev

# Dev + browser on the paired Windows node (usually legion on WSL)
./dialtone.sh robot src_v2 dev --browser-node legion

# Zero-config live robot connection (proxies to rover and opens UI)
./dialtone.sh robot src_v2 dev --live
```

# Dev + browser on chroma + backend routes proxied to rover
./dialtone.sh robot src_v2 dev --browser-node chroma --backend-url http://rover-1:18086

# Dev + browser on legion + backend routes proxied to the real rover public UI
./dialtone.sh robot src_v2 dev --browser-node legion --public-url http://127.0.0.1:3000 --backend-url https://rover-1.dialtone.earth

# Dev + browser on legion + backend routes proxied directly to rover on tailnet
./dialtone.sh robot src_v2 dev --browser-node legion --public-url http://127.0.0.1:3000 --backend-url http://rover-1:18086

# One-off backend proxy override stays on the command line
./dialtone.sh robot src_v2 dev --browser-node legion --backend-url http://rover-1:18086
```

Notes:
- If `--backend-url` omits a port (example `http://rover-1`), it targets port `80`.
- For rover runtime, use `http://rover-1:18086` to avoid `ECONNREFUSED` on proxied API/NATS routes.
- For the live headed browser on `legion`, use `--public-url http://127.0.0.1:3000` so the Windows Chrome session opens the WSL-forwarded dev port on the same machine.
- `robot src_v2 test` defaults to attach to `legion` as role `robot-test` when running from WSL.
- The current passing workflow is: local dev server on WSL, remote headed Chrome on `legion`, rover backend on `http://rover-1:18086`.
- Stop dev with `Ctrl+C`.

## 4) Local Dev + Test Loop

1. Build UI and binaries (local):
```bash
./dialtone.sh robot src_v2 build
./dialtone.sh go src_v1 exec build -o ../bin/plugins/autoswap/src_v1/dialtone_autoswap_v1 ./plugins/autoswap/src_v1/cmd/main.go
./dialtone.sh go src_v1 exec build -o ../bin/plugins/robot/src_v2/dialtone_robot_v2 ./plugins/robot/src_v2/cmd/server/main.go
./dialtone.sh camera src_v1 build --goos linux --goarch amd64 --out ../bin/plugins/camera/src_v1/dialtone_camera_v1 --podman=false
./dialtone.sh go src_v1 exec build -o ../bin/plugins/mavlink/src_v1/dialtone_mavlink_v1 ./plugins/mavlink/src_v1/cmd/main.go
./dialtone.sh go src_v1 exec build -o ../bin/plugins/repl/src_v1/dialtone_repl_v1 ./plugins/repl/src_v1/cmd/repld/main.go
```

2. Run robot test suite (includes UI mock E2E in test ctx pattern):
```bash
./dialtone.sh robot src_v2 test
```

3. Optional: run with remote browser node when local host has no Chrome:
```bash
./dialtone.sh robot src_v2 test --attach legion
./dialtone.sh robot src_v2 test --default-attach legion
./dialtone.sh robot src_v2 test --attach legion --browser-base-url https://rover-1.dialtone.earth
```

4. Focused UI steps while iterating:
```bash
./dialtone.sh robot src_v2 test --filter ui-terminal-routing-and-buttons
./dialtone.sh robot src_v2 test --filter ui-video-buttons
./dialtone.sh robot src_v2 test --filter ui-settings-and-keyparams
```

5. Dedicated filtered Three/arm flow:
```bash
./dialtone.sh robot src_v2 test --filter ui-three-buttons
```

6. Generated test artifacts:
```bash
src/plugins/robot/src_v2/TEST.md
src/plugins/robot/src_v2/test/screenshots/arm_failure_xterm.png
src/plugins/robot/src_v2/test/screenshots/three_system_arm.png
src/plugins/robot/src_v2/screenshots/auto_04-local-ui-mock-e2e-smoke.png
```

Available integrated test steps:
- `01-build-robot-v2-binary`
- `02-server-health-and-root-behavior`
- `03-manifest-has-required-sync-artifacts`
- `04-ui-section-navigation`
- `05-ui-table-buttons`
- `06-ui-steering-settings-buttons`
- `07-ui-three-buttons-three-system-arm`
- `08-ui-terminal-routing-and-buttons`
- `09-ui-video-buttons`
- `10-ui-settings-and-keyparams`
- `11-autoswap-compose-run-smoke`

The current headed UI suite is organized section by section:
- `04-ui-section-navigation`: menu navigation across all robot sections
- `05-ui-table-buttons`: telemetry table refresh/clear behavior
- `06-ui-steering-settings-buttons`: steering selection and save/reset controls
- `07-ui-three-buttons-three-system-arm`: Drive/System/Guided controls, including arm/disarm and mode changes
- `08-ui-terminal-routing-and-buttons`: unified NATS log routing, MAVLink error validation, filter/tail/command/select controls
- `09-ui-video-buttons`: camera feed switching plus bookmark capture
- `10-ui-settings-and-keyparams`: chatlog toggle plus key-params visibility

The terminal step explicitly validates the arm rejection path:
- publish mock `mavlink.command_ack` + `mavlink.statustext`
- assert terminal attrs:
  - `data-last-command-ack-result=MAV_RESULT_FAILED`
  - `data-last-status-text=Arm: Radio failsafe on`
- navigate to `Telemetry` and confirm the replayed command-ack state reaches the table section too
- save screenshots and include them in `TEST.md`

## 5) UI Log Architecture

The UI should stay thin. The source of truth is the embedded NATS bus exposed by `robot src_v2`.

Current model:
- backend publishes and proxies runtime events over NATS
- UI opens one `/natsws` connection
- UI maintains one shared robot event store
- sections render from shared state instead of owning separate subscriptions
- `Terminal` is the primary debug surface

Current subjects consumed by the UI event store:
- `mavlink.>`
- `camera.>`
- `rover.command`
- `robot.>`
- `logs.ui.robot`

Current backend-generated service subjects:
- `robot.service`
- `robot.autoswap.supervisor`
- `robot.autoswap.runtime`
- `mavlink.stats`

Why this architecture:
- section changes do not drop important events
- `legion` UI and rover backend can live on different machines cleanly
- tests can assert stable DOM attrs driven by structured events
- the terminal can filter one centralized stream instead of stitching multiple local buffers

Current terminal capabilities:
- unified tail of UI, MAVLink, rover commands, robot service, camera, and autoswap state
- filter buttons: `All`, `MAV`, `Cmd`, `UI`, `Cam`, `Svc`, `Err`
- tail pause/resume
- direct command buttons: `Arm`, `Disarm`, `Manual`, `Guided`, `Stop`

Current test/debug attrs on `Xterm Terminal`:
- `data-filter`
- `data-paused`
- `data-total-lines`
- `data-last-log-line`
- `data-last-log-category`
- `data-last-log-level`
- `data-last-error-line`
- `data-last-status-text`
- `data-last-command-ack-result`

Useful chrome debug commands on `legion`:
```bash
./dialtone.sh chrome src_v3 status --host legion --role robot-test
./dialtone.sh chrome src_v3 console --host legion --role robot-test
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role robot-test --label "Xterm Terminal" --attr data-last-status-text
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role robot-test --label "Xterm Terminal" --attr data-last-command-ack-result
./dialtone.sh chrome src_v3 screenshot --host legion --role robot-test --out src/plugins/robot/src_v2/test/screenshots/manual_debug.png
```

## 6) Publish Artifacts (No Deploy Side Effects)

`publish` only builds and uploads changed/missing release assets; it does not deploy remote hosts.
By default it publishes only the real robot target (`linux-arm64`).

Default routed publish command:
```bash
./dialtone.sh robot src_v2 publish --repo timcash/dialtone
```

Notes:
- keep `publish` on the normal REPL-routed `./dialtone.sh robot src_v2 publish ...` path
- if `gh` is missing, the workflow now installs a managed copy under `DIALTONE_ENV`
- release operations still require GitHub auth; set `GH_TOKEN` or `GITHUB_TOKEN` in `env/dialtone.json`
- `publish` already runs `robot src_v2 build` first, so local UI and local robot binaries are refreshed before release assets are assembled
- the validated default publish target is still `linux-arm64`
- the release staging area is separate from the stable local binary paths; local diagnostics should keep using `bin/plugins/...`, while publish assembles versioned assets under the robot release staging directory

Explicit pinned-leader path when you need it:
```bash
./dialtone.sh repl src_v3 inject --nats-url nats://127.0.0.1:47222 --user llm-codex robot src_v2 publish --repo timcash/dialtone
```

Optional fixed version tag:
```bash
./dialtone.sh robot src_v2 publish --repo timcash/dialtone --version <tag>
```

UI-only fast publish (skip binary builds, upload UI dist + refreshed manifest):
```bash
./dialtone.sh robot src_v2 publish --repo timcash/dialtone --ui
```

Publish all OS/arch assets (legacy/full matrix):
```bash
./dialtone.sh robot src_v2 publish --repo timcash/dialtone --all-targets
```

## 7) Bring Robot Up With Autoswap

Set token drop-in once on robot host:
```bash
TOKEN="$(gh auth token)"
./dialtone.sh ssh src_v1 run --host rover --cmd "mkdir -p ~/.config/systemd/user/dialtone_autoswap.service.d && cat > ~/.config/systemd/user/dialtone_autoswap.service.d/10-token.conf <<'EOF'
[Service]
Environment=GITHUB_TOKEN=${TOKEN}
EOF
systemctl --user daemon-reload"
```

Use autoswap deploy helper to install/update autoswap on robot and install service:
```bash
./dialtone.sh autoswap src_v1 deploy \
  --host rover \
  --service \
  --manifest-url https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_channel.json \
  --repo timcash/dialtone
```

Check service + managed runtime:
```bash
./dialtone.sh ssh src_v1 run --host rover --cmd 'systemctl --user status dialtone_autoswap.service --no-pager -l | sed -n "1,40p"'
./dialtone.sh ssh src_v1 run --host rover --cmd 'cat ~/.dialtone/autoswap/state/runtime.json'
./dialtone.sh ssh src_v1 run --host rover --cmd 'cat ~/.dialtone/autoswap/state/supervisor.json'
```

Force immediate update check (instead of waiting poll interval):
```bash
./dialtone.sh autoswap src_v1 update --host rover
```

Normal field update path:
```bash
# 1. Build and publish from WSL
./dialtone.sh robot src_v2 publish --repo timcash/dialtone

# 2. Let autoswap detect it, or force an immediate poll
./dialtone.sh autoswap src_v1 update --host rover

# 3. Verify rover is running downloaded artifacts
./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui --public-check=false
```

Practical note:
- after `autoswap src_v1 update --host rover`, the diagnostic may spend a short time waiting for the active manifest on rover to switch to the newest published release channel before it moves on to the endpoint checks

Optional rover Nix maintenance checks:
```bash
./dialtone.sh robot src_v2 nix-diagnostic --host rover
./dialtone.sh robot src_v2 nix-gc --host rover
```

## 8) Robot Diagnostic (Mandatory)

Run full diagnostic against robot host:
```bash
./dialtone.sh robot src_v2 diagnostic --host rover
```

Common variants:
```bash
./dialtone.sh robot src_v2 diagnostic --host link-local --skip-ui --public-check=false
./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui
./dialtone.sh robot src_v2 diagnostic --host rover --ui-url https://rover-1.dialtone.earth --browser-node chroma
./dialtone.sh robot src_v2 diagnostic --host rover --manifest /home/tim/.dialtone/autoswap/manifests/manifest-<hash>.json
```

Diagnostic checklist details:
- `src/plugins/robot/src_v2/diagnostic.md`

Current validated no-UI result:
- local artifact check passes after `robot src_v2 build`
- rover artifact/service/runtime checks pass after `autoswap src_v1 update --host rover`
- endpoint checks pass with `--skip-ui --public-check=false`
- local artifact checks now resolve the canonical repo-local binaries from `bin/plugins/...`

## 9) WSL Relay for Public UI

Run on WSL host to point tunnel to robot UI:
```bash
./dialtone.sh robot src_v2 relay --subdomain rover-1 --robot-ui-url http://rover-1:18086 --service
```

Default URL:
- `https://rover-1.dialtone.earth`

Verify local relay service:
```bash
systemctl --user status dialtone-proxy-rover-1.service --no-pager
```

## 10) Clean Remote Robot State (Reset)

If robot needs full teardown before re-bootstrap:
```bash
./dialtone.sh robot src_v2 clean --host rover
```

`clean` hard-verifies all of the following:
- repo removed (`~/dialtone`)
- matching `dialtone|robot|rover` services removed/disabled/inactive
- dialtone runtime processes gone
- autoswap binaries/artifacts/releases/manifests removed

## 11) Expected Working State

After publish + autoswap update + diagnostic:
- autoswap service is active on robot
- manifest path is correct and active
- managed processes `robot/camera/mavlink/repl` are running
- robot endpoints work (`/health`, `/api/init`, `/api/integration-health`, `/stream`)
- `/api/integration-health` reports live MAVLink health, not just static configuration:
  - `not-configured`: MAVLink disabled
  - `configured`: enabled but no telemetry received yet
  - `degraded`: telemetry stale
  - `ok`: recent telemetry received
- UI loads and sections/menu work
- telemetry + latency render from real MAVLink flow over `/natsws`
- terminal section shows the same MAVLink failures and command acks visible to other sections
- terminal can filter unified NATS events without losing history when sections change
- WSL relay service is active and public URL serves robot UI

## 12) Troubleshooting Order

1. Mesh reachability:
```bash
./dialtone.sh ssh src_v1 mesh --mode check
```
2. Remote Chrome service on `legion`:
```bash
./dialtone.sh chrome src_v3 status --host legion --role robot-test
```
3. Autoswap status/list on robot.
4. Robot `src_v2 diagnostic --skip-ui`.
5. Check terminal attrs directly on the remote browser when UI tests fail:
```bash
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role robot-test --label "Xterm Terminal" --attr data-last-error-line
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role robot-test --label "Xterm Terminal" --attr data-last-status-text
```

## 13) Deployment Architecture

Current publish flow should be treated as two layers:
- `robot src_v2 publish` creates immutable versioned release assets
- publish also writes `robot_src_v2_channel.json`, a stable channel asset that points to the current immutable manifest asset for that release
- autoswap `deploy` should use the channel URL, not the raw manifest URL
- autoswap resolves that channel, pins the resulting manifest by digest, then syncs assets into `~/.dialtone/autoswap/artifacts`

For a Nix-based rover runtime, the better field-update model is:
1. Keep autoswap as the update agent and supervisor over tailscale WLAN.
2. Publish Nix-backed artifacts alongside the legacy binaries.
3. Put only immutable release selection and digests in the resolved manifest.
4. On rover, autoswap should realize the correct Nix outputs for the resolved manifest version, then execute those store paths directly.
5. Fall back to raw release binaries only when Nix realization is unavailable.

That keeps the mutable part small and network-friendly for mobile rover updates, while keeping the runtime contract immutable once a release is selected.
