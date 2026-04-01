# CAD src_v1

`cad src_v1` is a REPL-routed plugin with:
- a Go HTTP server
- a Python CAD backend under `backend/`
- a browser UI under `ui/`

Use this mental model:
- run `./dialtone.sh cad src_v1 ...`
- REPL queues a task
- `DIALTONE>` stays short
- full CAD server logs and browser-test logs stay in the task log

## Real Test

From WSL, the real end-to-end test is:

```bash
./dialtone.sh cad src_v1 test
```

That command now auto-attaches to the default Windows Chrome host from WSL, runs the HTTP checks, and runs the real browser smoke against the CAD UI.

Use these only when you want narrower coverage:

```bash
./dialtone.sh cad src_v1 test --filter self-check
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke
```

## Current Status

Working now:
- `./dialtone.sh cad src_v1 install`
- `./dialtone.sh cad src_v1 format`
- `./dialtone.sh cad src_v1 lint`
- `./dialtone.sh cad src_v1 build`
- `./dialtone.sh cad src_v1 serve`
- `./dialtone.sh cad src_v1 status`
- `./dialtone.sh cad src_v1 stop`
- `./dialtone.sh cad src_v1 test` now passes from WSL and runs the real browser smoke through `chrome src_v3`
- the CAD UI loads and can generate STL-backed gear geometry
- the floor plane is now positioned from the loaded mesh bounds so it sits below the gear instead of intersecting it
- `./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke` now passes
- the browser smoke proves the bottom form buttons can change the gear and wait for regeneration between clicks

What to debug next:
1. Keep the CAD smoke isolated to its dedicated Chrome role and make sure future tests do not reintroduce stale console history.
2. Add one more smoke that uses the text input plus submit path, not only the thumb buttons.
3. Consider surfacing the same regeneration state in a more user-facing way in the legend/status text, not only as test attributes.
4. Decide whether screenshots from CAD smoke should be committed anywhere or always treated as local debug artifacts.

## Default Use

```bash
./dialtone.sh cad src_v1 test
./dialtone.sh cad src_v1 test --filter self-check
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke
```

```bash
./dialtone.sh cad src_v1 install
./dialtone.sh cad src_v1 format
./dialtone.sh cad src_v1 lint
./dialtone.sh cad src_v1 build
./dialtone.sh cad src_v1 serve
./dialtone.sh cad src_v1 status
./dialtone.sh cad src_v1 stop
```

The focused smoke currently verifies:
- the CAD page loads in `chrome src_v3`
- the initial model reaches generation `1`
- the form becomes idle before each next action
- `CAD Thumb 1`, `CAD Thumb 3`, `CAD Thumb 5`, and `CAD Thumb 7` each trigger a new generation
- the browser console has no CAD regenerate failure or browser exception

Expected shell pattern:

```text
legion> /cad src_v1 build
DIALTONE> Request received.
DIALTONE> Task queued as task-20260330-cad001.
DIALTONE> Task topic: task.task-20260330-cad001
DIALTONE> Task log: ~/.dialtone/logs/task-20260330-cad001.log
DIALTONE> cad build: installing ui dependencies
DIALTONE> cad build: building ui dist
DIALTONE> cad build: ui dist ready
DIALTONE> Task task-20260330-cad001 exited with code 0.
```

Additional expected patterns:

```text
DIALTONE> cad serve: checking ui bundle
DIALTONE> cad serve: starting backend on 127.0.0.1:8099
DIALTONE> cad serve: serving ui/dist from /home/user/dialtone/src/plugins/cad/src_v1/ui/dist
DIALTONE> cad serve: backend ready on 127.0.0.1:8099

DIALTONE> cad status: checking local server on 127.0.0.1:8099
DIALTONE> cad status: server healthy on 127.0.0.1:8099

DIALTONE> cad stop: checking for local server on 127.0.0.1:8099
DIALTONE> cad stop: server stopped
```

## Commands

```bash
# Show the CAD CLI help.
./dialtone.sh cad src_v1 help

# Start the Go server on a specific port.
./dialtone.sh cad src_v1 serve --port 8099

# Check a tracked server on a specific port.
./dialtone.sh cad src_v1 status --port 8099

# Stop a tracked server on a specific port.
./dialtone.sh cad src_v1 stop --port 8099

# Format Go and UI sources.
./dialtone.sh cad src_v1 format

# Verify/install backend and UI dependencies.
./dialtone.sh cad src_v1 install

# Build the UI assets.
./dialtone.sh cad src_v1 build

# Run the real WSL-backed CAD test flow.
./dialtone.sh cad src_v1 test

# Run the lightweight self-check slice.
./dialtone.sh cad src_v1 test --filter self-check

# Run only the browser smoke on the managed Chrome host.
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke
```

Notes:
- `--filter self-check` now covers the quick local object/layout checks.
- the HTTP generation path now uses managed `pixi` from `DIALTONE_ENV/pixi` when available.
- `./dialtone.sh pixi src_v1 install` and `./dialtone.sh cad src_v1 install` both prepare that managed runtime.

## REPL Standards

`DIALTONE>` should contain:
- task lifecycle
- short CAD stage summaries
- final exit code
- server lifecycle state like checking, ready, healthy, stopped

`DIALTONE>` should not contain:
- raw server trace spam
- long Python output
- full browser console streams
- repeated CAD API polling

That detail belongs in the task log.

## Task Logs

Use the REPL task views when a CAD command fails or looks incomplete.

```bash
# List recent tasks to find the CAD task id.
./dialtone.sh repl src_v3 task list

# Read the full log for one CAD run.
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 250
```

For browser smoke failures, the task log is where you will see:
- CAD server requests like `POST /api/cad/generate`
- browser console lines like `cad-model-ready:5`
- browser console failures like `[cad/ui] regenerate failed`
- the form/test state used to gate the next action

Example success pattern:

```text
[TEST] [STEP:cad-ui-browser-smoke-src-v1] [BROWSER][CONSOLE:log] "cad-model-ready:1"
[TEST] [STEP:cad-ui-browser-smoke-src-v1] [BROWSER][CONSOLE:log] "cad-model-ready:2"
[TEST] [STEP:cad-ui-browser-smoke-src-v1] [BROWSER][CONSOLE:log] "cad-model-ready:3"
[TEST] [STEP:cad-ui-browser-smoke-src-v1] [BROWSER][CONSOLE:log] "cad-model-ready:4"
[TEST] [STEP:cad-ui-browser-smoke-src-v1] [BROWSER][CONSOLE:log] "cad-model-ready:5"
[TEST][PASS] [STEP:cad-ui-browser-smoke-src-v1]
```

## Browser Debug Workflow

Use the same REPL flow as operators.

```bash
# 1. Build the CAD UI.
./dialtone.sh cad src_v1 build

# 2. Start the CAD server.
./dialtone.sh cad src_v1 serve --port 8099

# 3. Open the CAD UI in the managed Chrome session on legion.
./dialtone.sh chrome src_v3 goto --host legion --role cad-smoke --url http://127.0.0.1:8099

# 4. Click the bottom buttons by aria label.
./dialtone.sh chrome src_v3 click-aria --host legion --role cad-smoke --label "CAD Thumb 1"
./dialtone.sh chrome src_v3 click-aria --host legion --role cad-smoke --label "CAD Thumb 3"
./dialtone.sh chrome src_v3 click-aria --host legion --role cad-smoke --label "CAD Thumb 5"
./dialtone.sh chrome src_v3 click-aria --host legion --role cad-smoke --label "CAD Thumb 7"

# 5. Capture a screenshot.
./dialtone.sh chrome src_v3 screenshot --host legion --role cad-smoke --out src/plugins/cad/src_v1/screenshots/cad_manual.png
```

If a command looks incomplete, inspect the CAD task log and the Chrome task log with `repl src_v3 task list` and `task log`.

For long-lived local servers, the clean operator loop is:

```bash
# Start the server.
./dialtone.sh cad src_v1 serve --port 8099

# In a later turn, check health.
./dialtone.sh cad src_v1 status --port 8099

# Stop it when done.
./dialtone.sh cad src_v1 stop --port 8099
```

Note:
- if `serve` is occupying the active foreground REPL slot, other foreground commands may wait behind it
- for long-lived sessions, background REPL usage is still the cleanest operational model

## Regeneration Contract

The CAD UI now exposes an explicit regeneration contract for tests:
- `CAD Mode Form[data-busy]`
- `CAD Mode Form[data-generation]`
- `CAD Stage[data-model-state]`
- `CAD Stage[data-generation]`
- `CAD Model Status[data-state]`
- `CAD Model Status[data-generation]`

The browser smoke waits in this order:
1. button click
2. `CAD Mode Form[data-generation=<next>]`
3. `CAD Mode Form[data-busy=false]`
4. `CAD Stage[data-model-state=ready]`
5. console marker `cad-model-ready:<next>`

That prevents the next CAD change from being sent before the backend has finished rebuilding the current model.

## Python CAD Backend

The Go server calls the Python backend through `pixi`.

`./dialtone.sh cad src_v1 install` now does the dependency bootstrap/check path:
- verifies the backend and UI manifests exist
- ensures managed `pixi` is available under `DIALTONE_ENV/pixi`
- runs `pixi install` in `backend/`
- verifies the Python imports used by the CAD backend
- runs `bun install` in `ui/`

Current request path:
- `POST /api/cad/generate`
- Go calls `pixi run python main.py --outer_diameter ... --num_teeth ...`
- the Python process returns STL bytes
- the Go server returns `Content-Type: application/sla`

Relevant paths:
- backend source: [main.py](/home/user/dialtone/src/plugins/cad/src_v1/backend/main.py)
- Go CAD handlers: [cad.go](/home/user/dialtone/src/plugins/cad/src_v1/go/cad.go)
- Go HTTP server: [plugin.go](/home/user/dialtone/src/plugins/cad/src_v1/go/plugin.go)

When debugging backend generation problems, check:
- Python stderr in the CAD task log
- whether managed `pixi` exists under `DIALTONE_ENV/pixi` or `DIALTONE_PIXI_BIN`
- whether `POST /api/cad/generate` returns a non-200 status

## For LLM Agents

Use this order:

```bash
# 1. Run one CAD command.
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke

# 2. Inspect the task log if the command failed.
./dialtone.sh repl src_v3 task list
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 250

# 3. Then run the next command.
./dialtone.sh cad src_v1 build
```

Do not chain CAD commands with `&&`. Run one `./dialtone.sh` command at a time.
