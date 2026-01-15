# Dialtone CLI Reference

The `dialtone` CLI is the main entry point for building, deploying, and managing Dialtone nodes.

## Commands

### `full-build`
Builds the Web UI, local CLI, and the binary for the target system.
- `-full`: Standard full build (includes ARM64 via Podman).
- `-local`: Build natively on the current system (useful for WSL/Linux development).

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
- `--linux-wsl`: Install dependencies natively on the local Linux/WSL system.
- `-host`, `-port`, `-user`, `-pass`: Standard SSH flags for remote installation.

### `logs`
Tails remote execution logs via SSH.

### `provision`
Generates a fresh Tailscale Auth Key and updates `.env`.

## WSL Development

To develop natively on WSL with camera support:

1.  **Install Dependencies**:
    ```bash
    go build -o bin/dialtone .
    ./bin/dialtone install-deps --linux-wsl
    ```
2.  **Build Natively**:
    ```bash
    ./bin/dialtone full-build -local
    ```
    This ensures `CGO_ENABLED=1` is set to support the V4L2 drivers.

3.  **Run Locally**:
    ```bash
    ./bin/dialtone start -local-only
    ```

## Standard Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-hostname` | `dialtone-1` | Tailscale hostname for this node |
| `-port` | `4222` | NATS port on the tailnet |
| `-web-port` | `80` | Dashboard port |
| `-local-only`| `false` | Run without Tailscale for local debugging |
| `-ephemeral` | `false` | Node is removed from tailnet on exit |
