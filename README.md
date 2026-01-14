# Dialtone
![Web Interface](ui.png)

Dialtone is a high-performance **video teleoperation network** designed for robotic coordination and serves as a specialized **training ground for physical AI**. It provides a secure, encrypted, and ultra-low latency bridge between remote robotic hardware and humanoid/agentic control systems.

## 1. System Purpose

The project aims to solve the "last mile" connectivity problem for physical AI. By combining user-space networking with embedded messaging, Dialtone allows developers to:
- **Teleoperate Robots**: Low-latency MJPEG streaming and real-time NATS command loops.
- **Train AI Models**: Collect high-fidelity sensor and video data over private networks for imitation learning.
- **Coordinate Fleets**: Securely manage multiple robotic nodes without complex firewall or VPN configurations.

## 2. System Design

The system is designed to run as a single-binary appliance on ARM64-based robotic platforms.

### Hardware Stack
- **The Robot**: Target platforms like Raspberry Pi 4/5 or NVIDIA Jetson. These handle the physical interaction with the environment.
- **Connected Devices**:
    - **Cameras**: Supports V4L2-compatible USB and MIPI cameras (e.g., Raspberry Pi Camera Module).
    - **Motors/Servos**: Interface via GPIO or serial bridges (e.g., MAVLink) integrated into the NATS bus.

### Software Stack
- **Control Computer**: A Go application that orchestrates the camera feed, NATS server, and web interface.
- **Web UI**: A real-time dashboard built with Vite/TypeScript and embedded directly into the Go binary.

## 3. Network Architecture

Dialtone leverages a modern, identity-based networking stack to eliminate the need for port forwarding or public IPs.

- **NATS**: The "central nervous system" of the robot. Telemetry (video, sensors) and commands (velocity, attitude) are published to a built-in NATS server.
- **tsnet (Tailscale)**: The system embeds Tailscale directly. It appears as a first-class node on your private **tailnet**, providing automatic wireguard encryption and stable DNS (MagicDNS).
- **Web Server & UI**: Accessible via `http://<hostname>:80`. It provides:
    - **Live MJPEG Stream**: Low-latency video feedback.
    - **NATS Bridge**: A WebSocket interface for interacting with the NATS bus directly from the browser.
    - **System Metrics**: Real-time stats on uptime, connection count, and throughput.

## 4. Build System (Podman)

To ensure consistent builds for ARM64 robots from any development machine (Windows/Mac/Linux), Dialtone uses a containerized build loop.

- **Cross-Compilation**: The `dialtone` CLI uses **Podman** to spin up a specialized Linux container (`golang:1.25.5`) with the `aarch64-linux-gnu-gcc` toolchain.
- **CGO Support**: This enables building the V4L2 camera drivers (which require Linux headers) correctly for the target platform even when developing on Windows.
- **Asset Embedding**: The build script (`build_and_deploy.ps1`) compiles the Vite frontend and uses `go:embed` to package the entire UI into the final binary.

## 5. Automated Deployment (Unified CLI)

Deployment is handled directly through the `dialtone` binary, which serves as a unified manager for the robotic network.

```bash
# Build everything and deploy in one shot
dialtone full-build -deploy -host user@ip -pass password
```

The unified CLI performs:
1. **Web Build**: Runs `npm install` and `npm run build` programmatically.
2. **Compilation**: `dialtone build` triggers Podman-based cross-compilation.
3. **Transfer**: `dialtone deploy` handles SFTP upload and server restart.
4. **Observation**: `dialtone logs` tails the remote execution logs via SSH.

## 6. Development Workflow (TDD for AI Agents)

When adding features or fixing bugs (especially when utilizing LLM-based coding assistants), follow this Test-Driven Development (TDD) loop to ensure stability across the network.

### The Loop
1. **Create Test**: Add a local unit test in `src/dialtone_test.go` or a remote integration test in `src/remote_test.go`.
2. **Implement**: Write the minimal code needed to satisfy the test.
3. **Iterate**: Run `go test -v ./src/...` locally for immediate feedback.
4. **Build & Deploy**: Once local tests are green, run `dialtone full-build -deploy`.
5. **README Update**: If you changed interfaces (new NATS subjects, new API endpoints), update the documentation immediately.
6. **Verify Live**: Run system-level tests against the Tailscale IP of the robot to verify end-to-end functionality.

## 7. Build Instructions

### Prerequisites
To build Dialtone, your development machine must have:
- **Go 1.25.5+**: For compiling the backend.
- **Node.js v22+ & npm**: For building the TypeScript dashboard.
- **Podman**: Required for ARM64 cross-compilation.

### Required Environment Variables
To ensure successful operation and deployment, create a `.env` file in the project root with the following:

- **`TS_AUTHKEY`**: Your Tailscale auth key (required for headless operation).
- **`REMOTE_DIR_SRC`**: Remote path for source-based deployment (e.g., `/home/user/dialtone_src`).
- **`REMOTE_DIR_DEPLOY`**: Remote path for binary-based deployment (e.g., `/home/user/dialtone_deploy`).
- **`DIALTONE_HOSTNAME`**: The desired Tailscale hostname for your robot (e.g., `dialtone-1`).

The programs will fail at startup with a descriptive error message if any of these are missing.

### Full Build & Deploy
The recommended way to build the entire project is using the unified command:

```bash
go build -o bin/dialtone.exe ./src
bin/dialtone.exe full-build -deploy -host user@ip -pass password
```

This automates the following verified steps:
1.  **Web Assets**: Compiles the Vite/TS frontend.
2.  **CLI Tooling**: Self-updates the `dialtone.exe` manager.
3.  **ARM64 Cross-Build**: Invokes Podman for the target binary.
4.  **Remote Deployment**: SFTPs and restarts the service.

### Manual Build Steps
If you need to build components individually:

**1. Build the Web Interface**
```bash
cd src/web
npm install
npm run build
```

**2. Build the ARM64 Binary (via Podman)**
```bash
go build -o bin/dialtone.exe ./src
bin/dialtone.exe build
```

### Automated Tailscale Provisioning

Dialtone uses two types of Tailscale credentials to manage its secure, per-process VPN network without requiring a system-level installation:

1.  **Tailscale API Access Token**: (Conceptual "Master Key") Used only on your local machine to programmatically generate smaller "Visitor Keys".
    - **How to get it**: Go to [Tailscale Settings > Keys](https://login.tailscale.com/admin/settings/keys) and generate an "Access Token".
2.  **Tailscale Auth Key**: (Conceptual "Ephemeral Visitor Key") A temporary, short-lived credential that allows the `dialtone` process on the robot to join your network.
    - **How it works**: When you run `dialtone provision`, the CLI uses your API Token to request a one-time use, ephemeral key from Tailscale.

#### How the Key reaches the Robot
Security is maintained by ensuring the Auth Key is never permanently stored on the robot's disk:
1.  **Local Storage**: The `provision` command saves the generated `TS_AUTHKEY` in your local `.env` file.
2.  **SSH Propagation**: When you run `dialtone deploy`, the local CLI reads the key from `.env` and passes it to the remote computer via the **SSH environment**. 
3.  **Process Injection**: The remote `dialtone` binary is started via an `env TS_AUTHKEY=...` command inside a `nohup` block. The key exists only in the volatile memory of the running process.
4.  **Auto-Cleanup**: Because the key is marked as **ephemeral**, Tailscale will automatically remove the robot from your machine list as soon as the process disconnects, keeping your admin console clean.

**3. Provision Tailscale Key (Optional)**
If you have a Tailscale API Access Token, you can generate a new auth key and update `.env` automatically:
```bash
bin/dialtone.exe provision -api-key your_tailscale_api_token
```

**4. Deploy and View Logs**
```bash
# Deploy to robot
bin/dialtone.exe deploy -host user@ip -pass password

# Tail remote logs
bin/dialtone.exe logs -host user@ip -pass password
```

**4. Build Local-Only Binary (Windows/Mac)**
```bash
go build -o bin/dialtone.exe ./src
```

---

### Command-Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-hostname` | `dialtone-1` | Tailscale hostname for this node |
| `-port` | `4222` | NATS port on the tailnet |
| `-web-port` | `80` | Dashboard port |
| `-local-only`| `false` | Run without Tailscale for local debugging |
| `-ephemeral` | `false` | Node is removed from tailnet on exit |
