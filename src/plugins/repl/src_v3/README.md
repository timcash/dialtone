# REPL src_v3

`repl src_v3` is the default runtime for `./dialtone.sh`.

Use this mental model:
- plain `./dialtone.sh <plugin> ...` injects into the local REPL leader
- queued task submission is the default; only explicit query/operator commands stay foreground
- the leader turns the real command into a task or manages a long-lived service
- NATS is the control plane for requests, lifecycle updates, and topic traffic
- NATS KV stores durable task and service state
- service tasks publish heartbeats and the leader reconciles them if heartbeats stop
- `dialtone>` stays short and high-level
- full output stays in the task log

Public terminology:
- `dialtone>`: the top-level REPL and CLI control prefix
- `task`: one queued or running command request; each task gets a `task-id` immediately and may later get a local or remote PID
- `service`: one long-lived managed process, such as `chrome src_v3` on `legion`
- `topic`: the event/log stream for a task or service

## Control Plane Rules

These are the key rules the runtime should follow:

- every queued command gets a `task-id` before process launch
- `task-id` is the primary identity; PID is only an observed runtime field that may appear later
- task creation, task updates, and task lookup should be backed by NATS KV state
- service identity should be `(host, service-name)`, not just a bare service name
- service desired state and observed state should be backed by NATS KV state
- service heartbeats update observed state continuously
- missed heartbeats mark services unhealthy and should trigger reconcile or restart when desired state still says `running`
- query commands should read the control-plane state, not rely only on ad hoc process inspection

Short architecture summary:

`./dialtone.sh` -> leader autostart if needed -> command publish -> task record in NATS KV -> execution and log streaming -> service heartbeats -> reconcile loop

That is the center of gravity for `repl src_v3`: a control plane with durable state, not a thin wrapper around a single local process launch.

## Default Use

```bash
# Run a normal command through the local REPL leader.
./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui --public-check=false

# Publish a new robot release through the same path.
./dialtone.sh robot src_v2 publish --repo timcash/dialtone

# Run a simple SSH action through the REPL path.
./dialtone.sh ssh src_v1 run --host rover --cmd hostname

# Manage a long-lived Chrome daemon role through the same path.
./dialtone.sh chrome src_v3 service --host legion --mode start --role dev
./dialtone.sh chrome src_v3 status --host legion --role dev
```

Preferred shell pattern:

```text
host-name> /robot src_v2 diagnostic --host rover --skip-ui --public-check=false
dialtone> Request received.
dialtone> Task queued as task-20260327-abc123.
dialtone> Task topic: task.task-20260327-abc123
dialtone> Task log: ~/.dialtone/logs/task-20260327-abc123.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-abc123 --lines 10
```

The one-shot CLI should return immediately after these lines. Later lifecycle belongs in the shared REPL stream, `task show`, `service show`, and the task log.

Foreground query/operator commands are the exception to that queued pattern. They should stay synchronous, print the requested data directly, and may warm the background leader first so later queued work does not need a cold bootstrap.

## REPL Standards

For one-shot CLI commands, `dialtone>` should contain:
- request receipt
- queued task metadata
- a task log follow-up command

For explicit foreground query/operator commands, stdout should contain:
- the requested data
- optional leader-autostart preamble if the background leader was missing

Inside the shared REPL stream or a live watch session, `dialtone>` may also contain:
- task lifecycle
- service lifecycle
- short stage summaries
- final success or failure

`dialtone>` should not contain:
- raw JSON
- stack traces
- long build output
- repeated polling noise
- browser console spam

That detail belongs in the task log.

## Env Roots And Leader Isolation

By default, `./dialtone.sh` should use the `env/dialtone.json` that lives in the folder you launched from.

Use `--env` when you want to point at a different env root or a specific config file:

```bash
./dialtone.sh --env /tmp/dialtone-demo/env robot src_v2 publish --skip-release --ui
./dialtone.sh --env /tmp/dialtone-demo/env repl src_v3 task list
```

Important expectations:

- launch-folder default config and explicit `--env` runs should not bleed into each other
- the leader should reuse the matching env root instead of silently crossing into another one
- temp env roots are the right way to test isolated leader bootstrap and task state
- query commands should still warm the correct background leader when needed

## Tasks, Services, And The Operator Surface

Use normal plugin commands for one-shot work:

```bash
./dialtone.sh cad src_v1 build
./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui --public-check=false
./dialtone.sh ssh src_v1 run --host grey --cmd hostname
```

Use service mode for long-lived processes:

```bash
./dialtone.sh chrome src_v3 service --host legion --mode start --role cad-smoke
./dialtone.sh chrome src_v3 status --host legion --role cad-smoke
./dialtone.sh chrome src_v3 service --host legion --mode stop --role cad-smoke
```

The intended service contract is:

- `service start` reconciles desired running state
- `status` queries existing observed state
- later commands reuse the existing healthy service
- `service stop` clears desired running state and shuts down the owned process
- service heartbeats update observed state
- missed heartbeats mark the service unhealthy and trigger reconcile/restart

### Shared Service Contract Fixture

Use the fixture at `src/plugins/testdaemon/src_v1` to prove the shared service layer without depending on Chrome or any other plugin implementation.

```bash
./dialtone.sh testdaemon src_v1 service --host legion --mode start --name demo
./dialtone.sh testdaemon src_v1 service --host legion --mode status --name demo
./dialtone.sh testdaemon src_v1 service --host legion --mode stop --name demo
./dialtone.sh testdaemon src_v1 emit-progress --steps 5
./dialtone.sh testdaemon src_v1 exit-code --code 17
./dialtone.sh testdaemon src_v1 panic
./dialtone.sh testdaemon src_v1 hang
./dialtone.sh testdaemon src_v1 heartbeat --name demo --mode stop
./dialtone.sh testdaemon src_v1 heartbeat --name demo --mode resume
./dialtone.sh testdaemon src_v1 shutdown --name demo
```

The current local REPL fixture slice should at least prove build, one-shot progress, nonzero exit, panic, hang, service start or stop, heartbeat advance, and heartbeat pause or resume against this fixture before later KV, reconcile, or remote claims are treated as complete.

Target one-shot service submission pattern:

```text
host-name> /testdaemon src_v1 service --host legion --mode start --name demo
dialtone> Request received.
dialtone> Task queued as task-20260327-svc001.
dialtone> Task topic: task.task-20260327-svc001
dialtone> Task log: ~/.dialtone/logs/task-20260327-svc001.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-svc001 --lines 10
```

### Target REPL Operator Commands

The target REPL operator surface should make task and service state visible without forcing users to infer everything from one shared transcript:

```bash
./dialtone.sh repl src_v3 task list
./dialtone.sh repl src_v3 task show --task-id <task-id>
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200
./dialtone.sh repl src_v3 task kill --task-id <task-id>
./dialtone.sh repl src_v3 service list --host legion
./dialtone.sh repl src_v3 service show --host legion --name testdaemon-demo
./dialtone.sh repl src_v3 watch --subject 'repl.topic.index'
```

These commands should read canonical control-plane state:

- `task list` and `task show` should reflect NATS KV-backed task state
- `service list` and `service show` should reflect NATS KV-backed desired and observed service state
- `task log` should read the durable task log directly
- `watch` and `logs src_v1 stream` are event views, not the source of truth

Target `task list` example with a running remote service:

```text
host-name> /repl src_v3 task list --state running --host legion
dialtone> Running tasks:
dialtone> TASK ID                  KIND      STATE    HOST    SERVICE/COMMAND           PID    EXIT
dialtone> task-20260327-svc001     service   running  legion  testdaemon-demo          25516  -
dialtone> task-20260327-robot021   command   running  rover   robot src_v2 diagnostic  546289 -
```

Target `task show` example:

```text
host-name> /repl src_v3 task show --task-id task-20260327-svc001
dialtone> Task: task-20260327-svc001
dialtone> Kind: service_reconcile
dialtone> State: running
dialtone> Host: legion
dialtone> Service: testdaemon-demo
dialtone> PID: 25516
dialtone> Task topic: task.task-20260327-svc001
dialtone> Task log: ~/.dialtone/logs/task-20260327-svc001.log
dialtone> Log subject: logs.service.legion.testdaemon-demo
```

Target `service list` example:

```text
host-name> /repl src_v3 service list --host legion
dialtone> Services:
dialtone> HOST    NAME               ROLE       STATE    OWNER TASK              PID    HEALTH
dialtone> legion  testdaemon-demo    demo       running  task-20260327-svc001   25516  healthy
dialtone> legion  testdaemon-metrics metrics    running  task-20260327-svc002   25548  healthy
```

### Task Logs, Topics, And NATS Logs

The long-term logging model should follow the logs plugin:

- producers publish logs to NATS
- `dialtone>` renders short lifecycle lines for humans
- a task log writer persists a durable per-task record
- `logs src_v1 stream` can subscribe to the same subjects live

Target subject model:

- `logs.task.<task-id>`
- `logs.service.<host>.<service-name>`
- `logfilter.level.error.>`
- `logfilter.tag.fail.>`

Useful operator commands:

```bash
./dialtone.sh repl src_v3 task log --task-id task-20260327-svc001 --lines 80
./dialtone.sh logs src_v1 stream --topic 'logs.task.task-20260327-svc001'
./dialtone.sh logs src_v1 stream --topic 'logs.service.legion.testdaemon-demo'
./dialtone.sh logs src_v1 stream --topic 'logfilter.level.error.>'
./dialtone.sh logs src_v1 stream --topic 'logfilter.tag.fail.>'
```

Target `task log` example:

```text
host-name> /repl src_v3 task log --task-id task-20260327-svc001 --lines 6
dialtone> Streaming task log for task-20260327-svc001
[T+0000s|INFO|plugins/repl/src_v3/testdaemon/src_v1/service.go:51] service start requested host=legion name=demo
[T+0001s|INFO|plugins/repl/src_v3/testdaemon/src_v1/daemon.go:88] daemon connected to repl manager
[T+0002s|INFO|plugins/repl/src_v3/testdaemon/src_v1/daemon.go:104] heartbeat healthy pid=25516
[T+0003s|INFO|plugins/repl/src_v3/testdaemon/src_v1/daemon.go:117] service ready host=legion name=demo
```

Target `logs stream` example:

```text
host-name> /logs src_v1 stream --topic 'logs.service.legion.testdaemon-demo'
dialtone> Attached log stream logs.service.legion.testdaemon-demo
[T+0000s|INFO|plugins/repl/src_v3/testdaemon/src_v1/daemon.go:104] service heartbeat healthy pid=25516
[T+0001s|INFO|plugins/repl/src_v3/testdaemon/src_v1/daemon.go:128] emit-progress stage=ready
[T+0002s|ERROR|plugins/repl/src_v3/testdaemon/src_v1/daemon.go:141] exit-code requested code=17
```

### Chrome Browser Integration Walkthrough

Chrome is an important browser-specific service integration, but the shared service contract itself should be validated first with `testdaemon`.

Typical workflow:

```text
host-name> /chrome src_v3 service --host legion --mode start --role robot-test
dialtone> Request received.
dialtone> Task queued as task-20260327-chr001.
dialtone> Task topic: task.task-20260327-chr001
dialtone> Task log: ~/.dialtone/logs/task-20260327-chr001.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-chr001 --lines 10

host-name> /repl src_v3 task list --state running --host legion
dialtone> Running tasks:
dialtone> TASK ID                  KIND      STATE    HOST    SERVICE/COMMAND           PID    EXIT
dialtone> task-20260327-chr001     service   running  legion  chrome-src-v3-robot-test 25516  -

host-name> /chrome src_v3 goto --host legion --role robot-test --url http://127.0.0.1:3000/#robot-three-stage
dialtone> Request received.
dialtone> Task queued as task-20260327-nav001.
dialtone> Task topic: task.task-20260327-nav001
dialtone> Task log: ~/.dialtone/logs/task-20260327-nav001.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-nav001 --lines 10
```

This is the same control pattern that should support:

- `robot src_v2 dev --browser-node legion`
- `robot src_v2 test`
- `ui src_v1 test --attach legion`
- direct browser-driving commands like `goto`, `click-aria`, and `screenshot`

## Priority Order For Proof

The REPL core should be considered proven in this order:

1. `testdaemon` exists and can drive the shared task and service suites
2. task state is durable and queryable through NATS KV
3. service desired and observed state is durable and queryable through NATS KV
4. heartbeat loss marks services unhealthy and the reconcile loop restarts them when desired state still says `running`
5. remote `testdaemon` service behavior on `legion` proves the same semantics as local runs
6. only after that should Chrome, robot, Cloudflare, and public-edge flows be treated as layered integration proof

Chrome is still an important integration, but it should not be the thing that proves the generic control plane works.

For now, `./dialtone.sh repl src_v3 test` intentionally excludes the longer SSH- and Cloudflare-dependent integration suites from the default gate.
That includes the old SSH fixture observability and attach slices as well as the explicit SSH and Cloudflare suites.
Keep the default loop focused on core REPL behavior, `testdaemon`, and KV-backed task or service state.
Those integration suites can return later as separate opt-in coverage once the core proof surface is stable again.

### Errors, Kill, And Recovery

The REPL should make failures obvious without flooding the operator with raw internals.

Later failure lines in the shared REPL stream or task log for a Chrome command can look like:

```text
dialtone> Task task-20260327-wait001 assigned pid 25516 on legion.
dialtone> ERROR task task-20260327-wait001 on legion exited with code 28.
dialtone> ERROR task task-20260327-wait001 wait-aria timeout label=Open Camera
dialtone> Task log: ~/.dialtone/logs/task-20260327-wait001.log
```

Target kill example:

```text
host-name> /repl src_v3 task kill --task-id task-20260327-chr001
dialtone> Request received.
dialtone> Task queued as task-20260327-kill01.
dialtone> Kill requested for task-20260327-chr001.
dialtone> Target task task-20260327-chr001 is running on legion pid 25516.
dialtone> Service chrome-src-v3-robot-test on legion moved to stopping.
dialtone> Target task task-20260327-chr001 exited with code 143.
dialtone> Task task-20260327-kill01 exited with code 0.
```

Later recovery lines in the shared REPL stream can look like:

```text
dialtone> Existing chrome service on legion role=robot-test is missing.
dialtone> Task task-20260327-chr002 assigned pid 26111 on legion.
dialtone> chrome service on legion role=robot-test recovered and healthy.
dialtone> Task task-20260327-chr002 exited with code 0.
```

### Interleaved Long-Running Work

The shared `dialtone>` stream can contain service heartbeats, command tasks, and failure lines from many places at once.

```text
host-name> /chrome src_v3 service --host legion --mode start --role robot-test
dialtone> Task queued as task-20260327-chr001.

host-name> /robot src_v2 diagnostic --host rover --skip-ui --public-check=false
dialtone> Task queued as task-20260327-robot021.

host-name> /chrome src_v3 wait-aria --host legion --role robot-test --label "Open Camera" --timeout-ms 1500
dialtone> Task queued as task-20260327-wait001.

dialtone> Task task-20260327-chr001 assigned pid 25516 on legion.
dialtone> Task task-20260327-robot021 assigned pid 546289 on rover.
dialtone> ERROR task task-20260327-wait001 on legion exited with code 28.
dialtone> robot diagnostic: autoswap service and manifest look healthy
dialtone> chrome service on legion role=robot-test is healthy.
dialtone> Task task-20260327-chr001 exited with code 0.
dialtone> Task task-20260327-robot021 exited with code 0.
```

The main invariants are:

- task ids are always present on lifecycle and error lines
- task and service inspection commands can reconstruct current state even if the top-level transcript is interleaved
- `logs src_v1 stream` can attach to live task or service subjects for detail
- the durable task log remains the best source for a finished task

### Queued Commands

Target behavior:

- every request queues as a task and returns immediately with a `task-id`
- explicit trailing `&` becomes unnecessary for normal command submission
- the leader decides when the task gets a PID and starts running

Preferred queued pattern:

```text
host-name> /robot src_v2 publish --repo timcash/dialtone
dialtone> Request received.
dialtone> Task queued as task-20260327-def456.
dialtone> Task topic: task.task-20260327-def456
dialtone> Task log: ~/.dialtone/logs/task-20260327-def456.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-def456 --lines 10
```

## Single Command Rule

Run one `./dialtone.sh` command per turn.

These are rejected:

```bash
# Do not chain Dialtone commands like this.
./dialtone.sh robot src_v2 diagnostic && ./dialtone.sh autoswap src_v1 update --host rover

# Do not pass multiple commands into one Dialtone invocation.
./dialtone.sh robot src_v2 diagnostic '&&' autoswap src_v1 update --host rover
```

Error pattern:

```text
dialtone> ERROR: run exactly one ./dialtone.sh command at a time; command chaining with "&&" is not allowed.
```

## Explicit REPL Commands

Use these only when you need direct REPL control.

```bash
./dialtone.sh repl src_v3 task list
./dialtone.sh repl src_v3 task show --task-id <task-id>
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200
./dialtone.sh repl src_v3 service list --host legion

# Start or inspect the local leader directly.
./dialtone.sh repl src_v3 leader --nats-url nats://127.0.0.1:47222 --topic index
./dialtone.sh repl src_v3 status

# Inject to a specific leader or topic.
./dialtone.sh repl src_v3 inject --nats-url nats://127.0.0.1:47222 --user llm-codex robot src_v2 publish --repo timcash/dialtone

# Clean local REPL helper processes.
./dialtone.sh repl src_v3 process-clean
```

Use direct `repl src_v3` commands when you need to debug the runtime itself, not for ordinary plugin work.

```bash
# Clean local REPL helper processes before a fresh runtime test.
./dialtone.sh repl src_v3 process-clean

# Then let the next ordinary plugin command autostart the leader again.
./dialtone.sh chrome src_v3 status --host legion --role dev
```

## Windows To WSL Workflow

If you are editing from Windows but running the real runtime in WSL, keep this split:

- edit in `C:\Users\timca\dialtone`
- run REPL and plugin tests in `/home/user/dialtone`
- keep the WSL tmux session `windows` alive and reuse it

If you are on Windows, use [dialtone.ps1](/C:/Users/timca/dialtone/dialtone.ps1) `tmux` so REPL and plugin commands stay visible in the persistent WSL tmux pane:

```powershell
.\dialtone.ps1 tmux clean-state
.\dialtone.ps1 tmux "./dialtone.sh repl src_v3 process-clean"
.\dialtone.ps1 tmux "./dialtone.sh repl src_v3 test"
.\dialtone.ps1 tmux read
.\dialtone.ps1 tmux interrupt
```

For this repo, trust:

- native Windows Git for `C:\Users\timca\dialtone`
- WSL Git for `/home/user/dialtone`

Do not judge the Windows checkout from `/mnt/c/...` inside WSL because line endings and file mode handling can make that view misleading.

If you sync files from Windows into WSL, normalize line endings in WSL before testing:

```bash
perl -0pi -e 's/\r\n/\n/g' path/to/file
```

For a fuller Windows/WSL workflow, see the root [README.md](/C:/Users/timca/dialtone/README.md).

## Host Flags

For normal plugin commands, `--host` usually belongs to the plugin itself:

```bash
# Here --host means the rover target for robot.
./dialtone.sh robot src_v2 diagnostic --host rover

# Here --host means the rover target for autoswap.
./dialtone.sh autoswap src_v1 update --host rover

# Here --host means the legion target for chrome.
./dialtone.sh chrome src_v3 status --host legion --role dev
```

If you need REPL transport routing itself, use `--target-host` or `--ssh-host`, not the plugin-local `--host`.

## For LLM Agents

Use this default workflow:

```bash
# 1. Run one command.
./dialtone.sh robot src_v2 publish --repo timcash/dialtone

# 2. If it fails or looks incomplete, inspect task and service state first.
./dialtone.sh repl src_v3 task list
./dialtone.sh repl src_v3 service list --host legion
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200
./dialtone.sh logs src_v1 stream --topic 'logfilter.level.error.>'

# 3. Then run the next command.
./dialtone.sh autoswap src_v1 update --host rover
./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui --public-check=false
```

Do not guess from partial `dialtone>` output when a task log is available.

When working from Windows, prefer this testing pattern:

```powershell
# 1. Keep the persistent WSL tmux session alive.
.\dialtone.ps1 tmux clean-state
.\dialtone.ps1 tmux "./dialtone.sh repl src_v3 process-clean"

# 2. Run the full REPL suite visibly in WSL.
.\dialtone.ps1 tmux "./dialtone.sh repl src_v3 test"

# 3. If needed, rerun a focused step.
.\dialtone.ps1 tmux "./dialtone.sh repl src_v3 test --filter interactive-command-index-lifecycle-contract"
.\dialtone.ps1 tmux "./dialtone.sh repl src_v3 test --filter task-list-reads-from-kv,task-show-reads-from-kv"
.\dialtone.ps1 tmux "./dialtone.sh testdaemon src_v1 test"

# 4. Inspect the generated report files in the WSL repo.
.\dialtone.ps1 tmux "sed -n '1,80p' src/plugins/repl/src_v3/TEST.md"
```

For Chrome/CAD/UI debugging, use this pattern:

```bash
# 1. Start the daemon role once.
./dialtone.sh chrome src_v3 service --host legion --mode start --role cad-smoke

# 2. Confirm the daemon is healthy.
./dialtone.sh chrome src_v3 status --host legion --role cad-smoke
./dialtone.sh repl src_v3 task list --state running --host legion
./dialtone.sh repl src_v3 service list --host legion

# 3. Run the browser-driven command or test against that role.
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke

# 4. If it fails, inspect the task log.
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 250
./dialtone.sh logs src_v1 stream --topic 'logs.service.legion.chrome-src-v3-cad-smoke'
```

The main rule is:
- keep the leader running
- keep long-lived daemons running
- send commands over NATS through the REPL
- only redeploy or restart when health actually requires it
