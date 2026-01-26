#!/bin/bash
set -e

# 1. Resolve DIALTONE_ENV from args
for i in "$@"; do
    case $i in
        --env=*)
            DIALTONE_ENV="${i#*=}"
            shift
            ;;
        --env)
            DIALTONE_ENV="$2"
            shift 2
            ;;
    esac
done

# 2. Resolve DIALTONE_ENV from .env if not set by arg
if [ -z "$DIALTONE_ENV" ] && [ -f .env ]; then
    DIALTONE_ENV=$(grep "^DIALTONE_ENV=" .env | cut -d '=' -f2)
fi

# Ensure it is exported for child processes (Go binary)
if [ -n "$DIALTONE_ENV" ]; then
    export DIALTONE_ENV
fi

# 3. Warn if DIALTONE_ENV is missing
if [ -z "$DIALTONE_ENV" ]; then
    echo "WARNING: DIALTONE_ENV not found in args or .env."
    echo "Please add DIALTONE_ENV=/path/to/env to your .env file."
fi

# 4. If DIALTONE_ENV found, handle Go installation
if [ -n "$DIALTONE_ENV" ]; then
    GO_BIN="$DIALTONE_ENV/go/bin/go"
    
    # Check if golang is installed in that folder
    if [ ! -f "$GO_BIN" ]; then
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
    
    # Update PATH to use the environment's Go
    ABS_ENV=$(cd "$DIALTONE_ENV" && pwd)
    export PATH="$ABS_ENV/go/bin:$PATH"
    export GOROOT="$ABS_ENV/go"
    export GOCACHE="$ABS_ENV/cache"
    export GOMODCACHE="$ABS_ENV/pkg/mod"
fi

# 5. Run the dialtone-dev tool
if [ -n "$GO_BIN" ]; then
    exec "$GO_BIN" run src/cmd/dev/main.go "$@"
else
    exec go run src/cmd/dev/main.go "$@"
fi

