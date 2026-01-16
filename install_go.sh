#!/bin/bash
set -e

GO_VERSION="1.25.5"
INSTALL_DIR="$HOME/.local/go"
TAR_FILE="go$GO_VERSION.linux-amd64.tar.gz"
DOWNLOAD_URL="https://go.dev/dl/$TAR_FILE"

echo "Installing Go $GO_VERSION to $INSTALL_DIR..."

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

# Rename 'go' directory to the target name if needed, but standard extraction creates 'go' folder
# Since we want it in $HOME/.local/go, and tar extracts a 'go' folder, we are good.

echo "Go $GO_VERSION installed successfully."
echo "Please add the following to your shell configuration (e.g., ~/.bashrc or ~/.zshrc):"
echo "export PATH=\$HOME/.local/go/bin:\$PATH"
echo ""
echo "You can verify the installation by running:"
echo "export PATH=\$HOME/.local/go/bin:\$PATH && go version"
