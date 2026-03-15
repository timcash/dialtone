# ssh Plugin

SSH transport utilities used by plugins that need remote access/tunneling.

Config source of truth:
- `env/dialtone.json` (`mesh_nodes` and related keys)
- legacy `env/.env`, `env/mesh.json`, and ad hoc SSH host files are deprecated
- SSH auth comes from explicit config only: `mesh_nodes[].password`, `mesh_nodes[].ssh_private_key`, `mesh_nodes[].ssh_private_key_path`, or CLI flags

Current usage includes:
- robot deploy/dev SSH operations
- logs remote stream mode

## Core Commands

### Debug & Discovery
- `./dialtone.sh ssh src_v1 mesh` / `nodes`: List configured mesh nodes.
- `./dialtone.sh ssh src_v1 resolve --host grey`: Resolve mesh name to IP/host.
- `./dialtone.sh ssh src_v1 probe --host grey`: Test connectivity and auth.
- `./dialtone.sh ssh src_v1 tailnet-check`: Verify SSH connectivity over Tailscale for all nodes.

### Execution
- `./dialtone.sh ssh src_v1 run --host rover --cmd "hostname"`: Run command on one node.
- `./dialtone.sh ssh src_v1 run-all --cmd "uptime"`: Run on all nodes in parallel.
- `./dialtone.sh ssh src_v1 status --host all`: Get mesh-wide health (CPU, Mem, Disk).

### Code Sync & Lifecycle
- `./dialtone.sh ssh src_v1 sync-code --host gold --delete`: Rsync local changes (ignores `node_modules`, `.git`).
- `./dialtone.sh ssh src_v1 sync-repos --branch main`: Git-based sync for all nodes.
- `./dialtone.sh ssh src_v1 bootstrap --host rover`: One-shot remote setup (sync + install + verify).
- `./dialtone.sh ssh src_v1 key-setup --host wsl`: Bootstrap passwordless SSH keys.

## Agent & System Internals

### NATS-Logged Transport
When running SSH commands via the orchestrator (`./dialtone.sh ssh ...`), logs are typically captured and routed to NATS rather than direct stdout.
- **Recommended Debugging**: `./dialtone.sh repl src_v3 watch --subject 'repl.>' --filter 'ssh src_v1'`
- **Subtone Logs**: The orchestrator runs these as "subtones". Use `subtone-list` and `subtone-log` in the REPL to inspect hung or verbose remote processes.

### Mesh Behavior & Defaults
- **Source of Truth**: `env/dialtone.json` (the `mesh_nodes` array).
- **Auth**: Explicit only from mesh node config or CLI flags. No implicit `~/.ssh` scan or ssh-agent fallback.
- **Route Preference**: Nodes prioritize **Tailscale** (`.ts.net`) first, then **LAN IPs**, then link-local fallbacks.
- **Node Specifics**:
  - `legion`: Windows host; prefers PowerShell transport when called from WSL.
  - `rover`: Linux (Raspberry Pi); has complex route preferences including link-local debug fallbacks.
  - `gold`/`grey`: macOS hosts; use standard Go SSH transport.

## Bootstrap

`bootstrap` is the one-shot remote setup flow for new machines:
1. sync code with `rsync` (same excludes as `sync-code`)
2. run install commands remotely
3. run a verification command remotely

Supported flags:
- `--host <name|all>` required target host (`--node` alias still accepted)
- `--src <path>` source path on current machine (default: cwd)
- `--dest <path>` destination path on target (default: node-specific repo path)
- `--delete` remove files on target that are not in source
- `--no-sync` skip rsync and run install/verify only
- `--install-cmd "<command>"` repeatable remote install commands
- `--verify-cmd "<command>"` post-install verification command

Defaults:
- install command: `printf 'y\n' | ./dialtone.sh go src_v1 install`
- verify command: `./dialtone.sh go src_v1 exec version`

## Sync Code

`sync-code` mirrors a working tree to one mesh node or all mesh nodes using `rsync`.

Common flags:
- `--host <name|all>` required target (`--node` alias still accepted)
- `--src <path>` source path on local node (default: cwd)
- `--dest <path>` destination path on target node (default: node-specific repo path)
- `--delete` remove destination files that do not exist in source
- `--exclude <pattern>` extra rsync exclude (repeatable)

Service flags (local machine only):
- `--service` install/start local user systemd loop service
- `--interval <duration>` loop interval when `--service` is set (default: `30s`)
- `--service-status` show local service status
- `--service-stop` stop/disable local service

Notes:
- Service unit name: `dialtone-ssh-sync-code.service`
- Service mode runs on the machine where you execute the command (for example WSL), then syncs remote mesh targets from there.
- Service mode requires user `systemd` availability.

```bash
# One-time sync to gold
./dialtone.sh ssh src_v1 sync-code \
  --host gold \
  --src /home/user/dialtone \
  --delete

# One-time sync to all mesh nodes
./dialtone.sh ssh src_v1 sync-code \
  --host all \
  --src /home/user/dialtone \
  --delete

# Start continuous sync service (runs on this local machine)
./dialtone.sh ssh src_v1 sync-code \
  --host all \
  --src /home/user/dialtone \
  --delete \
  --service \
  --interval 30s

# Check service status
./dialtone.sh ssh src_v1 sync-code --service-status

# Stop/disable service
./dialtone.sh ssh src_v1 sync-code --service-stop
```

## New machine from scratch

```bash
# 1) Sync local working tree (no git required on remote)
./dialtone.sh ssh src_v1 bootstrap \
  --host darkmac \
  --src /home/user/dialtone \
  --dest /Users/tim/dialtone \
  --delete

# 2) Bootstrap all mesh nodes with node default destinations
./dialtone.sh ssh src_v1 bootstrap \
  --host all \
  --src /home/user/dialtone \
  --delete

# 3) Optional: add extra remote install steps
./dialtone.sh ssh src_v1 bootstrap \
  --host rover \
  --src /home/user/dialtone \
  --dest /home/tim/dialtone \
  --install-cmd "printf 'y\n' | ./dialtone.sh go src_v1 install" \
  --install-cmd "./dialtone.sh go src_v1 exec env GOROOT" \
  --verify-cmd "./dialtone.sh go src_v1 exec version"

# 4) Re-run install/verify only (no file sync)
./dialtone.sh ssh src_v1 bootstrap \
  --host darkmac \
  --no-sync \
  --install-cmd "printf 'y\n' | ./dialtone.sh go src_v1 install"
```


```bash
# 1) Sync local working tree (no git required on remote)
./dialtone.sh ssh src_v1 bootstrap \
  --host darkmac \
  --src /home/user/dialtone \
  --dest /Users/tim/dialtone \
  --delete

# 2) Bootstrap all mesh nodes with node default destinations
./dialtone.sh ssh src_v1 bootstrap \
  --host all \
  --src /home/user/dialtone \
  --delete

# 3) Optional: add extra remote install steps
./dialtone.sh ssh src_v1 bootstrap \
  --host rover \
  --src /home/user/dialtone \
  --dest /home/tim/dialtone \
  --install-cmd "printf 'y\n' | ./dialtone.sh go src_v1 install" \
  --install-cmd "./dialtone.sh go src_v1 exec env GOROOT" \
  --verify-cmd "./dialtone.sh go src_v1 exec version"

# 4) Re-run install/verify only (no file sync)
./dialtone.sh ssh src_v1 bootstrap \
  --host darkmac \
  --no-sync \
  --install-cmd "printf 'y\n' | ./dialtone.sh go src_v1 install"
```
