# REPL & Onboarding Plan (v2)

## Overview
The goal is a "Zero-Config" autonomous setup where a user can download a single script (`dialtone.sh`), run it, and be guided through a complete environment setup (runtimes, repository, and configuration) without needing pre-installed tools like Git, Nix, or Go.

## Core Components

### 1. Guided Onboarding (`dialtone.sh`)
- **Interactive Setup**: Triggers automatically if `env/dialtone.json` is missing.
- **Dependency Management**:
    - **Go**: Automatically downloads and extracts Go 1.24.0 to a managed `DIALTONE_ENV` folder.
    - **Bun**: (TODO) Add automated installation.
- **Git-less Bootstrap**: Downloads the repository as a tarball from GitHub and expands it into the user-defined `DIALTONE_REPO_ROOT`.
- **Configuration**: Stores all project settings in `env/dialtone.json` (JSON format).

### 2. REPL v2 (`src/plugins/repl/src_v2`)
- **Environment Verification**: Performs a "pre-flight" check of `.env`, `mesh.json`, and `ssh_config`.
- **Connectivity Testing**: Automatically validates SSH connectivity to key nodes (e.g., `wsl` on `legion`) upon startup.
- **Command Routing**: Allows direct execution of any Dialtone command from the `host-name>` prompt.
- **Test Mode**: Support for `--test` flag to verify the entire onboarding flow non-interactively.

## Current Progress & TODOs

### Completed
- [x] Basic interactive onboarding flow in `dialtone.sh`.
- [x] Automated Go installation for Darwin/Linux (ARM64/AMD64).
- [x] GitHub tarball expansion for Git-less environments.
- [x] JSON-based configuration (`env/dialtone.json`).
- [x] REPL v2 scaffold and basic interactive loop.
- [x] Enforced `CGO_ENABLED=0` for cross-platform stability.

### TODO
- [ ] **Simplify `dialtone.sh`**: Fix the re-execution bug where JSON files are accidentally sourced as shell scripts.
- [ ] **Robust JSON Parsing**: Improve the `read_json_val` helper or ensure it handles edge cases (whitespace, nested objects).
- [ ] **Bun Support**: Implement `install_bun` in `dialtone.sh`.
- [ ] **WSL Onboarding**: Add specific checks/tips for Windows/WSL users during the guided setup.
- [ ] **Multiplayer Integration**: Bridge REPL v2 with the existing NATS-based multiplayer features of v1.

## Testing Strategy
- **Blank Slate Test**: Run `dialtone.sh` in a completely empty directory with a restricted `PATH` (no Go/Git).
- **Custom Env Test**: Use `--env` to point to different JSON configuration profiles.
- **Non-Interactive Test**: Use `--test` to ensure the onboarding and verification logic remains healthy during CI/CD or automated sessions.
