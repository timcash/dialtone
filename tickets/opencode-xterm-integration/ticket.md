# Branch: opencode-xterm-integration
# Tags: <tags>

# Goal
<goal>

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start opencode-xterm-integration`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket opencode-xterm-integration`
- status: done

## SUBTASK: Implement NATS Bridge for Opencode
- name: implement-nats-bridge-for-opencode
- description: Modify the AI plugin in `ai.go` to bridge opencode stdin/stdout to NATS subjects `ai.opencode.input` and `ai.opencode.output`.
- test-description: Run the opencode bridge and verify it publishes to NATS when output is produced.
- test-command: `dialtone.sh test ticket opencode-xterm-integration`
- status: done

## SUBTASK: Connect Web UI Terminal to Opencode NATS Subjects
- name: connect-web-ui-to-opencode
- description: Update `src/core/web/src/main.ts` to subscribe to `ai.opencode.output` and publish `cmd-input` to `ai.opencode.input`.
- test-description: Verify in the browser (or via mock) that terminal output reflects NATS messages.
- test-command: `dialtone.sh test ticket opencode-xterm-integration`
- status: done

## SUBTASK: Implement Browser Console Capture in Diagnostic CLI
- name: implement-browser-diagnostics
- description: Add chromedp logic to the diagnostic CLI to capture and display browser console logs via the debug port.
- test-description: Run `dialtone diagnostic --remote` and verify console logs from the robot's dashboard are visible.
- test-command: `dialtone.sh diagnostic --remote`
- status: done

## SUBTASK: Fix Opencode Shim in Terminal Bridge
- name: fix-opencode-shim
- description: Create a persistent shim script on the robot that redirects opencode to `dialtone ai chat`.
- test-description: Verify `opencode` command works in the web terminal and returns a real AI response.
- test-command: `dialtone.sh diagnostic`
- status: done

## SUBTASK: Embed .env (Go Embed Workaround) and Improve Deployment Verification
- name: embed-env-and-verify-deploy
- description: Resolve Go `embed` directory constraints by copying `.env` into `src/core/config` during build. Update the `deploy` command to verify remote startup via logs.
- test-description: Run `dialtone.sh deploy` and see remote logs; verify app works without local .env on robot.
- test-command: `dialtone.sh deploy`
- status: done

## SUBTASK: Final End-to-End Verification
- name: final-verification
- description: Perform a full walkthrough of the dashboard to ensure the terminal bridge and opencode UI are fully functional with the new API key.
- test-description: Screenshot and video proof of working terminal.
- test-command: `dialtone.sh diagnostic`
- status: todo

## SUBTASK: Local UI Development Verification
- name: local-ui-dev
- description: Run `dialtone.sh ui dev` to verify the frontend components locally. Catch: No real backend (NATS/Mavlink) unless proxied or mocked.
- test-description: Frontend starts and is accessible. Terminal component loads.
- test-command: `dialtone.sh ui dev` (manual verification)
- status: done

## SUBTASK: Fix Local NATS Connection for Mock Data
- name: fix-local-nats
- description: Debug and fix the NATS WebSocket connection between the local UI (Vite) and the mock data server. Ensure `ui mock-data` correctly exposes the WS port and the UI can connect.
- test-description: `dialtone.sh ui dev` + `dialtone.sh ui mock-data` results in a green "ONLINE" status in the browser.
- test-command: Manual verification via browser.
- status: done

## SUBTASK: Implement Port Availability Check in Test
- name: implement-port-check
- description: Update the `opencode.go` test to ensure required ports (4222, 4223, 8080) are available/free before starting mock services.
- test-description: Run `dialtone.sh test ticket opencode-xterm-integration` and verify it checks ports.
- test-command: `dialtone.sh test ticket opencode-xterm-integration`
- status: todo

## SUBTASK: Create Opencode Integration Test
- name: create-opencode-integration-test
- description: Create a new test in `src/plugins/ui/test/opencode.go` that orchestrates `ui mock-data`, `ai` plugin, and `ui dev` to verify the full flow.
- test-description: Run `dialtone.sh test tags opencode` and verify it passes.
- test-command: `dialtone.sh test tags opencode`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done opencode-xterm-integration`
- status: todo

