# REPL Control-Plane Plan (v3)

## Objective
Make `./dialtone.sh` a thin client to a long-lived local Dialtone daemon that owns:
1. Embedded NATS broker.
2. Command routing/execution as subtones.
3. REPL UI stream and presence.
4. Logging pipeline through the logs plugin only.

From this point forward:
- `./dialtone.sh <command>` publishes a command request over NATS.
- `./dialtone.sh <command> --user <role>` sets caller identity for that request.
- REPL displays requests as `<role>/command> ...` (example: `llm-codex/command> chrome src_v3 status`).
- `--llm` test mode is removed; test harness uses NATS command injection + event acks.

---

## Inspiration from `repl src_v1`
Borrow directly:
- `repl.cmd` as command ingress subject.
- `repl.room.<room>` as stream output/presence room.
- Typed bus frames (`command`, `line`, `server`, `join`, `left`, `control`, `error`).
- Leader execution model: one process executes subtones and publishes line events.

Do not carry forward:
- Ambiguous split between direct local execution and bus execution.
- Test flows depending on stdin orchestration flags.

---

## Glossary
- **Dialtone Daemon**: Long-lived process started once per host; owns embedded NATS and command executor.
- **Dialtone Client**: `dialtone.sh` invocation that only publishes request + waits for response/stream.
- **Requester**: Caller identity (`user`), defaulting to local prompt name.
- **Role**: Logical persona attached to requester (`human`, `llm-codex`, `ci`, etc).
- **Session**: Correlation scope for one interactive stream (`session_id`).
- **Request**: One command submission from client to daemon.
- **Subtone**: Managed process execution unit launched by daemon.
- **Command Bus**: NATS subjects used for request/reply + streaming events.
- **Room**: REPL output channel (`repl.room.<room>`), default `index`.
- **Ack**: Explicit machine-consumable event that indicates phase completion (accepted, started, finished).
- **Command Envelope**: Canonical normalized command payload shared by CLI and REPL (no special-case parser per surface).
- **Logs Plugin Sink**: Single logging sink/API for daemon, subtones, and client-facing line events.

---

## Architecture Decisions (Locked)
1. `dialtone.sh` responsibilities:
   - bootstrap env/repo/go/repl binary if missing;
   - ensure daemon running;
   - act as NATS client for command submit + event streaming.
2. `dev.go` responsibilities:
   - lightweight plugin scaffold/dispatcher;
   - no long-running control-plane ownership.
3. `repl` responsibilities:
   - long-lived daemon process;
   - embedded NATS lifecycle;
   - command executor and room stream publisher.
4. Logging:
   - all logs route through logs plugin codepath;
   - no parallel ad hoc log writers.
5. Command model:
   - `./dialtone.sh <...>` and REPL slash commands compile to the same Command Envelope and submit path.

---

## Unified Command System

### Canonical command envelope
Every command source must compile to:
```json
{
  "request_id": "uuid",
  "session_id": "uuid",
  "room": "index",
  "user": "llm-codex",
  "origin": "dialtone.sh|repl",
  "command": ["plugin", "version", "action", "...args"],
  "cwd": "/abs/path",
  "submitted_at": "RFC3339Nano"
}
```

### Surface mapping
- CLI: `./dialtone.sh chrome src_v3 status --user llm-codex`
  - Compiles directly to command array.
- REPL slash: `/chrome src_v3 status`
  - Uses current REPL user identity; compiles to same command array.
- REPL built-ins:
  - `/help`, `/ps`, `/status`, `/exit` stay local-control commands (not plugin subtones).

Rule: plugin execution path is always daemon-subtone via NATS, regardless of origin.

---

## Logging Unification

### Single pipeline
- Daemon logs -> logs plugin sink.
- Subtone lifecycle/output -> logs plugin sink.
- Room-readable summaries -> derived from logs plugin events.

### No duplicate writers
- Remove or deprecate direct file logging paths that bypass logs plugin APIs.
- Keep log persistence implementation behind logs plugin (files, stream topics, retention policy).

### Stream split
- `dialtone.cmd.events.<session_id>`: machine events (`started`, `stdout`, `stderr`, `finished`).
- `dialtone.room.<room>`: human-facing condensed lines with prefixes.

---

## Target CLI Contract
All commands below are client-side submissions to daemon, not local direct execution.

### Core entry
- `./dialtone.sh <plugin> <version> <action> [args...]`
- Optional caller identity:
  - `--user <role>` (example: `--user llm-codex`)
  - `--room <room>`
  - `--session <id>` (optional override; else auto-generated)
  - `--wait` (default true for interactive, false for fire-and-forget automation when needed)

### Daemon lifecycle commands
- `./dialtone.sh daemon status`
- `./dialtone.sh daemon start`
- `./dialtone.sh daemon stop`
- `./dialtone.sh daemon restart`

### REPL observability commands
- `./dialtone.sh repl status`
- `./dialtone.sh repl ps`
- `./dialtone.sh repl tail [--session <id>]`

Notes:
- `dialtone.sh daemon start` ensures NATS + executor + repl stream are online.
- All non-daemon commands auto-bootstrap daemon if not running.
- `--user` defaults to hostname (not `human`) for auditability.

---

## NATS API Contract

### Subjects
- `dialtone.cmd.submit` (request/reply): command submission ingress.
- `dialtone.cmd.events.<session_id>` (pub/sub): ordered command lifecycle events for one session.
- `dialtone.room.<room>` (pub/sub): human-readable REPL stream.
- `dialtone.daemon.status` (request/reply): daemon health/status endpoint.

### Command Request Schema (`dialtone.cmd.submit`)
```json
{
  "request_id": "uuid",
  "session_id": "uuid",
  "room": "index",
  "user": "llm-codex",
  "origin": "dialtone.sh",
  "command": ["chrome", "src_v3", "status"],
  "cwd": "/Users/user/dialtone",
  "env": {
    "DIALTONE_USE_NIX": "1"
  },
  "submitted_at": "RFC3339Nano"
}
```

### Immediate Reply Schema
```json
{
  "ok": true,
  "request_id": "uuid",
  "session_id": "uuid",
  "accepted": true,
  "queued": true,
  "event_subject": "dialtone.cmd.events.<session_id>"
}
```

### Event Schema (`dialtone.cmd.events.<session_id>`)
```json
{
  "request_id": "uuid",
  "session_id": "uuid",
  "event": "accepted|started|stdout|stderr|line|finished|failed",
  "subtone_pid": 12345,
  "exit_code": 0,
  "line": "text",
  "timestamp": "RFC3339Nano",
  "user": "llm-codex",
  "command": ["chrome", "src_v3", "status"]
}
```

### REPL Stream Frame (`dialtone.room.<room>`)
Use v1-compatible style plus explicit requester prefix:
```json
{
  "type": "line",
  "prefix": "llm-codex/command",
  "message": "chrome src_v3 status",
  "request_id": "uuid",
  "session_id": "uuid",
  "timestamp": "RFC3339Nano"
}
```

---

## Execution Semantics
1. Client submits request to `dialtone.cmd.submit`.
2. Daemon validates and replies `accepted`.
3. Daemon writes REPL line: `<user>/command> <command...>`.
4. Daemon launches subtone and emits `started`.
5. Daemon streams stdout/stderr as events + optional condensed room lines.
6. Daemon emits terminal event:
   - `finished` with `exit_code=0`, or
   - `failed` with non-zero code + reason.

Guarantees:
- Exactly one terminal event per request.
- Event ordering per session is monotonic.
- `request_id` idempotency key supported to avoid duplicate execution on client retry.
- Single active queue consumer for command execution (`dialtone.cmd.submit` leader queue semantics).

---

## REPL UX Expectations
- Incoming command injection appears in room immediately:
  - `llm-codex/command> <command...>`
- Start/stop markers shown with request/session IDs.
- `/ps` reflects active subtones regardless of requester.
- `/status` shows daemon health + NATS status + active rooms.

---

## Test Strategy (replace `--llm`)
Move all automation to NATS-driven command injection:

1. Start daemon in test harness.
2. Subscribe to `dialtone.cmd.events.<session_id>`.
3. Submit command N.
4. Wait for terminal event for command N.
5. Submit command N+1 (possibly with different `user` role).
6. Assert REPL stream contains `<role>/command>` lines in correct order.
7. Assert logs plugin stream contains matching lifecycle records for each request.

Required fixtures:
- `user=llm-codex` command sequence.
- mixed roles (`human`, `llm-codex`, `ci`) in same room.
- retry/idempotency behavior on same `request_id`.
- failure path (`failed` event + propagated exit code).

---

## Migration Plan
### Phase 1: Spec + compatibility shell
- Add daemon/nats schemas and subjects.
- Keep existing command path available behind fallback flag.

### Phase 2: Command bus first
- Route `dialtone.sh <command>` to bus by default.
- Keep direct path only as emergency fallback (`DIALTONE_DIRECT_EXEC=1`).

### Phase 3: Remove `--llm`
- Delete flag handling from launcher/repl.
- Move tests to NATS injector helper.

### Phase 4: Hard cut
- Remove direct execution fallback.
- Require daemon availability (auto-start if missing).

---

## Remaining Open Decisions
1. Daemon scope:
   - one daemon per machine, or one daemon per repo root.
2. Room verbosity policy:
   - full stdout/stderr mirror, or summarized lines + pointer.
3. Timeout policy details:
   - exact default client deadline and daemon execution timeout.
