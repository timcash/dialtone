# REPL src_v3

`repl src_v3` is the default execution model for `./dialtone.sh`.

For normal operator and LLM use:
- `./dialtone.sh <command>` should find or start a background REPL leader on the local host.
- The command is injected over NATS into the REPL index room.
- The real work runs in a subtone.
- `DIALTONE>` should stay short and high-level.
- Detailed command output should stay in the subtone room and subtone log.

Important:
- `./dialtone.sh --test` is deprecated. Use `./dialtone.sh repl src_v3 test ...`.
- `--subtone` is internal-only and not a user command path.
- Runtime config source of truth is `env/dialtone.json`.

## Plugin Contract

Plugins that want to work cleanly with the REPL should follow the standard Dialtone layout:

```text
src/plugins/<plugin>/
  README.md
  scaffold/main.go
  src_v1/
    go/
    test/cmd/main.go
```

Rules:
- expose versioned commands: `./dialtone.sh <plugin> src_vN <command>`
- keep `scaffold/main.go` thin and move real logic into `src_vN`
- emit only short promoted summaries into `DIALTONE>` via `DIALTONE_INDEX: ...`
- keep detailed stdout, stderr, and debug logs in the subtone
- if tests publish to NATS, use `DIALTONE_REPL_NATS_URL` when present instead of hardcoding `127.0.0.1:4222`

Example:

```bash
# Start a versioned plugin server through the default REPL path.
./dialtone.sh cad src_v1 serve

# Run the plugin's smoke suite through the same REPL path.
./dialtone.sh cad src_v1 test
```

## Mental Model

- Every host should run a background REPL leader.
- The REPL leader owns the host-local embedded NATS endpoint.
- The default room is `index`.
- `./dialtone.sh <command>` usually means “inject this command into the local REPL index room.”
- The REPL leader spawns a subtone for each injected command.
- The subtone gets its own room, pid, and log file.
- Raw command payload belongs in the subtone, not in the index room.

In practice:
- Index room = control plane and status summary.
- Subtone room = command-specific detail.
- Subtone log file = durable full output on disk.

## Output Contract

LLM agents should expect this pattern.

Foreground command:

```text
user> /robot src_v2 diagnostic --host rover --skip-ui --public-check=false
DIALTONE> Request received. Spawning subtone for robot src_v2...
DIALTONE> Subtone started as pid 165795.
DIALTONE> Subtone room: subtone-165795
DIALTONE> Subtone log file: /home/user/dialtone/.dialtone/logs/subtone-165795-20260317-161431.log
DIALTONE> robot diagnostic: checking local artifacts
DIALTONE> robot diagnostic: checking rover runtime on rover
DIALTONE> robot diagnostic: completed
DIALTONE> Subtone for robot src_v2 exited with code 0.
```

Background command:

```text
user> /repl src_v3 watch --subject repl.room.index --filter bg &
DIALTONE> Request received. Spawning subtone for repl src_v3...
DIALTONE> Subtone started as pid 171214.
DIALTONE> Subtone room: subtone-171214
DIALTONE> Subtone log file: /tmp/.../subtone-171214-....log
DIALTONE> Subtone for repl src_v3 is running in background.
```

Attach mode:

```text
DIALTONE> Attached to subtone-172562.
DIALTONE:172562> Probe target=wsl transport=ssh user=user port=22
DIALTONE> Detached from subtone-172562.
```

Rules:
- `user>` echoes the command that entered the room.
- `DIALTONE>` is index-room lifecycle/status.
- `DIALTONE:<pid>` is attached subtone output.
- Full raw stdout/stderr should not be mirrored into `DIALTONE>` unless intentionally promoted as a short status line.

## Modes

### 1. Routed Command Mode

Use this most of the time:

```bash
./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui --public-check=false
./dialtone.sh robot src_v2 publish --repo timcash/dialtone
./dialtone.sh ssh src_v1 run --host rover --cmd hostname
```

Behavior:
- Dialtone finds the configured local REPL NATS URL from `env/dialtone.json`.
- If the leader is not running, Dialtone starts it in the background.
- The command is injected into the local REPL index room.
- The shell prints the same high-level index-room lifecycle that an interactive REPL user would see.

This is the default operator path. Do not use explicit `inject` unless you need to target a non-default leader.

### 2. Interactive Join Mode

Use this when you want to sit in the REPL room and type commands manually:

```bash
./dialtone.sh
./dialtone.sh repl src_v3 join --name observer --room index
```

Behavior:
- You attach to the index room.
- Commands entered there spawn subtones.
- `/ps`, `/subtone-attach`, `/subtone-detach`, and `/repl ...` are available.

### 3. Explicit Inject Mode

Use this when you need to pin a specific NATS endpoint or leader:

```bash
./dialtone.sh repl src_v3 inject --user llm-codex go src_v1 version
./dialtone.sh repl src_v3 inject --user llm-codex --host grey go src_v1 version
./dialtone.sh repl src_v3 inject --nats-url nats://127.0.0.1:47222 --user llm-codex robot src_v2 publish --repo timcash/dialtone
```

Use cases:
- isolated local test leader
- forcing a specific `--nats-url`
- targeting a specific REPL host for transport routing

### 4. Background Subtone Mode

Append `&` inside the REPL prompt:

```text
/repl src_v3 watch --subject repl.room.index --filter bg &
```

Behavior:
- Index room prints spawn metadata and “running in background”.
- The subtone continues after the command prompt returns.
- Inspect with `/ps`, `subtone-list`, and `subtone-log`.

### 5. Attach Mode

Attach to a live subtone room:

```text
/subtone-attach --pid 172562
/subtone-detach
```

Behavior:
- `DIALTONE:<pid>` streams the attached subtone.
- This is the correct place for detailed command output when you need it.

## Where Logs Are

There are three useful views:

1. Index room
- Short lifecycle and operator-facing summaries.

2. Subtone room
- Live command-specific detail.
- Only visible when attached or watching the bus.

3. Subtone log file
- Durable on-disk record of the subtone.
- Local path is printed as:

```text
DIALTONE> Subtone log file: /home/user/dialtone/.dialtone/logs/subtone-165795-....log
```

Common log locations:
- repo-local routed commands: `.dialtone/logs/subtone-<pid>-<timestamp>.log`
- tmp REPL bootstrap tests: `/tmp/.../repo/.dialtone/logs/subtone-<pid>-<timestamp>.log`

Useful commands:

```bash
./dialtone.sh repl src_v3 subtone-list --count 20
./dialtone.sh repl src_v3 subtone-log --pid 12345 --lines 200
./dialtone.sh repl src_v3 watch --subject 'repl.>'
./dialtone.sh logs src_v1 stream --topic 'repl.>'
```

Use `subtone-list` to map pid -> command.
Use `subtone-log --pid` to fetch the actual saved log content.

## NATS Model

- REPL leader owns the embedded NATS endpoint for the host.
- Default NATS URL comes from `DIALTONE_REPL_NATS_URL` in `env/dialtone.json`.
- Default room is `index`.
- REPL subjects use the `repl.*` namespace.
- `repl.room.index` is the human-facing index room.
- `repl.subtone.<pid>` is the subtone room stream.
- `repl.cmd` is the command subject used for injection.

Agents should assume:
- if the local leader is reachable, use it
- if it is not reachable, Dialtone should autostart it
- direct SSH is fallback/bootstrap, not the primary orchestration path

## Transport Routing

Current behavior:
- `./dialtone.sh <command>` routes to the local REPL by default.
- `--target-host <host>` routes to another REPL host over NATS/tsnet/LAN.
- `--ssh-host <host>` forces SSH transport instead of REPL/NATS.

Important exception:
- some plugins keep plugin-local `--host` semantics
- currently `robot`, `autoswap`, `ssh`, and `repl` keep plugin-local `--host`
- use `--target-host` when you want REPL transport routing and the command itself also has a `--host` flag

Examples:

```bash
./dialtone.sh go src_v1 version
./dialtone.sh go src_v1 version --target-host grey
./dialtone.sh go src_v1 version --ssh-host grey
./dialtone.sh robot src_v2 diagnostic --host rover
./dialtone.sh autoswap src_v1 update --host rover
```

## Core Commands

```bash
./dialtone.sh repl src_v3 help
./dialtone.sh repl src_v3 install
./dialtone.sh repl src_v3 format
./dialtone.sh repl src_v3 lint
./dialtone.sh repl src_v3 check
./dialtone.sh repl src_v3 build
./dialtone.sh repl src_v3 test

./dialtone.sh repl src_v3 run --nats-url nats://127.0.0.1:4222 --room index --name user
./dialtone.sh repl src_v3 leader --embedded-nats --nats-url nats://0.0.0.0:4222 --room index
./dialtone.sh repl src_v3 join --nats-url nats://127.0.0.1:4222 --room index --name observer
./dialtone.sh repl src_v3 status
./dialtone.sh repl src_v3 service --mode run --room index

./dialtone.sh repl src_v3 inject --user llm-codex go src_v1 version
./dialtone.sh repl src_v3 inject --user llm-codex --host grey go src_v1 version
./dialtone.sh repl src_v3 watch --subject 'repl.>' --filter 'DIALTONE:'
./dialtone.sh repl src_v3 subtone-list --count 20
./dialtone.sh repl src_v3 subtone-log --pid 12345 --lines 200

./dialtone.sh repl src_v3 bootstrap
./dialtone.sh repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user
./dialtone.sh repl src_v3 bootstrap-http --host 127.0.0.1 --port 8811

./dialtone.sh repl src_v3 process-clean --dry-run
./dialtone.sh repl src_v3 process-clean
./dialtone.sh repl src_v3 test-clean --dry-run
./dialtone.sh repl src_v3 test-clean
```

## Tests

`./dialtone.sh repl src_v3 test` is the main integration check.

What it validates:
- tmp bootstrap from empty workspace
- leader startup and room lifecycle
- index-room vs subtone-room separation
- background subtone lifecycle
- SSH resolve/probe/run through REPL
- subtone list/log consistency
- attach/detach behavior
- Cloudflare tunnel step when Cloudflare config is present

Current WSL behavior:
- SSH coverage passes when `wsl` mesh host has usable auth in `env/dialtone.json`
- Cloudflare tunnel coverage is skipped cleanly when this host does not have `DIALTONE_DOMAIN` and Cloudflare provisioning credentials

Examples:

```bash
./dialtone.sh repl src_v3 test

DIALTONE_REPL_V3_TEST_INSTALL_URL="https://shell.dialtone.earth/install.sh" \
./dialtone.sh repl src_v3 test

DIALTONE_REPL_V3_TEST_INSTALL_URL="https://shell.dialtone.earth/install.sh" \
DIALTONE_REPL_V3_TEST_WSL_HOST="wsl.shad-artichoke.ts.net" \
DIALTONE_REPL_V3_TEST_WSL_USER="user" \
./dialtone.sh repl src_v3 test
```

## Watching Traffic

Interactive observer:

```bash
./dialtone.sh repl src_v3 join --name observer --room index
```

Raw NATS tap:

```bash
./dialtone.sh logs src_v1 stream --topic 'repl.>'
./dialtone.sh logs src_v1 stream --topic 'logs.test.>'
```

Independent passive tap repo:
- `https://github.com/timcash/dialtone-tap`
- reconnects automatically
- never starts NATS
- subscribe-only
