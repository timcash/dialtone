# CAD src_v1

`cad src_v1` is a REPL-routed plugin with:
- a Go HTTP server
- a Python CAD backend under `backend/`
- a browser UI under `ui/`

Use this mental model:
- run `./dialtone.sh cad src_v1 ...`
- REPL starts a subtone
- `DIALTONE>` stays short
- full CAD server logs and browser-test logs stay in the subtone log

## Current Status

Working now:
- `./dialtone.sh cad src_v1 format`
- `./dialtone.sh cad src_v1 build`
- `./dialtone.sh cad src_v1 serve`
- `./dialtone.sh cad src_v1 status`
- `./dialtone.sh cad src_v1 stop`
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
# Format the CAD plugin sources through the REPL path.
./dialtone.sh cad src_v1 format

# Build the UI bundle into src/plugins/cad/src_v1/ui/dist.
./dialtone.sh cad src_v1 build

# Start the CAD server on the default port.
./dialtone.sh cad src_v1 serve

# Check whether a tracked local CAD server is healthy.
./dialtone.sh cad src_v1 status

# Stop the tracked local CAD server.
./dialtone.sh cad src_v1 stop

# Run the focused browser smoke against chrome src_v3 on legion.
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke
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
DIALTONE> Request received. Spawning subtone for cad src_v1...
DIALTONE> Subtone started as pid 389789.
DIALTONE> Subtone room: subtone-389789
DIALTONE> Subtone log file: /home/user/dialtone/.dialtone/logs/subtone-389789-20260318-135944.log
DIALTONE> cad build: installing ui dependencies
DIALTONE> cad build: building ui dist
DIALTONE> cad build: ui dist ready
DIALTONE> Subtone for cad src_v1 exited with code 0.
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

# Build the UI assets.
./dialtone.sh cad src_v1 build

# Run all CAD tests.
./dialtone.sh cad src_v1 test

# Run only the browser smoke on the managed Chrome host.
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke
```

## REPL Standards

`DIALTONE>` should contain:
- subtone lifecycle
- short CAD stage summaries
- final exit code
- server lifecycle state like checking, ready, healthy, stopped

`DIALTONE>` should not contain:
- raw server trace spam
- long Python output
- full browser console streams
- repeated CAD API polling

That detail belongs in the subtone log.

## Subtone Logs

Use the REPL logs when a CAD command fails or looks incomplete.

```bash
# List recent subtones to find the CAD pid.
./dialtone.sh repl src_v3 subtone-list --count 20

# Read the full log for one CAD run.
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 250
```

For browser smoke failures, the subtone log is where you will see:
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

If a command looks incomplete, inspect the CAD subtone log and the Chrome subtone log with `repl src_v3 subtone-list` and `subtone-log`.

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
- Python stderr in the CAD subtone log
- whether `pixi` is available in the backend environment
- whether `POST /api/cad/generate` returns a non-200 status

## For LLM Agents

Use this order:

```bash
# 1. Run one CAD command.
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke

# 2. Inspect the subtone log if the command failed.
./dialtone.sh repl src_v3 subtone-list --count 10
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 250

# 3. Then run the next command.
./dialtone.sh cad src_v1 build
```

Do not chain CAD commands with `&&`. Run one `./dialtone.sh` command at a time.
