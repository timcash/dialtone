# REPL Leader Plan

## Scope

`src/plugins/repl/src_v3/go/repl`

Main files to change first:

- `leader_ensure_v3.go`
- `core_service.go`
- `core_runtime.go`
- `run_commands_v3.go`

## Current State

Autostart currently does this:

- checks whether the NATS endpoint is reachable
- if not, runs detached `go run ./plugins/repl/scaffold/main.go src_v3 leader ...`
- waits for the endpoint to accept connections

This is fragile because it does not prove that the leader command loop is healthy, and detached `go run` is a poor foundation for a background runtime.

## Progress

Completed in this pass:

- added a real leader health reply on `repl.leader.health`
- added `.dialtone/repl-v3/leader.json`
- updated `EnsureLeaderRunning` to wait for leader health, not only TCP reachability
- updated autostart to re-exec the current binary instead of detached `go run`
- updated `repl src_v3 status` to report healthy leader metadata when available

Still open:

- move autostart fully onto the service/supervisor path
- add `leader doctor`
- add lighter restart/log tooling
- trim remaining top-level bootstrap noise outside the REPL leader itself

## Problems To Fix

### 1. Startup path is too weak

Current autostart in `leader_ensure_v3.go` starts a transient process instead of a durable installed worker or service.

Recommended change:

- stop using detached `go run` for ordinary autostart
- reuse the existing service/supervisor path from `core_service.go`
- if needed, add a "local service bootstrap" helper so normal shell invocations can ensure a local leader without reinstalling anything

### 2. Health check is too shallow

Current health logic is mostly endpoint reachability.

Recommended change:

- add a leader health probe over NATS
- require a successful ping/reply on the leader control subject before treating the leader as healthy
- keep endpoint reachability only as a low-level hint

### 3. No persistent leader state file

Recommended file:

- `.dialtone/repl-v3/leader.json`

Recommended contents:

- `pid`
- `nats_url`
- `room`
- `hostname`
- `started_at`
- `version`
- `embedded_nats`
- `bootstrap_http_url`
- `bootstrap_http_pid`
- `last_healthy_at`

### 4. Cleanup tools are too blunt

`process-clean` is useful, but too destructive for normal iteration.

Recommended change:

- keep `process-clean` for hard recovery
- add lighter commands:
  - `leader status`
  - `leader doctor`
  - `leader restart`
  - `leader logs`

### 5. DIALTONE output should distinguish startup from steady state

Desired shell behavior:

- if leader is already healthy:
  - route quietly into REPL
- if leader must be started:
  - print one short startup line
  - wait for leader health
  - route command
- if leader is stale:
  - print one short repair line
  - restart or fail with a precise diagnosis

## Proposed Milestones

### Milestone 1: Stable local autostart

- replace detached `go run`
- use a durable local binary/service path
- write leader state file

### Milestone 2: Real health and doctor

- implement leader ping
- add `leader doctor`
- surface stale state clearly

### Milestone 3: Better lifecycle tooling

- restart/log/status helpers
- less destructive cleanup

## Success Criteria

- `./dialtone.sh robot src_v2 diagnostic --host rover` works after shell restart without manual REPL prep
- `./dialtone.sh repl src_v3 status` can explain whether the leader is healthy or only partially alive
- killing the leader leaves enough state for the next command to recover cleanly
