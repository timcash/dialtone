# ssh Plugin

SSH transport utilities used by plugins that need remote access/tunneling.

Current usage includes:
- robot deploy/dev SSH operations
- logs remote stream mode

## Complete Shell Workflow

```bash
# Discover mesh nodes and transport mode
./dialtone.sh ssh src_v1 mesh
./dialtone.sh ssh src_v1 nodes
./dialtone.sh ssh src_v1 list

# Run one command on one host (preferred flag: --host; --node still works)
./dialtone.sh ssh src_v1 run --host rover --cmd "hostname"
./dialtone.sh ssh src_v1 run --host darkmac --cmd "pwd" --user tim --port 22
./dialtone.sh ssh src_v1 run --host legion --cmd "whoami"
./dialtone.sh ssh src_v1 run --host gold --cmd "uname -a"

# Run one command on all nodes
./dialtone.sh ssh src_v1 run-all --cmd "hostname"
./dialtone.sh ssh src_v1 run-all --cmd "pwd" --user tim

# Mesh health snapshot (cpu, mem-free, network, disk-free, battery)
./dialtone.sh ssh src_v1 status
./dialtone.sh ssh src_v1 status --host all
./dialtone.sh ssh src_v1 status --host darkmac,gold,legion
./dialtone.sh ssh src_v1 status --json

# Git-based repo sync across mesh
./dialtone.sh ssh src_v1 sync-repos
./dialtone.sh ssh src_v1 sync-repos --branch main
./dialtone.sh ssh src_v1 sync-repos --branch feat/my-branch --allow-dirty
./dialtone.sh ssh src_v1 sync-repos --branch feat/my-branch --repo-rover /home/tim/dialtone

# Rsync code sync (single run)
./dialtone.sh ssh src_v1 sync-code --host gold --src /home/user/dialtone
./dialtone.sh ssh src_v1 sync-code --host darkmac --src /home/user/dialtone --dest /Users/tim/dialtone --delete
./dialtone.sh ssh src_v1 sync-code --host all --src /home/user/dialtone --delete
# skip-self is true by default for --host all; set false to include current node
./dialtone.sh ssh src_v1 sync-code --host all --src /home/user/dialtone --delete --skip-self=false
./dialtone.sh ssh src_v1 sync-code --host rover --src /home/user/dialtone --exclude '.env.local'

# Rsync code sync (persistent service mode on local machine)
./dialtone.sh ssh src_v1 sync-code --host all --src /home/user/dialtone --delete --service --interval 30s
./dialtone.sh ssh src_v1 sync-code --service-status
./dialtone.sh ssh src_v1 sync-code --service-stop

# Bootstrap a host (sync + install + verify)
./dialtone.sh ssh src_v1 bootstrap --host gold --src /home/user/dialtone --dest /Users/user/dialtone --delete
./dialtone.sh ssh src_v1 bootstrap --host darkmac --src /home/user/dialtone --dest /Users/tim/dialtone --delete
./dialtone.sh ssh src_v1 bootstrap --host all --src /home/user/dialtone --delete
./dialtone.sh ssh src_v1 bootstrap --host legion --no-sync --install-cmd "./dialtone.sh go src_v1 install"
./dialtone.sh ssh src_v1 bootstrap --host rover --install-cmd "./dialtone.sh go src_v1 install" --verify-cmd "./dialtone.sh go src_v1 exec version"

# Plugin verification suite
./dialtone.sh ssh src_v1 test
```

## CLI

- `./dialtone.sh ssh src_v1 mesh`
- `./dialtone.sh ssh src_v1 run --host rover --cmd "hostname"`
- `./dialtone.sh ssh src_v1 run-all --cmd "hostname"`
- `./dialtone.sh ssh src_v1 status --host all`
- `./dialtone.sh ssh src_v1 sync-repos --branch feat/robot-src-v4-split-runtime`
- `./dialtone.sh ssh src_v1 sync-code --host rover --src /home/user/dialtone --dest /home/tim/dialtone --delete`
- `./dialtone.sh ssh src_v1 sync-code --host all --src /home/user/dialtone --delete --service --interval 30s`
- `./dialtone.sh ssh src_v1 sync-code --service-status`
- `./dialtone.sh ssh src_v1 sync-code --service-stop`
- `./dialtone.sh ssh src_v1 bootstrap --host darkmac --src /home/user/dialtone --dest /Users/tim/dialtone --delete`
- `./dialtone.sh ssh src_v1 test`

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

## Mesh behavior

- Node aliases are centralized in `src_v1/go/mesh.go`.
- Preferred command flag is `--host`; `--node` is retained as a backward-compatible alias.
- Active mesh hosts are `darkmac`, `gold`, `legion`, `rover`, and `wsl` (no `chroma` entry).
- Darkmac default mesh account is `tim` (home: `/Users/tim`).
- Gold default mesh account is `user` (home: `/Users/user`).
- Default transport is Go SSH (`golang.org/x/crypto/ssh`).
- Legion now uses SSH transport on port `2223` (user `user`) instead of local PowerShell transport.
- Darkmac host selection is LAN-first (`192.168.4.31`) with tailnet fallback.
- Rover host selection prefers tailscale (`rover-1.shad-artichoke.ts.net`) first, then link-local (`169.254.217.151`, "link-local"), then LAN (`192.168.4.36` on cashwifi). Other routes remain fallback.
- WSL host selection is LAN-first (`192.168.4.52`) with tailnet fallback.
- `sync-repos` updates each node to the same branch using node-specific repo paths.
- `sync-repos` skips dirty repos by default; use `--allow-dirty` to force.
- Per-node repo overrides are supported with flags like `--repo-legion /path/to/dialtone`.
- `sync-code` uses `rsync` to mirror working tree changes without requiring commits.
- `sync-code` excludes `node_modules`, `.pixi`, `.git`, and `bin` by default.
- `sync-code --service` installs a local user `systemd` loop service (`dialtone-ssh-sync-code.service`) that reruns sync at the requested interval.
- `sync-code --service-status` shows service status.
- `sync-code --service-stop` stops/disables the service.

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
