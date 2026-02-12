#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Enforce repository-root execution to keep all relative paths predictable.
if [ "$PWD" != "$SCRIPT_DIR" ]; then
    echo "Error: ./dialtone.sh must be run from the repository root."
    echo "Expected: $SCRIPT_DIR"
    echo "Current:  $PWD"
    echo "Run: cd \"$SCRIPT_DIR\" && ./dialtone.sh <command>"
    exit 1
fi

# --- CONFIGURATION ---
GRACEFUL_TIMEOUT=${GRACEFUL_TIMEOUT:-5}   # Used by explicit `proc stop` only
RUNTIME_DIR="$SCRIPT_DIR/.dialtone/run"

# Track wrapper depth; only top-level invocations are tracked by `proc`.
if [[ "${DIALTONE_WRAPPER_DEPTH:-0}" =~ ^[0-9]+$ ]]; then
    DIALTONE_WRAPPER_DEPTH=$((DIALTONE_WRAPPER_DEPTH + 1))
else
    DIALTONE_WRAPPER_DEPTH=1
fi
export DIALTONE_WRAPPER_DEPTH

TRACK_PROCESSES=1

# --- HELP MENU ---
print_help() {
    cat <<EOF
Usage: ./dialtone.sh <command> [options]
       ./dialtone.sh help
       ./dialtone.sh --help

Commands:
  start               Start the NATS and Web server
  install [path]      Install Go toolchain to DIALTONE_ENV (go plugin default)
  build               Build web UI and binary (--local, --full, --remote, --podman, --linux-arm, --linux-arm64)
  deploy              Deploy to remote robot
  camera              Camera tools (snapshot, stream)
  clone               Clone or update the repository
  sync-code           Sync source code to remote robot
  ssh                 SSH tools (upload, download, cmd)
  provision           Generate Tailscale auth key
  logs                Tail remote logs
  diagnostic          Run system diagnostics (local or remote)
  branch <name>       Create or checkout a feature branch
  ide <subcmd>        IDE tools (setup-workflows)
  github <subcmd>     GitHub tools (pr, check-deploy)
  www <subcmd>        Public webpage tools
  ui <subcmd>         Web UI tools (dev, build, install)
  ai <subcmd>         AI tools (opencode, developer, subagent)
  go <subcmd>         Go toolchain tools (install, lint, test, exec)
  bun <subcmd>        Bun toolchain tools (exec, run, x, test)
  kill <pid|all>      Kill one Dialtone process tree or all Dialtone shell processes
  ps <option>         List Dialtone processes (tracked, all, tree)
  proc <subcmd>       Process management (ps, stop, logs)
  test <subcmd>       Run tests (legacy)
  help                Show this help message

Global Options:
  --env <path>         Set DIALTONE_ENV directory
  --grace <sec>        Seconds to wait in proc stop before SIGKILL (default: 5)
  --timeout <sec>      Deprecated; ignored

Process Notes:
  - dialtone.sh does not auto-kill child processes on shell exit.
  - ./dialtone.sh help and ./dialtone.sh --help are equivalent.
  - Nested ./dialtone.sh subcommands are tracked while running.
  - Use explicit process commands:
      ./dialtone.sh ps
      ./dialtone.sh ps all
      ./dialtone.sh ps tracked
      ./dialtone.sh ps tree
      ./dialtone.sh kill <pid>
      ./dialtone.sh kill all
      ./dialtone.sh proc stop <key>

Examples:
  ./dialtone.sh dag dev src_v2
  ./dialtone.sh ps
  ./dialtone.sh kill all
  ./dialtone.sh proc stop dag_dev_src_v2
EOF
}

sanitize_process_key() {
    local key="$*"
    key="${key//\//_}"
    key="$(echo "$key" | tr '[:space:]' '_' | tr -cd '[:alnum:]_.:-' | sed -E 's/_+/_/g; s/^_+//; s/_+$//')"
    if [ -z "$key" ]; then
        key="unnamed"
    fi
    echo "$key"
}

proc_usage() {
    cat <<EOF
Usage: ./dialtone.sh proc <subcommand>

Subcommands:
  ps                 List tracked processes (top-level + nested while running)
  stop <key>         Stop tracked process by key
  logs <key>         Tail tracked log for key
EOF
}

ps_usage() {
    cat <<EOF
Usage: ./dialtone.sh ps <option>

Options:
  all                Show all running ./dialtone.sh processes (including nested) (default)
  tracked            Show tracked Dialtone processes (top-level + nested)
  tree               Show process tree view (dialtone.sh + go/bun children)
  help               Show this help
EOF
}

kill_usage() {
    cat <<EOF
Usage: ./dialtone.sh kill <pid|all>

Commands:
  kill <pid>          Kill PID and its descendant process tree
  kill all            Kill all running ./dialtone.sh process trees
  kill help           Show this help
EOF
}

proc_ps() {
    mkdir -p "$RUNTIME_DIR"
    shopt -s nullglob
    local files=("$RUNTIME_DIR"/*.pid)
    shopt -u nullglob

    if [ "${#files[@]}" -eq 0 ]; then
        echo "No tracked processes."
        return 0
    fi

    printf "%-40s %-8s %-8s %s\n" "KEY" "PID" "STATUS" "CMD"
    for pid_file in "${files[@]}"; do
        local key pid meta_file cmd status
        key="$(basename "$pid_file" .pid)"
        pid="$(cat "$pid_file" 2>/dev/null || true)"
        meta_file="$RUNTIME_DIR/$key.meta"
        cmd="$(grep '^CMD=' "$meta_file" 2>/dev/null | sed 's/^CMD=//' || true)"
        if [ -z "$cmd" ]; then
            cmd="(unknown)"
        fi

        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            status="running"
        else
            status="stale"
            rm -f "$pid_file" "$meta_file"
        fi
        printf "%-40s %-8s %-8s %s\n" "$key" "${pid:-n/a}" "$status" "$cmd"
    done
}

proc_stop() {
    local key="$1"
    local pid_file="$RUNTIME_DIR/$key.pid"
    local meta_file="$RUNTIME_DIR/$key.meta"
    if [ ! -f "$pid_file" ]; then
        echo "No tracked process for key: $key"
        return 1
    fi

    local pid
    pid="$(cat "$pid_file" 2>/dev/null || true)"
    if [ -z "$pid" ] || ! kill -0 "$pid" 2>/dev/null; then
        echo "Process is not running (cleaning stale state): $key"
        rm -f "$pid_file" "$meta_file"
        return 0
    fi

    local targets=("$pid")
    while IFS= read -r dpid; do
        [ -n "$dpid" ] && targets+=("$dpid")
    done < <(collect_descendants "$pid")

    echo "[dialtone] Stopping $key (pid=$pid) with SIGTERM..."
    for tpid in "${targets[@]}"; do
        kill -TERM "$tpid" 2>/dev/null || true
    done

    local waited=0
    while [ "$waited" -lt "$GRACEFUL_TIMEOUT" ]; do
        local any_alive=0
        for tpid in "${targets[@]}"; do
            if kill -0 "$tpid" 2>/dev/null; then
                any_alive=1
                break
            fi
        done
        if [ "$any_alive" -eq 0 ]; then
            break
        fi
        sleep 1
        waited=$((waited + 1))
    done

    local any_alive=0
    for tpid in "${targets[@]}"; do
        if kill -0 "$tpid" 2>/dev/null; then
            any_alive=1
            break
        fi
    done
    if [ "$any_alive" -eq 1 ]; then
        echo "[dialtone] Escalating to SIGKILL for $key (pid=$pid)..."
        for tpid in "${targets[@]}"; do
            kill -KILL "$tpid" 2>/dev/null || true
        done
    fi

    rm -f "$pid_file" "$meta_file"
}

proc_logs() {
    local key="$1"
    local log_file="$RUNTIME_DIR/$key.log"
    if [ ! -f "$log_file" ]; then
        echo "No log file found for key: $key"
        return 1
    fi
    tail -f "$log_file"
}

ps_all() {
    pgrep -fal '(^|/)dialtone\.sh( |$)' || true
}

ps_tree() {
    ps -eo pid=,ppid=,command= \
        | grep -E 'dialtone\.sh|cmd/dev/main\.go|bun exec|go run .*/src/cmd/dev/main.go' \
        | grep -v 'grep -E' || true
}

collect_descendants() {
    local parent="$1"
    local child
    while IFS= read -r child; do
        [ -z "$child" ] && continue
        echo "$child"
        collect_descendants "$child"
    done < <(pgrep -P "$parent" 2>/dev/null || true)
}

cleanup_metadata_for_pid() {
    local target_pid="$1"
    mkdir -p "$RUNTIME_DIR"
    shopt -s nullglob
    local pid_file
    for pid_file in "$RUNTIME_DIR"/*.pid; do
        local tracked_pid key meta_file
        tracked_pid="$(cat "$pid_file" 2>/dev/null || true)"
        if [ "$tracked_pid" = "$target_pid" ]; then
            key="$(basename "$pid_file" .pid)"
            meta_file="$RUNTIME_DIR/$key.meta"
            rm -f "$pid_file" "$meta_file"
        fi
    done
    shopt -u nullglob
}

kill_pid_tree() {
    local pid="$1"
    if ! [[ "$pid" =~ ^[0-9]+$ ]]; then
        echo "Invalid PID: $pid"
        return 1
    fi
    if ! kill -0 "$pid" 2>/dev/null; then
        echo "PID not running: $pid"
        cleanup_metadata_for_pid "$pid"
        return 1
    fi

    local targets=("$pid")
    while IFS= read -r dpid; do
        [ -n "$dpid" ] && targets+=("$dpid")
    done < <(collect_descendants "$pid")

    echo "[dialtone] Killing process tree for pid=$pid (SIGTERM)..."
    local tpid
    for tpid in "${targets[@]}"; do
        kill -TERM "$tpid" 2>/dev/null || true
    done

    local waited=0
    while [ "$waited" -lt "$GRACEFUL_TIMEOUT" ]; do
        local any_alive=0
        for tpid in "${targets[@]}"; do
            if kill -0 "$tpid" 2>/dev/null; then
                any_alive=1
                break
            fi
        done
        if [ "$any_alive" -eq 0 ]; then
            break
        fi
        sleep 1
        waited=$((waited + 1))
    done

    local any_alive=0
    for tpid in "${targets[@]}"; do
        if kill -0 "$tpid" 2>/dev/null; then
            any_alive=1
            break
        fi
    done
    if [ "$any_alive" -eq 1 ]; then
        echo "[dialtone] Escalating process tree for pid=$pid (SIGKILL)..."
        for tpid in "${targets[@]}"; do
            kill -KILL "$tpid" 2>/dev/null || true
        done
    fi

    cleanup_metadata_for_pid "$pid"
    return 0
}

kill_all_dialtone() {
    local pids pid
    pids="$(pgrep -f '(^|/)dialtone\.sh( |$)' || true)"
    if [ -z "$pids" ]; then
        echo "No running ./dialtone.sh processes found."
        return 0
    fi

    while IFS= read -r pid; do
        [ -z "$pid" ] && continue
        if [ "$pid" = "$$" ] || [ "$pid" = "$PPID" ]; then
            continue
        fi
        kill_pid_tree "$pid" || true
    done <<< "$pids"
}

# 0. Ensure critical directories exist for Go embed
mkdir -p "$SCRIPT_DIR/src/core/web/dist"

# 1. Resolve DIALTONE_ENV and identify command
DIALTONE_CMD=""
ARGS=()

# First pass: find --env flag to source it before any other logic
for arg in "$@"; do
    if [[ "$arg" == --env=* ]]; then
        DIALTONE_ENV_FILE="${arg#*=}"
    fi
done

# If --env flag wasn't found in first pass, find it as positional if it exists
for (( i=1; i<=$#; i++ )); do
    if [[ "${!i}" == "--env" ]]; then
        j=$((i+1))
        DIALTONE_ENV_FILE="${!j}"
    fi
done

if [ -z "$DIALTONE_ENV_FILE" ]; then
    DIALTONE_ENV_FILE="$SCRIPT_DIR/env/.env"
fi

# SOURCE THE ENV FILE EARLY
# This puts all variables (including TEST_VAR) into the current shell
if [ -f "$DIALTONE_ENV_FILE" ]; then
    # We use a subshell to parse and then export to avoid sourcing logic issues
    # but a simple source is usually enough if it's a standard .env
    set -a
    source "$DIALTONE_ENV_FILE"
    set +a
fi

# 2. Parse all flags including command
while [[ $# -gt 0 ]]; do
    case "$1" in
        --env=*)
            shift
            ;;
        --env)
            shift 2
            ;;
        --timeout=*)
            echo "[dialtone] --timeout is deprecated and ignored."
            shift
            ;;
        --timeout)
            echo "[dialtone] --timeout is deprecated and ignored."
            shift 2
            ;;
        --grace=*)
            GRACEFUL_TIMEOUT="${1#*=}"
            shift
            ;;
        --grace)
            GRACEFUL_TIMEOUT="$2"
            shift 2
            ;;
        -h|--help|help)
            # Only show shell help if no command set yet
            if [ -z "$DIALTONE_CMD" ]; then
                print_help
                exit 0
            else
                # Pass --help to the subcommand
                ARGS+=("$1")
                shift
            fi
            ;;
        *)
            if [ -z "$DIALTONE_CMD" ]; then
                DIALTONE_CMD="$1"
            fi
            ARGS+=("$1")
            shift
            ;;
    esac
done

# If --clean is present, remove the environment directory first (before Go runs)
for arg in "${ARGS[@]}"; do
    if [[ "$arg" == "--clean" ]]; then
        if [ -n "$DIALTONE_ENV" ] && [ -d "$DIALTONE_ENV" ]; then
            echo "Cleaning dependencies directory: $DIALTONE_ENV"
            # Use chmod to handle read-only files in Go module cache
            chmod -R u+w "$DIALTONE_ENV" 2>/dev/null || true
            rm -rf "$DIALTONE_ENV"
            echo "Successfully removed $DIALTONE_ENV"
        fi
        break
    fi
done

# If no command provided, show help
if [ -z "$DIALTONE_CMD" ]; then
    print_help
    exit 0
fi

# Process management commands are handled directly in the shell wrapper.
if [ "$DIALTONE_CMD" = "proc" ]; then
    subcmd="${ARGS[1]:-}"
    case "$subcmd" in
        ps)
            proc_ps
            ;;
        stop)
            key="${ARGS[2]:-}"
            if [ -z "$key" ]; then
                echo "Usage: ./dialtone.sh proc stop <key>"
                exit 1
            fi
            proc_stop "$key"
            ;;
        logs)
            key="${ARGS[2]:-}"
            if [ -z "$key" ]; then
                echo "Usage: ./dialtone.sh proc logs <key>"
                exit 1
            fi
            proc_logs "$key"
            ;;
        help|-h|--help|"")
            proc_usage
            ;;
        *)
            echo "Unknown proc command: $subcmd"
            proc_usage
            exit 1
            ;;
    esac
    exit $?
fi

if [ "$DIALTONE_CMD" = "ps" ]; then
    option="${ARGS[1]:-all}"
    case "$option" in
        tracked)
            proc_ps
            ;;
        all)
            ps_all
            ;;
        tree)
            ps_tree
            ;;
        help|-h|--help)
            ps_usage
            ;;
        *)
            echo "Unknown ps option: $option"
            ps_usage
            exit 1
            ;;
    esac
    exit $?
fi

if [ "$DIALTONE_CMD" = "kill" ]; then
    target="${ARGS[1]:-}"
    case "$target" in
        all)
            kill_all_dialtone
            ;;
        help|-h|--help|"")
            kill_usage
            ;;
        *)
            kill_pid_tree "$target"
            ;;
    esac
    exit $?
fi

# Install is handled directly by the go plugin installer script.
if [ "$DIALTONE_CMD" = "install" ]; then
    installer="$SCRIPT_DIR/src/plugins/go/install.sh"
    if [ ! -f "$installer" ]; then
        echo "Error: installer not found: $installer"
        exit 1
    fi
    bash "$installer" "${ARGS[@]:1}"
    exit $?
fi

# Tilde expansion for DIALTONE_ENV if sourced
if [[ "$DIALTONE_ENV" == "~"* ]]; then
    DIALTONE_ENV="${DIALTONE_ENV/#\~/$HOME}"
fi

# Ensure them exported for child processes (Go binary)
export DIALTONE_ENV
export DIALTONE_ENV_FILE

# Error if DIALTONE_ENV is still not set
if [ -z "$DIALTONE_ENV" ]; then
    echo "Error: DIALTONE_ENV is not set."
    echo ""
    echo "Please add DIALTONE_ENV to your $DIALTONE_ENV_FILE file:"
    echo "  echo 'DIALTONE_ENV=/path/to/your/env' >> $DIALTONE_ENV_FILE"
    echo ""
    echo "Or pass it as an argument:"
    echo "  ./dialtone.sh --env=/path/to/your/env <command>"
    exit 1
fi

# 3. Require managed Go toolchain for all non-install commands
GO_BIN=""
if [ -n "$DIALTONE_ENV" ]; then
    GO_BIN="$DIALTONE_ENV/go/bin/go"
fi

if [ ! -x "$GO_BIN" ]; then
    echo "Error: Go not found in $DIALTONE_ENV/go."
    echo "Please run './dialtone.sh install' first to set up the environment."
    exit 1
fi

# 4. Run the tool and track process metadata.
run_tool() {
    local go_cmd="$1"
    shift

    mkdir -p "$RUNTIME_DIR"

    local key pid_file meta_file log_file cmdline child_pid exit_code
    key="$(sanitize_process_key "$@")"
    if [ "$DIALTONE_WRAPPER_DEPTH" -gt 1 ]; then
        key="${key}__pid_$$"
    fi
    pid_file="$RUNTIME_DIR/$key.pid"
    meta_file="$RUNTIME_DIR/$key.meta"
    log_file="$RUNTIME_DIR/$key.log"
    cmdline="./dialtone.sh $*"

    cat >"$meta_file" <<EOF
CMD=$cmdline
LOG=$log_file
STARTED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

    "$go_cmd" run "$SCRIPT_DIR/src/cmd/dev/main.go" "$@" \
        > >(tee -a "$log_file") \
        2> >(tee -a "$log_file" >&2) &
    child_pid=$!
    echo "$child_pid" > "$pid_file"

    set +e
    wait "$child_pid"
    exit_code=$?
    set -e

    rm -f "$pid_file" "$meta_file"
    return "$exit_code"
}

run_tool "$GO_BIN" "${ARGS[@]}"
exit $?
