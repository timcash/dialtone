# Mods System (`src/mods`)

`mods` is the centralized command surface for the mod workflow:

- code lives in Git + Git submodules
- dev tooling is managed by Nix (dev path)
- host orchestration happens over mesh SSH
- GitHub repo creation/publishing is delegated to existing plugins

This folder is intentionally minimal and independent from `src/plugins` command routers.
It implements mod orchestration directly in `src/mods/main.go` with Go-native GitHub/SSH/Git flows where available.
One intentional CLI fallback remains: `git submodule add` (go-git lacks a robust equivalent).

## Command Shape

```bash
./dialtone.sh mods v1 <command> [args]
```

## Bootstrap Paths

Two bootstrap paths are supported by design.

### 1. Dev Path (full source + tooling)

Use this on laptops/workstations.

Flow:
1. Download `dialtone.sh` or `dialtone.ps1`
2. Run launcher (bootstrap REPL creates `env/.env`, installs Go)
3. Clone/bootstrap repo into local directory
4. Run:

```bash
./dialtone.sh mods bootstrap dev
```

This runs `./dialtone.sh dev install` and prepares managed runtimes.
After this, use `mods add/clone/sync` and then mod-level Nix flows (`install/build/test`).

### 2. Binary Path (lightweight runtime host)

Use this on edge hosts (for example Raspberry Pi) where you only need binaries/services.

Current status:
- reserved in `mods bootstrap binary`
- final binary-only installer/service flow should be implemented per app/service plugin
- no full repo clone required

## Core Commands

- `./dialtone.sh mods add <mod-name> [flags...]`
- `./dialtone.sh mods clone [flags...]`
- `./dialtone.sh mods list`
- `./dialtone.sh mods status`
- `./dialtone.sh mods sync`
- `./dialtone.sh mods sync-ui`
- `./dialtone.sh mods gh-create <mod-name> ...`
- `./dialtone.sh mods commit --mod <mod-name> [-m "..."]`
- `./dialtone.sh mods v1 push [--mod <mod-name>]`
- `./dialtone.sh mods v1 pull [--host all] [--from wsl]`

## Safety Rules

- `clone --host all` runs sequentially, not in parallel.
- `clone --host all` should default to skipping self (`--skip-self=true`) when requested.
- `push --host all` is blocked to prevent multi-host concurrent pushes.
- use single-writer policy for pushes (`--writer <host>` or explicit `--host <name>`).

## Mesh Demo (SSH + Git + GitHub + Nix Scaffolding)

### A. Create a new mod repo and add submodule

```bash
./dialtone.sh mods add mod-name --owner timcash --public
```

This uses native `mods add` behavior:
- GitHub repo naming convention: `dialtone-mod-name`
- parent mapping: `src/mods/mod-name`
- optional scaffold + UI seed

### B. Clone/sync repo across mesh hosts with per-host branch mapping

```bash
./dialtone.sh mods clone \
  --host all \
  --from wsl \
  --branch-map gold=main \
  --branch-map darkmac=feature-x \
  --skip-self=true
```

This is handled by native `mods clone` logic and supports hosts pulling different branches.

### C. Commit and push a mod safely

```bash
./dialtone.sh mods commit --mod mod-name -m "Update mod-name"
./dialtone.sh mods push --mod mod-name --host gold
```

### E. One-step push/pull for all mods

```bash
# Push all dirty mods, then commit/push parent submodule pointers
./dialtone.sh mods v1 push

# Clone/pull dialtone across hosts from a writer node, then sync submodules
./dialtone.sh mods v1 pull --host all --from wsl
```

### D. Nix for mod dev/build

After code is present, run mod commands under your Nix-managed dev flow:

```bash
./dialtone.sh mod-name v1 install
./dialtone.sh mod-name v1 build
./dialtone.sh mod-name v1 test
```

`mods` owns source orchestration; Nix owns reproducible dev/build dependencies.

## Implementation Notes

- Entrypoint: `src/mods/main.go`
- Router integration: `src/dev.go` (`./dialtone.sh mods ...`)
- `src/mods` does not import `src/plugins/*` command packages.
- Remaining CLI fallback: `git submodule add`/`update` during `mods add`.
