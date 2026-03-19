# REPL src_v3

`repl src_v3` is the default runtime for `./dialtone.sh`.

Use this mental model:
- plain `./dialtone.sh <plugin> ...` injects into the local REPL leader
- the leader runs the real command in a subtone or manages a long-lived service
- NATS is the control plane for requests, lifecycle updates, and room traffic
- `DIALTONE>` stays short and high-level
- full output stays in the subtone log

Current terminology:
- `DIALTONE>`: the top-level control room
- `subtone`: one foreground/background command execution
- `service`: one long-lived managed process, such as `chrome src_v3` on `legion`
- `room`: the event/log stream for a subtone or service

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

Expected shell pattern:

```text
legion> /robot src_v2 diagnostic --host rover --skip-ui --public-check=false
DIALTONE> Request received. Spawning subtone for robot src_v2...
DIALTONE> Subtone started as pid 546289.
DIALTONE> Subtone room: subtone-546289
DIALTONE> Subtone log file: /home/user/dialtone/.dialtone/logs/subtone-546289-20260317-181050.log
DIALTONE> robot diagnostic: checking local artifacts
DIALTONE> robot diagnostic: checking rover runtime on rover
DIALTONE> robot diagnostic: autoswap service and manifest look healthy
DIALTONE> robot diagnostic: active manifest matches latest release channel
DIALTONE> robot diagnostic: rover API and telemetry endpoints passed
DIALTONE> robot diagnostic: completed
DIALTONE> Subtone for robot src_v2 exited with code 0.
```

## REPL Standards

`DIALTONE>` should contain:
- request receipt
- subtone lifecycle
- service lifecycle
- short stage summaries
- final success or failure

`DIALTONE>` should not contain:
- raw JSON
- stack traces
- long build output
- repeated polling noise
- browser console spam

That detail belongs in the subtone log.

## Tasks And Services

Use normal plugin commands for one-shot work:

```bash
# Foreground task routed through REPL.
./dialtone.sh cad src_v1 build
./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui --public-check=false
```

Use service mode for long-lived processes:

```bash
# Start a daemon once and reuse it.
./dialtone.sh chrome src_v3 service --host legion --mode start --role cad-smoke

# Check the same daemon later without relaunching it.
./dialtone.sh chrome src_v3 status --host legion --role cad-smoke
```

The intended service contract is:
- `service start` may launch the service
- `status` should only query
- later commands should reuse the existing healthy service

## Subtone Logs

```bash
# List recent subtones and their pid/command.
./dialtone.sh repl src_v3 subtone-list --count 20

# Read the saved log for one subtone.
./dialtone.sh repl src_v3 subtone-log --pid 546289 --lines 200

# Watch live REPL bus traffic when debugging the control plane.
./dialtone.sh repl src_v3 watch --subject 'repl.room.index'
```

Use `subtone-list` first, then `subtone-log`.

When debugging a long-lived service, pair those with the service command itself:

```bash
# Check the service first.
./dialtone.sh chrome src_v3 status --host legion --role dev

# Then inspect the subtone that ran the status check.
./dialtone.sh repl src_v3 subtone-list --count 10
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200
```

## Background Commands

Background mode is allowed only for one command with a trailing `&`.

```bash
# Start a watcher in the background.
./dialtone.sh repl src_v3 inject --user llm-codex "repl src_v3 watch --subject repl.room.index &"

# A long-lived service manager command can also run this way when you are explicitly debugging rooms.
./dialtone.sh repl src_v3 inject --user llm-codex "repl src_v3 watch --subject 'repl.host.>' &"
```

Expected pattern:

```text
DIALTONE> Request received. Spawning subtone for repl src_v3...
DIALTONE> Subtone started as pid 171214.
DIALTONE> Subtone room: subtone-171214
DIALTONE> Subtone log file: /home/user/dialtone/.dialtone/logs/subtone-171214-....log
DIALTONE> Subtone for repl src_v3 is running in background.
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
DIALTONE> DIALTONE ERROR: run exactly one ./dialtone.sh command at a time; command chaining with "&&" is not allowed. Use one foreground command per turn, or a single command with a trailing & for background mode.
```

## Explicit REPL Commands

Use these only when you need direct REPL control.

```bash
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

# 2. If it fails or looks incomplete, inspect the subtone log.
./dialtone.sh repl src_v3 subtone-list --count 10
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200

# 3. Then run the next command.
./dialtone.sh autoswap src_v1 update --host rover
./dialtone.sh robot src_v2 diagnostic --host rover --skip-ui --public-check=false
```

Do not guess from partial `DIALTONE>` output when a subtone log is available.

For Chrome/CAD/UI debugging, use this pattern:

```bash
# 1. Start the daemon role once.
./dialtone.sh chrome src_v3 service --host legion --mode start --role cad-smoke

# 2. Confirm the daemon is healthy.
./dialtone.sh chrome src_v3 status --host legion --role cad-smoke

# 3. Run the browser-driven command or test against that role.
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke

# 4. If it fails, inspect the subtone log.
./dialtone.sh repl src_v3 subtone-list --count 10
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 250
```

The main rule is:
- keep the leader running
- keep long-lived daemons running
- send commands over NATS through the REPL
- only redeploy or restart when health actually requires it
