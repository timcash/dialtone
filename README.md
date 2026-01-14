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

- **Cross-Compilation**: The `ssh_tools` utility uses **Podman** to spin up a specialized Linux container (`golang:1.25.5`) with the `aarch64-linux-gnu-gcc` toolchain.
- **CGO Support**: This enables building the V4L2 camera drivers (which require Linux headers) correctly for the target platform even when developing on Windows.
- **Asset Embedding**: The build script (`build_and_deploy.ps1`) compiles the Vite frontend and uses `go:embed` to package the entire UI into the final binary.

## 5. Automated Deployment (SSH Tool)

Deployment is handled by the internal `ssh_tools.go` utility, which automates the transfer and lifecycle management of the application.

```powershell
# Full build and deploy cycle
./build_and_deploy.ps1
```

The deployment tool performs:
1. **Compilation**: Podman-based cross-compilation for `linux/arm64`.
2. **Transfer**: SFTP upload of the 30MB+ binary to the robot.
3. **Lifecycle**: Graceful stop of existing processes and a background `nohup` restart of the new version.
4. **Environment**: Automatic propagation of authentication keys and environment variables.

## 6. Development Workflow (TDD for AI Agents)

When adding features or fixing bugs (especially when utilizing LLM-based coding assistants), follow this Test-Driven Development (TDD) loop to ensure stability across the network.

### The Loop
1. **Create Test**: Add a local unit test in `src/dialtone_test.go` or a remote integration test in `src/remote_test.go`.
2. **Implement**: Write the minimal code needed to satisfy the test.
3. **Iterate**: Run `go test -v ./src/...` locally for immediate feedback.
4. **Build & Deploy**: Once local tests are green, run `./build_and_deploy.ps1` to push to the physical robot.
5. **README Update**: If you changed interfaces (new NATS subjects, new API endpoints), update the documentation immediately.
6. **Verify Live**: Run system-level tests against the Tailscale IP of the robot to verify end-to-end functionality.

## 7. Build Instructions

### Prerequisites
To build Dialtone, your development machine must have:
- **Go 1.25.5+**: For compiling the backend and SSH tools.
- **Node.js v22+ & npm**: For building the TypeScript dashboard.
### Required Environment Variables
To ensure successful operation and deployment, create a `.env` file in the project root with the following:

- **`TS_AUTHKEY`**: Your Tailscale auth key (required for headless operation).
- **`REMOTE_DIR_SRC`**: Remote path for source-based deployment (e.g., `/home/user/dialtone_src`).
- **`REMOTE_DIR_DEPLOY`**: Remote path for binary-based deployment (e.g., `/home/user/dialtone`).
- **`DIALTONE_HOSTNAME`**: The desired Tailscale hostname for your robot (e.g., `drone-nats`).

The programs will fail at startup with a descriptive error message if any of these are missing.

### Automated Build & Deploy
The recommended way to build the entire project is using the provided PowerShell script.

```powershell
./build_and_deploy.ps1
```

This script automates the following verified steps:
1.  **Web Assets**: Compiles the Vite/TS frontend in `src/web` and copies the output to `src/web_build`.
2.  **SSH Tooling**: Builds the custom `ssh_tools.exe` used for remote management.
3.  **ARM64 Cross-Build**: Invokes Podman to build the Linux binary with CGO enabled for camera support.
4.  **Remote Deployment**: SFTPs the binary to the robot and restarts the service.

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
go build -o bin/ssh_tools.exe src/ssh_tools.go
bin/ssh_tools.exe -podman-build
```

**3. Build Local-Only Binary (Windows/Mac)**
```bash
go build -o bin/dialtone.exe ./src
```

### Common Build Issues
- **Podman VM Not Running**: Ensure the Podman machine is started (`podman machine start`).
- **NPM Version Conflicts**: Use Node v22+ to ensure compatibility with the Vite build loop.
- **SSH Timeout**: Verfiy the robot's IP address in `build_and_deploy.ps1` matches your hardware.

---

### Command-Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-hostname` | `nats` | Tailscale hostname for this node |
| `-port` | `4222` | NATS port on the tailnet |
| `-web-port` | `80` | Dashboard port |
| `-local-only`| `false` | Run without Tailscale for local debugging |
| `-ephemeral` | `false` | Node is removed from tailnet on exit |
