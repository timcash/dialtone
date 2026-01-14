# Installation & Setup

## Prerequisites

To build Dialtone, your development machine must have:
- **Go 1.25.5+**: For compiling the backend.
- **Node.js v22+ & npm**: For building the TypeScript dashboard.
- **Podman**: Required for ARM64 cross-compilation.

## Required Environment Variables

Create a `.env` file in the project root with the following:

- **`TS_AUTHKEY`**: Your Tailscale auth key (required for headless operation).
- **`ROBOT_HOST`**: The SSH address or IP of your robot (e.g., `192.168.1.100`).
- **`ROBOT_USER`**: The SSH username for your robot (e.g., `pi`).
- **`ROBOT_PASSWORD`**: The SSH password for your robot.
- **`REMOTE_DIR_SRC`**: Remote path for source-based deployment (e.g., `/home/user/dialtone_src`).
- **`REMOTE_DIR_DEPLOY`**: Remote path for binary-based deployment (e.g., `/home/user/dialtone_deploy`).
- **`DIALTONE_HOSTNAME`**: The desired Tailscale hostname for your robot (e.g., `dialtone-1`).

The programs will fail at startup with a descriptive error message if any of these are missing.
