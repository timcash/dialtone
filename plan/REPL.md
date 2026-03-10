# REPL v3 Plan (Current)

## Goal
- `./dialtone.sh` is a thin client to a running REPL daemon.
- `--host` routes commands over NATS to another host daemon (tailnet/LAN fallback).
- `--ssh-host` is explicit SSH fallback transport.
- Bootstrap from `curl https://shell.dialtone.earth/install.sh | bash` should work from an empty `/tmp` workspace.
- REPL daemon owns tsnet runtime directly (embedded library path) and only falls back to standalone tsnet commands for test/diagnostics.

## Current State
- Verified: `--host` performs NATS-targeted routing and remote subtone execution.
- Verified: `--ssh-host` runs explicit SSH transport path.
- Verified: `./dialtone.sh --test` bootstraps from `install.sh` into OS temp (`/var/folders/...`), downloads repo tarball, installs Go, starts REPL test suite, and passes.
- Verified: REPL v3 suite steps pass in default mode:
  - `tmp-bootstrap-workspace`
  - `dialtone-help-surfaces`
  - `bootstrap-apply-updates-dialtone-json`
  - `injected-help-and-ps`
  - `injected-ssh-wsl-command`
  - `injected-cloudflare-tunnel-start`
  - `injected-tsnet-ephemeral-up` (skip/pass unless real mode enabled)
- Updated: `injected-tsnet-ephemeral-up` now validates daemon tsnet behavior (embedded endpoint or explicit native-tailscale skip signal), not `tsnet src_v1 up` subtone execution.
- Not fully verified: real tsnet ephemeral bring-up path (`DIALTONE_REPL_V3_TEST_REAL=1`) on a host without native tailscale.

## Work Remaining

### 1) Shell bootstrap payload freshness
- Ensure `shell.dialtone.earth/install.sh` and `/dialtone.sh` serve latest `gold` workspace build.
- Add a deterministic version marker command to compare local vs served payload.
- Add a `./dialtone.sh repl src_v3 bootstrap-http status` output that shows served commit/version.

### 2) REPL tsnet bring-up on clean host
- Validate non-interactive tsnet startup from REPL leader on clean host (`grey`) temp workspace with **no native tailscale installed/running**.
- Ensure daemon behavior contract:
  - if native tailscale is active: skip embedded tsnet startup.
  - if native tailscale is absent: start embedded tsnet with auth-key provisioning from `env/dialtone.json`.
- Add required assertions in real mode:
  - emitted line: `REPL tsnet NATS endpoint active: nats://<ephemeral>:4222`
  - emitted line indicating native tailscale skip OR embedded tsnet startup path (published to REPL room)
  - `./dialtone.sh <cmd> --host <ephemeral>:4222` injects and executes remotely.
  - Optional strict gate: set `DIALTONE_REPL_V3_TEST_REQUIRE_EMBEDDED_TSNET=1` to fail test if native tailscale is detected.

### 3) End-to-end bootstrap test (`--test`)
- Keep test fully in OS temp location.
- Test should prove sequence:
  1. Empty temp folder.
  2. Curl installer.
  3. Bootstrap repo + Go + REPL daemon.
  4. Leader starts embedded NATS.
  5. Inject command and wait for terminal event.
  6. Validate REPL room lines + subtone lifecycle.
- Add explicit assertion for no dependency on user home state (`~/.ssh`, local repo paths).
- Add real-mode lane (`DIALTONE_REPL_V3_TEST_REAL=1`) that asserts tsnet ephemeral startup when native tailscale is unavailable.

### 4) Transport behavior hardening
- Add tests for `--host` resolution order:
  1. explicit NATS URL/host:port
  2. mesh tailnet candidate
  3. mesh LAN candidate
  4. fallback `nats://<host>:4222`
- Add tests that `--ssh-host` never uses NATS injection path.

### 5) Subtone visibility/logging
- Ensure every injected command produces:
  - REPL request line (`<user-host>/command`),
  - subtone start line,
  - subtone log path,
  - terminal exit line.
- Ensure `subtone-list` and `subtone-log --pid` are part of test assertions.

### 6) Bootstrap UX checks
- On every `./dialtone.sh` run, verify printed checks include:
  - required paths,
  - env JSON validity,
  - NATS reachability,
  - REPL process status,
  - bootstrap HTTP status.
- Keep failures actionable with direct next commands.

## Runtime Architecture Outline

### A) REPL daemon process model
- Single daemon process per host owns:
  - embedded NATS broker,
  - REPL room/event bus,
  - command dispatcher,
  - embedded tsnet runtime state.
- `./dialtone.sh` should connect/inject; it should not become the long-running owner.

### B) Embedded vs subtone responsibilities
- Embed in REPL binary (always-on control-plane components):
  - NATS broker lifecycle,
  - tsnet library lifecycle (only if native tailscale is not active),
  - command queue + event publish path.
- Run as subtones (isolated execution units):
  - one-off plugin commands (`go src_v1 version`, `cloudflare ...`, `ssh ...`),
  - optional long-running app workloads started intentionally by user.
- Standalone plugin mode remains for independent testing:
  - `tsnet src_v1 ...` remains valid as its own plugin for diagnostics and dev.

### C) Command classes
- Control-plane commands (handled in daemon, no subtone):
  - help/ps/status/daemon lifecycle/introspection.
- One-off workload commands (subtone):
  - finish with terminal exit event.
- Long-running workload commands (managed subtone):
  - heartbeat + explicit stop/kill semantics.
- Disallowed/guarded injected commands:
  - commands that conflict with daemon-owned embedded services (example: direct `tsnet up` if daemon already owns tsnet lifecycle).

### D) `--subtone` flag semantics (why it exists)
- `DIALTONE_SUBTONE=1` currently prevents recursive reinjection loops when a subtone runs `dialtone.sh`.
- Target improvement:
  - replace env sentinel with explicit execution mode flag in command envelope/runtime context,
  - keep behavior: subtone executes locally/directly, never reinjects into REPL.

### E) tsnet behavior contract
- Default daemon behavior:
  - detect native tailscale; if active, skip embedded tsnet startup.
  - if inactive, start embedded tsnet runtime via library path with non-interactive key provisioning.
- `tsnet src_v1` plugin behavior:
  - independent test/diagnostic interface for tsnet features,
  - should not be required for normal REPL daemon operation.

### F) Subtone lifecycle contract
- Every subtone emits:
  - accepted/start,
  - command + pid + log path,
  - stream/heartbeat,
  - single terminal event with exit code.
- REPL daemon should survive subtone failures and not exit on workload command errors.

## Test Matrix (priority order)
- `T01` Local host: `./dialtone.sh` starts/joins REPL and injects local command.
- `T02` Local host: `--host grey` routes over LAN NATS and executes remote subtone.
- `T03` Local host: `--ssh-host grey` executes via SSH path only.
- `T04` Clean `grey` `/tmp`: curl installer -> bootstrap -> leader+NATS up.
- `T05` Clean `grey` `/tmp` with no native tailscale: embedded tsnet endpoint up -> `--host <ephemeral>:4222` works.
- `T06` `./dialtone.sh --test`: full flow with NATS event wait/ack between steps.

## Definition of Done
- `./dialtone.sh --test` passes full clean-host bootstrap flow in `/tmp`.
- `--host` remote execution works via:
  - mesh alias,
  - tailnet endpoint,
  - LAN fallback.
- REPL tsnet ephemeral endpoint is verified in test (not manual only).
- README and help output match implemented behavior exactly.
