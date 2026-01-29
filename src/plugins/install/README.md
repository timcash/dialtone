# Install Plugin

Manages development toolchain installation for Dialtone. Installs dependencies to a user-local directory (no sudo required).

## Folder Structure

```shell
src/plugins/install/
├── cli/
│   └── install.go      # Main installation logic
├── test/
│   └── install_suite.go # Test suite
└── README.md
```

## Command Line Help

```shell
./dialtone.sh install              # Auto-detect platform and install
./dialtone.sh install --check      # Verify all dependencies installed
./dialtone.sh install --clean      # Remove all dependencies
./dialtone.sh install --linux-wsl  # Force Linux/WSL installation
./dialtone.sh install --macos-arm  # Force macOS ARM installation
./dialtone.sh install --help       # Show help
./dialtone.sh install /custom/path # Install to custom directory
```

## Remote Installation (SSH)

```shell
./dialtone.sh install --host user@hostname --pass "password" --port 22
```

## Installed Dependencies

```shell
$DIALTONE_ENV/                     # Set in .env file
├── go/bin/go                      # Go 1.25.5
├── node/bin/node                  # Node.js 22.13.0
├── node/bin/vercel                # Vercel CLI (Linux x86_64 only)
├── gh/bin/gh                      # GitHub CLI 2.86.0
├── pixi/pixi                      # Pixi (latest)
├── zig/zig                        # Zig 0.13.0 (Linux x86_64, macOS ARM)
├── cloudflare/cloudflared         # Cloudflared 2025.1.0
├── gcc-aarch64/bin/               # AArch64 compiler 13.3.rel1 (Linux only)
├── gcc-armhf/bin/                 # ARMhf compiler 13.3.rel1 (Linux only)
└── usr/include/linux/videodev2.h  # V4L2 headers (Linux only)
```

## Platform Support

```shell
# Auto-detected platforms:
linux/amd64   → installLocalDepsWSL()
linux/arm64   → installLocalDepsLinuxARM64()
darwin/amd64  → installLocalDepsMacOSAMD64()
darwin/arm64  → installLocalDepsMacOSARM()
```

## Environment Variables

```shell
# Required in .env file (dialtone.sh will error if missing):
DIALTONE_ENV="/path/to/env"    # Installation directory

# Optional for remote installation:
ROBOT_HOST="user@host"         # Default SSH host
ROBOT_USER="username"          # Default SSH user
ROBOT_PASSWORD="password"      # Default SSH password
```

## PATH Setup

```shell
# Add to ~/.bashrc or ~/.zshrc (source DIALTONE_ENV from .env):
export PATH="$DIALTONE_ENV/go/bin:$DIALTONE_ENV/node/bin:$DIALTONE_ENV/zig:$DIALTONE_ENV/gh/bin:$DIALTONE_ENV/pixi:$DIALTONE_ENV/cloudflare:$PATH"
```

## How It Works

```shell
# 1. Platform Detection
#    Uses runtime.GOOS and runtime.GOARCH to select installer

# 2. Idempotent Installation
#    Checks if binary exists before downloading (safe to re-run)

# 3. Download & Extract
#    wget/curl → tar/unzip → cleanup tarball

# 4. User-Local Install
#    All tools in $DIALTONE_ENV, no root required
```

## Tests

```shell
./dialtone.sh test install-scaffold     # Environment setup test
./dialtone.sh test install-help         # Help flag verification
./dialtone.sh test install-local        # Full local installation
./dialtone.sh test install-idempotency  # Re-run safety test
```
