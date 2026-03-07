# Robot src_v2 Workflow (LLM Agent)

This document is the end-to-end operating workflow for `robot src_v2`:
- edit code
- test locally
- publish artifacts
- run/update on robot through autoswap + published release artifacts
- verify with diagnostics
- expose UI through local WSL relay

```bash
# From repo root: /home/user/dialtone

# Prefer direct-link fallback when WiFi is down:
# ./dialtone.sh robot src_v2 ... --host link-local

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

# Publish release artifacts
./dialtone.sh robot src_v2 publish --repo timcash/dialtone

# Install/update autoswap runtime on robot
./dialtone.sh autoswap src_v1 deploy --host rover --user tim --service --repo timcash/dialtone --manifest-url https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_channel.json

# Build and publish release artifacts from WSL
./dialtone.sh robot src_v2 publish --repo timcash/dialtone

# Force immediate rover poll instead of waiting for autoswap's interval
./dialtone.sh autoswap src_v1 update --host rover --user tim

# Validate runtime and UI integration
./dialtone.sh robot src_v2 diagnostic --host rover --user tim --skip-ui --public-check=false

# Public relay from WSL to robot UI
./dialtone.sh robot src_v2 relay --subdomain rover-1 --robot-ui-url http://rover-1:18086 --service

# Cleanup/reset robot host
./dialtone.sh robot src_v2 clean --host rover --user tim
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

## 2) Prerequisites

Run from repo root on WSL node:
- `/home/user/dialtone`

Required tools:
- `go`, `bun`, `gh` authenticated (`gh auth status`)
- SSH mesh connectivity to robot (`rover` alias)
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
  [--backend-url http://rover-1:18086]
```

Flags:
- `--host`: Vite bind host (default `0.0.0.0`)
- `--port`: Vite bind port (default `3000`)
- `--browser-node`: mesh node for headed browser (example `chroma`)
- `--public-url`: URL opened by remote browser (auto-inferred if omitted)
- `--backend-url`: shared proxy target for `/api`, `/stream`, `/natsws`, `/ws`

Environment:
- `ROBOT_DEV_BACKEND_URL`: optional default for `--backend-url`

Common dev flows:
```bash
# Local dev only
./dialtone.sh robot src_v2 dev

# Dev + browser on chroma (default browser node is chroma if available)
./dialtone.sh robot src_v2 dev --browser-node chroma

# Dev + browser on chroma + backend routes proxied to rover
./dialtone.sh robot src_v2 dev --browser-node chroma --backend-url http://rover-1:18086

# Same backend proxy via env var
ROBOT_DEV_BACKEND_URL=http://rover-1:18086 ./dialtone.sh robot src_v2 dev --browser-node chroma
```

Notes:
- If `--backend-url` omits a port (example `http://rover-1`), it targets port `80`.
- For rover runtime, use `http://rover-1:18086` to avoid `ECONNREFUSED` on proxied API/NATS routes.
- Stop dev with `Ctrl+C`.

## 4) Local Dev + Test Loop

1. Build UI and binaries (local):
```bash
./dialtone.sh robot src_v2 build
./dialtone.sh go src_v1 exec build -o ../bin/dialtone_autoswap_v1 ./plugins/autoswap/src_v1/cmd/main.go
./dialtone.sh go src_v1 exec build -o ../bin/dialtone_robot_v2 ./plugins/robot/src_v2/cmd/server/main.go
./dialtone.sh go src_v1 exec build -o ../bin/dialtone_camera_v1 ./plugins/camera/src_v1/cmd/main.go
./dialtone.sh go src_v1 exec build -o ../bin/dialtone_mavlink_v1 ./plugins/mavlink/src_v1/cmd/main.go
./dialtone.sh go src_v1 exec build -o ../bin/dialtone_repl_v1 ./plugins/repl/src_v1/cmd/repld/main.go
```

2. Run robot test suite (includes UI mock E2E in test ctx pattern):
```bash
./dialtone.sh robot src_v2 test
```

3. Optional: run with remote browser node when local host has no Chrome:
```bash
DIALTONE_TEST_BROWSER_NODE=chroma ./dialtone.sh robot src_v2 test
```

## 5) Publish Artifacts (No Deploy Side Effects)

`publish` only builds and uploads changed/missing release assets; it does not deploy remote hosts.
By default it publishes only the real robot target (`linux-arm64`).

```bash
./dialtone.sh robot src_v2 publish --repo timcash/dialtone
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

## 6) Bring Robot Up With Autoswap

Set token drop-in once on robot host:
```bash
TOKEN="$(gh auth token)"
./dialtone.sh ssh src_v1 run --node rover --cmd "mkdir -p ~/.config/systemd/user/dialtone_autoswap.service.d && cat > ~/.config/systemd/user/dialtone_autoswap.service.d/10-token.conf <<'EOF'
[Service]
Environment=GITHUB_TOKEN=${TOKEN}
EOF
systemctl --user daemon-reload"
```

Use autoswap deploy helper to install/update autoswap on robot and install service:
```bash
./dialtone.sh autoswap src_v1 deploy \
  --host rover \
  --user tim \
  --service \
  --manifest-url https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_channel.json \
  --repo timcash/dialtone
```

Check service + managed runtime:
```bash
./dialtone.sh ssh src_v1 run --node rover --cmd 'systemctl --user status dialtone_autoswap.service --no-pager -l | sed -n "1,40p"'
./dialtone.sh ssh src_v1 run --node rover --cmd 'cat ~/.dialtone/autoswap/state/runtime.json'
./dialtone.sh ssh src_v1 run --node rover --cmd 'cat ~/.dialtone/autoswap/state/supervisor.json'
```

Force immediate update check (instead of waiting poll interval):
```bash
./dialtone.sh autoswap src_v1 update --host rover --user tim
```

Normal field update path:
```bash
# 1. Build and publish from WSL
./dialtone.sh robot src_v2 publish --repo timcash/dialtone

# 2. Let autoswap detect it, or force an immediate poll
./dialtone.sh autoswap src_v1 update --host rover --user tim

# 3. Verify rover is running downloaded artifacts
./dialtone.sh robot src_v2 diagnostic --host rover --user tim --skip-ui --public-check=false
```

Optional rover Nix maintenance checks:
```bash
./dialtone.sh robot src_v2 nix-diagnostic --host rover --user tim
./dialtone.sh robot src_v2 nix-gc --host rover --user tim
```

## 7) Robot Diagnostic (Mandatory)

Run full diagnostic against robot host:
```bash
./dialtone.sh robot src_v2 diagnostic --host rover --user tim
```

Common variants:
```bash
./dialtone.sh robot src_v2 diagnostic --host link-local --user tim --skip-ui --public-check=false
./dialtone.sh robot src_v2 diagnostic --host rover --user tim --skip-ui
./dialtone.sh robot src_v2 diagnostic --host rover --user tim --ui-url https://rover-1.dialtone.earth --browser-node chroma
./dialtone.sh robot src_v2 diagnostic --host rover --user tim --manifest /home/tim/.dialtone/autoswap/manifests/manifest-<hash>.json
```

Diagnostic checklist details:
- `src/plugins/robot/src_v2/diagnostic.md`

## 8) WSL Relay for Public UI

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

## 9) Clean Remote Robot State (Reset)

If robot needs full teardown before re-bootstrap:
```bash
./dialtone.sh robot src_v2 clean --host rover --user tim
```

`clean` hard-verifies all of the following:
- repo removed (`~/dialtone`)
- matching `dialtone|robot|rover` services removed/disabled/inactive
- dialtone runtime processes gone
- autoswap binaries/artifacts/releases/manifests removed

## 10) Expected Working State

After publish + autoswap update + diagnostic:
- autoswap service is active on robot
- manifest path is correct and active
- managed processes `robot/camera/mavlink/repl` are running
- robot endpoints work (`/health`, `/api/init`, `/api/integration-health`, `/stream`)
- UI loads and sections/menu work
- telemetry + latency render from real MAVLink flow over `/natsws`
- WSL relay service is active and public URL serves robot UI

## 11) Troubleshooting Order

1. Mesh reachability:
```bash
./dialtone.sh ssh src_v1 mesh --mode check
```
2. Autoswap status/list on robot.
3. Robot `src_v2 diagnostic --skip-ui`.

## 12) Deployment Architecture

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
4. Full `src_v2 diagnostic` with browser node override.
5. Relay service status on WSL.
