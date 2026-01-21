#!/bin/bash
set -e

# Basic Go check
if ! command -v go >/dev/null 2>&1; then
    # Try common local install paths
    LOCAL_GO_PATHS=(
        "$HOME/.dialtone_env/go/bin/go"
        "$(pwd)/dialtone_dependencies/go/bin/go"
        "$HOME/.local/go/bin/go"
    )
    
    for path in "${LOCAL_GO_PATHS[@]}"; do
        if [ -f "$path" ]; then
            export PATH="$(dirname "$path"):$PATH"
            break
        fi
    done
fi

# Final Go check and bootstrap if still missing
if ! command -v go >/dev/null 2>&1; then
    GO_VERSION="1.25.5"
    INSTALL_DIR="$HOME/.dialtone_env/go"
    echo "Go not found. Bootstrapping Go $GO_VERSION to $INSTALL_DIR..."
    
    mkdir -p "$HOME/.dialtone_env"
    TAR_FILE="go$GO_VERSION.linux-amd64.tar.gz"
    curl -LO "https://go.dev/dl/$TAR_FILE"
    tar -C "$HOME/.dialtone_env" -xzf "$TAR_FILE"
    rm "$TAR_FILE"
    
    export PATH="$INSTALL_DIR/bin:$PATH"
fi

# Run the dialtone-dev tool
# All other environment resolution and dependency checks happen in Go
exec go run dialtone-dev.go "$@"
