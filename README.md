# Dialtone

A Go application that runs an embedded [NATS](https://nats.io/) server exposed via [Tailscale](https://tailscale.com/). This enables secure, private messaging accessible only to devices on your Tailscale network (tailnet), without any port forwarding or firewall configuration.

## Features

- Embedded NATS server (no external dependencies)
- Tailscale integration via [tsnet](https://tailscale.com/kb/1244/tsnet) - no separate Tailscale daemon required
- **Live MJPEG Camera Stream**: High-performance video streaming from Linux V4L2 devices
- **Real-time Dashboard**: Live status, NATS metrics, and camera feed via WebSocket updates
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
│   ├── dialtone.go       # Main application & Web Server
│   ├── camera_linux.go   # V4L2 camera implementation
│   ├── camera_stub.go    # Stub for non-Linux platforms
│   ├── camera_test.go    # Standalone diagnostic tool
│   ├── index.html        # Dashboard template with WebSocket client
│   ├── dialtone_test.go  # Integration tests
│   └── ssh_tools.go      # SSH utility & deployment tool
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

## Camera System

Dialtone includes a built-in camera streaming system designed for Raspberry Pi and other Linux-based robotics platforms.

### Streaming Endpoints

- **`/stream`**: Low-latency MJPEG video stream. Can be embedded in any `<img>` tag or viewed directly.
- **`/api/cameras`**: JSON list of detected V4L2 video devices.

### Hardware Compatibility

The system uses [go4vl](https://github.com/vladimirvivien/go4vl) to interact with V4L2 devices. It automatically searches for the first available capture device (usually `/dev/video0`) and configures it for 640x480 MJPEG captures at 30 FPS.

### Hardware Diagnostic Tool

If you encounter issues with the camera (e.g., "Device or resource busy"), use the included diagnostic tool:

```bash
# On the Raspberry Pi
cd ~/dialtone_src
go build -o camera_diagnostic src/camera_test.go
./camera_diagnostic
```

This tool will list all video devices, attempt to open them, and save a single frame `test_frame_videoX.jpg` if successful.

## Dashboard WebSocket

The web dashboard uses WebSockets for real-time updates. This avoids page flickering and provides sub-second monitoring of:
- **System Uptime**
- **NATS Connection Counts**
- **Message Throughput** (In/Out)
- **Data Transfer** (Bytes)

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

## Troubleshooting MagicDNS

If you can access the dashboard via IP but not the FQDN URL (e.g., `http://drone-nats.xxxx.ts.net`):

1. **Verify Tailscale is Running**: Ensure your local machine is connected to the same Tailscale network.
2. **Check DNS Settings**: In the Tailscale Admin Console, ensure **MagicDNS** is enabled.
3. **FQDN Resolution**: On some Windows machines, you may need to use the full FQDN including the trailing dot in some tools, though browser access usually handles this.
4. **Local Proxy/VPN**: Disable any other local proxies or VPNs that might be interfering with Tailscale's DNS resolution.

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
