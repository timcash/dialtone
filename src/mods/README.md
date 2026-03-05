# Mods System (`src/mods`)

`mods` is the orchestrated command surface for Dialtone.
It is intentionally **non-plugin based**: `src/cli.go` is the orchestrator, and each mod version is invoked as a subprocess.

## Mod Contract

- `src/cli.go` is responsible only for command routing.
  - It resolves `./dialtone_mod <mod-name> <version> <command> [args]`.
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
    fmt.Println("Usage: ./dialtone_mod my_mod v1 <command> [args]")
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
./dialtone_mod mods v1 list
./dialtone_mod my_mod v1 install
./dialtone_mod my_mod v1 build
./dialtone_mod my_mod v1 format
./dialtone_mod my_mod v1 test
```

## Orchestration (mods v1)

- `mods v1` should be run inside the Nix shell via `dialtone_mod` unless you are intentionally bypassing it.
  - Recommended: `./dialtone_mod mods v1 <command> [args]`
- Core workflow:
  - `./dialtone_mod mods v1 list`
    - List all registered mods discovered under `src/mods/*`.
  - `./dialtone_mod mods v1 status [--name <mod-name>] [--short]`
    - Show status for parent repo and known mods.
  - `./dialtone_mod mods v1 commit [--mod <mod-name>] [--message <msg>] [--all]`
    - Commit in target mod or parent repo.
  - `./dialtone_mod mods v1 push [--mod <mod-name>] [--message <msg>] [--dry-run]`
    - Push one mod, or all dirty mods + parent submodule pointers.
  - `./dialtone_mod mods v1 pull [--host <name|all|local>] [--from <name>] [--branch <branch>] [--source PATH] [--dest PATH] [--repo-dir PATH] [--skip-self=true|false] [--dry-run]`
    - Pull updates from remote mesh nodes (or local), fallback to GitHub if needed.
  - `./dialtone_mod mods v1 clean [--host <name|all|local>] [--repo-dir PATH] [--skip-self=true|false] [--dry-run] [--force]`
    - Force the target repo(s) to hard-reset to `origin/<current branch>` and clean dirty tree/submodules.
    - Useful when a host is dirty and blocking `pull`/`sync`.
  - `./dialtone_mod mods v1 reset [--host <name|all|local>] [--from <name>] [--branch <branch>] [--source PATH] [--dest PATH] [--repo-dir PATH] [--skip-self=true|false] [--branch-map host=branch ...] [--dry-run] [--force]`
    - Run `clean --force` and then `pull` for the same target host set in one command.
  - `./dialtone_mod mods v1 sync [--host <name|all|local>] [--repo-dir PATH] [--mod NAME|PATH ...] [--skip-self=true|false]`
    - Sync selected submodules for selected targets.
  - `./dialtone_mod mods v1 rsync [--host <local|name|all>] [--all-repo] [--mod NAME|PATH ...] [--repo-dir PATH] [--skip-self=true|false] [--dry-run]`
    - Performs true `rsync` to selected hosts.
    - Uses an auto-generated `--exclude-from` file from `git` ignore rules (`.gitignore`, `.git/info/exclude`, global+XDG excludes where configured).
    - Keeps `.git` content out and preserves ignored-file filtering for each sync path.
    - Respects nested mod `.gitignore` when syncing submodules.
  - `./dialtone_mod mods v1 new <mod-name> [--repo ...] [--path src/mods/<name>] [--branch main] [--public|--private]`
    - Create a new mod workspace and submodule pointer.
  - `./dialtone_mod mods v1 add --mod <mod-name> <paths...>`
    - Stage paths directly in a mod repo (or parent if omitted).
  - `./dialtone_mod mods v1 sync-ui [--mod ...] [--from PATH] [--dry-run] [--commit] [--push]`
    - Interactive helper for local sync with optional commit/push.
  - `./dialtone_mod mods v1 clone [--host <name|all|local>] [--from wsl] [--source PATH] [--dest PATH] [--branch BRANCH] [--branch-map host=branch] [--skip-self=true|false] [--dry-run]`
    - Clone/sync the dialtone repo across targets and then update mod submodules.
  - `./dialtone_mod mods v1 gh-create <mod-name> --owner <owner> [--repo-name <name>] [--private|--public]`
    - Create GitHub repos for a mod when needed.

## Typical `mods v1` patterns

- Make all host changes visible first:
  - `./dialtone_mod mods v1 pull --host all --dry-run`
  - `./dialtone_mod mods v1 pull --host all`
- Sync just a specific mod:
  - `./dialtone_mod mods v1 sync --host gold --mod mosh`
  - `./dialtone_mod mods v1 rsync --host gold --mod mosh`
- Recover a dirty remote and unblock pull/sync:
  - `./dialtone_mod mods v1 reset --host grey --force`
  - `./dialtone_mod mods v1 clean --host grey --force`
  - `./dialtone_mod mods v1 clean --host grey --dry-run --force`
- Gold sync uses the same command form:
  - `./dialtone_mod mods v1 rsync gold --mod mesh`
- Sync entire repo (all files, including submodules checked out under mod paths):
  - `./dialtone_mod mods v1 rsync gold --all-repo`
- Push parent + child modules in separate steps:
  - `./dialtone_mod mods v1 commit --all --message "Update mod tooling"`
  - `./dialtone_mod mods v1 push --message "Update mod tooling"`

## Nix + remote execution note

- When `runSSH` is used internally, remote commands are now executed by running `ssh v1` through `dialtone_mod` when available, so remote execution matches local CLI behavior.
- If `dialtone_mod` is not present on a host, fallback is `go run ./src/cli.go`, which bypasses `dialtone_mod` and may miss the pinned Nix shell behavior.

## Mesh and Git Safety

- Mesh sync commands use fast-forward only by default and fail on risky local conflicts.
- Mesh transport and auth logic is implemented in the mod implementation, not in the orchestrator.

## Note on Other Mods

Example mod entrypoints currently available:
- `./dialtone_mod mesh v1 <command>`
- `./dialtone_mod mosh v1 <command>`
- `./dialtone_mod tsnet v1 <command>`
- `./dialtone_mod mods v1 <command>`
