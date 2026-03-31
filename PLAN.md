# Plan

This is the current working plan for Dialtone.

It replaces the old repo-wide plugin sync checklist and the old REPL-only test plan with one shorter root plan that matches the current repo shape.

## Ground Rules

- Treat [README.md](README.md) and [src/plugins/README.md](src/plugins/README.md) as the public contract.
- Store normal configuration in `env/dialtone.json`.
- If a one-off override is needed, prefix that single `./dialtone.sh ...` command instead of exporting shell state.
- Optional behavior belongs on `--flags`, not hidden env vars.
- Prefer `./dialtone.sh <plugin> <src_vN> <command>` over raw toolchain commands.
- On Windows, keep WSL-side testing visible through [wsl-tmux.cmd](wsl-tmux.cmd).

## Current Status

These focused paths are currently in decent shape and should stay green while we continue:

- `repl src_v3` focused dispatch test
- `test src_v1` focused browser-context test
- `ui src_v1` focused build-and-serve browser test
- `cad src_v1` focused self-check
- `cloudflare src_v1` focused preflight
- `ssh src_v1` focused resolve/transport test
- `chrome src_v3` focused deploy/start test
- `robot src_v2` focused UI-table test

The recent alignment work also pushed the core plugins closer to the desired model:

- shared config lookups go through `config src_v1`
- core dev/browser attach behavior is flag-driven instead of env-toggle-driven
- one-off attach/default-attach flows are more consistent across `test`, `ui`, `cad`, and `robot`
- shared browser-service communication still goes through REPL/NATS and the `chrome src_v3` daemon
- `logs`, `ui`, and `ssh` top-level legacy CLI glue now routes through version-owned entrypoints
- the `robot` scaffold now parses the top-level CLI contract and dispatches into `src/plugins/robot/src_v2`
- `robot src_v2` entry/dispatch and remote-ops command families now live in separate version-owned files
- `robot src_v2` diagnostic now verifies the latest release-channel manifest/artifact digests and checks the public UI at `https://rover-1.dialtone.earth`
- `robot src_v2 publish` is now verified against the real GitHub release path, and the live rover diagnostic correctly fails when autoswap has not converged to the newest manifest yet
- `repl src_v3 test` is verified through the WSL bootstrap path, and its test runner now prints explicit start/pass lines while still writing `TEST.md` and `TEST_RAW.md`
- a visible WSL tmux sweep revalidated `logs`, `test`, `ssh`, `repl`, `cloudflare`, `robot`, and `cad`; `cloudflare` needed the public `cloudflare src_v1 install` step first so the UI lint/build toolchain was present

Current Windows/WSL REPL verification status from the latest visible `wsl-tmux.cmd` sweep:

- `chrome src_v3 test-actions -host legion -role test` now passes end-to-end through the REPL/NATS service path, including `set-html`, `type-aria`, `click-aria`, `wait-log`, and screenshot capture
- the Windows `chrome src_v3` daemon was rebuilt and redeployed from this repo branch, and the matching `src/plugins/chrome/src_v3/*.go` sources were synced into the WSL checkout while testing
- `cloudflare src_v1 test` now passes end-to-end through the REPL/NATS/Windows-browser path from visible `wsl-tmux.cmd` runs; latest passing task: `task-20260331-152533-000`
- shared-browser Cloudflare steps were standardized to rely on durable DOM/state assertions (`data-selected-cube`, `data-ready`, `data-active`, proof-of-life script execution) instead of cached browser-console messages
- cached browser-console log propagation in shared browser sessions is still weaker than the dedicated `chrome src_v3 test-actions` control-plane/browser-service proof, so console-capture remains covered there rather than as a hard gate inside the Cloudflare suite
- failed `cloudflare src_v1 test` workers are currently leaving stale `running` task entries / defunct worker processes in the REPL task registry, so task cleanup/reaping is part of the remaining control-plane work

## What To Do Next

### 1. Finish Shared Config And Install-State Work

The next foundation layer should be fully shared instead of plugin-local:

- finalize a dependency ledger shape in `env/dialtone.json`
- write install receipts under the shared cache root for the remaining core plugins
- keep tool/runtime paths resolved through `config src_v1`
- remove plugin-local install side effects where the shared receipt/cache model can replace them

### 2. Remove Remaining Legacy Compatibility Knobs

Some compatibility code still exists to avoid breaking older paths.

Work through these next:

- remove leftover env-only dev/browser compatibility from non-core callers
- migrate any remaining direct config-bearing `os.Getenv(...)` reads in active or near-active plugins to `config src_v1`
- keep OS/runtime detection env reads only when they are truly process/host facts rather than repo config

### 3. Keep Thinning Scaffolds

The repo should keep moving toward thin `scaffold/main.go` files and version-owned logic.

Recently completed in this lane:

- `logs` top-level CLI glue
- `ui` top-level CLI glue
- `ssh` remaining dispatch glue
- `robot` scaffold

Next cleanup targets:

- split the remaining `robot src_v2` publish/build family and the diagnostic helpers out of `src/plugins/robot/src_v2/plugin.go`
- use the stricter `robot src_v2 diagnostic` after publish/rollout work and treat stale-manifest failures as rollout convergence bugs, not as a reason to weaken the diagnostic
- keep new scaffolds version-first and keep tests routing through `src_vN/test/cmd/main.go`
- rerun public `./dialtone.sh <plugin> <src_vN> build` and focused `test --filter ...` flows after each split

The target is simple:

- scaffold parses version and high-level command
- `src_vN` owns the real behavior
- tests route through `src_vN/test/cmd/main.go`

### 4. Prove The REPL Control Plane More Deeply

The REPL still needs a stronger proof story than "browser flows happen to work".

Priority order:

1. prove queue-vs-foreground dispatch and leader reuse/autostart
2. build and use `testdaemon` as the generic service fixture
3. prove task state through NATS KV
4. prove service desired/observed state through NATS KV
5. prove heartbeat-driven unhealthy detection and reconcile/restart
6. prove the same model on remote hosts, especially `legion`

This matters because Chrome, Cloudflare, and robot success should sit on top of a proven control plane, not replace that proof.

Immediate debug follow-up inside this lane:

- keep using `chrome src_v3 test-actions -host legion -role test` as the fast control-plane/browser-service proof while Cloudflare step 04 is being stabilized
- finish the service-managed console/log handoff between `test src_v1` and the `chrome src_v3` daemon so shared-browser suites can optionally promote cached browser-console messages back into hard assertions without flaking
- fix REPL task reaping so failed foreground workers do not remain `running` after the worker process is already defunct

### 5. Keep Sweeping Docs And Tests Together

As code changes land:

- remove stale task-worker / room language
- keep plugin READMEs aligned with actual command flags and defaults
- keep examples versioned and current
- rerun focused tmux-visible tests after each substantial change
- periodically rerun broader suites once focused slices are stable

## Definition Of The Next Milestone

The next milestone is complete when all of these are true:

- core plugin config comes from `env/dialtone.json` by default
- one-off overrides are documented as prefixed command env vars
- optional behavior is exposed by flags in the active core plugin flows
- shared install receipts exist for the core plugin/toolchain paths that still need them
- the REPL task/service proof surface is moving through `testdaemon` and KV-backed tests rather than ad hoc integration-only checks

## LLM Agent Workflow

### 1. Start From The Repo Contract

Read the root docs first, then work through versioned plugin commands.

```bash
cd /path/to/dialtone
sed -n '1,180p' README.md
sed -n '1,220p' src/plugins/README.md
./dialtone.sh repl src_v3 help
./dialtone.sh <plugin> <src_vN> help
```

### 2. Use The Standard Plugin Loop

Use the versioned command surface unless there is a documented exception.

```bash
./dialtone.sh <plugin> <src_vN> install
./dialtone.sh <plugin> <src_vN> format
./dialtone.sh <plugin> <src_vN> lint
./dialtone.sh <plugin> <src_vN> build
./dialtone.sh <plugin> <src_vN> test
./dialtone.sh <plugin> <src_vN> test --filter <expr>
```

If you truly need lower-level tool access, route it through Dialtone first:

```bash
./dialtone.sh go src_v1 exec gofmt -w ./plugins/<plugin>/...
./dialtone.sh bun src_v1 exec --cwd ./plugins/<plugin>/<src_vN>/ui run build
./dialtone.sh pixi src_v1 version
```

### 3. Keep Normal Config In `env/dialtone.json`

Use `env/dialtone.json` for stable config, and use per-command prefixes only for intentional one-off overrides.

```bash
./dialtone.sh ui src_v1 test --attach legion
DIALTONE_TEST_BROWSER_NODE=legion ./dialtone.sh test src_v1 test --filter browser-stepcontext-aria-and-console
CLOUDFLARE_API_TOKEN=... ./dialtone.sh cloudflare src_v1 provision rover --domain dialtone.earth
```

### 4. On Windows, Use `wsl-tmux.cmd`

Do not hide WSL-side test work behind direct `wsl.exe bash -lc ...` commands when [wsl-tmux.cmd](wsl-tmux.cmd) is available.

Use this visible workflow:

```powershell
.\wsl-tmux.cmd clean-state
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 process-clean"
.\wsl-tmux.cmd "./dialtone.sh <plugin> <src_vN> format"
.\wsl-tmux.cmd "./dialtone.sh <plugin> <src_vN> build"
.\wsl-tmux.cmd "./dialtone.sh <plugin> <src_vN> test --filter <expr>"
.\wsl-tmux.cmd read
```

### 5. When A Command Queues A Task, Follow It Through The REPL

Use the visible tmux session to inspect the task rather than guessing.

```powershell
.\wsl-tmux.cmd "./dialtone.sh <plugin> <src_vN> test --filter <expr>"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 task show --task-id <task-id>"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 120"
.\wsl-tmux.cmd "./dialtone.sh logs src_v1 stream --topic logs.test.<suite>.>"
```

### 6. Use A Short, Repeatable Debug Loop

Keep the loop tight and visible.

```powershell
.\wsl-tmux.cmd clean-state
.\wsl-tmux.cmd "./dialtone.sh <plugin> <src_vN> test --filter <focused-step>"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 task show --task-id <task-id>"
.\wsl-tmux.cmd "./dialtone.sh repl src_v3 task log --task-id <task-id> --lines 200"
.\wsl-tmux.cmd "./dialtone.sh <plugin> <src_vN> test --filter <focused-step>"
```

## Short Reminder

When in doubt:

- use versioned commands
- use flags for optional behavior
- keep stable config in `env/dialtone.json`
- keep WSL-side work visible with `wsl-tmux.cmd`
- verify changes with focused tests before expanding scope
