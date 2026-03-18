## Process Manager Plan: 2026-03-18

### Short Answer

`src/plugins/proc/` is useful, but it is too small and too in-memory to be the full solution today.

Current status:

- useful as the process/subtone execution library
- useful for metrics (`cpu`, `rss`, `ports`)
- useful for event streaming (`started`, `stdout`, `stderr`, `exited`)
- outdated as a standalone process manager

So the right move is:

- keep `proc` as the host-local execution/runtime library
- move real supervisor logic into the REPL background runtime
- expand `proc` to support the process-manager features the REPL needs

### What `proc` Already Does Well

From `src/plugins/proc/src_v1/go/proc`:

- `RunSubtoneWithEvents(...)`
- `RunHostCommandWithEvents(...)`
- `TrackProcess(...)`
- `UntrackProcess(...)`
- `ListManagedProcesses()`
- `KillManagedProcess(pid)`

That makes `proc` a good foundation for:

- spawning commands
- tracking local pids
- collecting process metrics
- streaming lifecycle events into REPL

This is already why REPL can show:

- pid
- uptime
- cpu
- ports
- subtone logs

### Where `proc` Is Too Weak

The current `proc` model uses an in-memory map:

- if the owning process exits, tracking is gone
- it does not persist process metadata to disk
- it has no supervisor concept
- it does not distinguish:
  - foreground
  - background
  - service
- it does not support queue/admission policy
- it does not support restart policy
- it does not support process tree stop/terminate semantics well enough
- it does not rebuild state from live OS processes on startup

That means it is a library, not a durable process manager.

### What We Learned From Real Plugin Work

#### 1. `robot src_v2 publish`

What worked:

- subtone logging
- high-level `DIALTONE>` summaries

What hurt:

- when the leader disappeared between commands, the publish path still worked, but the runtime felt fragile

Implication:

- the process manager must make the leader stable between commands

#### 2. `cad src_v1 dev`

What worked:

- long-lived background dev loop
- Vite + backend workflow

What hurt:

- the leader still treated it like an active subtone without enough lifecycle control
- later foreground commands could appear blocked or confusing

Implication:

- long-lived commands need an explicit `service` or `background` mode
- operators need first-class stop/restart commands

#### 3. Help/status/process listing commands

What worked:

- `subtone-list`
- `ps`
- subtone logs

What hurt:

- the system could tell us a pid existed, but not whether it was:
  - intended to be long-lived
  - stale
  - blocking foreground work

Implication:

- listing is not enough
- mode and ownership must be explicit

### Better System Boundary

The better design is:

#### `proc`

Owns:

- spawn process
- stream stdout/stderr/lifecycle events
- collect process metrics
- terminate process tree
- optionally persist/reload process metadata

Should not own:

- command queue policy
- REPL rooms
- NATS transport
- plugin-facing status UX

#### `repl src_v3`

Owns:

- command routing from `./dialtone.sh`
- foreground FIFO queue
- background/service admission
- subtone rooms + index-room status
- subtone registry UX
- leader health
- supervisor health

#### `dialtone.sh`

Owns:

- bootstrap/env resolution
- detect local runtime
- proxy command to background REPL runtime
- stay thin

### Recommended Runtime Model

One background REPL runtime per machine/user.

That runtime should own:

- one supervisor
- one leader worker
- one local NATS endpoint
- one persistent process registry

All `./dialtone.sh <plugin> ...` commands should:

1. resolve config
2. connect to local REPL runtime
3. submit a command
4. receive index-room lifecycle back

Not:

1. spawn ad hoc detached leaders
2. guess process state from raw pids

### Command Model

Every routed command becomes a managed unit.

Suggested modes:

- `foreground`
- `background`
- `service`
- `oneshot`

Suggested states:

- `queued`
- `starting`
- `running`
- `stopping`
- `exited`
- `failed`

Suggested metadata:

- command id
- pid
- parent pid
- mode
- state
- command args
- started_at
- updated_at
- log_path
- exit_code
- restart_count
- owner room
- owner prompt/client

### FIFO Policy

Foreground commands should be FIFO.

That fits what you want:

- one clear active foreground command at a time
- later foreground commands queue behind it
- background/service commands keep running

Recommended policy:

- one active foreground command
- unlimited background/service commands within policy limits
- index room prints:
  - `queued behind pid ...`
  - `starting`
  - `running`
  - `completed`

### What To Add To `proc`

#### Phase 1

Add durable tracking:

- persisted registry file under `.dialtone/proc/`
- pid + mode + state + args + log path

Add better stop semantics:

- terminate process tree
- graceful stop with timeout
- hard kill fallback

#### Phase 2

Add recovery helpers:

- rebuild registry from live pids on startup
- mark dead entries exited/stale
- detect orphaned processes

#### Phase 3

Add restart policy support for `service` mode:

- `none`
- `on-failure`
- `always`

This is especially useful for:

- REPL leader worker
- Chrome daemon
- dev servers

### What To Add To REPL

#### Phase 1

Switch the background runtime to a supervised REPL service.

#### Phase 2

Use `proc` metadata as the subtone/process truth source.

#### Phase 3

Expose first-class commands:

- `repl src_v3 jobs`
- `repl src_v3 stop --pid <pid>`
- `repl src_v3 kill --pid <pid>`
- `repl src_v3 restart --pid <pid>`
- `repl src_v3 doctor`

#### Phase 4

Make `subtone-list` show:

- mode
- state
- queue position
- uptime
- cpu
- ports
- command

### README Changes Needed

`src/plugins/repl/src_v3/README.md` is directionally right, but it should be updated to describe:

- supervisor-first runtime, not raw leader-first runtime
- FIFO foreground queue
- background/service modes
- process-manager semantics for subtones
- recovery commands for stale jobs

### Recommendation

Do not throw `proc` away.

Instead:

- keep `proc` as the execution/runtime library
- grow it into a durable process-state layer
- let REPL become the host-local process manager UX
- let `dialtone.sh` become a thin client to that runtime

That gives a clearer stack:

- `dialtone.sh` = client
- `repl src_v3` = supervisor + queue + UX
- `proc` = process execution + metrics + lifecycle
