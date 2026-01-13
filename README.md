# Dialtone

A Go application that runs an embedded [NATS](https://nats.io/) server exposed via [Tailscale](https://tailscale.com/). This enables secure, private messaging accessible only to devices on your Tailscale network (tailnet), without any port forwarding or firewall configuration.

## Features

- Embedded NATS server (no external dependencies)
- Tailscale integration via [tsnet](https://tailscale.com/kb/1244/tsnet) - no separate Tailscale daemon required
- **Web Dashboard**: Real-time status, NATS metrics, and Tailscale networking info
- Headless authentication support for remote/SSH deployments
- Ephemeral node option for temporary deployments
- Local-only mode for development without Tailscale
- Graceful shutdown on SIGINT (Ctrl+C) or SIGTERM

## Requirements

- Go 1.25.5 or later
- A Tailscale account (free tier available)

## Project Structure

```
dialtone/
├── src/
│   ├── dialtone.go       # Main application
│   ├── dialtone_test.go  # Tests
│   └── ssh_tools.go      # SSH utility tool
├── bin/                   # Compiled binaries (gitignored)
├── go.mod
├── go.sum
└── README.md
```

## Installation

```bash
# Clone the repository
git clone <repository-url>
cd dialtone

# Download dependencies
go mod download

# Build the executable
go build -o bin/dialtone.exe src/dialtone.go

# Build SSH deployment tools
go build -o bin/ssh_tools.exe src/ssh_tools.go
```

## Usage

### Command-Line Options

```
-hostname string    Tailscale hostname for this NATS server (default "nats")
-port int           NATS port to listen on (default 4222)
-state-dir string   Directory to store Tailscale state (default ~/.config/dialtone)
-ephemeral          Register as ephemeral node (auto-cleanup on disconnect)
-local-only         Run without Tailscale (local NATS only)
-verbose            Enable verbose logging
```

### Running with Tailscale

```bash
# Basic usage - will prompt for authentication
./dialtone

# With custom hostname
./dialtone -hostname my-nats-server

# Ephemeral mode (node removed when disconnected)
./dialtone -ephemeral
```

### Local-Only Mode (No Tailscale)

```bash
./dialtone -local-only
```

### Automated Deployment (Recommended)

The included `ssh_tools.go` can automate building for ARM64, uploading, and restarting the service on a remote Raspberry Pi:

```bash
# Deploy to Raspberry Pi
bin/ssh_tools.exe -host user@192.168.4.36 -pass yourpassword -deploy
```

This command will:
1. Cross-compile `dialtone` for `linux/arm64`.
2. Stop the existing `dialtone` process on the Pi.
3. Upload the new binary to `~/dialtone`.
4. Start the service using `nohup`.

### Manual Deployment
```bash
# Cross-compile for Pi
GOOS=linux GOARCH=arm64 go build -o dialtone src/dialtone.go

# Copy to server
scp dialtone user@server:~/

# SSH and run
ssh user@server
export TS_AUTHKEY="tskey-auth-xxxxx-xxxxxxxxx"
./dialtone
```

### Step 3: Connect from Other Tailnet Devices

From any device on your tailnet:

```bash
# Using NATS CLI
nats sub test.subject -s nats://nats:4222

# In another terminal
nats pub test.subject "Hello from Tailscale!"

# Or programmatically
nc, _ := nats.Connect("nats://nats:4222")
```

### Without Auth Key (Interactive)

If no auth key is set, the server prints a login URL:

```
To start this tsnet server, restart with TS_AUTHKEY set, or go to:
https://login.tailscale.com/a/abc123def456
```

Visit this URL to authenticate. For headless servers, you can copy this URL and open it on any browser.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      dialtone                            │
│                                                          │
│  ┌──────────────┐     ┌──────────────┐                  │
│  │   tsnet      │────▶│  TCP Proxy   │                  │
│  │  (Tailscale) │     │              │                  │
│  │  :4222       │     │              │                  │
│  └──────────────┘     └──────┬───────┘                  │
│                              │                           │
│                              ▼                           │
│                       ┌──────────────┐                  │
│                       │ NATS Server  │                  │
│                       │ (localhost)  │                  │
│                       │   :14222     │                  │
│                       └──────────────┘                  │
└─────────────────────────────────────────────────────────┘
```

The NATS server runs on localhost (not exposed) while tsnet handles all external connections through the Tailscale network.

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `TS_AUTHKEY` | Tailscale auth key for headless authentication |

### State Directory

Tailscale state (node keys, config) is stored in:
- Default: `~/.config/dialtone/`
- Custom: Use `-state-dir /path/to/dir`

For ephemeral nodes (`-ephemeral`), state is temporary and cleaned up on disconnect.

## Testing

```bash
# Run all tests (Tailscale tests skip without TS_AUTHKEY)
go test -v ./src/...

# Run with Tailscale integration tests
TS_AUTHKEY="tskey-auth-xxxxx" go test -v ./src/...
```

### Test Categories

1. **NATS Server Tests** - Verify embedded NATS functionality
2. **Proxy Tests** - Verify the TCP proxy used for Tailscale integration
3. **Tailscale Tests** - Full integration tests (require TS_AUTHKEY)

## Security Considerations

- NATS is only accessible via your Tailscale network
- No ports exposed to the public internet
- Use Tailscale ACLs to control which devices can connect
- Auth keys should be treated as secrets (don't commit to git)
- Consider ephemeral mode for temporary/CI deployments

## Example: Secure Microservices Messaging

```go
// Service A (on any tailnet device)
nc, _ := nats.Connect("nats://nats:4222")
nc.Publish("orders.new", orderData)

// Service B (on any other tailnet device)
nc, _ := nats.Connect("nats://nats:4222")
nc.Subscribe("orders.new", func(m *nats.Msg) {
    processOrder(m.Data)
})
```

## Dependencies

- [nats-server](https://github.com/nats-io/nats-server) v2.12.3 - Embedded NATS server
- [tsnet](https://tailscale.com/kb/1244/tsnet) - Embedded Tailscale
- [nats.go](https://github.com/nats-io/nats.go) - NATS client (for testing)

## License

See the [NATS Server License](https://github.com/nats-io/nats-server/blob/main/LICENSE) and [Tailscale License](https://github.com/tailscale/tailscale/blob/main/LICENSE) for their respective components.
