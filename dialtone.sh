#!/bin/bash
set -e

# Determine the repository root
REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"

# Default DIALTONE_ENV if not set
if [ -z "$DIALTONE_ENV" ]; then
    if [ -d "$REPO_ROOT/dialtone_dependencies" ]; then
        export DIALTONE_ENV="$REPO_ROOT/dialtone_dependencies"
        echo "[dialtone] Using repo-local dependencies: $DIALTONE_ENV"
    else
        export DIALTONE_ENV="$HOME/.dialtone_env"
    fi
fi

# Handle --env-dir flag for any command
for arg in "$@"; do
    if [[ "$arg" == --env-dir=* ]]; then
        export DIALTONE_ENV="${arg#*=}"
        echo "[dialtone] Overriding environment directory: $DIALTONE_ENV"
    fi
done

GO_VERSION="1.25.5"
INSTALL_DIR="$HOME/.local/go"
TAR_FILE="go$GO_VERSION.linux-amd64.tar.gz"
DOWNLOAD_URL="https://go.dev/dl/$TAR_FILE"

# Function to check if go is installed and is the correct version
check_go_installed() {
    if command -v go >/dev/null 2>&1; then
        return 0
    fi
    # Check if go is in DIALTONE_ENV
    if [ -f "$DIALTONE_ENV/go/bin/go" ]; then
        export PATH="$DIALTONE_ENV/go/bin:$PATH"
        return 0
    fi
    # Check legacy install dir
    if [ -f "$INSTALL_DIR/bin/go" ]; then
        export PATH="$INSTALL_DIR/bin:$PATH"
        return 0
    fi
    return 1
}

if ! check_go_installed; then
    echo "Go not found. Installing Go $GO_VERSION to $INSTALL_DIR..."
    
    # Create directory if it doesn't exist
    mkdir -p "$HOME/.local"
    
    # Remove existing installation
    if [ -d "$INSTALL_DIR" ]; then
        echo "Removing existing Go installation..."
        rm -rf "$INSTALL_DIR"
    fi
    
    # Download Go
    echo "Downloading $DOWNLOAD_URL..."
    curl -LO "$DOWNLOAD_URL"
    
    # Extract archive
    echo "Extracting... (this might take a moment)"
    tar -C "$HOME/.local" -xzf "$TAR_FILE"
    
    # Clean up
    rm "$TAR_FILE"
    
    export PATH="$INSTALL_DIR/bin:$PATH"
fi

# Ensure necessary environment variables are set for the run
export DIALTONE_ENV="$DIALTONE_ENV"

# Run the dialtone-dev tool
# We use 'go run' to compile and run the tool on the fly.
# "$@" passes all arguments from this script to the Go program.
exec go run dialtone-dev.go "$@"
