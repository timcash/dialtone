# Cloudflare Plugin
The `cloudflare` plugin integrates Cloudflare Tunnels into Dialtone, enabling secure remote access and service forwarding without manual firewall configuration. It wraps the `cloudflared` CLI for a streamlined experience.

## Core Commands

```bash
# Authenticate with Cloudflare (opens browser).
./dialtone.sh cloudflare login

# Create a named tunnel.
./dialtone.sh cloudflare tunnel create <name>

# List all associated tunnels and their status.
./dialtone.sh cloudflare tunnel list

# Route a public hostname to a tunnel. Defaults to <DIALTONE_DOMAIN>.dialtone.earth.
./dialtone.sh cloudflare tunnel route <name> [<hostname>]

# Run a tunnel and optionally specify a service URL to forward.
./dialtone.sh cloudflare tunnel run <name> [--url <service-url>]

# Automate exposing a remote robot (direct Tailscale-to-Cloudflare relay).
# Target defaults to DIALTONE_DOMAIN or DIALTONE_HOSTNAME.
./dialtone.sh cloudflare robot [<hostname>]

# Set up the Cloudflare proxy as a local systemd service (WSL/Linux).
# Ensures the tunnel starts automatically with your computer.
./dialtone.sh cloudflare setup-service [--name <hostname>]

# Terminate all locally running cloudflared processes.
./dialtone.sh cloudflare tunnel cleanup

# Quickly forward a local port or URL using an ephemeral tunnel.
./dialtone.sh cloudflare serve <port-or-url>
```

## Versioned Source Commands (src_vN)
These commands are used for developing and testing the versioned Cloudflare plugin implementations (e.g., `src_v1`).

```bash
./dialtone.sh cloudflare install [src_vN]  # Install UI dependencies
./dialtone.sh cloudflare build   [src_vN]  # Build UI assets
./dialtone.sh cloudflare test    [src_vN]  # Run automated test suite
./dialtone.sh cloudflare dev     [src_vN]  # Run UI in dev mode with browser sync
```

## Workflow: Full Production Deployment (The "One Command" Workflow)

This is the recommended way to deploy a robot and expose it to the internet permanently.

```bash
./dialtone.sh deploy --service --proxy
```

This single command performs the following:
1. **Validates Sudo & SSH:** Checks for sudo rights on the robot and sets up SSH keys for future access.
2. **Cross-Compiles:** Builds the `dialtone` binary for the robot's architecture (e.g., arm64).
3. **Deploys Service:** Uploads the binary and installs it as a `systemd` service on the robot.
4. **Exposes Publicly:** Configures a **local systemd service** in your WSL/host machine that established a Cloudflare tunnel from `https://<domain>.dialtone.earth` directly to the robot's Tailscale address.
5. **Verifies:** Performs a strict self-test to ensure both services are active and the external URL returns the correct UI version.

## Workflow: Manual Robot Exposure

If the robot is already running and you just want to expose its UI:

1. **Expose via Cloudflare (Local machine)**
   ```bash
   ./dialtone.sh cloudflare robot drone-1
   ```
   This will target `http://drone-1:80` (MagicDNS) or the IP provided.

2. **Make it Persistent**
   ```bash
   # Install as a systemd service so it starts on boot
   sudo ./dialtone.sh cloudflare setup-service --name drone-1
   ```

## Workflow: Non-Interactive (CI/Headless)
If you have a `CF_TUNNEL_TOKEN` in your `.env`, you can run tunnels without needing to `login` or have a browser.

```bash
# Uses the token from .env automatically
./dialtone.sh cloudflare robot drone-1
```

## Architecture: Direct MagicDNS Relay
The current architecture leverages **Tailscale MagicDNS** for routing:
`Internet -> Cloudflare Edge -> Cloudflare Tunnel (Local Machine) -> Robot (Tailscale Hostname)`

- This computer acts as the tunnel entry point.
- It forwards traffic directly to `http://drone-1:80` over the private Tailscale network.
- No local ports (like 8080) are required unless Tailscale is not running globally on the host.

## Troubleshooting
- **502 Bad Gateway**: Usually means the tunnel is running but cannot reach the robot. Verify the robot is up: `ping drone-1`.
- **Cloudflared not found**: Run `./dialtone.sh install` to download dependencies.
- **Permission Denied**: `setup-service` and `deploy --service` require sudo. You will be prompted for your password.
- **Port Conflicts**: If port 8080 is required for a bridge, verify it via `lsof -i :8080`.
