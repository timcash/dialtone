# REPL Standards

This file defines the current and intended standard for how Dialtone plugins should behave when executed through the REPL-first `./dialtone.sh` flow.

The goal is:
- keep `DIALTONE>` high-level and low-noise
- keep detailed output in the subtone room/log
- make plugin behavior predictable for operators and LLM agents

## Core Model

Plain command execution should be REPL-first:

```bash
./dialtone.sh <plugin> <version> <command> ...
```

Expected flow:
1. `./dialtone.sh` finds or autostarts the local REPL leader.
2. The command is injected into the index room over NATS.
3. The REPL spawns a subtone for the plugin command.
4. The shell prints only index-room lifecycle and promoted summary lines.
5. Detailed output remains in the subtone room/log.

## Shared Modes

### Foreground

Normal shell usage:

```text
host> /plugin src_vN ...
DIALTONE> Request received. Spawning subtone for plugin src_vN...
DIALTONE> Subtone started as pid ...
DIALTONE> Subtone room: subtone-...
DIALTONE> Subtone log file: ...
DIALTONE> [optional plugin summary lines]
DIALTONE> Subtone for plugin src_vN exited with code 0.
```

### Background

For commands intentionally launched in the background:

```text
host> /plugin src_vN ... &
DIALTONE> Request received. Spawning subtone for plugin src_vN...
DIALTONE> Subtone started as pid ...
DIALTONE> Subtone room: subtone-...
DIALTONE> Subtone log file: ...
DIALTONE> Subtone for plugin src_vN is running in background.
```

### Attach

Attach to a running subtone for full detail:

```text
DIALTONE> Attached to subtone-12345.
DIALTONE:12345> ...
DIALTONE> Detached from subtone-12345.
```

### Logs

Use:

```bash
./dialtone.sh repl src_v3 subtone-list --count 20
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200
```

## Index-Room Rules

`DIALTONE>` should contain:
- request receipt
- subtone lifecycle
- 1-3 meaningful plugin summary lines per stage
- high-level recovery/autostart notices
- final success/failure line

`DIALTONE>` should not contain:
- raw JSON
- stack traces
- repeated polling
- long command output
- browser console spam
- large build logs
- repeated transport/helper noise

Those belong in the subtone log or attached subtone stream.

## Host Flag Standard

Top-level `./dialtone.sh` must not steal plugin-local `--host` for plugins where `--host` means operational target.

Current plugins that must retain plugin-local `--host`:
- `ssh`
- `repl`
- `autoswap`
- `robot`
- `chrome`

When transport routing is needed for REPL itself, use:
- `--target-host`
- `--ssh-host`

## Plugin Patterns

### `repl src_v3`

Role:
- owns the room/index/subtone model
- provides `watch`, `join`, `attach`, `subtone-list`, `subtone-log`

Expected summaries:
- mostly shared lifecycle only
- index room should remain generic and stable

### `robot src_v2`

Status:
- already close to the desired standard

Current promoted patterns:

```text
DIALTONE> robot publish: preparing release assets for ...
DIALTONE> robot diagnostic: checking rover runtime on rover
DIALTONE> robot diagnostic: completed
```

Expected use:
- foreground commands
- rollout/publish/diagnostic summaries
- full diagnostics in subtone log

### `chrome src_v3`

Status:
- now aligned closely with `robot`

Current promoted patterns:

```text
DIALTONE> chrome screenshot: capturing managed tab on legion role=dev
DIALTONE> chrome service: ensuring daemon on legion role=dev
DIALTONE> chrome screenshot: saved to /abs/path.png
```

Expected use:
- foreground commands
- service autostart surfaced in index room
- action/navigation summaries in index room
- browser console and raw response detail in subtone log

### `ssh src_v1`

Status:
- shared lifecycle works
- summary layer is still minimal

Current pattern:

```text
host> /ssh src_v1 resolve --host rover
DIALTONE> Request received. Spawning subtone for ssh src_v1...
...
DIALTONE> Subtone for ssh src_v1 exited with code 0.
```

Recommended future summaries:
- `ssh resolve: resolving rover`
- `ssh probe: checking transport/auth for rover`
- `ssh run: executing remote command on rover`
- `ssh sync-code: syncing repo to rover`
- `ssh bootstrap: provisioning rover`

### `cloudflare src_v1`

Status:
- shared lifecycle works
- summary layer is still minimal

Current pattern:

```text
host> /cloudflare src_v1 install
DIALTONE> Request received. Spawning subtone for cloudflare src_v1...
...
DIALTONE> Subtone for cloudflare src_v1 exited with code 0.
```

Recommended future summaries:
- `cloudflare tunnel: creating <name>`
- `cloudflare tunnel: starting <name> -> <url>`
- `cloudflare robot: exposing rover-1.dialtone.earth`
- `cloudflare serve: forwarding local service`

### `autoswap src_v1`

Status:
- shared lifecycle works
- plugin-local `--host` is preserved
- summary layer is still minimal

Current pattern:

```text
host> /autoswap src_v1 update --host rover
DIALTONE> Request received. Spawning subtone for autoswap src_v1...
...
DIALTONE> Subtone for autoswap src_v1 exited with code 0.
```

Recommended future summaries:

For deploy:

```text
DIALTONE> autoswap deploy: preparing service on rover
DIALTONE> autoswap deploy: manifest source is robot_src_v2_channel.json
DIALTONE> autoswap deploy: service installed
```

For update:

```text
DIALTONE> autoswap update: checking latest release channel on rover
DIALTONE> autoswap update: downloading changed artifacts
DIALTONE> autoswap update: switched active manifest
DIALTONE> autoswap update: completed
```

## Internal Helper Subtones

Some commands may trigger helper work such as:
- `tsnet src_v1 up`

These should not pollute the shell transcript for unrelated commands.

Rule:
- helper subtone details stay in logs unless explicitly invoked by the user

## Current Repo Direction

Plugins already using the desired promoted-summary style:
- `robot src_v2`
- `chrome src_v3`

Plugins still needing a summary pass:
- `ssh src_v1`
- `cloudflare src_v1`
- `autoswap src_v1`

## Recommended Next Steps

1. Add `DIALTONE_INDEX:` summary lines to `ssh src_v1`.
2. Add `DIALTONE_INDEX:` summary lines to `cloudflare src_v1`.
3. Add `DIALTONE_INDEX:` summary lines to `autoswap src_v1`.
4. Update the relevant READMEs so this contract is documented consistently.
