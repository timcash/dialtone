#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 1. Load Environment
ENV_FILE="$SCRIPT_DIR/env/.env"
if [ -f "$ENV_FILE" ]; then
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +a
fi

# Default DIALTONE_ENV if not set
if [ -z "${DIALTONE_ENV:-}" ]; then
    DIALTONE_ENV="$HOME/.dialtone_env"
fi

if [[ "$DIALTONE_ENV" == "~"* ]]; then
    DIALTONE_ENV="${DIALTONE_ENV/#\~/$HOME}"
fi

GO_BIN="$DIALTONE_ENV/go/bin/go"

# 2. Check for Go
if [ ! -x "$GO_BIN" ]; then
    echo "DIALTONE> Go runtime missing at $DIALTONE_ENV/go"
    printf "DIALTONE> Would you like to install it? [y/N] "
    read -r confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
        echo "DIALTONE> Installing Go..."
        # Use absolute path to installer
        bash "$SCRIPT_DIR/src/plugins/go/install.sh" "$DIALTONE_ENV"
    else
        echo "DIALTONE> Go is required. Exiting."
        exit 1
    fi
fi

# 3. Setup PATH and GOROOT
export GOROOT="$DIALTONE_ENV/go"
export PATH="$DIALTONE_ENV/go/bin:$PATH"

# 4. Hand over to Go-based orchestrator
# Current working directory should be 'src' for Go imports to work correctly.
cd "$SCRIPT_DIR/src"
exec go run dev.go "$@"
