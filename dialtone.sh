#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Enforce repository-root execution to keep all relative paths predictable.
if [ "$PWD" != "$SCRIPT_DIR" ]; then
    echo "Error: ./dialtone.sh must be run from the repository root."
    echo "Expected: $SCRIPT_DIR"
    echo "Current:  $PWD"
    echo "Run: cd \"$SCRIPT_DIR\" && ./dialtone.sh <command>"
    exit 1
fi

# --- CONFIGURATION ---
GRACEFUL_TIMEOUT=${GRACEFUL_TIMEOUT:-5}   # Seconds to wait after SIGTERM before SIGKILL
PROCESS_TIMEOUT=${PROCESS_TIMEOUT:-0}      # Max runtime in seconds (0 = no limit)

# Track nested wrapper invocations so only the outermost wrapper logs shutdown details.
if [[ "${DIALTONE_WRAPPER_DEPTH:-0}" =~ ^[0-9]+$ ]]; then
    DIALTONE_WRAPPER_DEPTH=$((DIALTONE_WRAPPER_DEPTH + 1))
else
    DIALTONE_WRAPPER_DEPTH=1
fi
export DIALTONE_WRAPPER_DEPTH

CLEANUP_VERBOSE=1
if [ "$DIALTONE_WRAPPER_DEPTH" -gt 1 ]; then
    CLEANUP_VERBOSE=0
fi

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
  bun <subcmd>  Bun toolchain tools (exec, run, x)
  clone         Clone or update the repository
  sync-code     Sync source code to remote robot
  ssh           SSH tools (upload, download, cmd)
  provision     Generate Tailscale Auth Key)
  logs          Tail remote logs
  diagnostic    Run system diagnostics (local or remote)
  branch <name>      Create or checkout a feature branch
  ide <subcmd>       IDE tools (setup-workflows)
  github <subcmd>    Manage GitHub interactions (pr, check-deploy)
  www <subcmd>       Manage public webpage (Vercel wrapper)
  ui <subcmd>        Manage web UI (dev, build, install)
  test <subcmd>      Run tests (legacy)

  ai <subcmd>        AI tools (opencode, developer, subagent)
  go <subcmd>        Go toolchain tools (install, lint)
  help               Show this help message

Global Options:
  --env <path>       Set DIALTONE_ENV directory
  --timeout <sec>    Max runtime before graceful shutdown (0 = no limit)
  --grace <sec>      Seconds to wait after SIGTERM before SIGKILL (default: 5)
EOF
}

# --- GRACEFUL SHUTDOWN ---
CHILD_PID=""
CLEANUP_RAN=0

cleanup() {
    # cleanup can be triggered by INT/TERM and then EXIT; run it once.
    if [ "$CLEANUP_RAN" -eq 1 ]; then
        return
    fi
    CLEANUP_RAN=1
    trap - EXIT INT TERM

    if [ -n "$CHILD_PID" ] && kill -0 "$CHILD_PID" 2>/dev/null; then
        if [ "$CLEANUP_VERBOSE" -eq 1 ]; then
            echo ""
            echo "[dialtone] Sending SIGTERM to process $CHILD_PID..."
        fi
        kill -TERM "$CHILD_PID" 2>/dev/null || true
        
        # Wait for graceful shutdown
        local waited=0
        while [ $waited -lt $GRACEFUL_TIMEOUT ] && kill -0 "$CHILD_PID" 2>/dev/null; do
            sleep 1
            waited=$((waited + 1))
            if [ "$CLEANUP_VERBOSE" -eq 1 ]; then
                echo "[dialtone] Waiting for graceful shutdown... ($waited/$GRACEFUL_TIMEOUT)"
            fi
        done
        
        # Force kill if still running
        if kill -0 "$CHILD_PID" 2>/dev/null; then
            if [ "$CLEANUP_VERBOSE" -eq 1 ]; then
                echo "[dialtone] Process did not exit, sending SIGKILL..."
            fi
            kill -KILL "$CHILD_PID" 2>/dev/null || true
        else
            if [ "$CLEANUP_VERBOSE" -eq 1 ]; then
                echo "[dialtone] Process exited gracefully."
            fi
        fi
    fi
}

# Trap signals and forward to child
trap cleanup EXIT
trap 'cleanup; exit 130' INT
trap 'cleanup; exit 143' TERM

# 0. Ensure critical directories exist for Go embed
mkdir -p "$SCRIPT_DIR/src/core/web/dist"

# 1. Resolve DIALTONE_ENV and identify command
DIALTONE_CMD=""
ARGS=()

# First pass: find --env flag to source it before any other logic
for arg in "$@"; do
    if [[ "$arg" == --env=* ]]; then
        DIALTONE_ENV_FILE="${arg#*=}"
    fi
done

# If --env flag wasn't found in first pass, find it as positional if it exists
for (( i=1; i<=$#; i++ )); do
    if [[ "${!i}" == "--env" ]]; then
        j=$((i+1))
        DIALTONE_ENV_FILE="${!j}"
    fi
done

if [ -z "$DIALTONE_ENV_FILE" ]; then
    DIALTONE_ENV_FILE="$SCRIPT_DIR/env/.env"
fi

# SOURCE THE ENV FILE EARLY
# This puts all variables (including TEST_VAR) into the current shell
if [ -f "$DIALTONE_ENV_FILE" ]; then
    # We use a subshell to parse and then export to avoid sourcing logic issues
    # but a simple source is usually enough if it's a standard .env
    set -a
    source "$DIALTONE_ENV_FILE"
    set +a
fi

# 2. Parse all flags including command
while [[ $# -gt 0 ]]; do
    case "$1" in
        --env=*)
            shift
            ;;
        --env)
            shift 2
            ;;
        --timeout=*)
            PROCESS_TIMEOUT="${1#*=}"
            shift
            ;;
        --timeout)
            PROCESS_TIMEOUT="$2"
            shift 2
            ;;
        --grace=*)
            GRACEFUL_TIMEOUT="${1#*=}"
            shift
            ;;
        --grace)
            GRACEFUL_TIMEOUT="$2"
            shift 2
            ;;
        -h|--help|help)
            # Only show shell help if no command set yet
            if [ -z "$DIALTONE_CMD" ]; then
                print_help
                exit 0
            else
                # Pass --help to the subcommand
                ARGS+=("$1")
                shift
            fi
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

# If --clean is present, remove the environment directory first (before Go runs)
for arg in "${ARGS[@]}"; do
    if [[ "$arg" == "--clean" ]]; then
        if [ -n "$DIALTONE_ENV" ] && [ -d "$DIALTONE_ENV" ]; then
            echo "Cleaning dependencies directory: $DIALTONE_ENV"
            # Use chmod to handle read-only files in Go module cache
            chmod -R u+w "$DIALTONE_ENV" 2>/dev/null || true
            rm -rf "$DIALTONE_ENV"
            echo "Successfully removed $DIALTONE_ENV"
        fi
        break
    fi
done

# If no command provided, show help
if [ -z "$DIALTONE_CMD" ]; then
    print_help
    exit 0
fi

# Tilde expansion for DIALTONE_ENV if sourced
if [[ "$DIALTONE_ENV" == "~"* ]]; then
    DIALTONE_ENV="${DIALTONE_ENV/#\~/$HOME}"
fi

# Ensure them exported for child processes (Go binary)
export DIALTONE_ENV
export DIALTONE_ENV_FILE

# Error if DIALTONE_ENV is still not set
if [ -z "$DIALTONE_ENV" ]; then
    echo "Error: DIALTONE_ENV is not set."
    echo ""
    echo "Please add DIALTONE_ENV to your $DIALTONE_ENV_FILE file:"
    echo "  echo 'DIALTONE_ENV=/path/to/your/env' >> $DIALTONE_ENV_FILE"
    echo ""
    echo "Or pass it as an argument:"
    echo "  ./dialtone.sh --env=/path/to/your/env <command>"
    exit 1
fi

# 3. Handle Go Installation / Check
GO_BIN=""
if [ -n "$DIALTONE_ENV" ]; then
    GO_BIN="$DIALTONE_ENV/go/bin/go"
fi

if [ "$DIALTONE_CMD" = "install" ]; then
    OS=$(uname | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    GO_ARCH="$ARCH"
    if [ "$GO_ARCH" = "x86_64" ]; then GO_ARCH="amd64"; fi
    if [ "$GO_ARCH" = "aarch64" ] || [ "$GO_ARCH" = "arm64" ]; then GO_ARCH="arm64"; fi

    mkdir -p "$DIALTONE_ENV"

    # Check for C compiler (required for CGO/DuckDB)
    if ! command -v gcc &> /dev/null && ! command -v clang &> /dev/null; then
        echo ""
        echo "WARNING: No C compiler (gcc/clang) found."
        echo "DuckDB features require a C compiler. To install:"
        echo "  sudo apt-get update && sudo apt-get install -y build-essential"
        echo ""
        echo "Continuing without CGO support..."
        export CGO_ENABLED=0
    fi

    # Perform Go installation if missing
    if [ -n "$DIALTONE_ENV" ] && [ ! -d "$DIALTONE_ENV/go" ]; then
        echo "Go not found in $DIALTONE_ENV/go. Installing..."
        GO_VERSION=$(grep "^go " "$SCRIPT_DIR/go.mod" | awk '{print $2}')
        TAR_FILE="go$GO_VERSION.$OS-$GO_ARCH.tar.gz"
        TAR_PATH="$DIALTONE_ENV/$TAR_FILE"
        echo "Downloading $TAR_FILE to $TAR_PATH..."
        curl -L -o "$TAR_PATH" "https://go.dev/dl/$TAR_FILE"
        tar -C "$DIALTONE_ENV" -xzf "$TAR_PATH"
        rm "$TAR_PATH"
    fi
elif [ -n "$DIALTONE_ENV" ] && [ ! -f "$GO_BIN" ]; then
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

# 5. Enable CGO if C compiler available
if command -v gcc &> /dev/null || command -v clang &> /dev/null; then
    export CGO_ENABLED=1
else
    export CGO_ENABLED=0
fi

# 6. Run the tool
run_with_timeout() {
    local go_cmd="$1"
    shift
    
    # Run in background and capture PID
    "$go_cmd" run "$SCRIPT_DIR/src/cmd/dev/main.go" "$@" &
    CHILD_PID=$!
    
    # If timeout is set, start watchdog
    if [ "$PROCESS_TIMEOUT" -gt 0 ]; then
        (
            sleep "$PROCESS_TIMEOUT"
            if kill -0 "$CHILD_PID" 2>/dev/null; then
                echo ""
                echo "[dialtone] Timeout ($PROCESS_TIMEOUT seconds) reached, initiating shutdown..."
                kill -TERM "$CHILD_PID" 2>/dev/null || true
            fi
        ) &
        WATCHDOG_PID=$!
    fi
    
    # Wait for child process
    wait "$CHILD_PID" 2>/dev/null
    EXIT_CODE=$?
    CHILD_PID=""
    
    # Kill watchdog if it's still running
    if [ -n "$WATCHDOG_PID" ] && kill -0 "$WATCHDOG_PID" 2>/dev/null; then
        kill "$WATCHDOG_PID" 2>/dev/null || true
    fi
    
    exit $EXIT_CODE
}

if [ -n "$GO_BIN" ] && [ -f "$GO_BIN" ]; then
    run_with_timeout "$GO_BIN" "${ARGS[@]}"
elif command -v go &> /dev/null; then
    echo "Using system Go..."
    run_with_timeout "go" "${ARGS[@]}"
else
    echo "Error: Go binary not found at $GO_BIN and system Go not found."
    exit 1
fi
