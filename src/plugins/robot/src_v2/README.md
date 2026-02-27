# Robot src_v2 Workflow (LLM Agent)

This document is the end-to-end operating workflow for `robot src_v2`:
- edit code
- test locally
- publish artifacts
- run/update on robot through autoswap + manifest
- verify with diagnostics
- expose UI through local WSL relay

## 1) Architecture Contract

`robot src_v2` runtime is manifest-driven by autoswap:
- `dialtone_autoswap_v1` (only OS service)
- `dialtone_robot_v2`
- `dialtone_camera_v1`
- `dialtone_mavlink_v1`
- `dialtone_repl_v1`
- `robot_src_v2_ui_dist` (directory artifact extracted from release tarball)

Primary manifest:
- `src/plugins/robot/src_v2/config/composition.manifest.json`

Important rule:
- Robot runtime must work without source repo on robot host. Runtime must come from manifest + downloaded/installed artifacts.

## 2) Prerequisites

Run from repo root on WSL node:
- `/home/user/dialtone`

Required tools:
- `go`, `bun`, `gh` authenticated (`gh auth status`)
- SSH mesh connectivity to robot (`rover` alias)
- Robot host autoswap service must have `GITHUB_TOKEN` set (to avoid GitHub API rate limits when pulling release artifacts)

Useful mesh host aliases are documented in:
- `plan/SSH_MESH_NODES.md`

## 3) Local Dev + Test Loop

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

## 4) Publish Artifacts (No Deploy Side Effects)

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

## 5) Bring Robot Up With Autoswap

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
  --manifest-url https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_composition_manifest.json \
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

## 6) Robot Diagnostic (Mandatory)

Run full diagnostic against robot host:
```bash
./dialtone.sh robot src_v2 diagnostic --host rover --user tim
```

Common variants:
```bash
./dialtone.sh robot src_v2 diagnostic --host rover --user tim --skip-ui
./dialtone.sh robot src_v2 diagnostic --host rover --user tim --ui-url https://rover-1.dialtone.earth --browser-node chroma
./dialtone.sh robot src_v2 diagnostic --host rover --user tim --manifest /home/tim/.dialtone/autoswap/manifests/manifest-<hash>.json
```

Diagnostic checklist details:
- `src/plugins/robot/src_v2/diagnostic.md`

## 7) WSL Relay for Public UI

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

## 8) Clean Remote Robot State (Reset)

If robot needs full teardown before re-bootstrap:
```bash
./dialtone.sh robot src_v2 clean --host rover --user tim
```

`clean` hard-verifies all of the following:
- repo removed (`~/dialtone`)
- matching `dialtone|robot|rover` services removed/disabled/inactive
- dialtone runtime processes gone
- autoswap binaries/artifacts/releases/manifests removed

## 9) Expected Working State

After publish + autoswap update + diagnostic:
- autoswap service is active on robot
- manifest path is correct and active
- managed processes `robot/camera/mavlink/repl` are running
- robot endpoints work (`/health`, `/api/init`, `/api/integration-health`, `/stream`)
- UI loads and sections/menu work
- telemetry + latency render from real MAVLink flow over `/natsws`
- WSL relay service is active and public URL serves robot UI

## 10) Troubleshooting Order

1. Mesh reachability:
```bash
./dialtone.sh ssh src_v1 mesh --mode check
```
2. Autoswap status/list on robot.
3. Robot `src_v2 diagnostic --skip-ui`.
4. Full `src_v2 diagnostic` with browser node override.
5. Relay service status on WSL.
