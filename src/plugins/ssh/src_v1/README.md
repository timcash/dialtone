# ssh Plugin

SSH transport utilities used by plugins that need remote access/tunneling.

Current usage includes:
- robot deploy/dev SSH operations
- logs remote stream mode

## CLI

- `./dialtone.sh ssh src_v1 mesh`
- `./dialtone.sh ssh src_v1 run --node rover --cmd "hostname"`
- `./dialtone.sh ssh src_v1 run-all --cmd "hostname"`
- `./dialtone.sh ssh src_v1 sync-repos --branch feat/robot-src-v4-split-runtime`
- `./dialtone.sh ssh src_v1 sync-code --node rover --src /home/user/dialtone --dest /home/tim/dialtone --delete`
- `./dialtone.sh ssh src_v1 bootstrap --node darkmac --src /home/user/dialtone --dest /Users/tim/dialtone --delete`
- `./dialtone.sh ssh src_v1 test`

## Bootstrap

`bootstrap` is the one-shot remote setup flow for new machines:
1. sync code with `rsync` (same excludes as `sync-code`)
2. run install commands remotely
3. run a verification command remotely

Supported flags:
- `--node <name|all>` required target node
- `--src <path>` source path on current machine (default: cwd)
- `--dest <path>` destination path on target (default: node-specific repo path)
- `--delete` remove files on target that are not in source
- `--no-sync` skip rsync and run install/verify only
- `--install-cmd "<command>"` repeatable remote install commands
- `--verify-cmd "<command>"` post-install verification command

Defaults:
- install command: `printf 'y\n' | ./dialtone.sh go src_v1 install`
- verify command: `./dialtone.sh go src_v1 exec version`

## Mesh behavior

- Node aliases are centralized in `src_v1/go/mesh.go`.
- Darkmac default mesh account is `dialtone` (home: `/Users/dialtone`).
- Default transport is Go SSH (`golang.org/x/crypto/ssh`).
- On WSL, commands targeting `legion` automatically use local `powershell.exe` transport so callers do not need WSL-specific branching.
- Chroma host selection is adaptive: plugin prefers LAN `192.168.4.53` when reachable, then falls back to `chroma-1.shad-artichoke.ts.net`.
- Rover host selection is adaptive: plugin prefers direct ethernet `169.254.217.151` when reachable, then falls back to `rover-1.shad-artichoke.ts.net`.
- Darkmac host selection is adaptive: plugin prefers LAN `192.168.4.31` when reachable, then falls back to `darkmac.shad-artichoke.ts.net`.
- `sync-repos` updates each node to the same branch using node-specific repo paths.
- `sync-repos` skips dirty repos by default; use `--allow-dirty` to force.
- Per-node repo overrides are supported with flags like `--repo-legion /path/to/dialtone`.
- `sync-code` uses `rsync` to mirror working tree changes without requiring commits.
- `sync-code` excludes `node_modules`, `.pixi`, `.git`, and `bin` by default.

## New machine from scratch

```bash
# 1) Sync local working tree (no git required on remote)
./dialtone.sh ssh src_v1 bootstrap \
  --node darkmac \
  --src /home/user/dialtone \
  --dest /Users/tim/dialtone \
  --delete

# 2) Bootstrap all mesh nodes with node default destinations
./dialtone.sh ssh src_v1 bootstrap \
  --node all \
  --src /home/user/dialtone \
  --delete

# 3) Optional: add extra remote install steps
./dialtone.sh ssh src_v1 bootstrap \
  --node rover \
  --src /home/user/dialtone \
  --dest /home/tim/dialtone \
  --install-cmd "printf 'y\n' | ./dialtone.sh go src_v1 install" \
  --install-cmd "./dialtone.sh go src_v1 exec env GOROOT" \
  --verify-cmd "./dialtone.sh go src_v1 exec version"

# 4) Re-run install/verify only (no file sync)
./dialtone.sh ssh src_v1 bootstrap \
  --node darkmac \
  --no-sync \
  --install-cmd "printf 'y\n' | ./dialtone.sh go src_v1 install"
```
