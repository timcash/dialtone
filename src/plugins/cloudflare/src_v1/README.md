# Cloudflare Plugin (src_v1)
The `cloudflare` plugin integrates Cloudflare Tunnels into Dialtone, enabling secure remote access and service forwarding without manual firewall configuration. It wraps the `cloudflared` CLI for runtime connectivity and uses versioned ops under `src_v1`.

## Runtime Note

Plain `./dialtone.sh cloudflare src_v1 ...` is the default operator path.

That command is normally routed through the local REPL leader, which means:
- `DIALTONE>` should stay high-level
- detailed `cloudflared` output stays in the subtone log
- use `./dialtone.sh repl src_v3 subtone-list --count 20`
- use `./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200`

Typical index-room summaries are:

```text
DIALTONE> cloudflare tunnel: starting <name> -> <url>
DIALTONE> cloudflare shell: checking shell tunnel status
DIALTONE> cloudflare robot: exposing <hostname> via <url>
```

## Core Commands

```bash
# Authenticate with Cloudflare (opens browser).
./dialtone.sh cloudflare src_v1 login

# Create a named tunnel.
./dialtone.sh cloudflare src_v1 tunnel create <name>

# List all associated tunnels and their status.
./dialtone.sh cloudflare src_v1 tunnel list

# Route a public hostname to a tunnel. Defaults to <DIALTONE_DOMAIN>.dialtone.earth.
./dialtone.sh cloudflare src_v1 tunnel route <name> [<hostname>]

# Run a tunnel and optionally specify a service URL to forward.
./dialtone.sh cloudflare src_v1 tunnel run <name> --url <service-url>
./dialtone.sh cloudflare src_v1 tunnel start <name> --url <service-url>

# Stop running cloudflared tunnel processes.
./dialtone.sh cloudflare src_v1 tunnel stop

# Tunnel status alias (lists tunnels).
./dialtone.sh cloudflare src_v1 tunnel status

# Automate exposing a remote robot (direct Tailscale-to-Cloudflare relay).
# Target defaults to DIALTONE_DOMAIN or DIALTONE_HOSTNAME.
./dialtone.sh cloudflare src_v1 robot [<hostname>]

# Quickly forward a local port or URL using an ephemeral tunnel.
./dialtone.sh cloudflare src_v1 serve <port-or-url>
```

## Versioned Source Commands
These commands are used for developing and testing `src_v1`.

```bash
./dialtone.sh cloudflare src_v1 install  # Install UI dependencies
./dialtone.sh cloudflare src_v1 build    # Build UI assets
./dialtone.sh cloudflare src_v1 test     # Run automated test suite
./dialtone.sh cloudflare src_v1 dev      # Run UI in dev mode with browser sync
```

## Architecture
`Internet -> Cloudflare Edge -> Cloudflare Tunnel (Local Machine) -> Target (local URL or robot hostname)`

- This computer acts as the tunnel entry point.
- The runtime connector is `cloudflared`.
- Tunnel/DNS provisioning and token handling are implemented in `src_v1/go`.

## Troubleshooting
- **502 Bad Gateway**: Tunnel is up but target URL is not reachable. Verify target locally first.
- **Origin cert errors on list/create/route**: Run `./dialtone.sh cloudflare src_v1 login` or use token-based run mode.
- **Cloudflared not found**: install cloudflared on the host and ensure it is on PATH.
