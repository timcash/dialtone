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

# Route a public hostname to a tunnel. Defaults to <DIALTONE_HOSTNAME>.dialtone.earth.
./dialtone.sh cloudflare tunnel route <name> [<hostname>]

# Run a tunnel and optionally specify a service URL to forward.
./dialtone.sh cloudflare tunnel run <name> [--url <service-url>]

# Terminate all locally running cloudflared processes.
./dialtone.sh cloudflare tunnel cleanup

# Quickly forward a local port or URL using an ephemeral tunnel.
./dialtone.sh cloudflare serve <port-or-url>
```


## Workflow: Exposing Local Services (e.g., VPN Dashboard)

This workflow demonstrates how to route a public domain (like `test.dialtone.earth`) to a local service running in Dialtone's VPN mode.

### 1. Start the VPN Dashboard
Ensure your local Dialtone is running in VPN mode and listening for local traffic.
```bash
# Starts VPN mode with a local loopback listener on port 8080.
./dialtone.sh vpn --hostname dialtone-vpn-test
```

### 2. Prepare the Tunnel
Create a named tunnel (the name is arbitrary and only used for your reference).
```bash
# 1. Create a tunnel with a name of your choice
./dialtone.sh cloudflare tunnel create <tunnel-name>

# 2. Route your public hostname to the tunnel
# Defaults to <DIALTONE_HOSTNAME>.dialtone.earth
./dialtone.sh cloudflare tunnel route <tunnel-name>
```

### 3. Run the Tunnel
Connect your local port to the Cloudflare edge using the tunnel you just created.
```bash
# Forwards traffic from <DIALTONE_HOSTNAME>.dialtone.earth to your local 8080 dashboard.
./dialtone.sh cloudflare tunnel run <tunnel-name> --url http://127.0.0.1:8080
```

## Workflow: Local-Only Mock Development

This workflow is ideal for testing the Web UI and telemetry systems without needing a Tailscale connection.

### 1. Start Dialtone with Mock Data
Run Dialtone in local-only mode. This starts the NATS server, Mavlink mock, and the Web UI on a specified local port.
```bash
# --local-only: Disables Tailscale integration
# --mock: Generates mock telemetry and camera data
./dialtone.sh start --local-only --mock --web-port 8080
```

### 2. Expose to the Public Web
Since the node isn't on Tailscale, use a Cloudflare Tunnel to share the UI with others using your `DIALTONE_HOSTNAME`.
```bash
# 1. Prepare the route (defaults to <DIALTONE_HOSTNAME>.dialtone.earth)
./dialtone.sh cloudflare tunnel route <tunnel-name>

# 2. Run the tunnel pointing to your local web port
./dialtone.sh cloudflare tunnel run <tunnel-name> --url http://127.0.0.1:8080
```

## Troubleshooting
- **Cloudflared not found**: Ensure you have run `./dialtone.sh install` to download the binary to your `DIALTONE_ENV`.
- **Auth issues**: If commands fail with 401/403, re-run `./dialtone.sh cloudflare login`.
- **Port conflicts**: Verification of port 8080 can be done via `lsof -i :8080`.
