# dialtone_mod Remote Tmux/Mosh Design

## Objective

Add host-aware startup and execution semantics for `./dialtone_mod` so one `tmux` session per node is created and reused across interactive and remote command uses.

Target behavior:
- Local interactive run: `./dialtone_mod` opens/attaches to `tmux` session `dialtone-<hostname>`, where `<hostname>` is `DIALTONE_HOSTNAME` (or fallback host derived from local config).
- Remote connect: `./dialtone_mod --host <node>` connects over Tailscale/Mosh (fallback to SSH) and attaches to that node’s `tmux` session `dialtone-<node-hostname>`.
- Remote commands should execute against that node in a Nix-capable shell, ensuring the node has an active `dialtone-<node-hostname>` session to use for future interaction.

## Current constraints

- `dialtone_mod` is already the main wrapper for Nix bootstrap + Go CLI dispatch.
- No `--host` semantics currently exist in `dialtone_mod`.
- Session currently uses fixed name `dialtone` and is not host-aware.
- `env` loading already supports `env/.env` and `--env` override.

## Proposed implementation (small, incremental)

### 1) Host-aware session naming

Add helpers in `dialtone_mod`:
- `dialtone_hostname`: choose session suffix from:
  - explicit `--host` destination when set,
  - `DIALTONE_HOSTNAME` env,
  - fallback of `hostname -s`.
- `dialtone_tmux_session`: fixed prefix `dialtone-` + hostname.
- Window 0 name should be set to the same hostname label (`-n "dialtone-${hostname}"`).

### 2) Parse global flags in `dialtone_mod`

Extend argument parsing to support:
- `--env <path>` / `--env=<path>` (existing behavior)
- `--host <name|alias>`

Preserve positional/command args for downstream `src/mods.go`.

### 3) Local bootstrap behavior remains

When no `--host` is provided:
- Use existing local bootstrap.
- In no-arg mode: create/attach to local session `dialtone-$(dialtone_hostname)` with window `0:$(dialtone_hostname)`.
- Window content uses `bash -i` inside the same Nix shell invocation.

### 4) Remote dispatch mode (`--host`)

Add a dedicated remote execution path in `dialtone_mod`:
- Build a remote command prefix that:
  - resolves/validates remote repo path,
  - optionally writes selected env from local `--env` into the remote command environment,
  - ensures remote `tmux` session exists.
- Connect target using:
  - `mosh` first (uses installed bootstrap package),
  - fallback to `ssh` if mosh is missing/unavailable.

Two branches:

- Interactive remote session (`./dialtone_mod --host <h>`):
  - run remote bootstrap path with no args.
  - remote command should enter local-style Nix env and attach to `tmux new-session -A -s dialtone-<host-hostname>`.

- Remote command (`./dialtone_mod --host <h> <mod> <ver> <cmd> ...`):
  - ensure remote session is started first (if not already running),
  - run the requested `dialtone_mod` command on the remote host via that shell path.
  - keep command execution in the same wrapper path so Nix + mod dispatch still work unchanged.

### 5) Repo path discovery on remote

To avoid hardcoding host-specific paths, resolution order:
1. `DIALTONE_REPO_ROOT` (if set)
2. `DIALTONE_REMOTE_REPO_ROOT` (optional remote override)
3. `~/dialtone`, `/home/user/dialtone`, `/Users/user/dialtone`, `/Users/tim/dialtone`

Fail with a clear error if no valid repo root is found.

## Operational examples (expected)

- `./dialtone_mod --host gold`  
  -> attaches (or creates) `dialtone-gold` via mosh.
- `./dialtone_mod --host rover --env env/gold.env mods v1 list`  
  -> executes command on `rover` in the launcher path and guarantees `tmux` session `dialtone-<rover-hostname>` exists.
- `./dialtone_mod mods v1 list --host gold`  
  -> remote execute form of same command; should succeed even if user does not manually start a remote tmux first.

## Open decision points

1) Keep host argument as `--host` only, or also support positional `host` before `<mod>` for backward compatibility?
2) For remote one-shot commands, is returning captured stdout from remote tmux execution required, or is direct non-interactive execution in remote shell sufficient after ensuring session exists?
3) Should remote path discovery include values from `env/mesh.json` (requires JSON parsing) or stay on fixed path heuristics for now?
