# Robot Plugin

The `robot` plugin is the central hub for all robot-specific logic, including MAVLink telemetry integration, NATS messaging, and the mobile-optimized 3D dashboard.

## ⚡ Quick Reference

```bash
# === DEVELOPMENT ===
# Start local UI dev server + Chrome (Mock Data)
./dialtone.sh robot dev src_v1

# Start local UI dev server + Remote Robot Data (SSH Tunnel)
./dialtone.sh robot dev src_v1 --robot

# Visual Debugging (Attach to existing Chrome session)
./dialtone.sh robot dev src_v1 --attach

# === DEPLOYMENT ===
# Build & Deploy binary to remote robot (updates service if exists)
./dialtone.sh robot deploy src_v1

# Deploy + Install/Restart Systemd Service
./dialtone.sh robot deploy src_v1 --service

# Deploy + Service + Setup Cloudflare Tunnel Proxy
./dialtone.sh robot deploy src_v1 --service --proxy

# === TESTING ===
# Run automated headless tests
./dialtone.sh robot test src_v1

# Run tests visually (watch in Chrome)
./dialtone.sh robot test src_v1 --attach

# === MAINTENANCE ===
# Verify live robot UI/telemetry status
./dialtone.sh robot diagnostic src_v1

# Monitor MAVLink latency on robot
./dialtone.sh robot telemetry
```

---

## 🚀 Development Workflow

Follow this step-by-step guide to add new features to the Robot UI.

### 1. Start Development Environment
Choose your data source:

**Option A: Local Mock (Fastest)**
Ideal for UI layout and logic changes. Uses simulated telemetry.
```bash
./dialtone.sh robot dev src_v1
```

**Option B: Remote Robot (Real Data)**
Ideal for tuning 3D visualization or testing hardware integration. Tunnels data from the robot via SSH.
```bash
./dialtone.sh robot dev src_v1 --robot
```
*   **Prerequisite**: Ensure `ROBOT_HOST`, `ROBOT_USER`, and `ROBOT_PASSWORD` are set in `env/.env`.

### 2. Make Changes
*   **UI Code**: `src/plugins/robot/src_v1/ui/src/`
    *   **Components**: `components/` (Three.js, Video, Xterm, etc.)
    *   **Layout**: `style.css` (UI V2 architecture)
    *   **Logic**: `main.ts` and `buttons.ts` (Button configurations)
*   **Backend Code**: `src/plugins/robot/src_v1/cmd/`

### 3. Verify Changes
The dev server (Vite) hot-reloads automatically.
*   **Mode Switching**: Use the `9:Mode` button or keys `1-9` to test interactions.
*   **Watchdog**: Verify video pauses after 3 minutes of inactivity.

### 4. Run Tests
Ensure you haven't broken existing functionality.
```bash
./dialtone.sh robot test src_v1
```

### 5. Deploy
Ship your changes to the robot. The deploy script handles version bumping, building (native ARM64), and service restarts.
```bash
./dialtone.sh robot deploy src_v1 --service --proxy
```
*   **--service**: Ensures the systemd service (`dialtone.service`) is installed/restarted.
*   **--proxy**: Ensures the Cloudflare tunnel (`drone-1.dialtone.earth`) is active.

---

## 🏗 Architecture & UI V2

The Robot UI is built on the **UI V2** shared library, ensuring consistent behavior across plugins.

### Layout System
*   **Overlay Primary**: The main content (3D Canvas, Video, Table). Fills the screen or split area.
*   **Mode Form**: The 3x4 grid of thumb-accessible controls at the bottom.
*   **Legend**: The top-left HUD overlay. Click to minimize.

### Feature Highlights
*   **3D Hero**: Interactive Inverse Kinematics (IK) robot arm visualization using Three.js.
*   **Video Watchdog**: Automatically pauses high-bandwidth MJPEG streams after 3 minutes of inactivity to save data.
*   **Chatlog**: Integrated xterm.js console in the 3D view for viewing MAVLink status messages.
*   **Latency HUD**: Real-time visualization of telemetry latency (Processing vs Network).
*   **Smart Updates**: The UI polls for version updates and prompts the user to reload, ensuring no stale cache issues.

---

## 📡 Connectivity

The system uses a hybrid architecture for low latency and accessibility:

1.  **Direct NATS (Telemetry)**:
    *   Robot publishes MAVLink -> NATS (`mavlink.>`).
    *   UI connects via WebSocket (`ws://<host>:4223`).
    *   **Latency**: < 20ms typically.

2.  **MJPEG Stream (Video)**:
    *   Go backend captures frames -> HTTP Stream (`/stream`).
    *   **Optimization**: Explicit flushing and aggressive cache-control headers ensure smooth playback over Cloudflare Tunnels.

3.  **Cloudflare Tunnel**:
    *   Secure public access via `https://drone-1.dialtone.earth`.
    *   Managed via user-level systemd service (`dialtone-proxy-drone-1`).
