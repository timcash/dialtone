#!/bin/bash
set -e

# --- HELP MENU ---
print_help() {
    cat <<EOF
Usage: ./dialtone.sh <command> [options]

Commands:
  start         Start the NATS and Web server
  install [path] Install dependencies (--linux-wsl for WSL, --macos-arm for Apple Silicon)
  build         Build web UI and binary (--local, --full, --remote, --podman, --linux-arm, --linux-arm64)
  deploy        Deploy to remote robot
  camera        Camera tools (snapshot, stream)
  clone         Clone or update the repository
  sync-code     Sync source code to remote robot
  ssh           SSH tools (upload, download, cmd)
  provision     Generate Tailscale Auth Key)
  logs          Tail remote logs
  diagnostic    Run system diagnostics (local or remote)
  branch <name>      Create or checkout a feature branch
  ticket <subcmd>    Manage GitHub tickets (start, next, done, etc.)
  plugin <subcmd>    Manage plugins (add, install, build)
  ide <subcmd>       IDE tools (setup-workflows)
  github <subcmd>    Manage GitHub interactions (pr, check-deploy)
  www <subcmd>       Manage public webpage (Vercel wrapper)
  ui <subcmd>        Manage web UI (dev, build, install)
  test <subcmd>      Run tests (ticket, plugin, tags)

  ai <subcmd>        AI tools (opencode, developer, subagent)
  go <subcmd>        Go toolchain tools (install, lint)
  help               Show this help message
EOF
}

# 1. Resolve DIALTONE_ENV and identify command
DIALTONE_CMD=""
ARGS=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --env=*)
            DIALTONE_ENV="${1#*=}"
            shift
            ;;
        --env)
            DIALTONE_ENV="$2"
            shift 2
            ;;
        -h|--help|help)
            print_help
            exit 0
            ;;
        *)
            if [ -z "$DIALTONE_CMD" ]; then
                DIALTONE_CMD="$1"
            fi
            ARGS+=("$1")
            shift
            ;;
    esac
done

# If no command provided, show help
if [ -z "$DIALTONE_CMD" ]; then
    print_help
    exit 0
fi

# 2. Resolve DIALTONE_ENV from .env if not set by arg
if [ -z "$DIALTONE_ENV" ] && [ -f .env ]; then
    DIALTONE_ENV=$(grep "^DIALTONE_ENV=" .env | cut -d '=' -f2)
fi

# Ensure it is exported for child processes (Go binary)
if [ -n "$DIALTONE_ENV" ]; then
    export DIALTONE_ENV
fi

# 3. Handle Go Installation / Check
GO_BIN=""
if [ -n "$DIALTONE_ENV" ]; then
    GO_BIN="$DIALTONE_ENV/go/bin/go"
fi

if [ "$DIALTONE_CMD" = "install" ]; then
    # Perform Go installation if missing
    if [ -n "$DIALTONE_ENV" ] && [ ! -f "$GO_BIN" ]; then
        echo "Go not found in $DIALTONE_ENV/go. Installing..."
        GO_VERSION=$(grep "^go " go.mod | awk '{print $2}')
        OS=$(uname | tr '[:upper:]' '[:lower:]')
        ARCH=$(uname -m)
        if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
        if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then ARCH="arm64"; fi
        
        TAR_FILE="go$GO_VERSION.$OS-$ARCH.tar.gz"
        echo "Downloading $TAR_FILE..."
        mkdir -p "$DIALTONE_ENV"
        curl -LO "https://go.dev/dl/$TAR_FILE"
        tar -C "$DIALTONE_ENV" -xzf "$TAR_FILE"
        rm "$TAR_FILE"
    fi
elif [ -n "$DIALTONE_ENV" ] && [ ! -f "$GO_BIN" ]; then
    # Command is not install, and Go is missing in the env folder
    echo "Error: Go not found in $DIALTONE_ENV/go."
    echo "Please run './dialtone.sh install' first to set up the environment."
    exit 1
fi

# 4. Setup PATH if Go is in DIALTONE_ENV
if [ -n "$GO_BIN" ] && [ -f "$GO_BIN" ]; then
    ABS_ENV=$(cd "$DIALTONE_ENV" && pwd)
    export PATH="$ABS_ENV/go/bin:$PATH"
    export GOROOT="$ABS_ENV/go"
    export GOCACHE="$ABS_ENV/cache"
    export GOMODCACHE="$ABS_ENV/pkg/mod"
fi

# 5. Run the tool
if [ -n "$GO_BIN" ] && [ -f "$GO_BIN" ]; then
    exec "$GO_BIN" run src/cmd/dev/main.go "${ARGS[@]}"
else
    # Fallback to system go if DIALTONE_ENV isn't set or doesn't have go (and we didn't error above)
    exec go run src/cmd/dev/main.go "${ARGS[@]}"
fi

