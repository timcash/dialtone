# Deploying Dialtone

This guide provides step-by-step instructions for building and deploying the Dialtone binary to a remote ARM-based robot (e.g., Raspberry Pi). The deployment system uses **local cross-compilation** and **remote SSH upload** to ensure a fast and reliable workflow.

---

## 1. Prerequisites

Before you begin, ensure your local environment and the remote robot meet the following requirements:

### Local Machine
- **Go**: Installed and configured.
- **Podman**: Required for ARM cross-compilation (if you are on an x86_64 machine).
- **Tailscale**: Signed in (if using Tailscale for remote access).

### Remote Robot
- **SSH Access**: You must have a user account with SSH access and sudo privileges.
- **Tailscale**: Service should be installed if you intend to use the Tailscale network.
- **OS**: Linux (e.g., Raspberry Pi OS, Ubuntu).

---

## 2. Configuration (`.env`)

Dialtone uses a `.env` file for credentials and remote settings. Create or update your `.env` in the project root:

```bash
# SSH Credentials
ROBOT_HOST=192.168.4.36
ROBOT_USER=pi
ROBOT_PASSWORD=your_password

# Project Settings
DIALTONE_HOSTNAME=drone-1
REMOTE_DIR_DEPLOY=/home/pi/dialtone_deploy

# Networking
TS_AUTHKEY=tskey-auth-... # Optional: For automatic Tailscale joining
MAVLINK_ENDPOINT=udp:0.0.0.0:14550
```

---

## 3. Deployment Workflow

Follow these steps to build and deploy the binary:

### Step 1: Run the Deploy Command
The `deploy` command automatically detects the remote robot's architecture (ARM or ARM64) and performs the following:
1.  **Locally cross-compiles** the binary for the target architecture using Podman.
2.  **Uploads** the binary and Web UI assets (`web_build`) to the robot.
3.  **Restarts** the Dialtone service on the remote.

```bash
./dialtone.sh deploy --remote
```

> [!NOTE]
> The first run may take longer as Podman assembles the ARM build container. Subsequent builds are much faster.

---

## 4. Verification

Once the deployment completes, verify the system is operational.

### Step 1: Stream Remote Logs
Check the startup sequence and real-time activity on the robot:

```bash
./dialtone.sh logs --remote
```

Expect to see logs confirming components:
```text
[INFO] Starting Dialtone...
[INFO] TSNet: Connected (IP: 100.x.y.z)
[INFO] Camera: Found /dev/video0
[SUCCESS] System Operational
```

### Step 2: Run Diagnostics
Run the diagnostic suite to verify system health and Web UI rendering:

```bash
./dialtone.sh diagnostic
```

This will check:
- CPU/Memory/Disk usage on the robot.
- NATS and Dialtone process status.
- **Web UI Check**: Uses `chromedp` (if Chrome is installed locally) to verify the dashboard is rendering correctly.

---

## 5. Helpful Notes

- **Architecture Overrides**: If auto-detection fails or you want to force an architecture, use:
  `./dialtone.sh deploy --remote --arch linux-arm` (for 32-bit) or `--arch linux-arm64` (for 64-bit).
- **Toolchain**: You do **not** need to install Go or Node on the robot. Deployment only moves pre-built binaries.
- **Log Files**: Remote logs are saved to `~/nats.log` on the robot by default.
- **Web UI**: Access the dashboard at `http://drone-1` over your Tailscale network.
