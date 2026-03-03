#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export DIALTONE_REPO_ROOT="$SCRIPT_DIR"
export DIALTONE_SRC_ROOT="$SCRIPT_DIR/src"
export DIALTONE_USE_NIX="${DIALTONE_USE_NIX:-1}"
NIX_EXPERIMENTAL_FLAGS=(--extra-experimental-features "nix-command flakes")

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

expand_home_path() {
    local p="$1"
    if [[ "$p" == "~"* ]]; then
        p="${p/#\~/$HOME}"
    fi
    printf "%s" "$p"
}

resolve_go_version() {
    if command_exists curl; then
        curl -fsSL https://go.dev/VERSION?m=text | awk 'NR==1{gsub(/^go/, "", $1); print $1}'
        return 0
    fi
    if command_exists wget; then
        wget -qO- https://go.dev/VERSION?m=text | awk 'NR==1{gsub(/^go/, "", $1); print $1}'
        return 0
    fi
    return 1
}

ensure_nix_installed() {
    if command_exists nix; then
        return 0
    fi
    echo "DIALTONE> Nix is required but not found. Please install Nix first."
    exit 1
}

enter_nix_shell_if_needed() {
    if [ "${DIALTONE_USE_NIX:-1}" != "1" ]; then
        return 0
    fi
    if [ -n "${IN_NIX_SHELL:-}" ] || [ "${DIALTONE_NIX_SHELL_BOOTSTRAPPED:-}" = "1" ]; then
        return 0
    fi
    if [ ! -f "$SCRIPT_DIR/flake.nix" ]; then
        return 0
    fi
    ensure_nix_installed
    echo "DIALTONE> Entering Nix dev shell..."
    exec nix "${NIX_EXPERIMENTAL_FLAGS[@]}" develop --command env DIALTONE_NIX_SHELL_BOOTSTRAPPED=1 "$SCRIPT_DIR/dialtone.sh" "$@"
}

bootstrap_clone_repo_in_place() {
    local repo_url="$1"
    local branch="$2"
    local backup="${SCRIPT_DIR}/dialtone.sh.back"

    if ! command_exists git; then
        echo "DIALTONE> Git is required to bootstrap the repository."
        return 1
    fi

    if [ -f "${SCRIPT_DIR}/dialtone.sh" ]; then
        cp "${SCRIPT_DIR}/dialtone.sh" "$backup"
        echo "DIALTONE> Backed up launcher: $backup"
    fi

    if [ ! -d "${SCRIPT_DIR}/.git" ]; then
        git -C "$SCRIPT_DIR" init >/dev/null
        git -C "$SCRIPT_DIR" remote add origin "$repo_url"
    else
        if ! git -C "$SCRIPT_DIR" remote get-url origin >/dev/null 2>&1; then
            git -C "$SCRIPT_DIR" remote add origin "$repo_url"
        fi
    fi

    git -C "$SCRIPT_DIR" fetch --depth 1 origin "$branch"
    git -C "$SCRIPT_DIR" checkout -f -B "$branch" FETCH_HEAD
    return 0
}

write_env_file() {
    local env_path="$1"
    local dialtone_env="$2"
    mkdir -p "$(dirname "$env_path")"
    cat >"$env_path" <<EOF
DIALTONE_ENV=$dialtone_env
DIALTONE_USE_NIX=1
EOF
}

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
if [ ! -f "$ENV_FILE" ] && [ -z "${DIALTONE_BOOTSTRAP_DONE:-}" ]; then
    echo "DIALTONE> Environment file missing ($ENV_FILE). Cannot continue."
    exit 1
fi
enter_nix_shell_if_needed "$@"

# Default DIALTONE_ENV if not set
if [ -z "${DIALTONE_ENV:-}" ]; then
    DIALTONE_ENV="$SCRIPT_DIR/.dialtone_env"
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
if [ ! -x "$GO_BIN" ] && command_exists go; then
    GO_BIN="$(command -v go)"
fi
if [ ! -x "$GO_BIN" ]; then
    echo "DIALTONE> Go runtime missing and not provided by Nix shell."
    echo "DIALTONE> Run with Nix enabled (DIALTONE_USE_NIX=1) or install managed Go."
    exit 1
fi

# 3. Setup PATH and GOROOT
if [ -x "$DIALTONE_ENV/go/bin/go" ]; then
    export GOROOT="$DIALTONE_ENV/go"
fi
if [ -x "$BUN_BIN" ]; then
    export PATH="$DIALTONE_ENV/go/bin:$DIALTONE_ENV/bun/bin:$PATH"
else
    if [ -x "$DIALTONE_ENV/go/bin/go" ]; then
        export PATH="$DIALTONE_ENV/go/bin:$PATH"
    fi
fi
export DIALTONE_GO_BIN="$GO_BIN"
if [ -x "$BUN_BIN" ]; then
    export DIALTONE_BUN_BIN="$BUN_BIN"
fi

# 4. Hand over to Go-based orchestrator
# Current working directory should be 'src' for Go imports to work correctly.
cd "$DIALTONE_SRC_ROOT"
exec "$GO_BIN" run dev.go "${PASSTHRU_ARGS[@]}"
