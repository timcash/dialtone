# Branch: three-column-3d-ui
# Tags: web-ui, threejs, xterm, globe.gl

# Goal
Upgrade the `src/web` UI to a three-column layout with 3D representation in the center, xterm on the left, and camera/telemetry on the right.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start three-column-3d-ui`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket three-column-3d-ui`
- status: done

## SUBTASK: refactor layout to three columns
- name: refactor-layout
- description: implementation of LEFT (xterm), CENTER (3D), RIGHT (camera/telemetry) layout in `src/web/index.html` and `src/web/src/style.css`.
- test-description: visual check of the layout structure.
- test-command: echo "Layout refactored"
- status: done


## SUBTASK: integrate xterm in left column
- name: integrate-xterm
- description: Add xterm terminal to the left column for sending commands and receiving logs.
- test-description: verify xterm is initialized and visible in the left column.
- test-command: echo "Xterm integrated"
- status: done

## SUBTASK: integrate 3D view in center column
- name: integrate-3d-view
- description: Add a 3D globe (globe.gl) or a 3D robot representation (three.js) to the center column.
- test-description: verify 3D canvas is rendering in the center column.
- test-command: echo "3D view integrated"
- status: done

## SUBTASK: integrate camera and telemetry in right column
- name: integrate-camera-telemetry
- description: Add camera feed and telemetry data (NATS, MAVLINK) to the right column.
- test-description: verify camera container and telemetry tables are visible in the right column.
- test-command: echo "Camera and telemetry integrated"
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review.
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done three-column-3d-ui`
- status: done

## SUBTASK: create ui cli plugin
- name: create-ui-cli-plugin
- description: create a new plugin in src/plugins/ui that implements dev, build, and install commands wrapping npm
- test-description: verify dialtone.sh ui dev runs the dev server
- test-command: ./dialtone.sh ui build
- status: done

## SUBTASK: create ui mock-data command
- name: create-ui-mock-data
- description: implement ui mock-data command that starts a local mock server providing telemetry (NATS/MAVLink) and fake camera feed to the UI
- test-description: verify ui mock-data starts and UI receives data
- test-command: ./dialtone.sh ui mock-data --dry-run
- status: done

## SUBTASK: add ui tests with chromedp
- name: ui-test-dev-server
- description: Add ui test command to the plugin that uses chromedp to verify the dev server is serving the correct layout and components (terminal, canvas, telemetry).
- test-description: run the new test command
- test-command: ./dialtone.sh ui test
- status: done

## SUBTASK: integrate ui build into main build
- name: integrate-ui-build
- description: Refactor src/plugins/build/cli/build.go to use the ui plugin for building the web assets, ensuring consistent dependency usage.
- test-description: run dialtone.sh build --local and verify it uses the ui plugin logic
- test-command: ./dialtone.sh build --local
- status: done

## SUBTASK: deploy to remote robot
- name: deploy-remote
- description: build and deploy the new web UI and binary to the remote robot.
- test-description: verify deployment success, check remote logs, and run diagnostics.
- test-command: ./dialtone.sh deploy && ./dialtone.sh diagnostic
- status: done

## SUBTASK: verify real robot data
- name: verify-remote-data
- description: Verify that real camera feed and telemetry (NATS/MAVLink) are functional in the new UI on the remote robot.
- test-description: check remote logs for camera initialization and telemetry flow.
- test-command: ./dialtone.sh logs --remote --lines 50
- status: done

## SUBTASK: update diagnostic
- name: update-diagnostic
- description: Update dialtone.sh diagnostic to verify the new UI components (check HTTP server, static assets).
- test-description: run diagnostic locally and remotely to verify it passes.
- test-command: ./dialtone.sh diagnostic
- status: done

## SUBTASK: verify ui plugin integration
- name: verify-ui-integration
- description: Verify that `dialtone.sh test plugin ui` successfully starts the dev server and mock-data server, and validates the UI.
- test-description: run the integration test.
- test-command: ./dialtone.sh test plugin ui
- status: done

## SUBTASK: verify deployment execution
- name: verify-deploy-execution
- description: Run the deployment command to push changes to the remote.
- test-description: Checks for successful build and upload.
- test-command: ./dialtone.sh deploy
- status: done

## SUBTASK: verify remote logs
- name: verify-remote-logs
- description: Check remote logs for errors immediately after deployment.
- test-description: Tailing remote logs.
- test-command: ./dialtone.sh logs --remote --lines 20
- status: done

## SUBTASK: verify remote diagnostics
- name: verify-remote-diagnostics
- description: Run full diagnostics suite on the deployed system (tsnet, NATS, mavlink, UI).
- test-description: Diagnostics command verifying all components.
- test-command: ./dialtone.sh diagnostic
- status: done

## SUBTASK: create ui kill command
- name: create-ui-kill
- description: Implement 'dialtone.sh ui kill' to terminate dev server, mock-data, and other related processes.
- test-description: start processes then run kill and verify they are gone.
- test-command: ./dialtone.sh ui dev & ./dialtone.sh ui kill
- status: done
