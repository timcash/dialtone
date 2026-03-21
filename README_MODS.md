# Mods Guide

This is the operating guide for the Dialtone mods system.

The goal is to make five things work together in one visible loop:

- `codex-view`: the left tmux pane where prompts go
- `dialtone-view`: the right tmux pane where `./dialtone_mod` commands run visibly
- `./dialtone_mod`: the mod launcher and visible command surface
- SQLite: the source of truth for DAG, targets, queue state, protocol runs, and test runs
- `src/mods/<mod-name>/<version>/`: the versioned mod implementation layout

## Core Model

Dialtone mods live under:

```text
src/mods/<mod-name>/<version>/
```

Examples:

```text
src/mods/ghostty/v1/
src/mods/tmux/v1/
src/mods/codex/v1/
src/mods/shell/v1/
src/mods/mod/v1/
```

The normal entrypoint is:

```sh
./dialtone_mod <mod-name> <version> <command> [args]
```

Examples:

```sh
./dialtone_mod ghostty v1 help
./dialtone_mod shell v1 start
./dialtone_mod shell v1 demo-protocol
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 40
./dialtone_mod mods v1 db sync
```

## SQLite First

The central state database is:

```text
.dialtone/state.sqlite
```

SQLite is the shared ledger for:

- the mod DAG
- test-plan ordering
- tmux targets
- prompt queue rows
- command queue rows
- protocol runs and protocol events
- test runs and test steps

Use the `mods` mod to keep the DB in sync:

```sh
# Resolve the current SQLite path.
./dialtone_mod mods v1 db path

# Initialize the schema if needed.
./dialtone_mod mods v1 db init

# Sync the repo scan, manifests, nix packages, env, topology, and test plan.
./dialtone_mod mods v1 db sync

# Render the DAG as a simple outline from SQLite.
./dialtone_mod mods v1 db graph --format outline

# Show the topological test order from SQLite.
./dialtone_mod mods v1 db topo
./dialtone_mod mods v1 db test-plan
```

Current SQLite outline:

```text
- chrome:v1
- mesh:v3
- mod:v1
- mosh:v1
- repl:v1
- ssh:v1
- shell:v1
  - ghostty:v1
  - tmux:v1
  - codex:v1
    - tmux:v1
- tsnet:v1
```

## Required Workflow

For interactive work, the system should look like this:

- one Ghostty window
- one Ghostty tab
- one tmux session: `codex-view`
- left pane: `codex-view:0:0`
- right pane: `codex-view:0:1`

Target roles:

- `tmux.prompt_target = codex-view:0:0`
- `tmux.target = codex-view:0:1`

That means:

- prompts go left
- visible `./dialtone_mod` commands go right
- SQLite tracks both

## Quick Start

```sh
# 1. Start the visible two-pane workflow.
# This creates one fresh Ghostty window, one tab, attaches tmux, keeps Codex on
# the left, and keeps `dialtone-view` on the right.
./dialtone_mod shell v1 start --run-tests=false

# 2. Confirm the tmux targets stored in SQLite.
# `command` should be the right pane and `prompt` should be the left pane.
./dialtone_mod tmux v1 target --all

# 3. Show the DAG outline from SQLite.
./dialtone_mod mods v1 db graph --format outline

# 4. Run the protocol smoke test against the current visible panes.
# `--bootstrap=false` keeps the current Ghostty/tmux layout and only exercises
# the prompt/command/SQLite protocol.
./dialtone_mod shell v1 demo-protocol --bootstrap=false

# 5. Inspect the panes after the smoke test.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 40
./dialtone_mod tmux v1 read --pane codex-view:0:1 --lines 80

# 6. Supervise the live system.
./dialtone_mod shell v1 supervise --limit 5

# 7. Inspect protocol rows directly without queueing another visible command.
# Use the proxy bypass when the controller wants immediate SQLite readback.
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db protocol-runs --limit 5
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db protocol-events --run 2
```

## LLM Workflow

All LLMs should operate as controllers, not hidden workers.

The intended loop is:

1. Read `README_MODS.md`.
2. Sync the SQLite DAG.
3. Start or verify the visible two-pane session.
4. Improve the user prompt before sending it.
5. Submit the improved prompt into `codex-view`.
6. Expect `./dialtone_mod` commands to appear visibly in `dialtone-view`.
7. Poll SQLite and pane output to decide whether the run is healthy, stuck, or looping.
8. Intervene by sending a better prompt or a recovery command.

The basic controller commands are:

```sh
# Submit a prompt into the left pane and record it in SQLite.
./dialtone_mod shell v1 prompt "Your improved prompt here"

# Inspect the live panes.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 60
./dialtone_mod tmux v1 read --pane codex-view:0:1 --lines 80

# Inspect queue health.
./dialtone_mod shell v1 supervise --limit 5

# Inspect the queued tmux commands.
./dialtone_mod mods v1 db queue --name tmux --limit 10
./dialtone_mod mods v1 db queue --name prompts --limit 10

# Inspect protocol rows directly from the controller shell.
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db protocol-runs --limit 5
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db protocol-events --run 2
```

### Prompt Pattern

When the user gives a task, the controller should tighten it into something like:

```text
Task: update src/mods/<mod>/<version>/...

Requirements:
- use the existing mod/version CLI contract
- keep SQLite as the source of truth for state and DAG data
- run visible ./dialtone_mod commands in dialtone-view
- record results precisely
- summarize blockers instead of guessing
```

## Protocol Smoke Test

The current smoke test is:

```sh
./dialtone_mod shell v1 demo-protocol --bootstrap=false
```

What it does:

- records a `protocol_runs` row in SQLite
- records ordered `protocol_events` rows in SQLite
- submits a prompt into `codex-view`
- writes one visible `./dialtone_mod` command into `dialtone-view`
- waits for expected output in the right pane
- marks the protocol run `passed` or `failed`

Current validated result:

```text
demo protocol run 2 passed
prompt_target	codex-view:0:0
command_target	codex-view:0:1
command	env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db graph --format outline
result	observed "- shell:v1" in codex-view:0:1
```

The live right-pane command that was observed:

```text
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db graph --format outline
- chrome:v1
- mesh:v3
- mod:v1
- mosh:v1
- repl:v1
- ssh:v1
- shell:v1
  - ghostty:v1
  - tmux:v1
  - codex:v1
    - tmux:v1
- tsnet:v1
```

## Deep Inspection

Use the shell mod first. If you need raw SQLite inspection, use Nix and query the DB directly.

```sh
# Show protocol runs through the mod CLI without queueing to dialtone-view.
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db protocol-runs --limit 5
env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db protocol-events --run 2

# Show the latest protocol runs directly from SQLite.
nix --extra-experimental-features 'nix-command flakes' develop .#default \
  --command sqlite3 .dialtone/state.sqlite \
  "select id,name,status,prompt_target,command_target,result_text from protocol_runs order by id desc limit 5;"

# Show the ordered events for one protocol run.
nix --extra-experimental-features 'nix-command flakes' develop .#default \
  --command sqlite3 .dialtone/state.sqlite \
  "select run_id,event_index,event_type,queue_name,queue_row_id,pane_target,command_text,message_text from protocol_events where run_id=2 order by event_index;"
```

Observed rows for run `2`:

```text
2|demo-protocol|passed|codex-view:0:0|codex-view:0:1|observed "- shell:v1" in codex-view:0:1
```

```text
2|1|workflow_ready||0|||prompt=codex-view:0:0 command=codex-view:0:1
2|2|prompt_submitted|prompts|27|codex-view:0:0|Protocol demo: the controller is recording this run in SQLite. A visible dialtone_mod command will run in dialtone-view while this prompt is shown in codex-view.|submitted prompt to codex-view
2|3|command_written|tmux|0|codex-view:0:1|env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db graph --format outline|wrote visible command to dialtone-view
2|4|command_observed|tmux|0|codex-view:0:1|env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db graph --format outline|- shell:v1
```

## Go + Nix

All Go build, format, and test work should run through Nix.

```sh
# Format and test a mod inside the repo Nix shell.
cd /Users/user/dialtone
nix --extra-experimental-features 'nix-command flakes' develop .#default \
  --command zsh -lc 'cd src && gofmt -w ./mods/shell/v1/main.go && go test ./mods/shell/v1'

# Build a mod binary inside the same shell.
cd /Users/user/dialtone
nix --extra-experimental-features 'nix-command flakes' develop .#default \
  --command zsh -lc 'cd src && go build -o /tmp/dialtone-shell-v1 ./mods/shell/v1'
```

For visible runs, prefer writing the command into `dialtone-view`:

```sh
./dialtone_mod tmux v1 write --pane codex-view:0:1 --enter \
  "cd /Users/user/dialtone && nix --extra-experimental-features 'nix-command flakes' develop .#default --command zsh -lc 'cd src && go test ./mods/shell/v1'"
```

## Test Strategy

There are three useful layers:

- unit tests
  Pure Go tests for DAG logic, queue transitions, protocol tables, and renderers
- integration tests
  Real SQLite + real tmux + visible command execution
- visible protocol smoke tests
  `shell v1 demo-protocol` against the current two-pane session

Current validated packages under Nix:

```text
ok  	dialtone/dev/internal/modstate
ok  	dialtone/dev/mods/mod/v1
ok  	dialtone/dev/mods/shell/v1
```

## Current Mod Order

The current SQLite-derived test order is:

1. `chrome v1`
2. `ghostty v1`
3. `mesh v3`
4. `mod v1`
5. `mosh v1`
6. `repl v1`
7. `ssh v1`
8. `tmux v1`
9. `codex v1`
10. `shell v1`
11. `tsnet v1`
