# Branch: three-column-3d-ui
# Tags: web-ui, threejs, xterm, globe.gl

# Goal
Upgrade the `src/web` UI to a three-column layout with 3D representation in the center, xterm on the left, and camera/telemetry on the right.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start three-column-3d-ui`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh ticket test three-column-3d-ui`
- status: todo

## SUBTASK: setup dev server and verify local workflow
- name: setup-dev-server
- description: Show ability to start and stop the dev server locally using `npm run dev` in `src/web`.
- test-description: verify dev server is running by checking the output or CURLing the local port.
- test-command: curl -s http://localhost:5173 | grep "Dialtone"
- status: todo

## SUBTASK: refactor layout to three columns
- name: refactor-layout
- description: implementation of LEFT (xterm), CENTER (3D), RIGHT (camera/telemetry) layout in `src/web/index.html` and `src/web/src/style.css`.
- test-description: visual check of the layout structure.
- test-command: echo "Layout refactored"
- status: todo

## SUBTASK: integrate xterm in left column
- name: integrate-xterm
- description: Add xterm terminal to the left column for sending commands and receiving logs.
- test-description: verify xterm is initialized and visible in the left column.
- test-command: echo "Xterm integrated"
- status: todo

## SUBTASK: integrate 3D view in center column
- name: integrate-3d-view
- description: Add a 3D globe (globe.gl) or a 3D robot representation (three.js) to the center column.
- test-description: verify 3D canvas is rendering in the center column.
- test-command: echo "3D view integrated"
- status: todo

## SUBTASK: integrate camera and telemetry in right column
- name: integrate-camera-telemetry
- description: Add camera feed and telemetry data (NATS, MAVLINK) to the right column.
- test-description: verify camera container and telemetry tables are visible in the right column.
- test-command: echo "Camera and telemetry integrated"
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review.
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done three-column-3d-ui`
- status: todo
