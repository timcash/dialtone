# Testing Plan for Dialtone Guided Setup and REPL

This plan outlines the verification steps for the improved `dialtone.sh` guided setup and the new REPL mode.

## 1. Autonomous Environment Setup
### Non-Nix Path Verification
- [ ] Run `./dialtone.sh --no-nix` with an empty `DIALTONE_ENV` folder.
- [ ] Verify that Go 1.24.0 is automatically downloaded and extracted to the managed folder.
- [ ] Verify that `dialtone.sh` correctly updates `PATH` and `GOROOT` to use the managed Go.
- [ ] Verify that CGO is disabled by default (`CGO_ENABLED=0`).

### Custom Environment Flag (`--env`)
- [ ] Run `./dialtone.sh --env test_env/.env` and verify it uses the specified configuration.
- [ ] Ensure that a "sterile" run (restricting system `PATH`) still succeeds by installing its own dependencies.

## 2. REPL Mode (Auto-Trigger)
### Startup Flow
- [ ] Run `./dialtone.sh` (or with flags but no command).
- [ ] Verify it enters REPL mode automatically.
- [ ] Verify the following checks are performed and logged with `DIALTONE>` prefix:
    - [ ] `env/.env` presence.
    - [ ] `env/mesh.json` presence.
    - [ ] `env/ssh_config` presence.
    - [ ] Go/Bun installation status.

### Repo Bootstrapping (Git-less)
- [ ] Test the "download and expand" feature for users without Git.
- [ ] Verify the repository is fetched via tar/zip from GitHub and expanded correctly.

### SSH Plugin Verification
- [ ] Within the REPL, verify connectivity to `wsl` on `legion`.
- [ ] Ensure the REPL reports success/failure of this connection clearly.

## 3. REPL Interactive Mode
### Interface
- [ ] Verify the prompt is set to `host-name>`.
- [ ] Verify the `help` command lists available Dialtone commands.
- [ ] Verify that commands can be executed directly from the REPL.

## 4. Integration & Regression
- [ ] Verify that existing commands (e.g., `./dialtone.sh ssh src_v1 ...`) still work as expected.
- [ ] Ensure Nix dev shell still works when enabled and available.
