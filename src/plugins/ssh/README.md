# ssh Plugin

SSH transport utilities used by plugins that need remote access/tunneling.

Current usage includes:
- robot deploy/dev SSH operations
- logs remote stream mode

## CLI

- `./dialtone.sh ssh src_v1 mesh`
- `./dialtone.sh ssh src_v1 run --node rover --cmd "hostname"`
- `./dialtone.sh ssh src_v1 run-all --cmd "hostname"`
- `./dialtone.sh ssh src_v1 test`

## Mesh behavior

- Node aliases are centralized in `src_v1/go/mesh.go`.
- Default transport is Go SSH (`golang.org/x/crypto/ssh`).
- On WSL, commands targeting `legion` automatically use local `powershell.exe` transport so callers do not need WSL-specific branching.
