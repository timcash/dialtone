# Dialtone

Dialtone is a task-first CLI and REPL runtime for plugin work, remote process control, and long-lived services.

The intended model is:

- `./dialtone.sh <plugin> ...` submits one task to the local REPL leader
- queued task submission is the default; only explicit query/operator commands stay foreground
- the leader keeps durable task and service state in NATS KV
- the launch folder's `env/dialtone.json` is the default runtime config source, and `--env` can point at another env root or file
- `dialtone>` stays short and high-level
- full detail belongs in the task log, task topic, and service state
- there is no public `subtone` language; `task` and `service` are the public operator terms

For long-lived services like `chrome src_v3`, the REPL is the control plane that should start, reuse, inspect, and reconcile the remote daemon instead of every plugin inventing its own launcher flow.
Service tasks should publish heartbeats, and the REPL leader should reconcile or restart them if those heartbeats stop.

## Using `dialtone.sh`

Use one Dialtone command per invocation:

```bash
./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui --public-check=false
./dialtone.sh ssh src_v1 run --host grey --cmd whoami
./dialtone.sh chrome src_v3 service --host legion --mode start --role dev
./dialtone.sh chrome src_v3 status --host legion --role dev
```

## Agent Rules

If you are working in this repo as an LLM agent or operator, use these rules:

- always prefer `./dialtone.sh <plugin> <src_vN> <command>` over raw `go`, `gofmt`, `go test`, `bun`, or ad hoc shell scripts
- use plugin verbs like `install`, `format`, `lint`, `build`, and `test` instead of calling toolchains directly
- use `./dialtone.sh repl src_v3 ...` for REPL operator queries and controls
- use `./dialtone.sh logs src_v1 ...` for NATS log streaming and NATS daemon checks
- use `./dialtone.sh` with no arguments when you want the shared interactive REPL
- use one Dialtone command per invocation; do not chain commands with `&&`, `||`, or `;`
- treat `dialtone.sh` as the public workflow layer and NATS as the control plane underneath it

For normal development, these are the preferred commands:

```bash
./dialtone.sh <plugin> <src_vN> install
./dialtone.sh <plugin> <src_vN> format
./dialtone.sh <plugin> <src_vN> lint
./dialtone.sh <plugin> <src_vN> build
./dialtone.sh <plugin> <src_vN> test
./dialtone.sh <plugin> <src_vN> test --filter <expr>
```

Do not replace those with:

- `go fmt ./...`
- `go build ./...`
- `go test ./...`
- `bun test`
- raw `nats` CLI calls

If you need lower-level tool access for debugging, route it through a Dialtone workflow when possible, for example `./dialtone.sh go src_v1 exec ...`, but prefer the plugin verb first.

## Choosing A Path

Use the one-shot CLI when you want one command from the shell:

- queued task submission for normal work like `robot publish`, `ssh run`, or `chrome service start`
- direct foreground output for explicit query/operator commands like `proc ps`, `task log`, or `logs stream`

Use the REPL when you want:

- many commands in one session
- shared `dialtone>` lifecycle output
- interleaved task/service progress
- quick follow-up slash commands without restarting the wrapper

Use `--env` when you want to run from a different runtime/config root:

```bash
./dialtone.sh --env /tmp/dialtone-demo/env robot src_v2 publish --skip-release --ui
./dialtone.sh --env /tmp/dialtone-demo/env repl src_v3 task list
```

Without `--env`, Dialtone uses `env/dialtone.json` in the folder you launch from.

## Working With Plugins

The generic plugin command shape is:

```bash
./dialtone.sh <plugin-name> <src_vN> <command> [args] [--flags]
```

Most plugins should expose the standard development verbs:

```bash
./dialtone.sh <plugin-name> <src_vN> install
./dialtone.sh <plugin-name> <src_vN> format
./dialtone.sh <plugin-name> <src_vN> lint
./dialtone.sh <plugin-name> <src_vN> build
./dialtone.sh <plugin-name> <src_vN> test
```

Examples:

```bash
./dialtone.sh repl src_v3 install
./dialtone.sh repl src_v3 format
./dialtone.sh repl src_v3 lint
./dialtone.sh repl src_v3 build
./dialtone.sh repl src_v3 test

./dialtone.sh chrome src_v3 build
./dialtone.sh chrome src_v3 test
./dialtone.sh ssh src_v1 test
```

When a plugin supports filtered tests, use the same shape with extra flags:

```bash
./dialtone.sh <plugin-name> <src_vN> test --filter <expr>
```

Examples:

```bash
./dialtone.sh repl src_v3 test --filter interactive-command-index-emits-task-queue-lines
./dialtone.sh chrome src_v3 test --filter service-start
```

The same plugin commands should also work naturally inside the REPL by adding a leading slash:

```text
host-name> /repl src_v3 format
host-name> /repl src_v3 test --filter interactive-command-index-emits-task-queue-lines
host-name> /chrome src_v3 test --host legion --role dev
host-name> /ssh src_v1 run --host grey --cmd hostname
```

Use direct plugin commands when you want one explicit task submission from the shell.

Use the REPL when you want to keep one long-lived session open, submit many plugin commands in a row, and watch `dialtone>` task and service lifecycle updates as they happen.

Common development loop for a plugin:

```bash
./dialtone.sh <plugin> <src_vN> install
./dialtone.sh <plugin> <src_vN> format
./dialtone.sh <plugin> <src_vN> build
./dialtone.sh <plugin> <src_vN> test
./dialtone.sh <plugin> <src_vN> test --filter <focused-step>
```

For REPL runtime work, the normal loop is:

```bash
./dialtone.sh repl src_v3 process-clean
./dialtone.sh repl src_v3 format
./dialtone.sh repl src_v3 build
./dialtone.sh repl src_v3 test --filter <focused-step>
```

The public direction for the CLI is task-first:

- queued task submission should be the default
- queued commands should return a `task-id` immediately
- PID is later runtime state, not the public identity
- that PID may be local or remote, for example a Chrome daemon PID on `legion`
- queued commands should print the queued-task summary, a helpful log-inspection command, and then return immediately
- the REPL leader should keep reporting task start, log path, PID assignment, stop, and exit code through `dialtone>`
- only explicit query/operator commands should stay foreground and print data directly

Foreground query/operator examples:

- `./dialtone.sh proc src_v1 ps`
- `./dialtone.sh proc src_v1 list`
- `./dialtone.sh logs src_v1 stream --topic 'logs.task.<task-id>'`
- `./dialtone.sh logs src_v1 tail --topic 'logs.task.<task-id>'`
- `./dialtone.sh logs src_v1 nats-status`
- `./dialtone.sh wsl src_v1 list`
- `./dialtone.sh wsl src_v1 status`
- `./dialtone.sh repl src_v3 task list`
- `./dialtone.sh repl src_v3 task show --task-id <task-id>`
- `./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 10`

Expected queued CLI pattern:

The routed user command should appear as `host-name> /command`:

```text
host-name> /chrome src_v3 status --host legion --role dev
dialtone> Request received.
dialtone> Task queued as task-20260327-abc123.
dialtone> Task topic: task.task-20260327-abc123
dialtone> Task log: ~/.dialtone/logs/task-20260327-abc123.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260327-abc123 --lines 10
```

What the one-shot CLI path should contain:

- request receipt
- queued `task-id`
- task topic
- task log path
- a helpful follow-up command to inspect the task log

What the one-shot CLI path should not contain:

- raw JSON
- stack traces
- long build output
- repeated polling noise
- browser console spam
- later `assigned pid` or final `exited with code ...` lines

That later lifecycle detail belongs in the REPL stream and the task log.

The important behavior for queued commands is:

- the CLI returns as soon as the task is queued and the log-inspection hint is printed
- the user gets the `task-id` right away
- the one-shot CLI does not wait for PID assignment, progress lines, or final exit status
- deeper lifecycle messages are still produced by the leader and can be watched in the REPL or task log

The important behavior for foreground query/operator commands is:

- they may start the background REPL leader if it is missing
- they return the requested data directly to stdout instead of a queued-task transcript
- they do not print `Request received.`, `Task queued as ...`, or a follow-up log-hint unless they are actually creating a task
- the bootstrap shell wrapper may still print startup diagnostics before the actual query output; treat that as wrapper-level preamble rather than task output

Example lifecycle the leader should emit for that same task in the REPL or task log:

```text
dialtone> Task task-20260327-abc123 assigned pid 25516 on legion.
dialtone> Task task-20260327-abc123 log confirmed at ~/.dialtone/logs/task-20260327-abc123.log
dialtone> chrome service on legion role=dev is healthy.
dialtone> Task task-20260327-abc123 exited with code 0.
```

## Running The REPL

Running plain `./dialtone.sh` should put you into the long-lived REPL.

After starting it, the session should look like:

```text
dialtone> Connected to repl.topic.index via nats://127.0.0.1:46222
dialtone> Leader online on DIALTONE-SERVER
dialtone> Shared REPL session ready on topic index.
```

Inside that REPL, the user should send commands with a leading slash:

```text
host-name> /chrome src_v3 status --host legion --role dev
dialtone> Request received.
dialtone> Task queued as task-20260327-def456.
dialtone> Task topic: task.task-20260327-def456
dialtone> Task log: ~/.dialtone/logs/task-20260327-def456.log
dialtone> Task task-20260327-def456 assigned pid 25516 on legion.
dialtone> chrome service on legion role=dev is healthy.
dialtone> Task task-20260327-def456 exited with code 0.
```

Another example with a longer-running task:

```text
host-name> /proc src_v1 sleep 20
dialtone> Request received.
dialtone> Task queued as task-20260327-sleep01.
dialtone> Task topic: task.task-20260327-sleep01
dialtone> Task log: ~/.dialtone/logs/task-20260327-sleep01.log
dialtone> Task task-20260327-sleep01 assigned pid 41122.
dialtone> Task task-20260327-sleep01 exited with code 0.
```

The REPL should be the place where the user can:

- watch `dialtone>` lifecycle output as tasks start and stop
- submit more commands with `/...`
- see task IDs immediately
- learn where the task logs are without needing raw internal output

## Interleaved Background Work

If the leader is running several background or service-class tasks at once, `dialtone>` output is a shared stream and can be non-deterministic. Lines from different tasks may interleave.

A realistic interactive session might look like this:

```text
dialtone> Connected to repl.topic.index via nats://127.0.0.1:46222
dialtone> Leader online on DIALTONE-SERVER
dialtone> Shared REPL session ready on topic index.

host-name> /proc src_v1 sleep 20
dialtone> Request received.
dialtone> Task queued as task-20260327-sleep01.
dialtone> Task topic: task.task-20260327-sleep01
dialtone> Task log: ~/.dialtone/logs/task-20260327-sleep01.log

host-name> /ssh src_v1 run --host grey --cmd 'echo ready'
dialtone> Request received.
dialtone> Task queued as task-20260327-echo01.
dialtone> Task topic: task.task-20260327-echo01
dialtone> Task log: ~/.dialtone/logs/task-20260327-echo01.log

host-name> /ssh src_v1 run --host grey --cmd 'echo boom >&2; exit 17'
dialtone> Request received.
dialtone> Task queued as task-20260327-fail01.
dialtone> Task topic: task.task-20260327-fail01
dialtone> Task log: ~/.dialtone/logs/task-20260327-fail01.log

dialtone> Task task-20260327-echo01 assigned pid 51102 on grey.
dialtone> Task task-20260327-fail01 assigned pid 51108 on grey.
dialtone> Task task-20260327-sleep01 assigned pid 41122.
dialtone> ssh run on grey for task-20260327-echo01: ready
dialtone> Task task-20260327-echo01 exited with code 0.
dialtone> ERROR task task-20260327-fail01 on grey exited with code 17.
dialtone> ERROR task task-20260327-fail01 stderr: boom
dialtone> Task task-20260327-sleep01 exited with code 0.
```

The important thing to match is not the total line order. The important things are:

- every lifecycle and error line identifies its task clearly
- each task has a valid local lifecycle even if other tasks print between its lines
- one failing task does not suppress unrelated successful task output
- the task log remains the detailed per-task source of truth

## Inspecting Tasks And Services

The REPL should also expose lightweight inspection commands so users can understand long-running remote work without guessing from one shared transcript.

Example: listing the active remote Chrome daemon task on `legion`:

```text
host-name> /repl src_v3 task list --state running --host legion
dialtone> Running tasks:
dialtone> TASK ID                  KIND      STATE    HOST    SERVICE/COMMAND           PID    EXIT
dialtone> task-20260327-chr001     service   running  legion  chrome-src-v3-dev        25516  -
dialtone> task-20260327-robot021   command   running  rover   robot src_v2 diagnostic  546289 -
```

Example: showing the task that owns that daemon:

```text
host-name> /repl src_v3 task show --task-id task-20260327-chr001
dialtone> Task: task-20260327-chr001
dialtone> Kind: service_reconcile
dialtone> State: running
dialtone> Host: legion
dialtone> Service: chrome-src-v3-dev
dialtone> PID: 25516
dialtone> Task topic: task.task-20260327-chr001
dialtone> Task log: ~/.dialtone/logs/task-20260327-chr001.log
dialtone> Log subject: logs.service.legion.chrome-src-v3-dev
```

Example: reading the detailed log for that task:

```text
host-name> /repl src_v3 task log --task-id task-20260327-chr001 --lines 6
dialtone> Streaming task log for task-20260327-chr001
[T+0000s|INFO|plugins/chrome/src_v3/ops.go:412] deploy requested for legion role=dev
[T+0001s|INFO|plugins/chrome/src_v3/daemon.go:221] daemon connected to repl manager
[T+0002s|INFO|plugins/chrome/src_v3/browser.go:301] chrome started headed pid=25516
[T+0003s|INFO|plugins/chrome/src_v3/browser.go:362] managed tab ready target=7E1A3D
```

Example: killing the remote daemon task cleanly:

```text
host-name> /repl src_v3 task kill --task-id task-20260327-chr001
dialtone> Request received.
dialtone> Task queued as task-20260327-kill01.
dialtone> Kill requested for task-20260327-chr001.
dialtone> Target task task-20260327-chr001 is running on legion pid 25516.
dialtone> Service chrome-src-v3-dev on legion moved to stopping.
dialtone> Target task task-20260327-chr001 exited with code 143.
dialtone> Task task-20260327-kill01 exited with code 0.
```

## NATS-First Logs

The long-term logging model should match the logs plugin: producers publish to NATS, and readers decide whether to render to `dialtone>`, a file, or another UI.

## NATS Control Plane

Think about the runtime in these layers:

- `dialtone.sh`: the public wrapper and operator entrypoint
- `repl src_v3`: the task/service control plane
- NATS: the transport for command frames, topics, heartbeats, and logs
- NATS KV: the durable state store for task/service state

As an operator or agent, you usually should not talk to NATS directly. Prefer these command surfaces:

- `./dialtone.sh repl src_v3 status`
- `./dialtone.sh repl src_v3 task list`
- `./dialtone.sh repl src_v3 task show --task-id <task-id>`
- `./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 50`
- `./dialtone.sh repl src_v3 watch --subject 'repl.>'`
- `./dialtone.sh logs src_v1 stream --topic 'logs.task.<task-id>'`
- `./dialtone.sh logs src_v1 stream --topic 'logs.service.<host>.<service-name>'`
- `./dialtone.sh logs src_v1 nats-status`

The mental model is:

- the shared REPL session lives on `repl.topic.index`
- queued commands get a per-task topic like `task.<task-id>`
- task/service logs are published onto NATS subjects and may also be persisted into task logs
- `dialtone>` is the short human summary stream, not the full raw event stream

Useful example commands:

```text
host-name> /logs src_v1 stream --topic 'logs.task.task-20260327-chr001'
host-name> /logs src_v1 stream --topic 'logs.service.legion.chrome-src-v3-dev'
host-name> /logs src_v1 stream --topic 'logfilter.level.error.>'
host-name> /logs src_v1 stream --topic 'logfilter.tag.fail.>'
```

The intended split is:

- `dialtone>` stays brief and lifecycle-oriented
- `task log` gives the durable per-task record
- `logs src_v1 stream` gives the live NATS log stream
- filtered subjects like `logfilter.level.error.>` help isolate failures across many tasks and services

Useful slash-command examples:

```text
host-name> /chrome src_v3 status --host legion --role dev
host-name> /chrome src_v3 test --host legion --role dev
host-name> /robot src_v2 diagnostic --host rover --skip-ui --public-check=false
```

Useful patterns:

```bash
# Start or reuse the leader.
./dialtone.sh repl src_v3 status

# Run the REPL suite.
./dialtone.sh repl src_v3 test

# Inspect task/service activity.
./dialtone.sh repl src_v3 task list
./dialtone.sh repl src_v3 service list --host legion
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200
./dialtone.sh logs src_v1 stream --topic 'logfilter.level.error.>'
```

## Windows + WSL Development

This repo may be edited from a Windows checkout while the real runtime and tests execute inside WSL.

Preferred layout:

- Windows repo: `C:\Users\timca\dialtone`
- WSL repo: `/home/user/dialtone`
- visible WSL tmux session: `windows`

Use the Windows repo for:

- editing
- code review
- native Windows Git operations

Use the WSL repo for:

- REPL and plugin tests
- SSH and mesh checks
- Linux runtime validation
- tmux-visible command execution

Use [wsl-tmux.cmd](wsl-tmux.cmd) from Windows so WSL commands run inside the visible tmux session:

```powershell
.\wsl-tmux.cmd help
.\wsl-tmux.cmd status
.\wsl-tmux.cmd clean-state
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 process-clean"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter shell-routed-command-autostarts-leader-when-missing"
.\wsl-tmux.cmd read
.\wsl-tmux.cmd interrupt
```

Preferred tmux rhythm:

1. send one command with `.\wsl-tmux.cmd "..."`
2. call `.\wsl-tmux.cmd read` until you see the prompt again
3. only then send the next command

Important behavior:

- `wsl-tmux.cmd` can queue input if you send a second command before the first one finishes
- `clean-state` is the safest way to reset the pane before a new visible sequence
- `interrupt` is better than piling on another command when the pane is wedged

If the pane gets wedged, recreating the tmux session is fine:

```powershell
wsl.exe bash -lc "tmux kill-session -t windows 2>/dev/null || true; tmux new-session -d -s windows -c /home/user/dialtone"
```

Git rules:

- trust native Windows Git for `C:\Users\timca\dialtone`
- trust WSL Git for `/home/user/dialtone`
- do not judge the Windows repo from `/mnt/c/...` inside WSL

Editing flow:

1. Edit in `C:\Users\timca\dialtone`.
2. Sync changed files into `/home/user/dialtone` when WSL needs the same patch.
3. Normalize line endings in WSL after copying if needed.

```bash
perl -0pi -e 's/\r\n/\n/g' path/to/file
```

When exact sync matters, prefer a byte-for-byte overwrite instead of `cp`:

```powershell
@'
from pathlib import Path
Path("/home/user/dialtone/src/dev.go").write_bytes(
    Path("/mnt/c/Users/timca/dialtone/src/dev.go").read_bytes()
)
'@ | wsl.exe python3 -
```

Config rules:

- use the launch folder's `env/dialtone.json` as the default config source, or pass `--env` to target another env root/file
- do not create accidental config copies under `src/env/`

If you changed REPL/bootstrap code, restart the long-lived helpers before rerunning isolated tests:

```powershell
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 process-clean"
```

Typical WSL test commands:

```powershell
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 process-clean"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 format"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 build"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 test --filter interactive-command-index-lifecycle-contract"
.\wsl-tmux.cmd "./dialtone.sh ssh src_v1 probe --host grey --timeout 5s"
.\wsl-tmux.cmd "./dialtone.sh chrome src_v3 status --host legion --role dev"
.\wsl-tmux.cmd "./dialtone.sh proc src_v1 ps"
.\wsl-tmux.cmd read
```

For generic service-control-plane tests, prefer `testdaemon` instead of Chrome:

```bash
./dialtone.sh testdaemon src_v1 service --host legion --mode start --name demo
./dialtone.sh testdaemon src_v1 service --host legion --mode status --name demo
./dialtone.sh testdaemon src_v1 service --host legion --mode stop --name demo
```

For Chrome, CAD, and UI work, prefer:

1. start or confirm the Chrome role once
2. reuse the healthy role
3. inspect task/service logs when something fails

Safe two-repo sync pattern:

1. Commit and push the Windows repo with native Windows Git.
2. Rebase the WSL repo onto the new `origin/main`.
3. Re-run WSL tests.
4. Commit and push the WSL repo.
5. Fast-forward the Windows repo again if needed.
