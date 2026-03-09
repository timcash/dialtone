#!/bin/bash
set -e

# --- 1. Configuration & Defaults ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export CGO_ENABLED=0

log_info() { echo "DIALTONE> $*"; }
log_err() { echo "DIALTONE> ERROR: $*" >&2; }

command_exists() { command -v "$1" >/dev/null 2>&1; }

expand_home_path() {
    local p="$1"
    [[ "$p" == "~"* ]] && p="${p/#\~/$HOME}"
    printf "%s" "$p"
}

# --- 2. Installation Helpers ---
install_go() {
    local target_dir="$1"
    local version="1.24.0" # Stable fallback version
    local os="linux"
    local arch="amd64"

    [[ "$(uname)" == "Darwin" ]] && os="darwin"
    [[ "$(uname -m)" == "arm64" || "$(uname -m)" == "aarch64" ]] && arch="arm64"

    local url="https://go.dev/dl/go${version}.${os}-${arch}.tar.gz"
    log_info "Downloading Go ${version} to ${target_dir}..."
    mkdir -p "$target_dir"
    curl -L "$url" | tar -xz -C "$target_dir"
}

bootstrap_repo() {
    local deps_dir="$1"
    local target_root="$2"
    local url="https://github.com/timcash/dialtone/archive/refs/heads/main.tar.gz"
    log_info "Bootstrapping repo into $target_root from $url..."
    mkdir -p "$deps_dir/repo_tmp"
    curl -L "$url" | tar -xz -C "$deps_dir/repo_tmp" --strip-components=1
    mkdir -p "$target_root"
    mv "$deps_dir/repo_tmp/"* "$target_root/"
    rm -rf "$deps_dir/repo_tmp"
    log_info "Repo bootstrap complete."
}

# --- 3. Argument Parsing ---
PASSTHRU_ARGS=()
ENV_OVERRIDE=""
FORCE_NO_NIX=0
IS_TEST=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --env) ENV_OVERRIDE="$2"; shift 2 ;;
        --no-nix) FORCE_NO_NIX=1; shift ;;
        --test) IS_TEST=1; shift ;;
        --stdout) export DIALTONE_LOG_STDOUT=1; shift ;;
        *) PASSTHRU_ARGS+=("$1"); shift ;;
    esac
done

if [ ${#PASSTHRU_ARGS[@]} -eq 0 ]; then
    PASSTHRU_ARGS=("repl" "src_v2" "run")
    [ "$IS_TEST" = "1" ] && PASSTHRU_ARGS+=("--test")
fi

[ "$FORCE_NO_NIX" = "1" ] && export DIALTONE_USE_NIX=0
export DIALTONE_USE_NIX="${DIALTONE_USE_NIX:-1}"

# --- 4. Load Environment ---
if [ -n "$ENV_OVERRIDE" ]; then
    ENV_FILE="$(expand_home_path "$ENV_OVERRIDE")"
    log_info "Using custom environment: $ENV_FILE"
else
    ENV_FILE="$SCRIPT_DIR/env/.env"
fi

export DIALTONE_ENV_FILE="$ENV_FILE"

if [ -f "$ENV_FILE" ]; then
    set -a; source "$ENV_FILE"; set +a
fi

# Set roots (respecting overrides from env)
export DIALTONE_REPO_ROOT="${DIALTONE_REPO_ROOT:-$SCRIPT_DIR}"
export DIALTONE_SRC_ROOT="${DIALTONE_REPO_ROOT}/src"
DIALTONE_ENV="$(expand_home_path "${DIALTONE_ENV:-$SCRIPT_DIR/.dialtone_env}")"

# --- 5. Guided Dependency Checks ---
log_info "Verifying dependencies..."

# Nix Check
if [ "$DIALTONE_USE_NIX" = "1" ] && [ -z "${IN_NIX_SHELL:-}" ] && [ -z "${DIALTONE_NIX_SHELL_BOOTSTRAPPED:-}" ]; then
    if [ -f "$DIALTONE_REPO_ROOT/flake.nix" ] && command_exists nix; then
        log_info "Nix found. Entering dev shell..."
        exec nix --extra-experimental-features "nix-command flakes" develop --command env DIALTONE_NIX_SHELL_BOOTSTRAPPED=1 "$SCRIPT_DIR/dialtone.sh" "${PASSTHRU_ARGS[@]}"
    fi
    log_info "Nix bypassed or unavailable."
fi

# Go/Bun Discovery & Install
GO_BIN="$DIALTONE_ENV/go/bin/go"
if [ ! -x "$GO_BIN" ]; then
    if command_exists go && [ "$FORCE_NO_NIX" != "1" ]; then
        GO_BIN="$(command -v go)"
        log_info "Using system Go: $GO_BIN"
    else
        log_info "Go runtime missing. Installing to ${DIALTONE_ENV}..."
        install_go "$DIALTONE_ENV"
        GO_BIN="$DIALTONE_ENV/go/bin/go"
    fi
else
    log_info "Using managed Go: $GO_BIN"
fi

# Repo Bootstrap Check
if [ ! -d "$DIALTONE_SRC_ROOT" ] && [ ! -d "$DIALTONE_REPO_ROOT/.git" ]; then
    bootstrap_repo "$DIALTONE_ENV" "$DIALTONE_REPO_ROOT"
fi

# Setup PATH
export GOROOT=""
[[ "$GO_BIN" == "$DIALTONE_ENV/go/bin/go" ]] && export GOROOT="$DIALTONE_ENV/go"
export PATH="$DIALTONE_ENV/go/bin:$DIALTONE_ENV/bun/bin:$PATH"
export DIALTONE_GO_BIN="$GO_BIN"

# --- 6. Hand over to Go orchestrator ---
log_info "Environment ready. Launching Dialtone..."
cd "$DIALTONE_SRC_ROOT"
exec "$GO_BIN" run dev.go "${PASSTHRU_ARGS[@]}"
