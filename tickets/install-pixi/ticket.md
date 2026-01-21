# Branch: install-pixi
# Task: Add Pixi installation to the dialtone install command

> IMPORTANT: Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh github pull-request` to create a draft pull request

## Goals
1. Update the `dialtone install` command in `src/install.go` to include Pixi.
2. Ensure Pixi is installed locally to the environment directory (specified by `GetDialtoneEnv()`).
3. Support Pixi installation on all major platforms (Linux/WSL, macOS ARM).
4. Verify the installation with Go integration tests.

## Non-Goals
1. DO NOT install Pixi globally using `sudo`.
2. DO NOT modify the user's shell configuration automatically (except for providing instructions at the end).

## Test
1. All ticket tests should be located at `tickets/install-pixi/test/`.
2. Tests should verify that `dialtone install` correctly downloads and places the Pixi binary.
3. Verify that `pixi --version` returns successfully from the installed location.

## Subtask: Research
- description: Identify the direct download URLs for Pixi binaries for Linux x86_64 and macOS ARM64. Standard script is `curl -fsSL https://pixi.sh/install.sh | sh`, but we might want to download the binary directly to the env dir.
- status: todo

## Subtask: Implementation
- description: [MODIFY] `src/install.go`: Add `installPixi()` function.
- description: [MODIFY] `src/install.go`: Call `installPixi()` inside `installLocalDepsWSL()`, `installLocalDepsMacOSARM()`, etc.
- description: [MODIFY] `src/install.go`: Update help usage text to mention Pixi.
- status: todo

## Subtask: Verification
- description: [NEW] `tickets/install-pixi/test/install_pixi_test.go`: Integration test that runs the install command and checks for the pixi binary.
- status: todo

## Collaborative Notes
- Pixi is a vital dependency for JAX and other Python-based plugins.
- We should follow the pattern used for Go/Node/Zig in `src/install.go`, which is to download a tarball/zip, extract it to the local env dir, and avoid system-wide changes.
- Pixi releases are available on GitHub: `https://github.com/prefix-dev/pixi/releases`.
- The `install.sh` from pixi.sh usually installs to `~/.pixi/bin`, but we want it in `~/.dialtone_env/pixi/bin` or similar to keep it grouped with other Dialtone tools.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
