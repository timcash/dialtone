# Branch: mavlink-serial-test
# Task: Verify MAVLink serial connectivity and Web UI arming on remote robot

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Refer to [mavlink.md](file:///home/user/dialtone/docs/vendor/mavlink.md) for vendor-specific guidance.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh github pull-request` to create a draft pull request

## Goals
1. **Remote MAVLink Serial**: Test that MAVLink is running **exclusively** via serial UART on the remote robot (e.g., `/dev/ttyAMA0`).
2. **NATS Proxy Only**: The MAVLink service should only proxy commands to/from NATS. Do NOT start a MAVLink server or GCS proxy.
3. **Web UI Arming**: Test that the robot can be armed via the Web UI or see an error for why it won't arm.
4. **Verification**: Confirm MAVLink heartbeats are received over the serial interface.

## Non-Goals
1. DO NOT implement new flight controller logic.
2. DO NOT start any MAVLink servers (UDP/TCP listeners) for external GCS connection.
3. DO NOT change local serial port defaults.

## Test
1. **Ticket Tests**: Run tests specific to this ticket.
   ```bash
   ./dialtone.sh ticket test mavlink-serial-test
   ```
2. **Remote Diagnostics**: Run diagnostics on the remote robot to verify version and MAVLink status.
   ```bash
   ./dialtone.sh diagnostic --remote --host $ROBOT_HOST --pass $ROBOT_PASSWORD
   ```

## Logging
1. **Mavlink Startup**: Standardized logs should indicate serial port initialization.
   ```text
   [INFO] MavlinkService: Starting event loop on serial:/dev/ttyAMA0:57600
   [INFO] MAVLink channel open
   [INFO] MAVLink: Heartbeat received from flight controller
   ```
2. **Arming Logs**: Logs should show the result of the arming command.
   ```text
   [INFO] MavlinkService: Sending ARM command
   [INFO] COMMAND_ACK: MAV_CMD_COMPONENT_ARM_DISARM result: 0
   [SUCCESS] Arming successful
   ```

## Subtask: Research Serial Setup
- description: Identify the correct serial device on the target Raspberry Pi (usually `/dev/ttyAMA0` or `/dev/serial0`). Verify user permissions for the serial port.
- test: `ls -l /dev/ttyAMA0` on the remote Pi shows correct permissions.
- status: done

## Subtask: Remote MAVLink Verification
- description: [MODIFY] `src/mavlink.go`: Ensure the serial endpoint can be configured via environment variables or flags for the remote robot.
- test: `dialtone.sh logs --remote` shows MAVLink heartbeats.
- status: done

## Subtask: Web UI Arming Test
- description: Test the arming sequence through the Web UI. If arming fails, ensure the error from MAVLink (STATUSTEXT or COMMAND_ACK) is visible in the UI or logs.
- test: Clicking "Arm" in the Web UI results in a successful arming state or a clear error message.
- status: done

## Subtask: Code-Deploy-Verify Workflow
- description: Make a visible change (e.g., update a version string in `src/dialtone.go`), then run build and deploy. Run diagnostics to confirm the new version is active.
- test: `dialtone.sh diagnostic --remote` shows the updated version string.
- status: done

## Subtask: Migrate to Plugin
- description: Create a new plugin for MAVLink using the CLI and migrate the existing logic.
  1. Run `./dialtone.sh plugin create mavlink`.
  2. Migrate `src/mavlink.go` logic to `src/plugins/mavlink/app/` and `src/plugins/mavlink/cli/`.
  3. Refactor `src/dev.go` to delegate MAVLink-related commands/logic to the new plugin.
- test: `dialtone.sh mavlink` (or equivalent plugin command) works as expected and local tests in `src/plugins/mavlink/test/` pass.
- status: done

## Subtask: Update Documentation
- description: Update [mavlink.md](file:///home/user/dialtone/docs/vendor/mavlink.md) with any new learnings about serial ports, MAVLink heartbeats, or arming errors encountered during this ticket.
- test: Documentation is comprehensive and reflects the final implementation.
- status: done

## Collaborative Notes
- Raspberry Pi Zero/3/4/5 serial ports can vary. `/dev/ttyAMA0` is often the hardware UART, while `/dev/ttyS0` is the mini UART.
- Ensure `enable_uart=1` is set in `/boot/config.txt` on the Pi.
- If arming fails, check for "Pre-arm" checks in the MAVLink `STATUSTEXT` messages.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
