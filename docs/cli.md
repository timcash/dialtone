# Dialtone CLI Reference

The `dialtone` CLI is the main entry point for building, deploying, and managing Dialtone nodes.

## Commands

### `full-build`
Builds the Web UI, local CLI, and the binary for the target system.
- `-full`: Standard full build (includes ARM64 via Podman).
- `-local`: Build natively on the current system. 
  - Automatically uses portable toolchain in `~/.dialtone_env` if present.
  - Automatically sets `CGO_ENABLED=1` and `CC` (to Zig if available) for camera support.

### `build`
Builds the ARM64 binary.
- `-local`: Build natively on the current system instead of using Podman.

### `deploy`
Deploys the built binary to a remote robot via SSH.
- `-host`: SSH host (user@host).
- `-port`: SSH port (default 22).
- `-user`: SSH user.
- `-pass`: SSH password.
- `-ephemeral`: Register as an ephemeral node on Tailscale.

### `install-deps`
Installs necessary dependencies on a target system.
- `--linux-wsl`: **No-Sudo Portable Installation**.
  - Downloads and extracts Go, Node.js, and Zig into `~/.dialtone_env`.
  - Extracts V4L2 headers from Ubuntu packages without root access.
- `-host`, `-port`, `-user`, `-pass`: Standard SSH flags for remote installation on the robot.

### `logs`
Tails remote execution logs via SSH.

### `provision`
Generates a fresh Tailscale Auth Key and updates `.env`.

---

## WSL/Linux Development

To develop natively on WSL with camera support using the source-based workflow:

1.  **Bootstrap (No Sudo Required)**:
    ```bash
    go run . install-deps --linux-wsl
    ```
    *This creates a self-contained development environment in `~/.dialtone_env`.*

2.  **Build from Source**:
    ```bash
    go run . full-build -local
    ```
    *This builds the Web Dashboard and the native binary with V4L2 drivers enabled.*

3.  **Run Locally**:
    ```bash
    ./bin/dialtone start -local-only
    ```

---

## Standard Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-hostname` | `dialtone-1` | Tailscale hostname for this node |
| `-port` | `4222` | NATS port on the tailnet |
| `-web-port` | `80` | Dashboard port |
| `-local-only`| `false` | Run without Tailscale for local debugging |
| `-ephemeral` | `false` | Node is removed from tailnet on exit |
