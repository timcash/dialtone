# Autoswap Plugin

```bash
# 1) Build autoswap in dev
./dialtone.sh go src_v1 exec build -o ../bin/dialtone_autoswap_v1 ./plugins/autoswap/src_v1/cmd/main.go

# 2) On target (standalone usage, no dialtone.sh needed):
./dialtone_autoswap_v1 service --mode install \
  --manifest-url https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_composition_manifest.json \
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
Use `--manifest-url` to fetch the manifest from GitHub/HTTP and re-check it on each poll interval.

```bash
./dialtone.sh autoswap src_v1 run \
  --manifest-url https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_composition_manifest.json \
  --listen :18086 --nats-port 18236 --nats-ws-port 18237 \
  --require-stream=true --stay-running=true
```

### `deploy` (dev helper)
Builds autoswap for target OS/arch and deploys via SSH mesh node routing.

```bash
./dialtone.sh autoswap src_v1 deploy \
  --host rover --user tim --pass password1 --service \
  --manifest-url https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_composition_manifest.json
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

## State Files

Autoswap writes machine-readable state under:

- `~/.dialtone/autoswap/state/supervisor.json`
- `~/.dialtone/autoswap/state/runtime.json`

Use:

```bash
./dialtone.sh autoswap src_v1 service --mode list
```

to inspect current worker/runtime state without hitting OS-specific tools directly.
