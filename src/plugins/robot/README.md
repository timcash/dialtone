# Robot Plugin

```bash
# QUICK START: All lifecycle commands (Must be validated)
./dialtone.sh robot install src_v1                 # Install UI dependencies
./dialtone.sh robot install src_v1 --remote        # Sync and install on remote robot
./dialtone.sh robot local-web-remote-robot src_v1  # Live UI with remote robot data
./dialtone.sh robot test src_v1                    # Run automated test suite
./dialtone.sh robot test src_v1 --attach           # Run tests and watch in browser
./dialtone.sh robot build src_v1                   # Build UI assets
./dialtone.sh robot build src_v1 --remote          # Sync and build on remote robot
./dialtone.sh robot serve src_v1                   # Serve built UI locally
./dialtone.sh robot sync-code src_v1               # Sync source code to robot
./dialtone.sh robot deploy src_v1                  # Build and ship to remote robot
./dialtone.sh robot deploy src_v1 --service        # Deploy as systemd service
./dialtone.sh robot deploy src_v1 --proxy          # Deploy with Cloudflare proxy
./dialtone.sh robot diagnostic src_v1              # Verify live robot UI/telemetry
./dialtone.sh robot telemetry                      # Monitor MAVLink latency on robot
```

The `robot` plugin is the central hub for all robot-specific logic, including MAVLink telemetry integration, NATS messaging, and the mobile-optimized 3D dashboard. 

---

## 🛠 Development Lifecycle

### 1. Remote Development (Native Build)
To iterate quickly on hardware, use the `--remote` flag with `install` and `build`. This mirrors your local directory structure on the robot and uses your local `env/.env`.

1.  **Requirement**: Set `ROBOT_HOST`, `ROBOT_USER`, and `ROBOT_PASSWORD` in your local `env/.env`.
2.  **Install**: This syncs code, installs Go/Bun on the robot, and resolves UI dependencies.
    ```bash
    ./dialtone.sh robot install src_v1 --remote
    ```
3.  **Build**: This syncs code and compiles the binary natively on the robot's architecture.
    ```bash
    ./dialtone.sh robot build src_v1 --remote
    ```

---

## 📡 Latency Debugging (MAVLink)

If you see high latency (e.g., > 100ms) in the UI HUD, follow these steps to isolate the bottleneck. The UI breakdown is `Total (P:Processing / Q:Queue / N:Network)`.

### 1. Monitor the Internal Pipeline
Run the telemetry tool directly on the robot to see **P** and **Q** latency components. This tool dynamically discovers the correct NATS port via the Web API.
```bash
# On the robot
./dialtone.sh robot telemetry
```
*   **P (Processing)**: Time from serial port arrival to NATS publication. Goal: `< 2ms`.
*   **Q (Queueing)**: Time from NATS publication to WebSocket relay. Goal: `< 10ms`.

### 2. Isolate Network (N)
If P and Q are low but the UI shows high latency, the issue is **N (Network)**.
*   **Direct vs Relay**: Check `tailscale status`. If using a DERP relay, latency will be > 200ms.
*   **Baud Rate**: Ensure the serial connection is set to `57600` or `115200` in the `robot start` command.

### 3. Verify Timestamps
Check the raw logs on the robot to see if `MAVLINK-RAW` frames are arriving at the expected frequency (e.g., 10Hz for heartbeats).
```bash
ssh $ROBOT_USER@$ROBOT_HOST "tail -f ~/dialtone_deploy/robot.log | grep MAVLINK-RAW"
```

---

## 🏗 Modular Architecture

| File | Responsibility |
|------|----------------|
| `robot.go` | Entry point and subcommand router. |
| `start.go` | Core service logic (NATS, Web, MAVLink bridge). |
| `deploy.go` | SSH management and auto-versioning. |
| `sync.go`   | Mirroring local project structure to the robot. |
| `telemetry.go` | CLI tool for real-time latency monitoring. |
| `mavlink_latency.md` | Detailed report on the 6-step message journey. |

---

## 🚀 Recent Improvements (Feb 2026)

- **Dynamic Port Discovery**: The UI and CLI tools now query `/api/init` to find the correct NATS/WS ports, eliminating port conflict issues.
- **Mirrored Remote Sync**: `sync-code` now reproduces your local folder name on the robot (e.g., `/home/user/dialtone`) and copies your `env/.env` automatically.
- **Precision Tracking**: Every `ATTITUDE` and `HEARTBEAT` message now carries `t_raw` and `t_pub` timestamps for millisecond-accurate profiling.
- **Unit Normalization**: Fixed a bug where mock telemetry used seconds, causing 30,000ms "ghost" latency in the UI.
