#!/bin/bash
set -e

# --- 1. Configuration & Defaults ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LAUNCH_DIR="$(pwd -P)"
export CGO_ENABLED=0
ENV_FILE_JSON_DEFAULT="$LAUNCH_DIR/env/dialtone.json"

log_info() {
    if [ "${DIALTONE_CONTEXT:-}" = "repl" ]; then
        echo "$*"
        return
    fi
    echo "dialtone> $*"
}
log_err() {
    if [ "${DIALTONE_CONTEXT:-}" = "repl" ]; then
        echo "ERROR: $*" >&2
        return
    fi
    echo "dialtone> ERROR: $*" >&2
}

command_exists() { command -v "$1" >/dev/null 2>&1; }

should_quiet_bootstrap() {
    local cmd="$1"
    case "$cmd" in
        ""|"help"|"-h"|"--help"|"exit"|"branch"|"plugins"|"dev"|"repl")
            return 1
            ;;
        *)
            return 0
            ;;
    esac
}

expand_home_path() {
    local p="$1"
    [[ "$p" == "~"* ]] && p="${p/#\~/$HOME}"
    printf "%s" "$p"
}

resolve_path_from_launch_dir() {
    local raw="$1"
    raw="$(expand_home_path "$raw")"
    case "$raw" in
        "")
            printf "%s" "$LAUNCH_DIR"
            ;;
        /*)
            printf "%s" "$raw"
            ;;
        *)
            printf "%s" "$LAUNCH_DIR/$raw"
            ;;
    esac
}

resolve_env_json_path() {
    local raw="$1"
    local resolved=""
    if [ -n "$raw" ]; then
        resolved="$(resolve_path_from_launch_dir "$raw")"
    else
        resolved="$ENV_FILE_JSON_DEFAULT"
    fi
    if [ "$resolved" != "/" ]; then
        resolved="${resolved%/}"
    fi
    if [ -d "$resolved" ]; then
        printf "%s/env/dialtone.json" "$resolved"
        return
    fi
    case "$resolved" in
        *.json)
            printf "%s" "$resolved"
            ;;
        */env)
            printf "%s/dialtone.json" "$resolved"
            ;;
        *)
            printf "%s/env/dialtone.json" "$resolved"
            ;;
    esac
}

resolve_config_root_from_env_file() {
    local env_file="$1"
    local env_dir=""
    if [ -z "$env_file" ]; then
        printf "%s" "$LAUNCH_DIR"
        return
    fi
    env_dir="$(dirname "$env_file")"
    if [ "$(basename "$env_dir")" = "env" ]; then
        dirname "$env_dir"
        return
    fi
    printf "%s" "$env_dir"
}

# --- 2. Configuration Helpers ---
read_json_val() {
    local key="$1"
    local file="$2"
    [ ! -f "$file" ] && return
    grep -m 1 "\"$key\":" "$file" | sed -E 's/.*: *"([^"]*)".*/\1/'
}

write_json_config() {
    local home_dir="$1"
    local env_dir="$2"
    local repo_dir="$3"
    local config_path="$ENV_FILE_JSON"
    mkdir -p "$(dirname "$config_path")"
    cat > "$config_path" <<EOF
{
  "DIALTONE_HOME": "$home_dir",
  "DIALTONE_ENV": "$env_dir",
  "DIALTONE_GO_CACHE_DIR": "$env_dir/cache/go",
  "DIALTONE_BUN_CACHE_DIR": "$env_dir/cache/bun",
  "DIALTONE_REPO_ROOT": "$repo_dir",
  "DIALTONE_USE_NIX": "0"
}
EOF
    local key value tmp_file
    for key in \
        CLOUDFLARE_API_TOKEN \
        CLOUDFLARE_ACCOUNT_ID \
        CF_TUNNEL_TOKEN_SHELL \
        DIALTONE_DOMAIN \
        DIALTONE_HOSTNAME \
        TS_AUTHKEY \
        TS_API_KEY \
        TS_TAILNET
    do
        value="${!key:-}"
        if [ -n "$value" ]; then
            tmp_file="$(mktemp)"
            python3 - "$config_path" "$tmp_file" "$key" "$value" <<'PY'
import json
import sys

src, dst, key, value = sys.argv[1:5]
with open(src, "r", encoding="utf-8") as f:
    doc = json.load(f)
doc[key] = value
with open(dst, "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
PY
            mv "$tmp_file" "$config_path"
        fi
    done
}

# --- 3. Installation Helpers ---
install_go() {
    local target_dir="$1"
    local version="1.25.5"
    local os="linux"
    local arch="amd64"
    [[ "$(uname)" == "Darwin" ]] && os="darwin"
    [[ "$(uname -m)" == "arm64" || "$(uname -m)" == "aarch64" ]] && arch="arm64"

    local url="https://go.dev/dl/go${version}.${os}-${arch}.tar.gz"
    local tarball="go${version}.${os}-${arch}.tar.gz"
    local cache_dir="${DIALTONE_GO_CACHE_DIR:-$target_dir/cache/go}"
    local cache_tar="${cache_dir}/${tarball}"
    mkdir -p "$target_dir"
    mkdir -p "$cache_dir"
    if [ -f "$cache_tar" ]; then
        log_info "Using cached Go ${version} tarball from ${cache_tar}..."
    else
        log_info "Downloading Go ${version} to shared cache ${cache_tar}..."
        curl -L "$url" -o "$cache_tar"
    fi
    log_info "Installing Go ${version} into ${target_dir}..."
    tar -xzf "$cache_tar" -C "$target_dir"
}

bootstrap_repo() {
    local deps_dir="$1"
    local target_root="$2"
    local method="${DIALTONE_BOOTSTRAP_METHOD:-tar}"
    if [ "$method" = "git-go" ] || [ "$method" = "git" ]; then
        bootstrap_repo_git_go "$deps_dir" "$target_root"
        return
    fi
    local url="${DIALTONE_BOOTSTRAP_REPO_URL:-https://github.com/timcash/dialtone/archive/refs/heads/main.tar.gz}"
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
        cp -f "$0" "$target_root/dialtone.sh" 2>/dev/null || true
    fi
    log_info "Repo bootstrap complete."
}

bootstrap_repo_git_go() {
    local deps_dir="$1"
    local target_root="$2"
    local git_url="${DIALTONE_BOOTSTRAP_GIT_URL:-https://github.com/timcash/dialtone.git}"
    local git_branch="${DIALTONE_BOOTSTRAP_GIT_BRANCH:-main}"
    local git_depth="${DIALTONE_BOOTSTRAP_GIT_DEPTH:-1}"
    local go_bin="${DIALTONE_GO_BIN:-$DIALTONE_ENV/go/bin/go}"

    if [ ! -x "$go_bin" ]; then
        if command_exists go; then
            go_bin="$(command -v go)"
        else
            log_info "Go runtime missing. Installing to ${DIALTONE_ENV} for git-go bootstrap..."
            install_go "$DIALTONE_ENV"
            go_bin="$DIALTONE_ENV/go/bin/go"
        fi
    fi

    log_info "Bootstrapping repo via go-git into $target_root from $git_url (branch=$git_branch depth=$git_depth)..."
    local work="$deps_dir/git_bootstrap_tmp"
    local clone_dir="$work/clone"
    local tool_dir="$work/tool"
    rm -rf "$work"
    mkdir -p "$clone_dir" "$tool_dir"

    cat > "$tool_dir/main.go" <<'EOF'
package main

import (
	"flag"
	"log"
	"os"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func main() {
	url := flag.String("url", "", "git url")
	dest := flag.String("dest", "", "destination")
	branch := flag.String("branch", "main", "branch")
	depth := flag.Int("depth", 1, "depth")
	flag.Parse()
	if strings.TrimSpace(*url) == "" || strings.TrimSpace(*dest) == "" {
		log.Fatalf("missing required --url/--dest")
	}
	opts := &git.CloneOptions{URL: strings.TrimSpace(*url), Progress: os.Stdout}
	if strings.TrimSpace(*branch) != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(strings.TrimSpace(*branch))
		opts.SingleBranch = true
	}
	if *depth > 0 {
		opts.Depth = *depth
	}
	if _, err := git.PlainClone(strings.TrimSpace(*dest), false, opts); err != nil {
		log.Fatalf("clone failed: %v", err)
	}
}
EOF
    cat > "$tool_dir/go.mod" <<'EOF'
module dialtone-bootstrap-git

go 1.25

require github.com/go-git/go-git/v5 v5.16.3
EOF

    (cd "$tool_dir" && "$go_bin" run . --url "$git_url" --dest "$clone_dir" --branch "$git_branch" --depth "$git_depth")
    mkdir -p "$target_root"

    local saved_script=""
    if [ -f "$target_root/dialtone.sh" ]; then
        saved_script=$(cat "$target_root/dialtone.sh")
    fi

    if [ "$target_root" = "$SCRIPT_DIR" ]; then
        mv "$clone_dir/src" "$target_root/" || true
        mv "$clone_dir/flake.nix" "$target_root/" || true
        shopt -s dotglob nullglob
        for item in "$clone_dir"/*; do
            base="$(basename "$item")"
            [ "$base" = ".git" ] && continue
            [ "$base" = "src" ] && continue
            [ "$base" = "flake.nix" ] && continue
            cp -rn "$item" "$target_root/" || true
        done
        shopt -u dotglob nullglob
    else
        shopt -s dotglob nullglob
        for item in "$clone_dir"/*; do
            mv "$item" "$target_root/" || true
        done
        shopt -u dotglob nullglob
    fi
    rm -rf "$work"

    if [ -n "$saved_script" ] || [ ! -f "$target_root/dialtone.sh" ] || [ "$target_root" != "$SCRIPT_DIR" ]; then
        cp -f "$0" "$target_root/dialtone.sh" 2>/dev/null || true
    fi
    log_info "Repo bootstrap complete (method=git-go)."
}

run_onboarding() {
    local is_test="$1"
    local config_root=""
    log_info "Welcome to Dialtone! Let's get you set up."
    
    config_root="$(resolve_config_root_from_env_file "$ENV_FILE_JSON")"
    DEFAULT_HOME="$config_root/.dialtone"
    DEFAULT_ENV="$config_root/.dialtone_env"
    if [ "$is_test" = "1" ]; then
        input_home="${TEST_ANS_HOME:-$DEFAULT_HOME}"
        log_info "Where should Dialtone runtime state live? [$DEFAULT_HOME]: $input_home (Auto)"
    else
        printf "dialtone> Where should Dialtone runtime state live? [%s]: " "$DEFAULT_HOME"
        read -r input_home
    fi
    input_home="$(expand_home_path "${input_home:-$DEFAULT_HOME}")"
    if [ "$is_test" = "1" ]; then
        input_env="${TEST_ANS_ENV:-$DEFAULT_ENV}"
        log_info "Where should dependencies (Go/Bun) be installed? [$DEFAULT_ENV]: $input_env (Auto)"
    else
        printf "dialtone> Where should dependencies (Go/Bun) be installed? [%s]: " "$DEFAULT_ENV"
        read -r input_env
    fi
    input_env="$(expand_home_path "${input_env:-$DEFAULT_ENV}")"

    DEFAULT_REPO="$SCRIPT_DIR"
    if [ "$is_test" = "1" ]; then
        input_repo="${TEST_ANS_REPO:-$DEFAULT_REPO}"
        log_info "Where is the repository root? [$DEFAULT_REPO]: $input_repo (Auto)"
    else
        printf "dialtone> Where is the repository root? [%s]: " "$DEFAULT_REPO"
        read -r input_repo
    fi
    input_repo="$(expand_home_path "${input_repo:-$DEFAULT_REPO}")"

    export DIALTONE_HOME="$input_home"
    export DIALTONE_ENV="$input_env"
    export DIALTONE_REPO_ROOT="$input_repo"
    
    write_json_config "$input_home" "$input_env" "$input_repo"
    log_info "Configuration saved to $ENV_FILE_JSON"
}

# --- 4. Argument Parsing ---
PASSTHRU_ARGS=()
ENV_OVERRIDE=""
FORCE_NO_NIX=0
IS_TEST=0
IS_LLM=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --env) ENV_OVERRIDE="$2"; shift 2 ;;
        --no-nix) FORCE_NO_NIX=1; shift ;;
        --test)
            log_err "Top-level --test is no longer supported."
            log_err "Use: ./dialtone.sh repl src_v3 test [args]"
            exit 1
            ;;
        --llm)
            IS_LLM=1
            PASSTHRU_ARGS+=("--llm")
            shift
            ;;
        --stdout) export DIALTONE_LOG_STDOUT=1; shift ;;
        *) PASSTHRU_ARGS+=("$1"); shift ;;
    esac
done

DEFAULT_CMD_NEEDED=0
if [ "${#PASSTHRU_ARGS[@]}" -eq 0 ]; then
    DEFAULT_CMD_NEEDED=1
elif [[ "${PASSTHRU_ARGS[0]}" == --* ]]; then
    DEFAULT_CMD_NEEDED=1
fi

if [ "$DEFAULT_CMD_NEEDED" = "1" ]; then
    PASSTHRU_ARGS=("repl" "src_v3" "run" "${PASSTHRU_ARGS[@]}")
fi

# --- 5. Environment Loading ---
ENV_FILE_JSON="$(resolve_env_json_path "")"
if [ -n "$ENV_OVERRIDE" ]; then
    ENV_FILE_JSON="$(resolve_env_json_path "$ENV_OVERRIDE")"
    log_info "Using custom environment (JSON): $ENV_FILE_JSON"
fi
CONFIG_ROOT="$(resolve_config_root_from_env_file "$ENV_FILE_JSON")"

if [ ! -f "$ENV_FILE_JSON" ] && [ -z "$DIALTONE_ONBOARDING_DONE" ]; then
    if [ -z "$ENV_OVERRIDE" ]; then
        if [ -n "$TEST_ANS_ENV" ] || [ -n "$TEST_ANS_REPO" ]; then
            IS_TEST=1
        fi
        run_onboarding "$IS_TEST"
    fi
fi

if [ -f "$ENV_FILE_JSON" ]; then
    export DIALTONE_HOME="${DIALTONE_HOME:-$(read_json_val "DIALTONE_HOME" "$ENV_FILE_JSON")}"
    export DIALTONE_ENV="${DIALTONE_ENV:-$(read_json_val "DIALTONE_ENV" "$ENV_FILE_JSON")}"
    export DIALTONE_REPO_ROOT="${DIALTONE_REPO_ROOT:-$(read_json_val "DIALTONE_REPO_ROOT" "$ENV_FILE_JSON")}"
    export DIALTONE_USE_NIX="${DIALTONE_USE_NIX:-$(read_json_val "DIALTONE_USE_NIX" "$ENV_FILE_JSON")}"
    export DIALTONE_GO_CACHE_DIR="${DIALTONE_GO_CACHE_DIR:-$(read_json_val "DIALTONE_GO_CACHE_DIR" "$ENV_FILE_JSON")}"
    export DIALTONE_BUN_CACHE_DIR="${DIALTONE_BUN_CACHE_DIR:-$(read_json_val "DIALTONE_BUN_CACHE_DIR" "$ENV_FILE_JSON")}"
fi

[ "$FORCE_NO_NIX" = "1" ] && export DIALTONE_USE_NIX=0
[ -z "$DIALTONE_USE_NIX" ] && export DIALTONE_USE_NIX=1

export DIALTONE_HOME="$(expand_home_path "${DIALTONE_HOME:-$CONFIG_ROOT/.dialtone}")"
export DIALTONE_ENV="$(expand_home_path "${DIALTONE_ENV:-$CONFIG_ROOT/.dialtone_env}")"
export DIALTONE_REPO_ROOT="$(expand_home_path "${DIALTONE_REPO_ROOT:-$SCRIPT_DIR}")"
export DIALTONE_SRC_ROOT="${DIALTONE_REPO_ROOT}/src"
export DIALTONE_ENV_FILE="$ENV_FILE_JSON"
export DIALTONE_MESH_CONFIG="$ENV_FILE_JSON"

QUIET_BOOTSTRAP=0
PRIMARY_CMD="${PASSTHRU_ARGS[0]:-}"
if should_quiet_bootstrap "$PRIMARY_CMD"; then
    QUIET_BOOTSTRAP=1
fi

# --- 6. Guided Dependency Checks ---
if [ "$QUIET_BOOTSTRAP" != "1" ]; then
    log_info "Verifying dependencies..."
    log_info "Bootstrap path checks:"
    [ -n "$DIALTONE_REPO_ROOT" ] && { [ -d "$DIALTONE_REPO_ROOT" ] && log_info "- repo root: $DIALTONE_REPO_ROOT (dir)" || log_info "- repo root: $DIALTONE_REPO_ROOT (missing)"; }
    [ -n "$DIALTONE_SRC_ROOT" ] && { [ -d "$DIALTONE_SRC_ROOT" ] && log_info "- src root: $DIALTONE_SRC_ROOT (dir)" || log_info "- src root: $DIALTONE_SRC_ROOT (missing)"; }
    [ -n "$DIALTONE_ENV" ] && { [ -d "$DIALTONE_ENV" ] && log_info "- env dir: $DIALTONE_ENV (dir)" || log_info "- env dir: $DIALTONE_ENV (missing)"; }
    [ -n "$ENV_FILE_JSON" ] && { [ -f "$ENV_FILE_JSON" ] && log_info "- env json: $ENV_FILE_JSON (file)" || log_info "- env json: $ENV_FILE_JSON (missing)"; }
    [ -n "$DIALTONE_MESH_CONFIG" ] && { [ -f "$DIALTONE_MESH_CONFIG" ] && log_info "- mesh config: $DIALTONE_MESH_CONFIG (file)" || log_info "- mesh config: $DIALTONE_MESH_CONFIG (missing)"; }
fi

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
        [ "$QUIET_BOOTSTRAP" != "1" ] && log_info "Using system Go: $GO_BIN"
    else
        log_info "Go runtime missing. Installing to ${DIALTONE_ENV}..."
        install_go "$DIALTONE_ENV"
        GO_BIN="$DIALTONE_ENV/go/bin/go"
    fi
else
    [ "$QUIET_BOOTSTRAP" != "1" ] && log_info "Using managed Go (Cached): $GO_BIN"
fi


export GOROOT=""
[[ "$GO_BIN" == "$DIALTONE_ENV/go/bin/go" ]] && export GOROOT="$DIALTONE_ENV/go"
export PATH="$DIALTONE_ENV/go/bin:$DIALTONE_ENV/bun/bin:$PATH"
export DIALTONE_GO_BIN="$GO_BIN"

# --- 7. Hand over to Go orchestrator ---
[ "$QUIET_BOOTSTRAP" != "1" ] && log_info "Environment ready. Launching Dialtone..."
cd "$DIALTONE_SRC_ROOT"
exec "$GO_BIN" run dev.go "${PASSTHRU_ARGS[@]}"
