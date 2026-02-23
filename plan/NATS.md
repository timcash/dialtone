# NATS Subject Architecture

This document describes the NATS subject hierarchy used across the Dialtone codebase for logging, telemetry, testing, and remote control.

## 1. Unified Logging (`logs.>`)

All logs in the system are ideally published to NATS to allow for real-time observation from any node.

*   **`logs.<topic>`**: General purpose logging.
*   **`logs.test.<suite-version>.<step-name>`**: Logs specific to a single test step.
*   **`logs.test.<suite-version>.browser`**: Aggregated console and error logs from browsers controlled by tests.
*   **`logs.test.<suite-version>.error`**: A stream of all error-level events for a given test suite.
*   **`logs.test.<suite-version>.status.pass`**: Signal when a test step completes successfully.
*   **`logs.test.<suite-version>.status.fail`**: Signal when a test step fails.

## 2. REPL Control Plane (`repl.<room>`)

The `repl` plugin uses a single subject per "room" to coordinate multi-client interactive sessions.

*   **`input`**: User command frames sent from a client to the room's server.
*   **`line`**: Output line frames sent from the server to all connected clients.
*   **`probe`**: Discovery frames used by clients to detect an active server.
*   **`server`**: Presence and metadata announcements from the server.
*   **`heartbeat`**: Periodic "alive" signals from the server.
*   **`join` / `left`**: Notifications when clients enter or exit the room.

## 3. Robot & Telemetry (`mavlink.>`, `rover.>`)

Used by the `robot` plugin for high-frequency hardware communication and telemetry.

*   **`mavlink.stats`**: Periodic statistics about the MAVLink connection and hardware health.
*   **`mavlink.attitude`**: Real-time orientation data (roll, pitch, yaw).
*   **`mavlink.>`**: General catch-all for various MAVLink message types.
*   **`rover.command`**: Control frames sent to the robot's onboard controller.

## 4. Discovery & Coordination

*   **`test.subject`**: Used by deployment and sanity check scripts to verify NATS connectivity.
*   **`ui.>`**: Used by mock backends to simulate UI-driven state changes or telemetry.

---

## Technical Details

### Embedded NATS
Dialtone often starts an embedded NATS server on port `4222` when running in dev or test mode (see `src/plugins/logs/src_v1/go/nats.go`).

### Connection Defaults
*   **URL**: `nats://127.0.0.1:4222`
*   **Timeout**: Usually `1200ms` - `1500ms` for initial connection.
