# Dialtone Shell Architecture Options

## Goal
Make `./dialtone.sh` predictable as a single entrypoint while avoiding recursive wrapper calls, duplicate signal handling, and fragile dev-process lifecycle.

## Current Pain Points
- Recursive calls: `./dialtone.sh` sometimes calls `./dialtone.sh` internally.
- Duplicate signal forwarding: nested wrappers can both send `SIGTERM`.
- Dev lifecycle ambiguity: whether a process should survive terminal/session end is unclear.
- Process discovery is ad hoc: no single source of truth for "what is running".
- Logs are inconsistent across plugins.

## Option A: Keep Current Wrapper Pattern (Incremental Fixes)
Description:
- Keep current shell wrapper architecture.
- Add guardrails around nesting and signal forwarding.

Benefits:
- Smallest migration.
- Minimal rewrite of existing plugins.

Problems:
- Structural complexity remains (nested wrappers + traps).
- Hard to reason about ownership of child processes.
- Easy to regress when adding new plugins.

When to choose:
- Short-term stabilization only.

---

## Option B: `./dialtone.sh` as a Process Manager + Dispatcher (Recommended)
Description:
- `./dialtone.sh` is the only process supervisor.
- Internal plugin calls should not invoke `./dialtone.sh` again.
- Wrapper dispatches directly to Go/Bun/system binaries as needed.
- Managed commands (like `dag dev`) have stable process keys and PID/log metadata.

Core rules:
- No internal self-calls to `./dialtone.sh`.
- Managed commands use one key, e.g. `dag:dev:src_v2`.
- Re-running same managed command: stop old process, start new one.
- Standard log path per managed command.

Suggested runtime state:
- Directory: `.dialtone/run/`
- Files per key:
  - `<key>.pid`
  - `<key>.meta.json` (command, cwd, start time, port, log path)
  - `<key>.log` (or plugin-specific log path if requested)

Suggested shell API:
- `./dialtone.sh dag dev src_v2` -> restart managed process for key `dag:dev:src_v2`
- `./dialtone.sh ps` -> list managed processes
- `./dialtone.sh stop dag dev src_v2` -> stop by key
- `./dialtone.sh logs dag dev src_v2` -> tail log

Benefits:
- Clear ownership: one supervisor layer.
- Predictable behavior on rerun (kill old, start new).
- Better UX for long-running dev tasks.
- Easier integration tests for lifecycle behavior.

Problems:
- Requires refactor of plugin internals that currently shell out through wrapper recursively.
- Need robust stale PID handling and lock strategy.

When to choose:
- Best medium/long-term architecture for this repo.

---

## Option C: Dedicated Background Daemon (`dialtoned`) + Thin CLI
Description:
- `./dialtone.sh` (or Go CLI) sends commands to a long-lived local daemon.
- Daemon owns all managed processes and logs.

Benefits:
- Most robust process model.
- First-class status/events/health handling.
- Easier to support rich commands (`restart`, `watch`, subscriptions).

Problems:
- Highest complexity.
- Requires IPC protocol, daemon lifecycle, and more ops handling.

When to choose:
- If Dialtone evolves into a multi-service local platform with heavy process orchestration.

---

## Option D: No Process Management, Pure Pass-through CLI
Description:
- `./dialtone.sh` only dispatches and exits; user manages processes manually.

Benefits:
- Very simple implementation.

Problems:
- Bad UX for dev workflows.
- No standard lifecycle/log behavior.

When to choose:
- Not recommended for current needs.

## Recommendation
Adopt **Option B** now.

Why:
- Solves current failures (recursive kills, duplicate SIGTERM logs).
- Delivers requested behavior: rerun `dag dev` kills old and starts new.
- Keeps complexity manageable without introducing a daemon yet.

## Concrete Design for Option B
1. Command classification
- Managed long-running: `dag dev`, `template dev`, `www dev`, etc.
- One-shot: `build`, `lint`, `test`, `smoke`, `go exec`, `bun exec`.

2. Process identity
- Key format: `<plugin>:<command>:<variant>`
- Example: `dag:dev:src_v2`

3. Launch behavior
- On start:
  - Read existing PID for key.
  - If alive, send SIGTERM + timeout + SIGKILL fallback.
  - Start new process and persist PID/meta.
- On exit:
  - Clear PID/meta if owned by current launch token.

4. Logging
- Default: `.dialtone/run/<key>.log`
- Allow plugin override (e.g. DAG requirement): `src/plugins/dag/src_v2/dev.log`
- Always capture stdout + stderr.

5. Signal model
- Only top-level `./dialtone.sh` handles traps.
- Child commands must not install overlapping shell-level forwarding that re-signals already-managed children.
- Prefer process-group kill for managed keys when needed.

6. Internal execution
- Plugins should call direct executables (`go`, managed bun binary, etc.) via shared helper functions, not wrapper recursion.
- Keep environment prep centralized in one place.

## Migration Plan
1. Add process manager utilities in one module (keying, pid/meta IO, stop/start/status).
2. Convert `dag dev` first as reference implementation.
3. Convert other `* dev` commands.
4. Remove recursive `./dialtone.sh` internal calls across plugins.
5. Add `ps`, `stop`, `logs` commands.
6. Add integration tests for:
- rerun replaces old process
- log capture includes stderr/stdout
- stale PID cleanup
- signal timeout behavior

## Risks and Mitigations
- Risk: stale PID files after crashes.
- Mitigation: validate PID command line + start time before trusting.

- Risk: accidental kill of unrelated process with reused PID.
- Mitigation: compare metadata (command fingerprint/start time) before kill.

- Risk: behavior change surprises users.
- Mitigation: print explicit lifecycle messages and document managed-mode semantics.

## Decision Summary
- Short term: implement Option B for managed dev commands.
- Medium term: expand Option B to all long-running plugin flows.
- Long term: consider Option C only if orchestration complexity grows significantly.
