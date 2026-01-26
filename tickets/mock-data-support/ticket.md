# Branch: mock-data-support
# Tags: mock, telemetry, development

# Goal
Add a `--mock` flag to `dialtone start` that provides fake telemetry and camera data for local development without hardware.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `./dialtone.sh ticket start mock-data-support`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `./dialtone.sh test ticket mock-data-support`
- status: todo

## SUBTASK: add mock flag to dialtone start
- name: add-mock-flag
- description: Add a `--mock` flag to the `start` command in `src/dialtone.go` and propagate it to relevant functions.
- test-description: Run `dialtone start --mock --help` and verify the flag exists in the help output (if help reflects it) or just verify it parses without error.
- test-command: `./dialtone.sh build && ./dialtone --help`
- status: todo

## SUBTASK: implement mock telemetry publisher
- name: implement-mock-telemetry
- description: Implement a mock telemetry publisher that sends fake heartbeat, attitude, and position data to the NATS `mavlinkPubChan` when the mock flag is active.
- test-description: Run dialtone in mock mode and verify NATS messages are being published.
- test-command: `./dialtone.sh test ticket mock-data-support --subtask implement-mock-telemetry`
- status: todo

## SUBTASK: implement mock camera stream
- name: implement-mock-camera
- description: Implement a mock MJPEG stream handler that serves a moving color gradient, and use it in the web dashboard when mock mode is active.
- test-description: Run dialtone in mock mode and verify the `/stream` endpoint returns an MJPEG stream.
- test-command: `./dialtone.sh test ticket mock-data-support --subtask implement-mock-camera`
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review.
- test-description: validates all ticket subtasks are done
- test-command: `./dialtone.sh ticket done mock-data-support`
- status: todo

