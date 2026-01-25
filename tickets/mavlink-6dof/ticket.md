# Branch: mavlink-6dof
# Tags: mavlink, 6dof, telemetry, gps, orientation, ui

# Goal
The goal of this ticket is to enable MAVLink 6DOF telemetry data (GPS position and orientation) to be extracted from MAVLink messages on the robot and displayed in the Web UI. Additionally, the `test plugin ui` tests must be fixed to work locally with the dev server and mock data.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start mavlink-6dof`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket mavlink-6dof`
- status: done

## SUBTASK: extract mavlink telemetry
- name: mavlink-telemetry-extract
- description: Extract GPS position and orientation data from MAVLink messages in the backend.
- test-description: Verify that MAVLink messages are correctly parsed and data is extracted.
- test-command: `./dialtone.sh test ticket mavlink-6dof --subtask mavlink-telemetry-extract`
- status: done

## SUBTASK: transmit telemetry to UI
- name: mavlink-telemetry-transport
- description: Send the extracted GPS and orientation data to the Web UI via the communication layer.
- test-description: Verify that the UI receives the telemetry data updates.
- test-command: `./dialtone.sh test ticket mavlink-6dof --subtask mavlink-telemetry-transport`
- status: done

## SUBTASK: display telemetry in UI
- name: ui-telemetry-display
- description: Update the Web UI to display the GPS position and orientation in real-time.
- test-description: Verify that the UI displays the correct coordinates and orientation values.
- test-command: `./dialtone.sh test ticket mavlink-6dof --subtask ui-telemetry-display`
- status: done

## SUBTASK: fix ui plugin tests
- name: fix-ui-plugin-tests
- description: Fix the `test plugin ui` tests to work locally with the dev server and `mock-data`.
- test-description: Verify that UI plugin tests pass locally.
- test-command: `./dialtone.sh test plugin ui`
- status: done

## SUBTASK: improve chromedp reliability
- name: chromedp-reliability
- description: Ensure chromedp tests work reliably by handling port conflicts and service readiness.
- test-description: Run the UI plugin tests multiple times to ensure stability.
- test-command: `./dialtone.sh test plugin ui`
- status: done

## SUBTASK: remote deployment and verification
- name: remote-verify
- description: Deploy the updated codebase to the remote drone and verify telemetry in the Web UI.
- test-description: Verify telemetry (GPS, Orientation) on the live drone dashboard.
- test-command: `./dialtone.sh deploy && ./dialtone.sh logs --remote`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done mavlink-6dof`
- status: done


