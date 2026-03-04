# Mods System (`src/mods`)

`mods` is the orchestrated command surface for Dialtone.
It is intentionally **non-plugin based**: `src/cli.go` is the orchestrator, and each mod version is invoked as a subprocess.

## Mod Contract

- `src/cli.go` is responsible only for command routing.
  - It resolves `./dialtone2.sh <mod-name> <version> <command> [args]`.
  - It does not execute mod logic directly.
  - It launches the mod entrypoint as a subprocess (`go run <entry> ...`).
- Mod behavior lives in `src/mods/<mod>/<version>/cli/*.go`.
- Standard lifecycle commands should be implemented in the mod CLI:
  - `install`
  - `build`
  - `format`
  - `test`
- Every mod CLI is expected to declare its own command wiring and dependency checks (typically via Nix + Go CLI logic).
- `mods` and `plugins` are separate systems. `mods` does not depend on plugin code.

## Fast Workflow for a New Mod

```sh
# 1) Create a new mod version and CLI directory
mkdir -p src/mods/my_mod/v1/cli

# 2) Add a minimal CLI main
cat > src/mods/my_mod/v1/cli/main.go <<'EOF'
package main

import (
  "fmt"
  "os"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("Usage: ./dialtone2.sh my_mod v1 <command> [args]")
    return
  }
  switch os.Args[1] {
  case "install":
    // run nix checks/install for this mod
  case "build":
    // compile/build artifacts to <repo-root>/bin when possible
  case "format":
    // run gofmt or formatter checks
  case "test":
    // run module tests
  default:
    fmt.Printf("unknown command: %s\n", os.Args[1])
  }
}
EOF

# 3) Compile or syntax-check this CLI
go run ./src/mods/my_mod/v1/cli help

# 4) Validate mod orchestration
./dialtone2.sh mods v1 list
./dialtone2.sh my_mod v1 install
./dialtone2.sh my_mod v1 build
./dialtone2.sh my_mod v1 format
./dialtone2.sh my_mod v1 test
```

## Orchestration (mods v1)

- `./dialtone2.sh mods v1 list`  
  List registered mods and versions.
- `./dialtone2.sh mods v1 new <mod-name>`  
  Create a new mod workspace (subject to current in-repo conventions).
- `./dialtone2.sh mods v1 pull [--host all]`  
  Pull updates across mesh peers then fallback to GitHub.
- `./dialtone2.sh mods v1 status [--short] [--name <mod-name>]`  
  Show status for parent and known mod paths.
- `./dialtone2.sh mods v1 sync [--host <name|all|local>] [--mod NAME...]`  
  Sync selected mods across the mesh.
- `./dialtone2.sh mods v1 rsync [--mod NAME...] [--from <name>] [--repo-dir PATH] [--skip-self=true|false]`  
  Shorthand alias for `sync --host all`.

## Mesh and Git Safety

- Mesh sync commands use fast-forward only by default and fail on risky local conflicts.
- Mesh transport and auth logic is implemented in the mod implementation, not in the orchestrator.

## Note on Other Mods

Example mod entrypoints currently available:
- `./dialtone2.sh mesh v1 <command>`
- `./dialtone2.sh mosh v1 <command>`
- `./dialtone2.sh tsnet v1 <command>`
- `./dialtone2.sh mods v1 <command>`
