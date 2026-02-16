# TODO

- **Three Section (6DOF Data)**: The Three section (`src/plugins/robot/src_v1/ui/src/components/three/index.ts`) has been updated to integrate live 6DOF (roll, pitch, yaw) orientation data from the robot via WebSocket, replacing the previous auto-spinning behavior with real-time attitude visualization.
- **Telemetry Table (Enhanced Data)**: The Telemetry Table (`src/plugins/robot/src_v1/ui/src/components/table/index.ts`) now displays comprehensive live telemetry, including GPS coordinates (lat, lon, alt, sats), battery voltage, system uptime, NATS message count, and connected clients. The backend (`src/core/web/server.go`) has been updated to broadcast this data over the WebSocket.
- **Controls Section**: A new 'Controls' section (`src/plugins/robot/src_v1/ui/src/components/controls/`) has been added, featuring Arm, Disarm, Manual, and Guided buttons. These buttons publish commands to the `rover.command` NATS subject, restoring core robot control functionality from the old UI.
- **Migrate Remaining Functionality**: Reference the old core web UI (`src/core/web/src/main.ts`) to identify and replicate any remaining functionalities into the new section-based robot UI.

# Robot Plugin

The `robot` plugin centralizes core robot functionalities, including starting the robot's services (NATS, Web UI, Mavlink), deployment to remote robots, and code synchronization. It leverages the versioned source pattern (`src_vN`) for its UI and testing infrastructure, similar to the `template` plugin's `src_v3` style.

## Current Progress

### ✅ Core Functionality Migrated
The `start` command logic, previously part of the core `dialtone.go`, has been fully migrated into the `robot` plugin. This includes:
- NATS server startup and management.
- Mavlink service integration.
- Web UI serving using a generic `CreateWebHandler` that accepts an embedded filesystem (`embed.FS`).

### ✅ Deployment Commands
- **`./dialtone.sh robot deploy`**: This command now handles the deployment of the Dialtone binary to a remote robot via SSH.
    - **Architecture Detection**: Automatically detects the remote robot's architecture and cross-compiles the Dialtone binary locally using Podman.
    - **SSH Key Setup**: Ensures SSH key access on the robot for seamless operations.
    - **Sudo Validation**: Validates and optionally configures passwordless sudo for the remote user to streamline automation.
    - **`--proxy` flag**: Configures a Cloudflare tunnel proxy on the local machine (via the `cloudflare` plugin) that targets the remote robot's Tailscale address. This enables exposing the Web UI via a public Cloudflare proxy (e.g., `drone-1.dialtone.earth`).
    - **`--service` flag**: Configures and starts Dialtone as a systemd service on the remote robot, ensuring it runs persistently and automatically restarts.
- **`./dialtone.sh robot sync-code`**: Synchronizes local source code to the remote robot's development directory, excluding build artifacts and node modules.

### ✅ Versioned Source (`src_v1`)
A `src_v1` directory has been scaffolded for the `robot` plugin, mirroring the `src/plugins/template/src_v3` structure. This includes:
- `cmd/`: Go entrypoint for serving the UI.
- `ui/`: Vite-based TypeScript UI.
- `test/`: Automated browser and logic validation suite.

### ✅ Robot UI (src_v1)
The robot's UI has been set up with sections similar to the template plugin, tailored for robot-specific data:
- **Hero Section**: Generic hero visualization.
- **Docs Section**: Placeholder for robot documentation.
- **Telemetry Section**: Replaces the generic 'Table' section to display robot telemetry data.
- **3D Section**: Full-screen 3D visualization.
- **Terminal Section**: Full-screen terminal for sending commands to the robot.
- **Camera Section**: Video display for robot camera feeds.

### ✅ Generic Web Handler
The `CreateWebHandler` in `src/core/web/server.go` has been refactored to accept an `fs.FS` argument, allowing plugins like `robot` and `vpn` to embed and serve their own UIs without modifying core web serving logic.

## Remaining in Core

The core `dialtone` application (`src/dialtone.go`) now primarily acts as a dispatcher, delegating `start`, `robot`, and other plugin-specific commands to their respective plugins. Utility functions that are generally useful across plugins (e.g., `CheckStaleHostname`, `ProxyListener`) remain in `src/core/util`.

## Next Steps: Enhancing `./dialtone.sh robot test src_v1`

The goal is to make the `./dialtone.sh robot test src_v1` command as robust and comprehensive as the `template` plugin's `src_v3` test suite.

### 🛠 To Do: Comprehensive Testing Workflow
1.  **Local UI Testing with Mock Server**: Implement a mock server within the `robot` plugin's test suite to simulate robot telemetry and camera feeds. This will allow the `robot` UI to be thoroughly tested locally without requiring a physical robot or remote deployment.
    *   Leverage existing `dialtone/cli/src/core/mock` for mock data generation.
    *   The test suite should start this mock server during UI tests and verify UI behavior against mock data.

2.  **Real Deployment Verification Test**: Create a dedicated test step that performs a real deployment to a remote robot (using the `./dialtone.sh robot deploy` command) and then verifies that the robot UI starts and functions correctly on the deployed system. This will involve:
    *   Executing the `deploy` command with appropriate flags (`--proxy`, `--service`).
    *   Connecting to the deployed robot's UI (via Tailscale IP or Cloudflare proxy URL).
    *   Running browser-based assertions (using `src/libs/test_v2`) to confirm UI responsiveness and data display.
    *   Verifying that systemd services are active and the robot's web server is accessible.

3.  **Align Test Structure with `template/src_v3`**:
    *   Ensure the `robot/src_v1/test/` directory contains a `main.go` that orchestrates a suite of tests (e.g., 18 steps like `template/src_v3`).
    *   Each test step should cover specific UI sections (`hero`, `docs`, `telemetry`, `3d`, `terminal`, `camera`) and verify their functionality and lifecycle.
    *   Implement assertions for console logs, screenshot capture, and invariant checks using `src/libs/test_v2`.

## Vision: Towards a Core-less Dialtone

The ongoing migration of core functionalities into plugins like `robot` reinforces the vision of a highly modular and extensible Dialtone CLI. Eventually, `src/core` could be reduced to only truly generic, universally shared utilities, with all domain-specific logic residing within plugins. This allows for greater flexibility, easier maintenance, and independent evolution of different Dialtone components.
