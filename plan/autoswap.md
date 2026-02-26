# Multi-Worker Autoswap + Robot Runtime Options

## Summary
Build a generic `dialtone_autoswap` supervisor that can manage multiple Dialtone workers (starting with `dialtone_repl` and `dialtone_robot`) without systemd/launchd.
This replaces OS-level service management with one long-running process that:
1. polls GitHub releases,
2. downloads worker binaries,
3. swaps binaries atomically,
4. restarts workers,
5. monitors health/restarts on crash.

For robot runtime, keep options open for now. The key goal is to avoid embedding `tsnet` in `dialtone_robot`.

## Scope
- In scope:
  - General autoswap supervisor for N workers.
  - Worker spec model (`name`, `repo`, `asset`, `args`, `env`, `health`).
  - REPL leader command path to trigger remote install/switch/update on any host.
  - Verification that robot UI stays reachable at `http://drone-1` and `https://drone-1.dialtone.earth`.
- Out of scope (this phase):
  - Full removal of robot embedded NATS unless chosen in option track.
  - Replacing MAVLink/camera internals.

## Public Interfaces / CLI Changes

## 1) New Autoswap CLI (`./dialtone.sh autoswap src_v1 ...`)
- `run --config <path>`
- `init --profile <name>` (writes starter config)
- `status [--json]`
- `list-releases --worker <name> [--limit N]`
- `install --worker <name> --version <tag|latest>`
- `start --worker <name>`
- `stop --worker <name>`
- `restart --worker <name>`
- `pin --worker <name> --version <tag>`
- `unpin --worker <name>`
- `logs --worker <name> [--tail N]`

## 2) REPL control commands (leader-routed)
- `/repl src_v1 install <worker> <version> --host <hostname>`
- `/repl src_v1 start <worker> --host <hostname>`
- `/repl src_v1 stop <worker> --host <hostname>`
- `/repl src_v1 restart <worker> --host <hostname>`
- `/repl src_v1 pin <worker> <version> --host <hostname>`
- `/repl src_v1 unpin <worker> --host <hostname>`
- `/repl src_v1 who` includes autoswap daemon + worker versions/states

## 3) Config file (new)
`~/.dialtone/autoswap/config.json`:
- global:
  - `poll_interval`
  - `github_token_env`
  - `download_dir`
  - `state_dir`
- workers[]:
  - `name` (`dialtone_repl`, `dialtone_robot`)
  - `repo` (`timcash/dialtone`)
  - `asset` (platform-specific binary name)
  - `args` (runtime args)
  - `env` (env vars)
  - `health`:
    - `type` (`http`, `tcp`, `none`)
    - `target`
    - `interval`
    - `timeout`
  - `update_policy`:
    - `channel` (`latest`, `pinned`)
    - `allow_downgrade` (bool)

## Architecture (Common, regardless of robot network option)
1. Create `src/plugins/autoswap/src_v1/` plugin.
2. Move reusable release download/swap/worker monitor logic from REPL service code into autoswap library.
3. Keep `repl` plugin using autoswap library (or delegating to autoswap CLI) for compatibility.
4. Replace robot deploy “service swap” path with autoswap-managed `dialtone_robot`.
5. Keep cloudflare relay/wake/sleep commands as robot-specific ops until later consolidation.

## Robot Runtime Options (unselected for now)

## Option A: No robot tsnet, keep robot embedded NATS
- `dialtone_robot`:
  - keeps local embedded NATS + `/natsws`,
  - drops `tsnet` listeners entirely.
- Reachability:
  - `http://drone-1` must come from system tailscale on robot host.
  - `drone-1.dialtone.earth` via existing cloudflare relay/tunnel.
- Pros: smallest change from current robot internals.
- Cons: still ships embedded NATS in robot binary.

## Option B: No robot tsnet, no embedded NATS (robot as NATS client)
- `dialtone_robot` connects to REPL/leader NATS as client.
- Robot web server still serves UI/camera/mavlink endpoints.
- `/natsws` must proxy to external NATS WS endpoint (provided by leader/broker).
- Pros: smallest robot binary footprint.
- Cons: requires guaranteed external NATS WS availability and failover handling.

## Option C: No robot tsnet, broker sidecar worker on robot
- `dialtone_robot` stays web+mavlink+camera only.
- `dialtone_nats` worker (autoswap-managed) runs local NATS/WS.
- Robot UI uses local `/natsws` proxy to sidecar.
- Pros: clean separation of concerns, no tsnet in robot.
- Cons: one more worker to manage.

## Recommended Build Sequence (Decision-Independent First)
1. Implement autoswap plugin + config format + worker monitor loop.
2. Register two workers: `dialtone_repl`, `dialtone_robot`.
3. Add REPL leader commands to trigger autoswap actions on remote hosts via control frames.
4. Convert current REPL service implementation into thin wrapper over autoswap.
5. Add robot deploy transition mode:
   - `deploy` continues uploading binary/UI,
   - but hands runtime lifecycle to autoswap (start/restart/pin).
6. After that, choose and execute Option A/B/C.

## Data Flow
1. Host boots `dialtone_autoswap run --config ...`.
2. Autoswap reads worker specs and starts required workers.
3. Poll cycle checks GitHub release tags/assets.
4. New version -> download to versioned dir -> checksum (if available) -> atomic `current` pointer swap -> graceful worker restart.
5. Worker status published over REPL/NATS presence frames.
6. Leader receives `/repl src_v1 install ... --host ...`, emits control frame, target host autoswap executes action, returns status line frames.

## Failure Modes / Recovery
- GitHub unavailable: keep current workers running, backoff retry.
- Download failure/corrupt asset: keep prior version, mark worker degraded.
- Worker crash loop: exponential restart backoff + status frame.
- Health check failure after swap: rollback to previous known-good binary.
- Host offline for remote command: leader reports unreachable; no partial state mutation.

## Test Cases and Scenarios

## Autoswap unit/integration
1. Parse config with 2+ workers.
2. Release discovery resolves correct platform asset.
3. Atomic swap leaves previous version recoverable.
4. Crash restart policy and backoff works.
5. Health check failure triggers rollback.
6. Pinned version blocks unwanted upgrades.

## REPL command path
1. `/repl src_v1 install dialtone_robot vX --host drone-1` sends control frame.
2. Target host autoswap applies install and reports success/failure lines.
3. `who`/`versions` includes daemon version + worker version + OS/arch.

## End-to-end host validation
1. `drone-1` autoswap runs with both workers configured.
2. Upgrade `dialtone_robot` to new release tag remotely via leader command.
3. Validate:
   - `http://drone-1/health` returns ok,
   - `http://drone-1` UI loads,
   - `https://drone-1.dialtone.earth` UI loads.
4. Verify camera endpoint `/stream` and MAVLink telemetry visible in UI.

## Acceptance Criteria
- One daemon (`dialtone_autoswap`) manages multiple workers without systemd/launchd.
- `dialtone_robot` and `dialtone_repl` can both be installed/updated/swapped by autoswap.
- Leader can trigger host-targeted install/update/start/stop via REPL commands.
- Robot UI is reachable at both required URLs after autoswap update.
- Robot deploy command no longer required for routine version rollout (kept only as transitional upload helper).

## Assumptions and Defaults
- Default release source: GitHub Releases (`timcash/dialtone`).
- Poll interval default: `5m`.
- Host identity uses hostnames, not hardcoded IPs.
- Robot network runtime option remains open (A/B/C above); implementation starts with decision-independent autoswap foundation first.
