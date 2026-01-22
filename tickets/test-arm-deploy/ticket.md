# Branch: test-arm-deploy
# Task: Debug and test deploying to a remote robot (Raspberry Pi)

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh github pull-request` to create a draft pull request

## Goals
1. Use tests in `tickets/test-arm-deploy/test/` to drive all work.
2. Support `dialtone.sh deploy --remote` with auto-detection of arch/OS.
3. **Deployment Only**: Deploy the cross-compiled binary. Do NOT set up a full development environment (Go/Node) on the remote.
4. **Verification**: Ensure TSNet, NATS, Web UI, Camera, and MAVLink are verified upon startup.
5. **Logging**: Produce a clear sequence of logs verifying each component's status.
6. Support `dialtone.sh logs --remote` to view these logs.
7. Use `.env` variables for configuration (`ROBOT_HOST`, `ROBOT_USER`, `ROBOT_PASSWORD`).

## Non-Goals
1. DO NOT implement UI for camera streaming in this ticket.
2. DO NOT change default local deployment behavior.

## Test
1. **Ticket Tests**: Run tests specific to remote deployment logic and flag parsing.
   ```bash
   ./dialtone.sh ticket test test-arm-deploy
   ```
2. **Feature Tests**: Run remote diagnostic tests.
   ```bash
   ./dialtone.sh test diagnostic --remote
   ```
3. **All Tests**: Run the entire test suite.
   ```bash
   ./dialtone.sh test
   ```

## logging
1. Use `src/core/logger` for standardized logging.
2. **Startup Verification Logs**: The binary should emit logs similar to:
   ```text
   [INFO] Starting Dialtone on dialtone-robot-1...
   [INFO] TSNet: Connected (IP: 100.x.y.z)
   [INFO] NATS: Connected
   [INFO] Web UI: Serving at http://100.x.y.z:8080
   [INFO] Camera: Found /dev/video0 (640x480 @ 30fps)
   [INFO] MAVLink: Heartbeat received from flight controller
   [SUCCESS] System Operational
   ```

## Subtask: Research
- description: Analyze `src/deploy.go` and `src/plugins/install/cli/install.go` for existing remote SSH patterns.
- test: Summarize findings in Collaborative Notes.
- status: done

## Subtask: Implementation (Logs)
- description: [NEW] `src/plugins/logs/cli/logs.go`: Implement `dialtone logs --remote` to stream logs from the remote service (e.g., using `journalctl` or reading a log file via SSH).
- test: `dialtone logs --remote` shows live logs from the robot.
- status: done

## Subtask: Implementation (Deploy)
- description: [MODIFY] `src/deploy.go`: Implement architecture auto-detection. Upload only the binary and required assets (Web UI). Do not install Go/Node. Restart service.
- test: Deploy completes quickly by just moving the binary.
- status: done

## Subtask: Implementation (Diagnostic)
- description: [MODIFY] `src/diagnostic.go`: Integrate `github.com/chromedp/chromedp` to visit the robot's Web UI (via Tailscale IP) and verify the dashboard title/content.
- test: `dialtone diagnostic` reports success for Web UI verification.
- status: todo

## Subtask: Web UI Verification
- description: Verify Web UI is reachable over Tailscale network at `http://drone_1`.
- test: Browser successfully loads the Dialtone dashboard at the Tailscale URL.
- status: done

## Collaborative Notes
- Remote robots often use 32-bit ARM (`linux-arm`) for older Pi models (ZeroW) and 64-bit (`linux-arm64`) for Pi 4/5. 
- MAVLink endpoint should support both UDP and Serial paths.
- Default `REMOTE_DIR_SRC` should be `~/dialtone_src` and `REMOTE_DIR_DEPLOY` should be `~/dialtone_deploy`.
- Use `RunSSHCommand` and `uploadFile` from the existing `ssh_tools` (shared in `deploy.go` or moved to `core`).

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
