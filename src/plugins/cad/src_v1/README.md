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
- the CAD UI loads and can generate STL-backed gear geometry
- the floor plane is now positioned from the loaded mesh bounds so it sits below the gear instead of intersecting it

Not working yet:
- `./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke` is still failing
- the browser smoke reaches `cad-model-ready:1` through `cad-model-ready:5`, then logs `[cad/ui] regenerate failed`
- the failing run also shows `[TEST_ACTION] click aria=Toggle Global Menu` immediately before an extra `POST /api/cad/generate`

What to debug next:
1. Verify whether the extra generate is caused by the browser test harness, the shared UI menu system, or a duplicate CAD UI action.
2. Capture the actual browser-side error text from the failed regenerate instead of only logging `[cad/ui] regenerate failed`.
3. Run the same button sequence manually with `chrome src_v3` and compare it to the failing smoke test transcript.
4. If the extra generate is test-only, tighten the smoke test so it ignores unrelated post-action noise and validates the real CAD buttons only.

## Default Use

```bash
# Format the CAD plugin sources through the REPL path.
./dialtone.sh cad src_v1 format

# Build the UI bundle into src/plugins/cad/src_v1/ui/dist.
./dialtone.sh cad src_v1 build

# Start the CAD server on the default port.
./dialtone.sh cad src_v1 serve

# Run the focused browser smoke against chrome src_v3 on legion.
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke
```

Expected shell pattern:

```text
legion> /cad src_v1 build
DIALTONE> Request received. Spawning subtone for cad src_v1...
DIALTONE> Subtone started as pid 389789.
DIALTONE> Subtone room: subtone-389789
DIALTONE> Subtone log file: /home/user/dialtone/.dialtone/logs/subtone-389789-20260318-135944.log
DIALTONE> cad build: installing ui dependencies
DIALTONE> cad build: building ui dist
DIALTONE> Subtone for cad src_v1 exited with code 0.
```

## Commands

```bash
# Show the CAD CLI help.
./dialtone.sh cad src_v1 help

# Start the Go server on a specific port.
./dialtone.sh cad src_v1 serve --port 8099

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

Example failure pattern:

```text
[TEST] [STEP:cad-ui-browser-smoke-src-v1] [BROWSER][CONSOLE:log] "cad-model-ready:5"
[TEST] [STEP:cad-ui-browser-smoke-src-v1] [BROWSER][CONSOLE:log] "[cad/ui] regenerate failed"
[TEST] [STEP:cad-ui-browser-smoke-src-v1] [BROWSER][CONSOLE:log] "[TEST_ACTION] click aria=Toggle Global Menu"
cad serve: handling POST /api/cad/generate
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
