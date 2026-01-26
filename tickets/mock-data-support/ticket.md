# Branch: mock-data-support
# Tags: mock, telemetry, development

# Goal
Add a `--mock` flag to `dialtone start` that provides fake telemetry and camera data for local development without hardware.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `./dialtone.sh ticket start mock-data-support`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `./dialtone.sh test ticket mock-data-support`
- status: done

## SUBTASK: add mock flag to dialtone start
- name: add-mock-flag
- description: Add a `--mock` flag to the `start` command in `src/dialtone.go` and propagate it to relevant functions.
- test-description: Run `dialtone start --mock --help` and verify the flag exists in the help output (if help reflects it) or just verify it parses without error.
- test-command: `./dialtone.sh build && ./dialtone --help`
- status: done

## SUBTASK: implement mock telemetry publisher
- name: implement-mock-telemetry
- description: Implement a mock telemetry publisher that sends fake heartbeat, attitude, and position data to the NATS `mavlinkPubChan` when the mock flag is active.
- test-description: Run dialtone in mock mode and verify NATS messages are being published.
- test-command: `./dialtone.sh test ticket mock-data-support --subtask implement-mock-telemetry`
- status: done

## SUBTASK: implement mock camera stream
- name: implement-mock-camera
- description: Implement a mock MJPEG stream handler that serves a moving color gradient, and use it in the web dashboard when mock mode is active.
- test-description: Run dialtone in mock mode and verify the `/stream` endpoint returns an MJPEG stream.
- test-command: `./dialtone.sh test ticket mock-data-support --subtask implement-mock-camera`
- status: done

## SUBTASK: demonstrate mock data in browser dashboard
- name: demonstrate-mock-data
- description: Run dialtone in mock mode using `./dialtone.sh start --local-only --mock`. Once the server is operational, use the `browser_subagent` to navigate to `http://localhost:80`. Verify that the dashboard UI accurately reflects the mock telemetry (heartbeat, attitude, and global position). Ensure the xterm.js console component at the bottom of the dashboard is receiving and displaying the NATS message stream in real-time. Document the verification with screenshots.
- test-description: The dashboard map should show a moving pin, and the attitude indicators should reflect changing values. The integrated terminal must show a scrolling stream of mock MAVLink JSON messages.
- test-command: `./dialtone.sh start --local-only --mock`
- status: done

## SUBTASK: implement chromedp browser verification test
- name: implement-chromedp-test
- description: Create an integration test in `src/core/test` that starts dialtone in mock mode uses `chromedp` in debug mode to verify that the dashboard loads, telemetry is flowing, and the terminal is receiving messages. Ensure it works via `./dialtone.sh test tags core mock`.
- test-description: Run the test and verify it passes with browser logs visible.
- test-command: `./dialtone.sh test tags core mock`
- status: done

## SUBTASK: implement codename generator
- name: implement-codename-generator
- description: Create a `src/core/util/codename.go` utility that generates random military-style code names (e.g. "falcon-eagle") from a predefined dictionary of words. This will be used to generate unique ephemeral hostnames for testing to avoid MagicDNS collisions.
- test-description: Create a simple test in `src/core/util/codename_test.go` that verifies the generator returns non-empty strings and that multiple calls return different values (probabilistic).
- test-command: `go test ./src/core/util/...`
- status: done

## SUBTASK: add ephemeral flag to vpn command
- name: add-ephemeral-flag
- description: Add an `--ephemeral` boolean flag to the `vpn` command in `src/dialtone.go`. Propagate this flag to the `tsnet.Server` configuration. This ensures that test nodes are cleaned up by Tailscale when they disconnect.
- test-description: Run `dialtone vpn --help` and verify the flag is present.
- test-command: `./dialtone.sh build && ./dialtone vpn --help`
- status: done

## SUBTASK: refactor vpn provisioning logic
- name: refactor-vpn-provisioning
- description: Refactor `src/plugins/vpn/cli/vpn.go` to expose a public `ProvisionKey(token string) (string, error)` function that returns the generated auth key string instead of just writing it to the `.env` file. This allows the integration test to programmatically generate fresh keys.
- test-description: Verify that the existing `dialtone vpn-provision` command still works as expected.
- test-command: `./dialtone.sh build`
- status: done

## SUBTASK: improve vpn verification test
- name: improve-vpn-verification-test
- description: Update `src/core/test/tsnet_verification.go` to: 1. Check for `TS_API_KEY` and provision a fresh ephemeral auth key if available. 2. Generate a random hostname using the codename generator. 3. Run `dialtone vpn` with `--ephemeral` and the fresh key. 4. Verify connectivity via IP (primary) and Domain (secondary).
- test-description: Run the improved test and verify it passes reliably without interference from stale DNS entries.
- test-command: `./dialtone.sh test tags tailscale`
- status: done

## SUBTASK: integrate vpn info into main dashboard
- name: integrate-vpn-info
- description: Remove `src/core/test/index.html` and the corresponding embedding logic. Instead, update the main web dashboard (in `src/core/web` or embedded assets) to include a `/vpn` route that displays the Tailscale/Tsnet configuration and status (hostname, IPs, connection state) using the same styling as the main dashboard. The `vpn` subcommand should serve this unified dashboard.
- test-description: Run `dialtone vpn --ephemeral --hostname test-dashboard` and verify `http://localhost:80/vpn` loads the styled status page.
- test-command: `./dialtone.sh build && ./dialtone vpn --local-only`
- status: done

- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review.
- test-description: validates all ticket subtasks are done
- test-command: `./dialtone.sh ticket done mock-data-support`
- status: done

