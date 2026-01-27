# Cloudflare Plugin
The `cloudflare` plugin integrates Cloudflare Tunnels into Dialtone, enabling secure remote access and service forwarding without manual firewall configuration. It wraps the `cloudflared` CLI for a streamlined experience.

## Core Commands

### `cloudflare login`
```bash
# Authenticate with Cloudflare. This will open a browser to authorize the CLI.
# Once complete, your credentials (cert.pem) are stored locally.
./dialtone.sh cloudflare login
```

### `cloudflare tunnel create <name>`
```bash
# Create a named tunnel. The name is arbitrary (e.g., 'local-proxy').
./dialtone.sh cloudflare tunnel create <name>
```

### `cloudflare tunnel list`
```bash
# List all tunnels associated with your account and their status.
./dialtone.sh cloudflare tunnel list
```

### `cloudflare tunnel route <name> [hostname]`
```bash
# Route a public hostname to a tunnel. If no hostname is provided,
# it defaults to <DIALTONE_HOSTNAME>.dialtone.earth.
./dialtone.sh cloudflare tunnel route <name>
```

### `cloudflare tunnel run <name> [options]`
```bash
# Run a tunnel and optionally specify a service URL to forward.
./dialtone.sh cloudflare tunnel run <name> --url http://127.0.0.1:8080
```

### `cloudflare tunnel cleanup`
```bash
# Terminate all locally running cloudflared processes.
./dialtone.sh cloudflare tunnel cleanup
```

### `cloudflare serve <port|url>`
```bash
# Quickly forward a local service to the web using an ephemeral tunnel.
# Ideal for sharing a local dev server (e.g., localhost:8080).
./dialtone.sh cloudflare serve 8080
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

## Troubleshooting
- **Cloudflared not found**: Ensure you have run `./dialtone.sh install` to download the binary to your `DIALTONE_ENV`.
- **Auth issues**: If commands fail with 401/403, re-run `./dialtone.sh cloudflare login`.
- **Port conflicts**: Verification of port 8080 can be done via `lsof -i :8080`.
