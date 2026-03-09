#!/bin/bash
set -e

# --- 1. Configuration & Defaults ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export CGO_ENABLED=0
ENV_FILE_JSON_DEFAULT="$SCRIPT_DIR/env/dialtone.json"

log_info() { echo "DIALTONE> $*"; }
log_err() { echo "DIALTONE> ERROR: $*" >&2; }

command_exists() { command -v "$1" >/dev/null 2>&1; }

expand_home_path() {
    local p="$1"
    [[ "$p" == "~"* ]] && p="${p/#\~/$HOME}"
    printf "%s" "$p"
}

# --- 2. Configuration Helpers ---
read_json_val() {
    local key="$1"
    local file="$2"
    [ ! -f "$file" ] && return
    grep -m 1 "\"$key\":" "$file" | sed -E 's/.*: *"([^"]*)".*/\1/'
}

write_json_config() {
    local env_dir="$1"
    local repo_dir="$2"
    mkdir -p "$(dirname "$ENV_FILE_JSON_DEFAULT")"
    cat > "$ENV_FILE_JSON_DEFAULT" <<EOF
{
  "DIALTONE_ENV": "$env_dir",
  "DIALTONE_REPO_ROOT": "$repo_dir",
  "DIALTONE_USE_NIX": "0"
}
EOF
}

# --- 3. Installation Helpers ---
install_go() {
    local target_dir="$1"
    local version="1.24.0"
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
    
    local saved_script=""
    if [ -f "$target_root/dialtone.sh" ]; then
        saved_script=$(cat "$target_root/dialtone.sh")
    fi

    if [ "$target_root" = "$SCRIPT_DIR" ]; then
        mv "$deps_dir/repo_tmp/src" "$target_root/" || true
        mv "$deps_dir/repo_tmp/flake.nix" "$target_root/" || true
        cp -rn "$deps_dir/repo_tmp/"* "$target_root/" || true
    else
        mv "$deps_dir/repo_tmp/"* "$target_root/"
    fi
    rm -rf "$deps_dir/repo_tmp"

    if [ -n "$saved_script" ] || [ ! -f "$target_root/dialtone.sh" ] || [ "$target_root" != "$SCRIPT_DIR" ]; then
        cp "$0" "$target_root/dialtone.sh"
    fi
    log_info "Repo bootstrap complete."
}

run_onboarding() {
    local is_test="$1"
    log_info "Welcome to Dialtone! Let's get you set up."
    
    DEFAULT_ENV="$HOME/.dialtone_env"
    if [ "$is_test" = "1" ]; then
        input_env="${TEST_ANS_ENV:-$DEFAULT_ENV}"
        log_info "Where should dependencies (Go/Bun) be installed? [$DEFAULT_ENV]: $input_env (Auto)"
    else
        printf "DIALTONE> Where should dependencies (Go/Bun) be installed? [%s]: " "$DEFAULT_ENV"
        read -r input_env
    fi
    input_env="$(expand_home_path "${input_env:-$DEFAULT_ENV}")"

    DEFAULT_REPO="$SCRIPT_DIR"
    if [ "$is_test" = "1" ]; then
        input_repo="${TEST_ANS_REPO:-$DEFAULT_REPO}"
        log_info "Where is the repository root? [$DEFAULT_REPO]: $input_repo (Auto)"
    else
        printf "DIALTONE> Where is the repository root? [%s]: " "$DEFAULT_REPO"
        read -r input_repo
    fi
    input_repo="$(expand_home_path "${input_repo:-$DEFAULT_REPO}")"

    export DIALTONE_ENV="$input_env"
    export DIALTONE_REPO_ROOT="$input_repo"
    
    write_json_config "$input_env" "$input_repo"
    log_info "Configuration saved to $ENV_FILE_JSON_DEFAULT"
}

# --- 4. Argument Parsing ---
PASSTHRU_ARGS=()
ENV_OVERRIDE=""
FORCE_NO_NIX=0
IS_TEST=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --env) ENV_OVERRIDE="$2"; shift 2 ;;
        --no-nix) FORCE_NO_NIX=1; shift ;;
        --test)
            IS_TEST=1
            PASSTHRU_ARGS+=("--test")
            shift
            ;;
        --stdout) export DIALTONE_LOG_STDOUT=1; shift ;;
        *) PASSTHRU_ARGS+=("$1"); shift ;;
    esac
done

DEFAULT_CMD_NEEDED=1
for arg in "${PASSTHRU_ARGS[@]}"; do
    [[ "$arg" != -* ]] && DEFAULT_CMD_NEEDED=0 && break
done

if [ "$DEFAULT_CMD_NEEDED" = "1" ]; then
    PASSTHRU_ARGS=("repl" "src_v2" "run" "${PASSTHRU_ARGS[@]}")
fi

# --- 5. Environment Loading ---
ENV_FILE_JSON="$ENV_FILE_JSON_DEFAULT"
if [ -n "$ENV_OVERRIDE" ]; then
    ENV_FILE_JSON="$(expand_home_path "$ENV_OVERRIDE")"
    log_info "Using custom environment (JSON): $ENV_FILE_JSON"
fi

if [ ! -f "$ENV_FILE_JSON" ] && [ -z "$DIALTONE_ONBOARDING_DONE" ]; then
    if [ -z "$ENV_OVERRIDE" ]; then
        run_onboarding "$IS_TEST"
    fi
fi

if [ -f "$ENV_FILE_JSON" ]; then
    export DIALTONE_ENV="${DIALTONE_ENV:-$(read_json_val "DIALTONE_ENV" "$ENV_FILE_JSON")}"
    export DIALTONE_REPO_ROOT="${DIALTONE_REPO_ROOT:-$(read_json_val "DIALTONE_REPO_ROOT" "$ENV_FILE_JSON")}"
    export DIALTONE_USE_NIX="${DIALTONE_USE_NIX:-$(read_json_val "DIALTONE_USE_NIX" "$ENV_FILE_JSON")}"
fi

[ "$FORCE_NO_NIX" = "1" ] && export DIALTONE_USE_NIX=0
[ -z "$DIALTONE_USE_NIX" ] && export DIALTONE_USE_NIX=1

export DIALTONE_REPO_ROOT="$(expand_home_path "$DIALTONE_REPO_ROOT")"
export DIALTONE_SRC_ROOT="${DIALTONE_REPO_ROOT}/src"
export DIALTONE_ENV="$(expand_home_path "$DIALTONE_ENV")"
export DIALTONE_ENV_FILE="$ENV_FILE_JSON"
export DIALTONE_MESH_CONFIG="$ENV_FILE_JSON"

# --- 6. Guided Dependency Checks ---
log_info "Verifying dependencies..."

if [ "$SCRIPT_DIR" != "$DIALTONE_REPO_ROOT" ] && [ -f "$DIALTONE_REPO_ROOT/dialtone.sh" ] && [ -z "$DIALTONE_TRANSFERRED" ]; then
    log_info "Transferring execution to $DIALTONE_REPO_ROOT/dialtone.sh"
    exec env DIALTONE_TRANSFERRED=1 "$DIALTONE_REPO_ROOT/dialtone.sh" --env "$ENV_FILE_JSON" "${PASSTHRU_ARGS[@]}"
fi

if [ ! -d "$DIALTONE_SRC_ROOT" ]; then
    bootstrap_repo "$DIALTONE_ENV" "$DIALTONE_REPO_ROOT"
    if [ -z "$DIALTONE_ONBOARDING_DONE" ]; then
        exec env DIALTONE_ONBOARDING_DONE=1 "$DIALTONE_REPO_ROOT/dialtone.sh" --env "$ENV_FILE_JSON" "${PASSTHRU_ARGS[@]}"
    fi
fi
# Go Installation
GO_BIN="$DIALTONE_ENV/go/bin/go"
if [ ! -x "$GO_BIN" ]; then
    if command_exists go && [ "$DIALTONE_USE_NIX" != "0" ]; then
        GO_BIN="$(command -v go)"
        log_info "Using system Go: $GO_BIN"
    else
        log_info "Go runtime missing. Installing to ${DIALTONE_ENV}..."
        install_go "$DIALTONE_ENV"
        GO_BIN="$DIALTONE_ENV/go/bin/go"
    fi
else
    log_info "Using managed Go (Cached): $GO_BIN"
fi


export GOROOT=""
[[ "$GO_BIN" == "$DIALTONE_ENV/go/bin/go" ]] && export GOROOT="$DIALTONE_ENV/go"
export PATH="$DIALTONE_ENV/go/bin:$DIALTONE_ENV/bun/bin:$PATH"
export DIALTONE_GO_BIN="$GO_BIN"

# --- 7. Hand over to Go orchestrator ---
log_info "Environment ready. Launching Dialtone..."
cd "$DIALTONE_SRC_ROOT"
exec "$GO_BIN" run dev.go "${PASSTHRU_ARGS[@]}"
