# REPL src_v3

`repl src_v3` is the default runtime for `./dialtone.sh`.

Use this mental model:
- plain `./dialtone.sh <plugin> ...` injects into the local REPL leader
- the leader turns the real command into a task or manages a long-lived service
- NATS is the control plane for requests, lifecycle updates, and room traffic
- `dialtone>` stays short and high-level
- full output stays in the task log

Public terminology:
- `dialtone>`: the top-level control room
- `task`: one queued or running command request; each task gets a `task-id` immediately and may later get a local or remote PID
- `service`: one long-lived managed process, such as `chrome src_v3` on `legion`
- `room`: the event/log stream for a task or service

Compatibility note:
- the current implementation and some existing CLI commands still use the old `subtone` name internally
- the public direction is `task` everywhere, with PID treated as later runtime state instead of the public identity

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
dialtone> Task room: task.task-20260327-abc123
dialtone> Task log file: /home/user/dialtone/.dialtone/logs/task-20260327-abc123.log
dialtone> Task task-20260327-abc123 assigned pid 546289.
dialtone> robot diagnostic: checking local artifacts
dialtone> robot diagnostic: checking rover runtime on rover
dialtone> robot diagnostic: autoswap service and manifest look healthy
dialtone> robot diagnostic: active manifest matches latest release channel
dialtone> robot diagnostic: rover API and telemetry endpoints passed
dialtone> robot diagnostic: completed
dialtone> Task task-20260327-abc123 exited with code 0.
```

## REPL Standards

`dialtone>` should contain:
- request receipt
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

### Target REPL Operator Commands

The target REPL operator surface should make task and service state visible without forcing users to infer everything from one shared transcript:

```bash
./dialtone.sh repl src_v3 task list
./dialtone.sh repl src_v3 task show --task-id <task-id>
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200
./dialtone.sh repl src_v3 task kill --task-id <task-id>
./dialtone.sh repl src_v3 service list --host legion
./dialtone.sh repl src_v3 service show --host legion --name chrome-src-v3-dev
./dialtone.sh repl src_v3 watch --subject 'repl.room.index'
```

Target `task list` example with a running remote Chrome daemon:

```text
host-name> /repl src_v3 task list --state running --host legion
dialtone> Running tasks:
dialtone> TASK ID                  KIND      STATE    HOST    SERVICE/COMMAND           PID    EXIT
dialtone> task-20260327-chr001     service   running  legion  chrome-src-v3-dev        25516  -
dialtone> task-20260327-robot021   command   running  rover   robot src_v2 diagnostic  546289 -
```

Target `task show` example:

```text
host-name> /repl src_v3 task show --task-id task-20260327-chr001
dialtone> Task: task-20260327-chr001
dialtone> Kind: service_reconcile
dialtone> State: running
dialtone> Host: legion
dialtone> Service: chrome-src-v3-dev
dialtone> PID: 25516
dialtone> Task room: task.task-20260327-chr001
dialtone> Task log: /home/user/dialtone/.dialtone/logs/task-20260327-chr001.log
dialtone> Log subject: logs.service.legion.chrome-src-v3-dev
```

Target `service list` example:

```text
host-name> /repl src_v3 service list --host legion
dialtone> Services:
dialtone> HOST    NAME               ROLE       STATE    OWNER TASK              PID    HEALTH
dialtone> legion  chrome-src-v3-dev  dev        running  task-20260327-chr001   25516  healthy
dialtone> legion  chrome-src-v3-ui   robot-test running  task-20260327-ui001    25548  healthy
```

### Task Logs, Rooms, And NATS Logs

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
./dialtone.sh repl src_v3 task log --task-id task-20260327-chr001 --lines 80
./dialtone.sh logs src_v1 stream --topic 'logs.task.task-20260327-chr001'
./dialtone.sh logs src_v1 stream --topic 'logs.service.legion.chrome-src-v3-dev'
./dialtone.sh logs src_v1 stream --topic 'logfilter.level.error.>'
./dialtone.sh logs src_v1 stream --topic 'logfilter.tag.fail.>'
```

Target `task log` example:

```text
host-name> /repl src_v3 task log --task-id task-20260327-chr001 --lines 6
dialtone> Streaming task log for task-20260327-chr001
[T+0000s|INFO|plugins/chrome/src_v3/ops.go:412] deploy requested for legion role=dev
[T+0001s|INFO|plugins/chrome/src_v3/daemon.go:221] daemon connected to repl manager
[T+0002s|INFO|plugins/chrome/src_v3/browser.go:301] chrome started headed pid=25516
[T+0003s|INFO|plugins/chrome/src_v3/browser.go:362] managed tab ready target=7E1A3D
```

Target `logs stream` example:

```text
host-name> /logs src_v1 stream --topic 'logs.service.legion.chrome-src-v3-dev'
dialtone> Attached log stream logs.service.legion.chrome-src-v3-dev
[T+0000s|INFO|plugins/chrome/src_v3/daemon.go:144] service heartbeat healthy pid=25516
[T+0001s|INFO|plugins/chrome/src_v3/browser.go:488] command goto url=http://127.0.0.1:3000
[T+0002s|ERROR|plugins/chrome/src_v3/actions.go:211] wait-aria timeout label=Open Camera
```

### Remote Chrome Service Walkthrough

The most important long-running REPL service right now is the remote Chrome daemon on `legion`.

Typical workflow:

```text
host-name> /chrome src_v3 service --host legion --mode start --role robot-test
dialtone> Request received.
dialtone> Task queued as task-20260327-chr001.
dialtone> Task room: task.task-20260327-chr001
dialtone> Task log: /home/user/dialtone/.dialtone/logs/task-20260327-chr001.log
dialtone> Task task-20260327-chr001 assigned pid 25516 on legion.
dialtone> chrome service on legion role=robot-test is healthy.
dialtone> Task task-20260327-chr001 exited with code 0.

host-name> /repl src_v3 task list --state running --host legion
dialtone> Running tasks:
dialtone> TASK ID                  KIND      STATE    HOST    SERVICE/COMMAND           PID    EXIT
dialtone> task-20260327-chr001     service   running  legion  chrome-src-v3-robot-test 25516  -

host-name> /chrome src_v3 goto --host legion --role robot-test --url http://127.0.0.1:3000/#robot-three-stage
dialtone> Request received.
dialtone> Task queued as task-20260327-nav001.
dialtone> Task task-20260327-nav001 assigned pid 25516 on legion.
dialtone> chrome goto on legion role=robot-test completed.
dialtone> Task task-20260327-nav001 exited with code 0.
```

This is the same control pattern that should support:

- `robot src_v2 dev --browser-node legion`
- `robot src_v2 test`
- `ui src_v1 test --attach legion`
- direct browser-driving commands like `goto`, `click-aria`, and `screenshot`

### Errors, Kill, And Recovery

The REPL should make failures obvious without flooding the operator with raw internals.

Target failure example for a Chrome command:

```text
host-name> /chrome src_v3 wait-aria --host legion --role robot-test --label "Open Camera" --timeout-ms 1500
dialtone> Request received.
dialtone> Task queued as task-20260327-wait001.
dialtone> Task task-20260327-wait001 assigned pid 25516 on legion.
dialtone> ERROR task task-20260327-wait001 on legion exited with code 28.
dialtone> ERROR task task-20260327-wait001 wait-aria timeout label=Open Camera
dialtone> Task log: /home/user/dialtone/.dialtone/logs/task-20260327-wait001.log
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

Target recovery example:

```text
host-name> /chrome src_v3 service --host legion --mode start --role robot-test
dialtone> Request received.
dialtone> Task queued as task-20260327-chr002.
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

### Compatibility Commands

Until the rename lands, the current compatibility commands are still:

```bash
./dialtone.sh repl src_v3 subtone-list --count 20
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200
```

These should be treated as migration helpers, not the final operator surface.

### Queued Commands

Target behavior:

- every request queues as a task and returns immediately with a `task-id`
- explicit trailing `&` becomes unnecessary for normal command submission
- the leader decides when the task gets a PID and starts running

Current compatibility behavior still allows one command with a trailing `&`:

```bash
./dialtone.sh repl src_v3 inject --user llm-codex "repl src_v3 watch --subject repl.room.index &"
./dialtone.sh repl src_v3 inject --user llm-codex "repl src_v3 watch --subject 'repl.host.>' &"
```

Preferred queued pattern:

```text
host-name> /repl src_v3 watch --subject repl.room.index
dialtone> Request received.
dialtone> Task queued as task-20260327-def456.
dialtone> Task room: task.task-20260327-def456
dialtone> Task log file: /home/user/dialtone/.dialtone/logs/task-20260327-def456.log
dialtone> Task task-20260327-def456 assigned pid 171214.
dialtone> Task task-20260327-def456 is running.
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
dialtone> ERROR: run exactly one ./dialtone.sh command at a time; command chaining with "&&" is not allowed. Use one foreground command per turn, or a single command with a trailing & for background mode.
```

## Explicit REPL Commands

Use these only when you need direct REPL control.

```bash
./dialtone.sh repl src_v3 task list
./dialtone.sh repl src_v3 task show --task-id <task-id>
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200
./dialtone.sh repl src_v3 service list --host legion

# Start or inspect the local leader directly.
./dialtone.sh repl src_v3 leader --nats-url nats://127.0.0.1:47222 --room index
./dialtone.sh repl src_v3 status

# Inject to a specific leader or room.
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

Use the `wsl-tmux` wrapper from Windows so commands stay visible in the persistent WSL tmux pane:

```powershell
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh repl src_v3 process-clean"
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh repl src_v3 test"
wsl-tmux read
wsl-tmux interrupt
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
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh repl src_v3 process-clean"

# 2. Run the full REPL suite visibly in WSL.
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh repl src_v3 test"

# 3. If needed, rerun a focused step.
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh repl src_v3 test --filter interactive-ssh-wsl-command"

# 4. Inspect the generated report files in the WSL repo.
wsl-tmux "cd /home/user/dialtone && sed -n '1,80p' src/plugins/repl/src_v3/TEST.md"
```

For this environment, the SSH-backed REPL test path is most reliable when the reachable default node is `grey`:

```bash
./dialtone.sh ssh src_v1 probe --host grey --timeout 5s
./dialtone.sh ssh src_v1 run --host grey --cmd whoami
```

If an SSH-focused REPL test refers to the logical host name `wsl`, the current test setup may still resolve that through `grey.shad-artichoke.ts.net` as the preferred reachable transport target.

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
