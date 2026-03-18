# Chrome v3 Daemon Plan

## Scope

`src/plugins/chrome/src_v3`

Main files to change first:

- `browser.go`
- `daemon.go`
- `local.go`
- `types.go`

## Current State

The current daemon does all of the following:

- starts and stops Chrome
- owns the remote allocator connection
- owns the managed tab/session
- stores console state in memory
- runs an embedded NATS command bus

This makes recovery harder than it needs to be.

## Progress

Completed in this pass:

- `reset` now prefers managed-session reset over browser teardown
- the daemon can reattach to a live browser allocator instead of always starting a new browser
- added per-role daemon state at `.dialtone/chrome-src-v3/<role>/state.json`
- verified on `legion` that `reset` no longer tears down the daemon
- verified with `cad src_v1` browser smoke that the REPL-driven CAD workflow still passes

Still open:

- expose more of the daemon state directly in CLI/operator output
- add an explicit hard browser restart command separate from `reset`
- improve per-role port allocation instead of fixed global defaults
- verify remote state-file inspection and recovery paths more directly

## Core Recommendation

Treat the daemon as a long-lived control plane that can attach to a browser, not only as a browser supervisor that must recreate everything when state drifts.

## Problems To Fix

### 1. Browser lifecycle and control plane are too coupled

Current behavior in `browser.go`:

- `ensureBrowser()` may launch Chrome itself
- allocator loss marks the daemon unhealthy
- `close` and `reset` are still too close to full browser teardown

Recommended change:

- separate "browser process ownership" from "automation control"
- default to reconnecting to a live Chrome debug port if possible
- only start a new Chrome process when explicitly required

### 2. Reset is too heavy

Recommended change:

- make ordinary `reset` mean:
  - recreate managed tab
  - clear console history
  - reset current URL state
- add an explicit harder operation for full browser restart if needed

Suggested command split:

- `reset` = tab/session reset
- `browser-restart` = restart browser process
- `browser-stop` = stop browser process without removing daemon

### 3. Runtime state is too in-memory

Recommended per-role state file:

- `.dialtone/chrome-src-v3/<role>/state.json`

Recommended contents:

- `service_pid`
- `browser_pid`
- `chrome_port`
- `nats_port`
- `role`
- `profile_dir`
- `websocket_url`
- `current_url`
- `managed_target`
- `started_at`
- `last_healthy_at`
- `last_error`

### 4. Shared fixed ports do not scale

Current defaults in `types.go`:

- `19464`
- `19465`

Recommended change:

- keep defaults for `role=dev`
- introduce per-role port allocation or persisted per-role assigned ports
- expose those ports via the state file and `status`

### 5. Reconnect path needs to be first-class

Recommended change:

- on startup, if Chrome debug port is live, try to attach before deciding to launch
- on allocator disconnect, mark session degraded first, not permanently unhealthy
- allow the daemon to rebuild allocator/tab state without killing Chrome

## Proposed Milestones

### Milestone 1: Explicit daemon state

- write/read per-role state file
- improve `status`

### Milestone 2: Tab-scoped reset

- make `reset` cheap
- add explicit hard restart command

### Milestone 3: Reconnectable daemon

- attach to existing browser
- rebuild allocator and managed tab after disconnect

### Milestone 4: Better per-role isolation

- assign per-role ports
- avoid collisions between test roles and dev roles

## Success Criteria

- `chrome src_v3 status --host legion --role dev` can explain daemon state without guessing
- CAD/UI browser tests do not need a full browser restart between normal runs
- stale allocator state can be repaired without killing Chrome
- different roles can coexist without port conflicts
