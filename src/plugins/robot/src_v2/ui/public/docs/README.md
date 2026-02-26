# Robot Plugin

The `robot` plugin is the central hub for all robot-specific logic, including MAVLink telemetry integration, NATS messaging, and the mobile-optimized 3D dashboard.

## ⚡ Quick Reference

```bash
# INSTALL & SETUP
./dialtone.sh robot src_v1 install                 # Install local UI dependencies
./dialtone.sh robot src_v1 install --remote        # Sync & install on remote robot (Native Build)

# DEVELOPMENT
./dialtone.sh robot src_v1 dev                     # Start local UI dev server + Chrome
./dialtone.sh robot src_v1 local-web-remote-robot  # Local UI dev server connected to Remote Robot data
./dialtone.sh robot src_v1 serve                   # Run the Go backend locally

# TESTING
./dialtone.sh robot src_v1 test                    # Run automated test suite (Headless)
./dialtone.sh robot src_v1 test --attach           # Run tests watching via local Chrome

# BUILD & DEPLOY
./dialtone.sh robot src_v1 build                   # Build UI assets locally
./dialtone.sh robot src_v1 build --remote          # Sync & build on remote robot
./dialtone.sh robot src_v1 deploy                  # Build & ship binary to remote robot
./dialtone.sh robot src_v1 deploy --service        # Deploy as systemd service
./dialtone.sh robot src_v1 deploy --relay          # Deploy + setup local Cloudflare relay
./dialtone.sh robot src_v1 wake                    # Repoint Cloudflare relay back to robot

# MAINTENANCE
./dialtone.sh robot src_v1 sleep                   # Switch robot to low-power "Sleeping" mode

# UTILITIES
./dialtone.sh robot src_v1 sync-code               # Sync source code to robot (no build)
./dialtone.sh robot src_v1 diagnostic              # Verify live robot UI/telemetry
./dialtone.sh robot telemetry                      # Monitor MAVLink latency on local NATS
```

---

## 🛠 Remote Development (Native Build)

The `--remote` flag allows you to offload the build process to the robot itself. This is critical for ensuring native compilation (ARM64) and proper dependency resolution on the target hardware.

### prerequisites
1.  **SSH Access**: Ensure you can SSH into the robot.
2.  **Environment**: Set the following in your local `env/.env` file:
    ```bash
    ROBOT_HOST=192.168.4.36
    ROBOT_USER=tim
    ROBOT_PASSWORD=secret
    ```

### Workflow
1.  **Sync & Install**: Copies your local source code to the robot and runs `bun install` / `go mod download` on the remote machine.
    ```bash
    ./dialtone.sh robot src_v1 install --remote
    ```
2.  **Sync & Build**: Copies code and compiles the Go binary and Vite UI on the remote machine.
    ```bash
    ./dialtone.sh robot src_v1 build --remote
    ```

> **Note**: These commands mirror your local project structure to `~/dialtone` on the remote robot.

---

## 💤 Sleep Mode & PWA

The `sleep` command switches the robot into a lightweight maintenance mode.

```bash
./dialtone.sh robot src_v1 sleep
```

*   **Mechanism**: Builds/Runs a minimal Go web server locally (`dialtone-sleep`) on port 8080.
*   **Proxy**: Automatically reconfigures the local Cloudflare tunnel to point to `localhost:8080` instead of the robot.
*   **Behavior**: Serves a static "Sleeping..." page.
*   **PWA**: The "Sleeping" page is a full PWA. It caches itself offline, so users see the status even if the network drops.
*   **Wake Up**: Run `robot deploy src_v1` to restore the full application and repoint the proxy to the robot.
*   **Relay-only Wake**: Run `robot src_v1 wake` to repoint Cloudflare to robot without deploying.

**Both the Main UI and Sleep Server are Progressive Web Apps (PWA):**
- **Installable**: Can be added to the home screen.
- **Offline-First**: Assets are cached via Service Worker.
- **Auto-Update**: Updates apply immediately on navigation/refresh (no stale cache issues).

---

## 🌐 Cloudflare Relay (`--relay`)

The `--relay` flag establishes a public tunnel to your robot via your local machine.

```bash
./dialtone.sh robot src_v1 deploy --relay
```

*   **Architecture**: `Public URL` -> `Cloudflare` -> `Local Machine (cloudflared)` --[Tailscale]--> `Robot`.
*   **User Service**: The local proxy runs as a **user-level systemd service** (no sudo required).
*   **Requirement**: You must enable lingering for your user to keep the proxy running in the background:
    ```bash
    loginctl enable-linger $USER
    ```

---

## 🧪 Testing Strategy

We use a "Bottom-Up" testing approach managed by `test_v2`.

### 1. Local Automated Tests
Always run this before deploying. It spins up a local mock server and checks all UI sections.
```bash
./dialtone.sh robot src_v1 test
```
*   **Artifacts**: Generates `TEST.md`, `test.log`, and screenshots in `src/plugins/robot/src_v1/test/screenshots/`.

### 2. Visual Debugging (`--attach`)
If a test fails, watch the browser execution live:
```bash
./dialtone.sh robot src_v1 test --attach
```

### 3. Live Data Verification
To test your local UI changes against *real* data from the robot:
```bash
./dialtone.sh robot src_v1 local-web-remote-robot
```
*   This proxies your local Vite server to the robot's NATS bus.
*   Great for tuning 3D visualizations or HUD latency without deploying.

---

## 📡 Latency & Architecture

The system uses a **Direct NATS** architecture for minimal latency.

### Data Path
1.  **Robot (Go)**: Reads MAVLink → Publishes to NATS (`mavlink.>`).
2.  **Browser (UI)**: Connects *directly* to NATS via WebSocket (`ws://<host>:4223`).
3.  **Visualization**: Three.js / Table components subscribe to topics and render frames.

### Why Direct NATS?
Previous versions relayed data through a Go-based WebSocket handler (`/ws`), causing double-serialization and queueing delays. The current architecture connects the UI directly to the broker, reducing "Queueing" latency (`Q`) to effectively zero.

### Debugging Latency
If the HUD shows high latency (>100ms):
1.  **Check Metrics**: Look at the breakdown in the 3D HUD legend: `Total (P:Processing / N:Network)`.
    *   **P**: Internal Go processing delay (Serial -> NATS).
    *   **N**: Network transit time (Tailscale/LAN).
2.  **Run Monitor**: SSH into the robot and run the telemetry tool to isolate internal timing.
    ```bash
    ./dialtone.sh robot telemetry
    ```

---

## 📂 Versioning (`src_vN`)

The robot plugin supports side-by-side versions.
*   **Current**: `src_v1`
*   **Structure**:
    *   `cmd/`: Go entrypoint
    *   `ui/`: Vite/TypeScript frontend
    *   `test/`: Automated test harness

Always specify the version when running commands (e.g., `robot deploy src_v1`).
