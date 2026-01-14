#!/bin/bash
# build.sh - Dialtone Build Script

set -e

echo "Starting Build Process..."

# 1. Build Web UI
echo "Building Web UI..."
pushd src/web
npm install
npm run build
popd

# 2. Sync web assets
echo "Syncing web assets to src/web_build..."
WEB_BUILD_DIR="src/web_build"
DIST_DIR="src/web/dist"

rm -rf "$WEB_BUILD_DIR"
mkdir -p "$WEB_BUILD_DIR"
cp -r "$DIST_DIR/"* "$WEB_BUILD_DIR/"

# 3. Build Dialtone binary
echo "Building Dialtone binary..."
mkdir -p bin
go build -o bin/dialtone .

echo "Build successful!"
