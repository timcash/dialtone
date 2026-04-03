# GA CAD src_v1

`ga_cad src_v1` is a REPL-routed plugin with:
- a Go HTTP server
- a browser UI under `ui/`
- a lightweight 2D geometric-algebra-inspired scene graph demo

Use this mental model:
- run `./dialtone.sh ga_cad src_v1 ...`
- REPL queues the task by default
- `dialtone>` stays short and high-level
- full Vite, Go server, and browser output belong in the task log

## Current Status

Working now:
- `./dialtone.sh ga_cad src_v1 install`
- `./dialtone.sh ga_cad src_v1 format`
- `./dialtone.sh ga_cad src_v1 lint`
- `./dialtone.sh ga_cad src_v1 build`
- `./dialtone.sh ga_cad src_v1 serve`
- `./dialtone.sh ga_cad src_v1 status`
- `./dialtone.sh ga_cad src_v1 stop`
- `./dialtone.sh ga_cad src_v1 dev`
- the UI renders the parent gear and dependent arm linkage
- the canvas supports pan and zoom

## Default Use

```bash
./dialtone.sh ga_cad src_v1 install
./dialtone.sh ga_cad src_v1 format
./dialtone.sh ga_cad src_v1 lint
./dialtone.sh ga_cad src_v1 build
./dialtone.sh ga_cad src_v1 serve --port 8082
./dialtone.sh ga_cad src_v1 status --port 8082
./dialtone.sh ga_cad src_v1 stop --port 8082
./dialtone.sh ga_cad src_v1 dev --port 3013
```

Expected shell pattern:

```text
host-name> /ga_cad src_v1 build
dialtone> Request received.
dialtone> Task queued as task-20260402-gacad001.
dialtone> Task topic: task.task-20260402-gacad001
dialtone> Task log: ~/.dialtone/logs/task-20260402-gacad001.log
dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-20260402-gacad001 --lines 10
```

Additional expected patterns:

```text
dialtone> ga_cad serve: checking ui bundle
dialtone> ga_cad serve: starting server on 127.0.0.1:8082
dialtone> ga_cad serve: serving ui/dist from /home/user/dialtone/src/plugins/ga_cad/src_v1/ui/dist
dialtone> ga_cad serve: ready on 127.0.0.1:8082

dialtone> ga_cad status: checking local server on 127.0.0.1:8082
dialtone> ga_cad status: server healthy on 127.0.0.1:8082

dialtone> ga_cad stop: checking for local server on 127.0.0.1:8082
dialtone> ga_cad stop: server stopped
```

## Commands

```bash
# Show the GA CAD CLI help.
./dialtone.sh ga_cad src_v1 help

# Install UI dependencies.
./dialtone.sh ga_cad src_v1 install

# Format Go and UI sources.
./dialtone.sh ga_cad src_v1 format

# Check Go and UI sources.
./dialtone.sh ga_cad src_v1 lint

# Build the UI assets.
./dialtone.sh ga_cad src_v1 build

# Start the Go server.
./dialtone.sh ga_cad src_v1 serve --port 8082

# Check a tracked server on a specific port.
./dialtone.sh ga_cad src_v1 status --port 8082

# Stop a tracked server on a specific port.
./dialtone.sh ga_cad src_v1 stop --port 8082

# Run the local Vite dev flow.
./dialtone.sh ga_cad src_v1 dev --port 3013
```

## REPL Standards

`dialtone>` should contain:
- task lifecycle
- short GA CAD stage summaries
- final exit code
- server lifecycle state like checking, ready, healthy, stopped

`dialtone>` should not contain:
- raw JSON
- stack traces
- long Vite output
- repeated polling noise

That detail belongs in the task log.

## Task Logs

Use the REPL task views when a GA CAD command fails or looks incomplete.

```bash
./dialtone.sh repl src_v3 task list
./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200
```

## Scene Graph Math

The UI reproduces this hierarchy:

```text
M_gear  = Rotor(t)
M_local = Translator(x, y) * Rotor(sin(t))
M_world = M_parent * M_local
P_new   = M × P × M~
```

The browser stage renders:
- a parent gear rotating at the origin
- a dependent arm attached to the gear edge
- pan and zoom interactions
- the PGA equations in a side panel
