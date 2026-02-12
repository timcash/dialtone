#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

usage() {
  cat <<EOF
Usage: ./dialtone.sh install [path]

Installs the Go toolchain into DIALTONE_ENV/go.

Arguments:
  [path]    Optional install root. Overrides DIALTONE_ENV from env/.env.
EOF
}

if [[ "${1:-}" == "help" || "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

if [ -n "${1:-}" ]; then
  export DIALTONE_ENV="$1"
fi

if [ -z "${DIALTONE_ENV:-}" ]; then
  ENV_FILE="${DIALTONE_ENV_FILE:-$REPO_ROOT/env/.env}"
  if [ -f "$ENV_FILE" ]; then
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +a
  fi
fi

if [ -z "${DIALTONE_ENV:-}" ]; then
  echo "Error: DIALTONE_ENV is not set."
  echo "Set it in env/.env or pass an install path:"
  echo "  ./dialtone.sh install /path/to/env"
  exit 1
fi

if [[ "$DIALTONE_ENV" == "~"* ]]; then
  DIALTONE_ENV="${DIALTONE_ENV/#\~/$HOME}"
fi

mkdir -p "$DIALTONE_ENV"

GO_VERSION="$(grep "^go " "$REPO_ROOT/go.mod" | awk '{print $2}')"
OS="$(uname | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then ARCH="arm64"; fi

if ! command -v gcc >/dev/null 2>&1 && ! command -v clang >/dev/null 2>&1; then
  echo "WARNING: no C compiler (gcc/clang) found."
  echo "Some CGO-based builds may fail until a compiler is installed."
fi

GO_DIR="$DIALTONE_ENV/go"
GO_BIN="$GO_DIR/bin/go"
if [ -x "$GO_BIN" ]; then
  echo "Go $GO_VERSION already installed at $GO_BIN"
  exit 0
fi

TARBALL="go${GO_VERSION}.${OS}-${ARCH}.tar.gz"
TAR_PATH="$DIALTONE_ENV/$TARBALL"
URL="https://go.dev/dl/$TARBALL"

echo "Installing Go $GO_VERSION to $GO_DIR"
echo "Downloading: $URL"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$TAR_PATH" "$URL"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$TAR_PATH" "$URL"
else
  echo "Error: need curl or wget to download Go"
  exit 1
fi

rm -rf "$GO_DIR"
tar -C "$DIALTONE_ENV" -xzf "$TAR_PATH"
rm -f "$TAR_PATH"

echo "Go installed: $GO_BIN"
