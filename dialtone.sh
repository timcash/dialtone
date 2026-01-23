#!/bin/bash
set -e

# 1. Resolve DIALTONE_ENV from args
for i in "$@"; do
    case $i in
        --env=*)
            DIALTONE_ENV="${i#*=}"
            shift
            ;;
        --env)
            DIALTONE_ENV="$2"
            shift 2
            ;;
    esac
done

# 2. Resolve DIALTONE_ENV from .env if not set by arg
if [ -z "$DIALTONE_ENV" ] && [ -f .env ]; then
    DIALTONE_ENV=$(grep "^DIALTONE_ENV=" .env | cut -d '=' -f2)
fi

# 3. Warn if DIALTONE_ENV is missing
if [ -z "$DIALTONE_ENV" ]; then
    echo "WARNING: DIALTONE_ENV not found in args or .env."
    echo "Please add DIALTONE_ENV=/path/to/env to your .env file."
fi

# 4. If DIALTONE_ENV found, handle Go installation
if [ -n "$DIALTONE_ENV" ]; then
    GO_BIN="$DIALTONE_ENV/go/bin/go"
    
    # Check if golang is installed in that folder
    if [ ! -f "$GO_BIN" ]; then
        echo "Go not found in $DIALTONE_ENV/go. Installing..."
        GO_VERSION="1.25.5"
        mkdir -p "$DIALTONE_ENV"
        TAR_FILE="go$GO_VERSION.linux-amd64.tar.gz"
        curl -LO "https://go.dev/dl/$TAR_FILE"
        tar -C "$DIALTONE_ENV" -xzf "$TAR_FILE"
        rm "$TAR_FILE"
    fi
    
    # Update PATH to use the environment's Go
    export PATH="$DIALTONE_ENV/go/bin:$PATH"
fi

# 5. Run the dialtone-dev tool
exec go run src/cmd/dev/main.go "$@"

