# Mods System (`src/mods`)

`mods` is the unified command surface for the Dialtone **Mesh-First** workflow. It abstracts standard Git operations into a mesh-aware API that prioritizes LAN speed and local reliability while maintaining GitHub as the final source of truth.

## Philosophy: Together as a Mesh

The `mods v1` workflow is designed for high-performance synchronization across a LAN SSH mesh. 
- **Mesh-First Sync**: Commands like `pull` and `sync` attempt to fetch code from other LAN nodes before falling back to GitHub.
- **Safety First**: All mesh-to-mesh pulls use `--ff-only` to protect local dirty changes.
- **Abstracted Git**: Complex Git/Submodule logic is wrapped into a simple, predictable CLI that preserves standard "Add -> Commit -> Push" habits.
- **Nix-Managed**: Every command runs inside a consistent, reproducible toolchain (Go, Git, SSH, GitHub CLI) managed by Nix.

## Core Commands

### Orchestration
- `./dialtone.sh mods v1 new <mod-name>`  
  Create a new mod, provision a GitHub repo, and register it as a submodule.
- `./dialtone.sh mods v1 pull [--host all]`  
  Broadcast a pull request to the mesh. Remote hosts will pull from **your current host** first, then fall back to GitHub.
- `./dialtone.sh mods v1 status [--short]`  
  Detailed project health report showing dirty/staged files for the parent repo and all mods.

### Standard Git Workflow (Wrapped)
- `./dialtone.sh mods v1 add [--mod <name>] <paths...>`  
  Stage files for commit. Defaults to the parent repo; use `--mod` for specific submodules.
- `./dialtone.sh mods v1 commit [--mod <name>] [-m "msg"]`  
  Commit staged changes. **Never auto-stages or auto-commits.**
- `./dialtone.sh mods v1 push [--mod <name>]`  
  Push committed changes to GitHub. (Pushing with no args iterates through all mods + parent).

## Standard Workflow Example

### 1. Create a Mod
```bash
./dialtone.sh mods v1 new my-feature --owner timcash --public
```

### 2. Make Changes & Commit
```bash
# Edit files...
./dialtone.sh mods v1 add --mod my-feature .
./dialtone.sh mods v1 commit --mod my-feature -m "implement core logic"
```

### 3. Share with the Mesh
```bash
# Push your changes to GitHub
./dialtone.sh mods v1 push --mod my-feature

# Coordinate all other mesh nodes to pull from you (LAN speed)
./dialtone.sh mods v1 pull --host all
```

## Safety & Deconfliction

The `mods` API is designed to be safe for both **Humans** and **LLM Agents**.

- **Dirty Change Protection**: `pull` and `sync` will fail if local changes would be overwritten.
- **FF-Only**: Mesh synchronization strictly uses Fast-Forward only. If branches have diverged, the command reports failure.
- **Agent Reasoning**: Because the API is predictable, an LLM agent can catch a failed pull, inspect the state with `mods v1 status`, and perform a standard `git merge` or manual edit to resolve conflicts before re-syncing.

## Environment Toolchain

Managed via `flake.nix`:
- **Source**: `git`, `gh`, `openssh`
- **Language**: `go`, `nodejs`
- **Build**: `gcc`, `cmake`, `ninja`, `pkg-config`

Run `./dialtone.sh` to automatically enter the Nix dev shell.

---
*Implementation Note: `src/mods/main.go` implements this logic using Go-native GitHub/SSH/Git flows.*
