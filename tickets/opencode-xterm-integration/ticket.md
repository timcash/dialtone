# Branch: opencode-xterm-integration
# Tags: <tags>

# Goal
<goal>

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start opencode-xterm-integration`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket opencode-xterm-integration`
- status: todo

## SUBTASK: Implement NATS Bridge for Opencode
- name: implement-nats-bridge-for-opencode
- description: Modify the AI plugin in `ai.go` to bridge opencode stdin/stdout to NATS subjects `ai.opencode.input` and `ai.opencode.output`.
- test-description: Run the opencode bridge and verify it publishes to NATS when output is produced.
- test-command: `dialtone.sh test ticket opencode-xterm-integration`
- status: todo

## SUBTASK: Connect Web UI Terminal to Opencode NATS Subjects
- name: connect-web-ui-to-opencode
- description: Update `src/core/web/src/main.ts` to subscribe to `ai.opencode.output` and publish `cmd-input` to `ai.opencode.input`.
- test-description: Verify in the browser (or via mock) that terminal output reflects NATS messages.
- test-command: `dialtone.sh test ticket opencode-xterm-integration`
- status: todo

## SUBTASK: Deploy and Verify Discord Integration
- name: deploy-and-verify-discord
- description: Deploy the full build to the robot and verify that the Discord integration works as expected from the robot environment.
- test-description: Run diagnostics on the remote robot and check Discord logs.
- test-command: `dialtone.sh diagnostic --remote`
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done opencode-xterm-integration`
- status: todo

