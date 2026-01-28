# Branch: three-column-3d-ui
# Tags: web-ui, threejs, xterm, globe.gl

# Goal
Upgrade the `src/web` UI to a three-column layout with 3D representation in the center, xterm on the left, and camera/telemetry on the right.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start three-column-3d-ui`
- test-condition-1: run `./dialtone.sh plugin test <plugin-name>` to verify the ticket is valid
- test-condition-2: `./dialtone.sh plugin test <plugin-name>`
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: refactor layout to three columns
- name: refactor-layout
- description: implementation of LEFT (xterm), CENTER (3D), RIGHT (camera/telemetry) layout in `src/web/index.html` and `src/web/src/style.css`.
- test-condition-1: visual check of the layout structure.
- test-condition-2: echo "Layout refactored"
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done


## SUBTASK: integrate xterm in left column
- name: integrate-xterm
- description: Add xterm terminal to the left column for sending commands and receiving logs.
- test-condition-1: verify xterm is initialized and visible in the left column.
- test-condition-2: echo "Xterm integrated"
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: integrate 3D view in center column
- name: integrate-3d-view
- description: Add a 3D globe (globe.gl) or a 3D robot representation (three.js) to the center column.
- test-condition-1: verify 3D canvas is rendering in the center column.
- test-condition-2: echo "3D view integrated"
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: integrate camera and telemetry in right column
- name: integrate-camera-telemetry
- description: Add camera feed and telemetry data (NATS, MAVLINK) to the right column.
- test-condition-1: verify camera container and telemetry tables are visible in the right column.
- test-condition-2: echo "Camera and telemetry integrated"
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review.
- test-condition-1: validates all ticket subtasks are done
- test-condition-2: `dialtone.sh ticket done three-column-3d-ui`
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: create ui cli plugin
- name: create-ui-cli-plugin
- description: create a new plugin in src/plugins/ui that implements dev, build, and install commands wrapping npm
- test-condition-1: verify dialtone.sh ui dev runs the dev server
- test-condition-2: ./dialtone.sh ui build
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: create ui mock-data command
- name: create-ui-mock-data
- description: implement ui mock-data command that starts a local mock server providing telemetry (NATS/MAVLink) and fake camera feed to the UI
- test-condition-1: verify ui mock-data starts and UI receives data
- test-condition-2: ./dialtone.sh ui mock-data --dry-run
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: add ui tests with chromedp
- name: ui-test-dev-server
- description: Add ui test command to the plugin that uses chromedp to verify the dev server is serving the correct layout and components (terminal, canvas, telemetry).
- test-condition-1: run the new test command
- test-condition-2: ./dialtone.sh ui test
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: integrate ui build into main build
- name: integrate-ui-build
- description: Refactor src/plugins/build/cli/build.go to use the ui plugin for building the web assets, ensuring consistent dependency usage.
- test-condition-1: run dialtone.sh build --local and verify it uses the ui plugin logic
- test-condition-2: ./dialtone.sh build --local
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: deploy to remote robot
- name: deploy-remote
- description: build and deploy the new web UI and binary to the remote robot.
- test-condition-1: verify deployment success, check remote logs, and run diagnostics.
- test-condition-2: ./dialtone.sh deploy && ./dialtone.sh diagnostic
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: verify real robot data
- name: verify-remote-data
- description: Verify that real camera feed and telemetry (NATS/MAVLink) are functional in the new UI on the remote robot.
- test-condition-1: check remote logs for camera initialization and telemetry flow.
- test-condition-2: ./dialtone.sh logs --remote --lines 50
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: update diagnostic
- name: update-diagnostic
- description: Update dialtone.sh diagnostic to verify the new UI components (check HTTP server, static assets).
- test-condition-1: run diagnostic locally and remotely to verify it passes.
- test-condition-2: ./dialtone.sh diagnostic
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: verify ui plugin integration
- name: verify-ui-integration
- description: Verify that `./dialtone.sh plugin test ui` successfully starts the dev server and mock-data server, and validates the UI.
- test-condition-1: run the integration test.
- test-condition-2: ./dialtone.sh plugin test ui
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: verify deployment execution
- name: verify-deploy-execution
- description: Run the deployment command to push changes to the remote.
- test-condition-1: Checks for successful build and upload.
- test-condition-2: ./dialtone.sh deploy
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: verify remote logs
- name: verify-remote-logs
- description: Check remote logs for errors immediately after deployment.
- test-condition-1: Tailing remote logs.
- test-condition-2: ./dialtone.sh logs --remote --lines 20
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: verify remote diagnostics
- name: verify-remote-diagnostics
- description: Run full diagnostics suite on the deployed system (tsnet, NATS, mavlink, UI).
- test-condition-1: Diagnostics command verifying all components.
- test-condition-2: ./dialtone.sh diagnostic
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done

## SUBTASK: create ui kill command
- name: create-ui-kill
- description: Implement 'dialtone.sh ui kill' to terminate dev server, mock-data, and other related processes.
- test-condition-1: start processes then run kill and verify they are gone.
- test-condition-2: ./dialtone.sh ui dev & ./dialtone.sh ui kill
- tags: 
- dependencies: 
- agent-notes: 
- pass-timestamp: 
- fail-timestamp: 
- status: done
