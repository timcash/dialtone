#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

usage() {
  cat <<EOF
Usage: ./dialtone.sh install [path]
       ./dialtone.sh install --latest [path]

Installs the Go toolchain into DIALTONE_ENV/go.

Arguments:
  [path]    Optional install root. Overrides DIALTONE_ENV from env/.env.
Flags:
  --latest  Install the latest stable Go version from go.dev.
EOF
}

INSTALL_LATEST=0
POSITIONAL_ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --latest)
      INSTALL_LATEST=1
      shift
      ;;
    help|--help|-h)
      usage
      exit 0
      ;;
    *)
      POSITIONAL_ARGS+=("$1")
      shift
      ;;
  esac
done

if [ -n "${POSITIONAL_ARGS[0]:-}" ]; then
  export DIALTONE_ENV="${POSITIONAL_ARGS[0]}"
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

if [ "$INSTALL_LATEST" -eq 1 ]; then
  if command -v curl >/dev/null 2>&1; then
    GO_VERSION="$(curl -fsSL https://go.dev/VERSION?m=text | awk 'NR==1{gsub(/^go/, "", $1); print $1}')"
  elif command -v wget >/dev/null 2>&1; then
    GO_VERSION="$(wget -qO- https://go.dev/VERSION?m=text | awk 'NR==1{gsub(/^go/, "", $1); print $1}')"
  else
    echo "Error: need curl or wget to resolve latest Go version"
    exit 1
  fi
else
  GO_VERSION="$(grep "^go " "$REPO_ROOT/go.mod" | awk '{print $2}')"
fi

if [ -z "${GO_VERSION:-}" ]; then
  echo "Error: failed to resolve Go version"
  exit 1
fi

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
  INSTALLED_GO_VERSION="$("$GO_BIN" version 2>/dev/null | awk '{gsub(/^go/, "", $3); print $3}')"
  if [ "$INSTALLED_GO_VERSION" = "$GO_VERSION" ]; then
    echo "Go $GO_VERSION already installed at $GO_BIN"
    exit 0
  fi
  echo "Upgrading Go from ${INSTALLED_GO_VERSION:-unknown} to $GO_VERSION"
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
