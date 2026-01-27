# Branch: opencode-ui-integration
# Tags: p1, enhancement, web-ui, robot

# Goal
Integrate the OpenCode CLI into the robot's web UI using an xterm.js element to allow for remote command execution and monitoring.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start opencode-ui-integration`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket opencode-ui-integration`
- status: done

## SUBTASK: Research Robot Web UI
- name: research-web-ui
- description: Audit the current robot web UI codebase to identify where to integrate the xterm.js element.
- test-description: Document the target file and component in the collaborative notes.
- test-command: `ls src/web/index.html` (or relevant path)
- status: todo

## SUBTASK: Research OpenCode Web interface
- name: research-opencode-web
- description: Research the existing OpenCode web interface for patterns that can be reused in Dialtone.
- test-description: Capture findings in the collaborative notes.
- test-command: `ls tickets/opencode-ui-integration/opencode-research.md`
- status: todo

## SUBTASK: Integrate xterm.js with OpenCode stream
- name: stream-opencode-xterm
- description: Implement the logic to stream the OpenCode CLI output into an xterm.js element in the robot web UI.
- test-description: Verify that CLI output is visible in the web UI when OpenCode is running.
- test-command: `dialtone.sh test ticket opencode-ui-integration --subtask stream-opencode-xterm`
- status: todo

## SUBTASK: Verify Discord integration on robot
- name: verify-discord-robot
- description: Deploy the code to the robot and verify that Discord integration (via OpenCode) functions correctly.
- test-description: Verify message delivery to Discord from the robot environment.
- test-command: `./dialtone.sh deploy && dialtone.sh logs --remote | grep discord`
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done opencode-ui-integration`
- status: todo

## Collaborative Notes
- **Reference**: https://github.com/timcash/dialtone/issues/83
- Target Web UI Path: `src/web/`
- Tooling: `xterm.js`, `opencode cli`

