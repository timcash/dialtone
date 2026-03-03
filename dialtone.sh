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
    if [ ! -t 0 ]; then
        echo "DIALTONE> Nix is required but not found, and shell is non-interactive."
        exit 1
    fi
    echo "DIALTONE> Nix is required for bootstrap."
    printf "DIALTONE> Install Nix now? [Y/n] "
    read -r confirm
    if [[ -n "$confirm" && ! "$confirm" =~ ^[Yy]$ ]]; then
        echo "DIALTONE> Nix install declined. Exiting."
        exit 1
    fi
    if command_exists curl; then
        sh <(curl -L https://nixos.org/nix/install) --daemon
    elif command_exists wget; then
        sh <(wget -qO- https://nixos.org/nix/install) --daemon
    else
        echo "DIALTONE> Need curl or wget to install Nix."
        exit 1
    fi
    if [ -f "$HOME/.nix-profile/etc/profile.d/nix.sh" ]; then
        # shellcheck disable=SC1090
        . "$HOME/.nix-profile/etc/profile.d/nix.sh"
    fi
    if [ -f "/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh" ]; then
        # shellcheck disable=SC1091
        . "/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh"
    fi
    if ! command_exists nix; then
        echo "DIALTONE> Nix install finished but nix is not on PATH yet."
        echo "DIALTONE> Open a new shell and rerun ./dialtone.sh"
        exit 1
    fi
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
    exec nix "${NIX_EXPERIMENTAL_FLAGS[@]}" develop "path:$SCRIPT_DIR" --command env DIALTONE_NIX_SHELL_BOOTSTRAPPED=1 "$SCRIPT_DIR/dialtone.sh" "$@"
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

run_bootstrap_repl() {
    local env_file="$1"
    local default_env="${SCRIPT_DIR}/.dialtone_env"
    local default_repo="https://github.com/timcash/dialtone.git"
    local default_branch="main"
    local input_env input_repo input_branch

    echo "DIALTONE> Bootstrap REPL started."
    echo "DIALTONE> This will configure env/.env, set up Nix shell, and bootstrap the dialtone repo."

    while true; do
        printf "DIALTONE> Install directory for Go/Bun [default: %s]: " "$default_env"
        read -r input_env
        input_env="$(expand_home_path "${input_env:-$default_env}")"
        if [ -n "$input_env" ]; then
            break
        fi
    done
    mkdir -p "$input_env"
    export DIALTONE_ENV="$input_env"

    write_env_file "$env_file" "$DIALTONE_ENV"
    echo "DIALTONE> Wrote $env_file"

    ensure_nix_installed
    if [ -z "${IN_NIX_SHELL:-}" ]; then
        echo "DIALTONE> Re-entering bootstrap inside Nix shell..."
        export DIALTONE_BOOTSTRAP_DONE=1
        exec nix "${NIX_EXPERIMENTAL_FLAGS[@]}" develop "path:$SCRIPT_DIR" --command env DIALTONE_NIX_SHELL_BOOTSTRAPPED=1 DIALTONE_BOOTSTRAP_DONE=1 "$SCRIPT_DIR/dialtone.sh" "$@"
    fi
    if ! command_exists git || ! command_exists go; then
        echo "DIALTONE> Nix shell does not have git/go available. Check flake.nix."
        exit 1
    fi

    printf "DIALTONE> Git repo to bootstrap [default: %s]: " "$default_repo"
    read -r input_repo
    input_repo="${input_repo:-$default_repo}"
    printf "DIALTONE> Branch [default: %s]: " "$default_branch"
    read -r input_branch
    input_branch="${input_branch:-$default_branch}"

    echo "DIALTONE> Bootstrapping repo in $SCRIPT_DIR ..."
    bootstrap_clone_repo_in_place "$input_repo" "$input_branch"
    echo "DIALTONE> Repo bootstrap complete."
    echo "DIALTONE> Launching new dialtone runtime..."

    export DIALTONE_BOOTSTRAP_DONE=1
    exec "${SCRIPT_DIR}/dialtone.sh" "$@"
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
    if [ -t 0 ]; then
        run_bootstrap_repl "$ENV_FILE" "$@"
    else
        echo "DIALTONE> Environment file missing ($ENV_FILE) and shell is non-interactive. Cannot bootstrap."
        exit 1
    fi
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
