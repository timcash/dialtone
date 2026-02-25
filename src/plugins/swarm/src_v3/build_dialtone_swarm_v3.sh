#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "==============================================="
echo " 1. Installing System Dependencies             "
echo "==============================================="
sudo apt-get update
sudo apt-get install -y \
  curl git build-essential cmake ninja-build clang lld \
  libuv1-dev libuv1 pkg-config python3

if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
  sudo apt-get install -y nodejs npm
fi

echo "==============================================="
echo " 2. Installing bare-make                       "
echo "==============================================="
if ! command -v bare-make >/dev/null 2>&1; then
  sudo npm install -g bare-runtime bare-make
fi

echo "==============================================="
echo " 3. Preparing Workspace (src_v3)               "
echo "==============================================="
LIBUDX_DIR="$SCRIPT_DIR/libudx"
SOURCE_FILE="$SCRIPT_DIR/dialtone_swarm_v3.c"
OUTPUT_BIN="$SCRIPT_DIR/dialtone_swarm_v3_x86_64"

if [ ! -f "$SOURCE_FILE" ]; then
  echo "Error: missing source file $SOURCE_FILE"
  exit 1
fi

echo "==============================================="
echo " 4. Fetching and Building libudx               "
echo "==============================================="
if [ ! -d "$LIBUDX_DIR" ]; then
  git clone https://github.com/holepunchto/libudx.git "$LIBUDX_DIR"
fi

cd "$LIBUDX_DIR"
npm install
bare-make generate
bare-make build
cd "$SCRIPT_DIR"

echo "==============================================="
echo " 5. Compiling x86_64 Static Binary             "
echo "==============================================="

UDX_LIB="$(find "$LIBUDX_DIR/build" -name "libudx.a" | head -n 1)"
UV_STATIC_LIB="$(find "$LIBUDX_DIR/build" -name "libuv.a" | head -n 1)"

if [ -z "$UDX_LIB" ]; then
    echo "Error: libudx.a not found! The bare-make build failed."
    exit 1
fi

if [ -z "$UV_STATIC_LIB" ]; then
  echo "Error: libuv.a not found inside $LIBUDX_DIR/build"
  exit 1
fi

gcc "$SOURCE_FILE" \
  -O2 -Wall -Wextra \
  -I"$LIBUDX_DIR/include" \
  "$UDX_LIB" \
  "$UV_STATIC_LIB" \
  -static -lpthread -ldl -lrt -lm \
  -o "$OUTPUT_BIN"

echo "==============================================="
echo " Build Complete!                               "
echo " Your x86_64 static binary is ready.           "
echo "                                               "
echo " You can safely send the file located at:      "
echo " $OUTPUT_BIN                                   "
echo " to your friends. They just need to run it!    "
echo "==============================================="
