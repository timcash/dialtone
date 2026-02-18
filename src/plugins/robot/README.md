# Robot Plugin

The `robot` plugin is the central hub for all robot-specific logic, including MAVLink telemetry integration, NATS messaging, and the mobile-optimized 3D dashboard.

## ⚡ Quick Reference

```bash
# INSTALL & SETUP
./dialtone.sh robot install src_v1                 # Install local UI dependencies
./dialtone.sh robot install src_v1 --remote        # Sync & install on remote robot (Native Build)

# DEVELOPMENT
./dialtone.sh robot dev src_v1                     # Start local UI dev server + Chrome
./dialtone.sh robot local-web-remote-robot src_v1  # Local UI dev server connected to Remote Robot data
./dialtone.sh robot serve src_v1                   # Run the Go backend locally

# TESTING
./dialtone.sh robot test src_v1                    # Run automated test suite (Headless)
./dialtone.sh robot test src_v1 --attach           # Run tests watching via local Chrome

# BUILD & DEPLOY
./dialtone.sh robot build src_v1                   # Build UI assets locally
./dialtone.sh robot build src_v1 --remote          # Sync & build on remote robot
./dialtone.sh robot deploy src_v1                  # Build & ship binary to remote robot
./dialtone.sh robot deploy src_v1 --service        # Deploy as systemd service
./dialtone.sh robot deploy src_v1 --proxy          # Deploy + setup local Cloudflare proxy

# MAINTENANCE
./dialtone.sh robot sleep src_v1                   # Switch robot to low-power "Sleeping" mode

# UTILITIES
./dialtone.sh robot sync-code src_v1               # Sync source code to robot (no build)
./dialtone.sh robot diagnostic src_v1              # Verify live robot UI/telemetry
./dialtone.sh robot telemetry                      # Monitor MAVLink latency on robot
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
    ./dialtone.sh robot install src_v1 --remote
    ```
2.  **Sync & Build**: Copies code and compiles the Go binary and Vite UI on the remote machine.
    ```bash
    ./dialtone.sh robot build src_v1 --remote
    ```

> **Note**: These commands mirror your local project structure to `~/dialtone` on the remote robot.

---

## 💤 Sleep Mode & Maintenance

The `sleep` command switches the robot into a lightweight maintenance mode.

```bash
./dialtone.sh robot sleep src_v1
```

*   **Mechanism**: Replaces the heavy Dialtone binary with a minimal Go web server (`dialtone-sleep`).
*   **Behavior**: Serves a static "Sleeping..." page.
*   **Connectivity**: Uses `tsnet` to maintain the robot's Tailscale identity (`drone-1`), ensuring the hostname remains resolvable.
*   **Auto-Proxy**: Automatically ensures the local Cloudflare tunnel is running so the public URL remains accessible.
*   **Wake Up**: Run `robot deploy src_v1` to restore the full application.

---

## 🌐 Cloudflare Proxy (`--proxy`)

The `--proxy` flag establishes a public tunnel to your robot via your local machine.

```bash
./dialtone.sh robot deploy src_v1 --proxy
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
./dialtone.sh robot test src_v1
```
*   **Artifacts**: Generates `TEST.md`, `test.log`, and screenshots in `src/plugins/robot/src_v1/test/screenshots/`.

### 2. Visual Debugging (`--attach`)
If a test fails, watch the browser execution live:
```bash
./dialtone.sh robot test src_v1 --attach
```

### 3. Live Data Verification
To test your local UI changes against *real* data from the robot:
```bash
./dialtone.sh robot local-web-remote-robot src_v1
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
