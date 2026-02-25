#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

BUILD_DIR="$SCRIPT_DIR/libudx/build-arm64-local"
SOURCE_FILE="$SCRIPT_DIR/dialtone_swarm_v3.c"
OUTPUT_BIN="$SCRIPT_DIR/dialtone_swarm_v3_arm64"

if ! command -v aarch64-linux-gnu-gcc >/dev/null 2>&1; then
  echo "error: missing aarch64-linux-gnu-gcc"
  echo "install: sudo apt-get install -y gcc-aarch64-linux-gnu g++-aarch64-linux-gnu binutils-aarch64-linux-gnu"
  exit 1
fi

if [ ! -d "$SCRIPT_DIR/libudx" ]; then
  echo "error: missing $SCRIPT_DIR/libudx"
  echo "run ./build_dialtone_swarm_v3.sh once to initialize libudx checkout"
  exit 1
fi

echo "==============================================="
echo " 1. Building ARM64 libudx/libuv (local cross)  "
echo "==============================================="
cmake -S "$SCRIPT_DIR/libudx" -B "$BUILD_DIR" -G Ninja \
  -DCMAKE_SYSTEM_NAME=Linux \
  -DCMAKE_SYSTEM_PROCESSOR=aarch64 \
  -DCMAKE_C_COMPILER=aarch64-linux-gnu-gcc \
  -DCMAKE_CXX_COMPILER=aarch64-linux-gnu-g++
cmake --build "$BUILD_DIR" -j"$(nproc)"

UDX_LIB="$(find "$BUILD_DIR" -name libudx.a | head -n 1)"
UV_STATIC_LIB="$(find "$BUILD_DIR" -name libuv.a | head -n 1)"

if [ -z "$UDX_LIB" ] || [ -z "$UV_STATIC_LIB" ]; then
  echo "error: missing ARM64 static libraries from build"
  exit 1
fi

echo "==============================================="
echo " 2. Building ARM64 static binary               "
echo "==============================================="
rm -f "$SCRIPT_DIR/dialtone_swarm_v3" "$SCRIPT_DIR/dialtone_swarm_v3_arm64_static"
aarch64-linux-gnu-gcc "$SOURCE_FILE" \
  -O2 -Wall -Wextra \
  -I"$SCRIPT_DIR/libudx/include" \
  "$UDX_LIB" "$UV_STATIC_LIB" \
  -static -lpthread -ldl -lrt -lm \
  -o "$OUTPUT_BIN"

echo "==============================================="
echo " Build Complete!                               "
echo " Output : $OUTPUT_BIN"
echo "==============================================="
