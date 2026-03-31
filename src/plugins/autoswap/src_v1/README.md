# Autoswap Plugin

## Runtime Note

Plain `./dialtone.sh autoswap src_v1 ...` is the default operator path.

That command is normally routed through the local REPL leader, which means:
- `DIALTONE>` should stay high-level
- full command output stays in the task log
- use `./dialtone.sh repl src_v3 task list --count 20`
- use `./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200`

`--host` is plugin-local for `autoswap` and should keep meaning the target mesh node, for example `rover`.

```bash
# 1) Build autoswap in dev
./dialtone.sh go src_v1 exec build -o ../bin/plugins/autoswap/src_v1/dialtone_autoswap_v1 ./plugins/autoswap/src_v1/cmd/main.go

# 2) On target (standalone usage, no dialtone.sh needed):
./dialtone_autoswap_v1 service --mode install \
  --manifest-url https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_channel.json \
  --repo timcash/dialtone \
  --check-interval 5m

# 3) Cross-OS lifecycle controls through autoswap abstraction:
./dialtone_autoswap_v1 service --mode is-active
./dialtone_autoswap_v1 service --mode status
./dialtone_autoswap_v1 service --mode restart
./dialtone_autoswap_v1 service --mode list
```

`autoswap src_v1` is a generic runtime supervisor abstraction for composition manifests.

- Only `autoswap` is installed as an OS service.
- `autoswap` then acts as a process manager for manifest workloads (any set of binaries/processes).
- `autoswap` polls GitHub releases (default every 5 minutes), swaps to newer artifacts, and restarts managed runtime.

## Robot Publish Workflow

The validated rover workflow is:

```bash
# 1) Build and publish robot binaries/UI from WSL
./dialtone.sh robot src_v2 publish --repo timcash/dialtone

# 2) Install autoswap once on the rover
./dialtone.sh autoswap src_v1 deploy \
  --host rover \
  --service \
  --repo timcash/dialtone \
  --manifest-url https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_channel.json

# 3) Let autoswap poll, or force an immediate refresh
./dialtone.sh autoswap src_v1 update --host rover

# 4) Verify the rover is running downloaded release artifacts
./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui --public-check=false
```

For the active `robot src_v2` deployment, autoswap downloads and runs release artifacts directly from:

- `~/.dialtone/autoswap/artifacts/dialtone_robot_v2`
- `~/.dialtone/autoswap/artifacts/dialtone_camera_v1`
- `~/.dialtone/autoswap/artifacts/dialtone_mavlink_v1`
- `~/.dialtone/autoswap/artifacts/dialtone_repl_v1`
- `~/.dialtone/autoswap/artifacts/robot_src_v2_ui_dist`

## Model

Autoswap is split into two layers:

1. **OS launcher layer (thin, platform-aware)**
- Linux: user `systemd` unit `dialtone_autoswap.service`
- macOS: user `launchd` agent `dev.dialtone.dialtone_autoswap`

2. **Runtime manager layer (platform-neutral)**
- Reads manifest sync contract
- Starts process graph in dependency order (`robot` first, then sidecars)
- Restarts failed children automatically
- Replaces binaries on release update and rolls runtime forward

This means callers use autoswap commands, not OS-specific unit tooling.

## Main Commands

### `stage`
Validates manifest and resolved artifact paths.

```bash
./dialtone.sh autoswap src_v1 stage \
  --manifest src/plugins/robot/src_v2/config/composition.manifest.json
```

### `run`
Runs manifest composition directly (foreground). With `--stay-running`, autoswap supervises child processes.
If manifest defines `runtime.processes`, autoswap starts exactly those processes using manifest dependency order.
Use `--manifest-url` to fetch either a direct manifest or a channel document from GitHub/HTTP. Channel documents are preferred because they resolve to an immutable release-pinned manifest.

```bash
./dialtone.sh autoswap src_v1 run \
  --manifest-url https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_channel.json \
  --listen :18086 --nats-port 18236 --nats-ws-port 18237 \
  --require-stream=true --stay-running=true
```

### `deploy` (dev helper)
Builds autoswap for target OS/arch and deploys via SSH mesh node routing.

```bash
./dialtone.sh autoswap src_v1 deploy \
  --host rover --service \
  --manifest-url https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_channel.json
```

### `service`
Cross-OS service control surface (systemd-like capabilities):

- `install`: install + enable autoswap launcher
- `run`: supervisor loop (called by launcher)
- `start`
- `stop`
- `restart`
- `is-active`
- `status`: launcher status + state dump
- `list`: print autoswap state files

Examples:

```bash
./dialtone.sh autoswap src_v1 service --mode install
./dialtone.sh autoswap src_v1 service --mode start
./dialtone.sh autoswap src_v1 service --mode status
./dialtone.sh autoswap src_v1 service --mode stop
```

## Update and Swap Behavior

- Poll interval: `--check-interval` (default `5m`)
- Source: GitHub latest release of `--repo`
- Preferred manifest source: stable channel asset (`robot_src_v2_channel.json`) that points at an immutable versioned manifest asset for that release
- Swapped artifacts: from `manifest.artifacts.release` mapping (generic), or legacy fallback keys.
- Supports file artifacts and directory artifacts (`type=dir`, e.g. UI dist archive extraction).
- Downloaded artifacts are checksum-verified before activation:
  - preferred: GitHub release asset `digest` metadata (`sha256:...`)
  - fallback: companion checksum assets (`<asset>.sha256`, `.sha256sum`, `.sha256.txt`) when present
- After sync:
1. stop current autoswap worker
2. switch current worker pointer
3. start new worker
4. worker starts/supervises manifest processes

This means you can keep builds on WSL and use autoswap only for:
- update detection
- artifact download
- process restart on the rover

## State Files

Autoswap writes machine-readable state under:

- `~/.dialtone/autoswap/state/supervisor.json`
- `~/.dialtone/autoswap/state/runtime.json`

Use:

```bash
./dialtone.sh autoswap src_v1 service --mode list
```

to inspect current worker/runtime state without hitting OS-specific tools directly.
