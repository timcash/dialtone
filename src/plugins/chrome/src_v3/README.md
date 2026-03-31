# Chrome src_v3

`chrome src_v3` is the current Dialtone remote Chrome control path.

Its core runtime model is still:

- one daemon per role
- one long-lived Chrome process per role
- one preserved Chrome profile per role
- one managed content tab reused by default
- one NATS request/reply control surface for browser commands

Its operator model should now be understood through the REPL task system:

- `./dialtone.sh chrome src_v3 ...` should submit one task to the REPL leader
- `dialtone>` should stay short and task-focused
- detailed command output belongs in the task log and service state
- long-lived Chrome roles should be managed as service state, not as one-off shell launches

## REPL Integration

Plain `./dialtone.sh chrome src_v3 ...` is the default operator path.

In the task-first REPL design, a normal Chrome command should behave like this:

```text
host-name> /chrome src_v3 status --host legion --role dev
dialtone> Request received.
dialtone> Task queued as task-20260327-abc123.
dialtone> Task topic: task.task-20260327-abc123
dialtone> Task log: ~/.dialtone/logs/task-20260327-abc123.log
dialtone> chrome service on legion role=dev is healthy.
dialtone> Task task-20260327-abc123 exited with code 0.
```

Important points:

- the public identity is `task-id`
- PID is later runtime state, not the public identity
- that PID may be local or remote, for example the Chrome daemon PID on `legion`
- Chrome service state should ultimately be read from REPL-managed desired/observed state in NATS

`--host` is still plugin-local for `chrome` and should keep meaning the target mesh node, for example `legion`.

## Core Model

Normal behavior:

- keep one Chrome process running on the target host for each role
- keep one managed tab for that role
- reuse that tab across tests and dev flows
- preserve the Chrome user-data directory

Recovery behavior:

- if the managed target is stale or unhealthy, recreate the managed tab
- if the browser process is gone, restart the browser service
- if the daemon is gone, reconcile the desired service state for that role

What should not happen by default:

- creating a new visible tab for every test
- deleting the Chrome user-data directory during normal reset or deploy
- silently running multiple independent headed sessions against one role on the same host

## Main Commands

Lifecycle:

- `./dialtone.sh chrome src_v3 deploy --host <host> --role <role> --service`
- `./dialtone.sh chrome src_v3 service --host <host> --mode start|stop|status --role <role>`
- `./dialtone.sh chrome src_v3 status --host <host> --role <role>`
- `./dialtone.sh chrome src_v3 instances --host <host> [--role <role>]`
- `./dialtone.sh chrome src_v3 doctor --host <host>`
- `./dialtone.sh chrome src_v3 logs --host <host>`
- `./dialtone.sh chrome src_v3 reset --host <host>`
- `./dialtone.sh chrome src_v3 close-all --host <host> [--role <role>]`

Navigation:

- `./dialtone.sh chrome src_v3 open --host <host> --role <role> --url <url>`
- `./dialtone.sh chrome src_v3 goto --host <host> --role <role> --url <url>`
- `./dialtone.sh chrome src_v3 get-url --host <host> --role <role>`
- `./dialtone.sh chrome src_v3 tabs --host <host> --role <role>`
- `./dialtone.sh chrome src_v3 tab-open --host <host> --role <role> [--url <url>]`
- `./dialtone.sh chrome src_v3 tab-close --host <host> --role <role> [--index <n>]`
- `./dialtone.sh chrome src_v3 close --host <host> --role <role>`

Element actions:

- `./dialtone.sh chrome src_v3 click-aria --host <host> --role <role> --label <aria-label>`
- `./dialtone.sh chrome src_v3 type-aria --host <host> --role <role> --label <aria-label> --value <text>`
- `./dialtone.sh chrome src_v3 wait-aria --host <host> --role <role> --label <aria-label> [--timeout-ms 5000]`
- `./dialtone.sh chrome src_v3 wait-aria-attr --host <host> --role <role> --label <aria-label> --attr <name> --expected <value> [--timeout-ms 5000]`
- `./dialtone.sh chrome src_v3 get-aria-attr --host <host> --role <role> --label <aria-label> --attr <name>`

Debugging:

- `./dialtone.sh chrome src_v3 console --host <host> --role <role>`
- `./dialtone.sh chrome src_v3 wait-log --host <host> --role <role> --contains <text> [--timeout-ms 5000]`
- `./dialtone.sh chrome src_v3 screenshot --host <host> --role <role> --out <png-path>`

## Typical Workflow

```sh
# From repo root
cd /home/user/dialtone

# Reconcile and confirm a headed role on legion
./dialtone.sh chrome src_v3 deploy --host legion --role dev --service
./dialtone.sh chrome src_v3 status --host legion --role dev

# Inspect service/runtime state
./dialtone.sh chrome src_v3 instances --host legion
./dialtone.sh chrome src_v3 logs --host legion
./dialtone.sh chrome src_v3 doctor --host legion

# Drive the managed tab
./dialtone.sh chrome src_v3 goto --host legion --role dev --url http://127.0.0.1:8766/chrome_src_v3_action.html
./dialtone.sh chrome src_v3 type-aria --host legion --role dev --label "Name Input" --value dialtone
./dialtone.sh chrome src_v3 click-aria --host legion --role dev --label "Do Thing"

# Inspect live DOM state
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role dev --label "Name Input" --attr value
./dialtone.sh chrome src_v3 wait-log --host legion --role dev --contains "clicked:" --timeout-ms 5000

# Capture a screenshot of the managed tab
./dialtone.sh chrome src_v3 screenshot --host legion --role dev --out src/plugins/chrome/src_v3/screenshots/manual_debug.png
```

## Roles

Use roles to isolate long-lived browser sessions:

- `robot-test`: integrated robot suite on `legion`
- `robot-dev`: live dev browser for `robot src_v2 dev`
- `dev`: generic/manual use

Recommendations:

- keep one role per workflow
- do not mix unrelated tests on the same role
- do not run concurrent headed flows against the same role unless you explicitly want them to share one tab

## Config And State

Use `env/dialtone.json` as the shared runtime config source.

Important current configuration examples:

- REPL manager/client NATS URLs
- Chrome headed/headless defaults
- visible action pacing
- interaction count defaults used by browser-driven tests

For the Chrome runtime, the main state buckets are:

1. REPL task and service state in NATS
2. daemon persisted state on the target host
3. daemon stdout/stderr logs on the target host
4. browser console snapshots returned by the daemon
5. task logs for CLI-side request handling

## Logs And Observability

The useful log streams are:

1. **Task logs**
   These are the detailed CLI-side logs for a submitted Chrome command.
   Read them with:
   - `./dialtone.sh repl src_v3 task list`
   - `./dialtone.sh repl src_v3 task show --task-id <task-id>`
   - `./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200`

2. **Daemon logs**
   Stored on the target host at:
   - `~/.dialtone/chrome-v3/<role>/service/daemon.out.log`
   - `~/.dialtone/chrome-v3/<role>/service/daemon.err.log`

3. **Browser console**
   Query via:
   - `./dialtone.sh chrome src_v3 console --host <host> --role <role>`

4. **Service state**
   This should be the main truth for whether the daemon exists, is healthy, and which role/host it belongs to.

5. **Daemon response state**
   `status`, `tabs`, `get-url`, and related commands expose the live daemon/browser view.

Operationally:

- start with `status`
- if `unhealthy=true`, inspect daemon logs
- if the browser is running but misbehaving, inspect console, DOM attrs, and screenshot output
- if the CLI transcript is too thin, inspect the task log

## NATS And Control Surface

The browser control surface is one request/reply API over NATS.

Current daemon routing includes:

- host-scoped command subjects
- REPL-managed service discovery

The important operator takeaway is:

- browser-driving commands should feel like task submissions against a durable service
- service startup and reuse should be controlled by the REPL control plane
- the daemon should not be treated as an ad hoc hidden side effect

## Integration With `test/src_v1`

`chrome src_v3` is the preferred remote headed-browser backend for Dialtone tests.

What the test library expects:

- one attach node, often `legion`
- one attach role, for example `robot-test`
- one reusable managed tab

The test flow should conceptually be:

1. queue a task through `dialtone.sh`
2. ensure or reuse the Chrome service for the target role
3. drive the managed tab through daemon commands
4. record screenshots, console state, and reports
5. keep top-level `dialtone>` output short

## Lifecycle Rules

- `open` and `goto` reuse the managed tab
- screenshots should use the same managed tab
- `deploy --service` should preserve the running browser when the binary is already current
- `reset` should preserve the Chrome profile/user-data directory
- explicit `tab-open`, `tab-close`, or `close` are the normal commands that change tab/browser lifecycle on purpose
- `instances` lists only Dialtone-managed Chrome roles by looking for the `--dialtone-role` process flag, so personal Chrome windows are excluded
- `close-all` closes only Dialtone-managed Chrome browser instances on the target host and leaves personal Chrome untouched

## Hybrid Windows + WSL Workflow

When the normal service path is unhealthy or you need direct visibility into WSL command execution, use the hybrid workflow as an escape hatch, not the primary model.

Recommended pattern:

- keep the visible WSL commands in the `windows` tmux session
- use the WSL repo for runtime checks
- keep Windows-only helper scripts as explicit operator tools, not the default service contract

Example:

```powershell
wsl-tmux clean-state
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh chrome src_v3 status --host legion --role dev"
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh chrome src_v3 test --host legion --role dev"
```

If you explicitly need the manual Windows helper path:

- `.\scripts\windows\chrome-src-v3-manual.ps1` is a fallback operator helper
- it is useful when the normal remote launcher path is flaky
- it should not be treated as the long-term primary control-plane design

## Troubleshooting

1. Check daemon state:

```sh
./dialtone.sh chrome src_v3 status --host legion --role dev
```

2. If the tab lands on `chrome-error://chromewebdata/`, verify the target UI server is actually running.

3. Read browser console:

```sh
./dialtone.sh chrome src_v3 console --host legion --role dev
```

4. Inspect live UI attrs:

```sh
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role dev --label "Name Input" --attr value
```

5. Capture a screenshot of the managed tab:

```sh
./dialtone.sh chrome src_v3 screenshot --host legion --role dev --out src/plugins/chrome/src_v3/screenshots/manual_debug.png
```

6. Only use `reset` when normal recovery is not enough:

```sh
./dialtone.sh chrome src_v3 reset --host legion
```

7. If the top-level transcript is not enough, inspect the detailed task log:

```sh
./dialtone.sh repl src_v3 task list
./dialtone.sh repl src_v3 task show --task-id <task-id>
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200
```

## Related Docs

- [README.md](/C:/Users/timca/dialtone/src/plugins/repl/src_v3/README.md)
- [DESIGN.md](/C:/Users/timca/dialtone/src/plugins/repl/src_v3/DESIGN.md)
- [DESIGN.md](/C:/Users/timca/dialtone/src/plugins/chrome/src_v3/DESIGN.md)
- [README.md](/C:/Users/timca/dialtone/src/plugins/test/src_v1/README.md)
