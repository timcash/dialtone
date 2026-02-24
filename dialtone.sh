#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export DIALTONE_REPO_ROOT="$SCRIPT_DIR"
export DIALTONE_SRC_ROOT="$SCRIPT_DIR/src"

# 1. Load Environment
ENV_FILE="$SCRIPT_DIR/env/.env"
if [ -z "${DIALTONE_ENV_FILE:-}" ]; then
    export DIALTONE_ENV_FILE="$ENV_FILE"
fi
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
BUN_BIN="$DIALTONE_ENV/bun/bin/bun"

# Optional global log mirror: pass --stdout anywhere to mirror logs to stdout
PASSTHRU_ARGS=()
for arg in "$@"; do
    if [ "$arg" = "--stdout" ]; then
        export DIALTONE_LOG_STDOUT=1
        continue
    fi
    PASSTHRU_ARGS+=("$arg")
done

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
if [ -x "$BUN_BIN" ]; then
    export PATH="$DIALTONE_ENV/go/bin:$DIALTONE_ENV/bun/bin:$PATH"
else
    export PATH="$DIALTONE_ENV/go/bin:$PATH"
fi
export DIALTONE_GO_BIN="$GO_BIN"
if [ -x "$BUN_BIN" ]; then
    export DIALTONE_BUN_BIN="$BUN_BIN"
fi

# 4. Hand over to Go-based orchestrator
# Current working directory should be 'src' for Go imports to work correctly.
cd "$DIALTONE_SRC_ROOT"
exec "$GO_BIN" run dev.go "${PASSTHRU_ARGS[@]}"
