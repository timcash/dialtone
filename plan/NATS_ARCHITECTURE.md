# NATS Architecture Plan

## Goal

Make NATS the control plane for the full Dialtone runtime:

- `./dialtone.sh` is a client
- `repl src_v3` is the host supervisor
- plugin commands run as managed subtones or managed services
- logs, heartbeats, queues, state, and service lifecycle all flow through NATS

The system should support:

- one background REPL supervisor per host
- FIFO foreground command execution
- durable background services
- host-local and remote process management
- high-signal `DIALTONE>` summaries for LLM agents
- full logs and detailed state outside the index room

## What We Learned

### What Works

- `robot src_v2` works well through REPL
- `cad src_v1` one-shot commands and long-lived `serve` work through REPL
- REPL subtone logging and registry are good foundations
- NATS request/reply already works for REPL command injection and leader health

### What Is Still Weak

- REPL leader autostart is still too ad hoc
- long-lived foreground commands block later foreground management commands
- remote service lifecycle is still too SSH/shell-centric
- `chrome src_v3` is only partially NATS-native
- service liveness is still inferred from ports/processes instead of heartbeats

### Concrete Failure Seen

`chrome src_v3 status --host legion --role dev` failed with:

- `nats: no responders available for request`

That means the control plane for Chrome on `legion` is not supervised strongly enough. The daemon was expected to exist, but there was no responder on its NATS subject.

## Target Runtime Model

### Layer 1: `./dialtone.sh`

`./dialtone.sh` should be a thin client:

- bootstrap environment if needed
- find the local REPL supervisor
- start it if missing
- proxy a single command into the supervisor over NATS
- stream high-level `DIALTONE>` events back to the shell

It should not directly manage long-lived plugin processes.

### Layer 2: REPL Host Supervisor

Each host should run one background REPL supervisor.

Responsibilities:

- own the host-local NATS endpoint
- accept command injection
- manage the foreground FIFO queue
- manage background subtones
- manage named long-lived services
- track logs, heartbeats, state, and failure reasons
- expose all lifecycle state over NATS

Examples of managed services:

- `chrome/dev`
- `autoswap/default`
- `cloudflare/shell`
- `cad/dev`

### Layer 3: Managed Subtones and Services

Everything should be one of:

- foreground subtone
- background subtone
- managed service

Each must have:

- a stable identity
- a mode
- a PID / process tree
- a log path or log stream
- a heartbeat
- a durable state record

## NATS as the Center

### Request/Reply

Use request/reply for:

- injecting commands
- service start/stop/reset/status
- subtone stop/kill
- structured status/doctor calls

Examples:

- `repl.host.<host>.cmd`
- `repl.host.<host>.service.<service>.<role>.cmd`

### Pub/Sub

Use pub/sub for:

- index-room lifecycle messages
- subtone logs
- service lifecycle events
- heartbeat broadcasts
- promoted `DIALTONE>` summaries

Examples:

- `repl.room.index`
- `repl.subtone.<pid>`
- `repl.host.<host>.event`
- `repl.host.<host>.service.<service>.<role>.event`
- `repl.host.<host>.service.<service>.<role>.log`

### JetStream KV

Use KV for latest state:

- leader state
- service registry
- subtone registry
- heartbeat snapshots
- locks / leases
- foreground queue metadata

Proposed KV buckets:

- `repl_leaders`
- `repl_services`
- `repl_subtones`
- `repl_heartbeats`
- `repl_locks`
- `repl_queue`

### JetStream Streams

Use streams for durable history:

- command lifecycle events
- service lifecycle events
- errors and crash reasons
- operator/LLM audit trail

Proposed streams:

- `REPL_EVENTS`
- `REPL_LOGS`
- `REPL_HEARTBEATS`

### Object Store

Use object store for larger payloads:

- screenshots
- large logs
- crash bundles
- generated artifacts when needed

## Heartbeat Contract

All long-lived managed processes should emit heartbeats over NATS.

This includes:

- REPL leader
- background subtones
- managed services
- remote host services like Chrome

### Heartbeat Subject

Examples:

- `repl.host.local.heartbeat.leader.default`
- `repl.host.local.heartbeat.subtone.778528`
- `repl.host.legion.heartbeat.service.chrome.dev`
- `repl.host.rover.heartbeat.service.autoswap.default`

### Heartbeat Payload

```json
{
  "host": "legion",
  "kind": "service",
  "name": "chrome",
  "role": "dev",
  "pid": 12345,
  "state": "running",
  "started_at": "2026-03-18T16:31:44Z",
  "last_ok_at": "2026-03-18T16:32:01Z",
  "uptime_sec": 17,
  "cpu_percent": 2.1,
  "mem_rss": 73400320,
  "ports": [19464, 19465],
  "details": {
    "browser_pid": 67890,
    "nats_port": 19465,
    "chrome_port": 19464
  }
}
```

### Supervisor Behavior

The REPL supervisor should:

- subscribe to heartbeat subjects
- update KV state on each heartbeat
- detect stale heartbeat expiry
- emit high-level failure events when heartbeats stop
- surface useful `DIALTONE>` summaries from those events

## Service Management Model

Remote services should not be managed primarily through raw SSH commands.

Instead:

1. bootstrap a REPL supervisor on the remote host if missing
2. send service lifecycle requests to that host supervisor over NATS
3. let the host supervisor spawn, track, and restart the service

This means Chrome service management becomes:

- `repl.host.legion.service.chrome.dev.cmd start`
- `repl.host.legion.service.chrome.dev.cmd status`
- `repl.host.legion.service.chrome.dev.cmd stop`
- `repl.host.legion.service.chrome.dev.cmd reset`

Not:

- remote `nohup ... &` plus later direct NATS assumptions

## `DIALTONE>` Contract

The index room should stay high-level and operational.

It should show:

- request accepted
- queue position / blocked reason
- subtone started
- service start requested
- service ready
- heartbeat healthy / stale
- completed / stopped / failed

It should not show:

- raw shell logs
- JSON blobs
- repeated polling noise
- long stack traces

### Example Chrome Flow

```text
DIALTONE> chrome service: requesting start on legion role=dev
DIALTONE> chrome service: supervisor accepted request
DIALTONE> chrome service: daemon pid 12345
DIALTONE> chrome service: browser attached on legion role=dev
DIALTONE> chrome status: daemon ready on legion role=dev browser_pid=67890
```

## Process Modes

Every managed process should have an explicit mode:

- `foreground`
- `background`
- `service`

This mode should appear in:

- registry output
- heartbeat payload
- `ps`
- `subtone-list`
- service status views

## Phased Plan

### Phase 1: Stabilize REPL Supervisor

- make REPL service/supervisor the default background runtime
- stop autostarting ad hoc detached leader processes
- keep leader state in KV and local state file
- add `doctor` output from leader health + heartbeat state

### Phase 2: Standardize Heartbeats

- define one heartbeat schema
- make subtones emit heartbeats explicitly
- make long-lived services emit heartbeats explicitly
- persist heartbeat snapshots in KV with TTL semantics

### Phase 3: Service Registry

- add REPL-managed service registry
- support `start|stop|status|reset|logs`
- register services by host/name/role
- move PID/process-tree tracking into the supervisor

### Phase 4: Chrome Migration

- make `chrome src_v3` service lifecycle REPL-managed
- remote host service start/status via REPL NATS API
- Chrome daemon publishes heartbeats
- remove SSH-first remote lifecycle assumptions

### Phase 5: Queueing and Foreground Policy

- keep one foreground lane
- make queue state explicit on NATS
- allow background/service work in parallel
- add queue visibility in `DIALTONE>`

### Phase 6: Documentation

- update `src/plugins/repl/src_v3/README.md`
- document host supervisor architecture
- document NATS subjects and KV buckets
- document service lifecycle and heartbeat contracts

## Immediate Next Steps

1. Add REPL-managed service registry and service mode.
2. Add a shared heartbeat schema and publisher helper.
3. Move Chrome service start/status/stop into the REPL supervisor path.
4. Add `repl src_v3 doctor` driven by request/reply + KV + heartbeat freshness.
5. Make `DIALTONE>` show queue/service/heartbeat state clearly for LLMs.

## Success Criteria

The runtime is in a good state when:

- `./dialtone.sh <plugin> ...` always routes through a healthy local REPL supervisor
- remote services are controlled through host REPL supervisors over NATS
- all long-lived processes emit heartbeats over NATS
- `DIALTONE>` stays concise and high-signal
- full logs and state are available outside the index room
- Chrome, CAD, robot, autoswap, and cloudflare all follow the same control model
