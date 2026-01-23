# Branch: wsl-workflow-check
# Task: Verify and improve the Dialtone workflow on Windows WSL

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh github pull-request` to create a draft pull request

## Goals
1. Verify `./dialtone.sh install --linux-wsl` correctly installs all dependencies.
2. Ensure `./dialtone.sh build --full` correctly rebuilds Web UI assets in the WSL environment.
3. Confirm `./dialtone.sh deploy` successfully reaches and updates the remote robot from WSL.
4. Validate `./dialtone.sh diagnostic` reliably discovers and uses Chrome/Chromium (Linux-side or Windows-side via `chromedp`).
5. Ensure `./dialtone.sh logs --remote` streams robot logs without issues.

## Context
Recent updates to the macOS ARM workflow (installation, build pipelines, and remote diagnostics) have been finalized. We must now ensure parity for Windows users using WSL. WSL presents unique challenges, particularly around browser discovery for headless automation (`chromedp`) and networking over Tailscale.

## Test
1. **Ticket Tests**: Run tests verifying WSL-specific logic (e.g., path resolution for Windows Chrome).
   ```bash
   ./dialtone.sh ticket test wsl-workflow-check
   ```
2. **Feature Tests**: Run system-wide tests to ensure no regressions in common CLI logic.
   ```bash
   ./dialtone.sh test wsl
   ```

## Logging
- Use `dialtone.LogInfo` to track environment discovery steps (e.g., "Found Windows Chrome at...").
- Log any network timeouts or permission issues specific to the WSL/NTFS interop.

## #SUBTASK: Environment Setup
- description: [VERIFY] `./dialtone.sh install --linux-wsl` installs required tools and sets up the local environment.
- test: Create `TestWSLDependencies` in `tickets/wsl-workflow-check/test/unit_test.go` to check for binary existence (`go`, `node`, `gh`).
- status: done

## #SUBTASK: Build Workflow
- description: [VERIFY] `./dialtone.sh build --full` rebuilds web assets and compiles the CLI.
- test: Run build and verify `dist/` folder timestamp or content changes.
- status: done

## #SUBTASK: Browser Discovery
- description: [VERIFY] `./dialtone.sh diagnostic` or `dialtone chrome` correctly identifies a Chrome path on WSL.
- test: Successful execution of a headless check in `src/plugins/chrome/test/integration_test.go` on a WSL runner.
- status: done

## #SUBTASK: Remote Operations
- description: [VERIFY] `deploy` and `logs --remote` work over the Tailscale network from WSL.
- test: Manual verification or E2E test if a simulator is available.
- status: done

## #SUBTASK: Remove sudo
- description: [REFAC] Modify `install.go` to download compilers and podman binaries directly to `DIALTONE_ENV` instead of using `sudo apt-get`.
- test: Run `dialtone install --linux-wsl` without sudo privileges and verify success.
- status: done

## Collaborative Notes
- WSL networking: Ensure Tailscale is running on the host or inside WSL depending on the user's setup.
- Chrome Discovery: The CLI should prioritize Linux-native Chromium but fallback to the Windows host `chrome.exe` if available via `/mnt/c/`.
- File Permissions: Watch out for NTFS mount points (`/mnt/c/...`) which can cause issues with Go or Node builds if not handled correctly.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
