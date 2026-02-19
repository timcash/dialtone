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

GRACEFUL_TIMEOUT=${GRACEFUL_TIMEOUT:-5}
PROCESS_TIMEOUT=${PROCESS_TIMEOUT:-0}
RUNTIME_DIR="$SCRIPT_DIR/.dialtone/run"
LOG_FILE="$SCRIPT_DIR/dialtone.log"

# Track wrapper nesting depth; nested wrappers are tracked with unique keys.
if [[ "${DIALTONE_WRAPPER_DEPTH:-0}" =~ ^[0-9]+$ ]]; then
    DIALTONE_WRAPPER_DEPTH=$((DIALTONE_WRAPPER_DEPTH + 1))
else
    DIALTONE_WRAPPER_DEPTH=1
fi
export DIALTONE_WRAPPER_DEPTH

print_help() {
    if [ -f "$SCRIPT_DIR/help.txt" ]; then
        cat "$SCRIPT_DIR/help.txt"
        return
    fi
    echo "Usage: ./dialtone.sh <command> [options]"
    echo "Run './dialtone.sh install' to install managed Go."
}

dialtone_say() {
    local msg="DIALTONE> $*"
    echo "$msg"
    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ") | INFO | REPL] $msg" >> "$LOG_FILE"
}

dialtone_block() {
    local first=1
    local timestamp
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    while IFS= read -r line; do
        if [ "$first" -eq 1 ]; then
            echo "DIALTONE> $line"
            echo "[$timestamp | INFO | REPL] DIALTONE> $line" >> "$LOG_FILE"
            first=0
        else
            echo "$line"
            echo "[$timestamp | INFO | REPL] $line" >> "$LOG_FILE"
        fi
    done
}

run_subtone_stream() {
    local subtone_pid fifo line exit_code
    local -a subtone_cmd=("$@")

    mkdir -p "$RUNTIME_DIR"
    fifo="$RUNTIME_DIR/repl-subtone-$$.fifo"
    rm -f "$fifo"
    mkfifo "$fifo"

    "$SCRIPT_DIR/dialtone.sh" "${subtone_cmd[@]}" >"$fifo" 2>&1 &
    subtone_pid=$!

    dialtone_say "Signatures verified. Spawning subtone subprocess via PID $subtone_pid..."
    dialtone_say "Streaming stdout/stderr from subtone PID $subtone_pid."
    while IFS= read -r line; do
        local log_line="DIALTONE:${subtone_pid}:> $line"
        echo "$log_line"
        echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ") | INFO | REPL] $log_line" >> "$LOG_FILE"
    done <"$fifo"

    set +e
    wait "$subtone_pid"
    exit_code=$?
    set -e

    rm -f "$fifo"
    dialtone_say "Process $subtone_pid exited with code $exit_code."
    return "$exit_code"
}

start_repl() {
    [ ! -f "$LOG_FILE" ] && touch "$LOG_FILE"
    cat <<EOF | dialtone_block
Virtual Librarian online.
I can bootstrap dev tools, route commands through dev.go, and help install plugins.
Type 'help' for commands, or 'exit' to quit.
EOF

    while true; do
        printf "USER-1> "
        if ! IFS= read -r user_input; then
            echo
            dialtone_say "Session closed."
            break
        fi

        echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ") | INFO | REPL] USER-1> $user_input" >> "$LOG_FILE"
        user_input="$(echo "$user_input" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
        [ -z "$user_input" ] && continue

        # Normalize prefix: handle @DIALTONE and @dialtone.sh
        if [[ "$user_input" == "@DIALTONE "* ]]; then
            user_input="${user_input#@DIALTONE }"
        elif [[ "$user_input" == "@dialtone.sh "* ]]; then
            user_input="${user_input#@dialtone.sh }"
        fi

        case "$user_input" in
            exit|quit)
                dialtone_say "Goodbye."
                break
                ;;
            help)
                cat <<EOF | dialtone_block
Help

### Bootstrap
\`dev install\`
Install latest Go and bootstrap dev.go command scaffold

### Plugins
\`robot install src_v1\`
Install robot src_v1 dependencies

### System
\`<any command>\`
Forward to @./dialtone.sh <command>
EOF
                continue
                ;;
            "dev install")
                dialtone_say "dev install"
                run_subtone_stream "__bootstrap_dev"
                continue
                ;;
            "robot install src_v1")
                dialtone_say "Request received. Spawning subtone for robot install..."
                run_subtone_stream "robot" "install" "src_v1"
                continue
                ;;
        esac

        read -r -a cmd_parts <<< "$user_input"
        dialtone_say "Running: ${cmd_parts[*]}"
        if "$SCRIPT_DIR/dialtone.sh" "${cmd_parts[@]}"; then
            dialtone_say "Done."
        else
            dialtone_say "Command failed (exit $?)."
        fi
    done
}

proc_usage() {
    cat <<EOF_PROC
Usage: ./dialtone.sh proc <subcommand>

Subcommands:
  ps                 List tracked processes (top-level + nested while running)
  stop <key>         Stop tracked process by key
  logs <key>         Tail tracked log for key
EOF_PROC
}

ps_usage() {
    cat <<EOF_PS
Usage: ./dialtone.sh ps <option>

Options:
  all                Show all running ./dialtone.sh processes (default)
  tracked            Show tracked Dialtone processes (top-level + nested)
  tree               Show process tree view (dialtone.sh + go/bun children)
  help               Show this help
EOF_PS
}

kill_usage() {
    cat <<EOF_KILL
Usage: ./dialtone.sh kill <pid|all>

Commands:
  kill <pid>         Kill PID and its descendant process tree
  kill all           Kill all running ./dialtone.sh process trees
  kill help          Show this help
EOF_KILL
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
    local pid_file
    for pid_file in "${files[@]}"; do
        local key pid meta_file cmd status
        key="$(basename "$pid_file" .pid)"
        pid="$(cat "$pid_file" 2>/dev/null || true)"
        meta_file="$RUNTIME_DIR/$key.meta"
        cmd="$(grep '^CMD=' "$meta_file" 2>/dev/null | sed 's/^CMD=//' || true)"
        [ -z "$cmd" ] && cmd="(unknown)"

        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            status="running"
        else
            status="stale"
            rm -f "$pid_file" "$meta_file"
        fi

        printf "%-40s %-8s %-8s %s\n" "$key" "${pid:-n/a}" "$status" "$cmd"
    done
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
        [ "$any_alive" -eq 0 ] && break
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

    kill_pid_tree "$pid" || true
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

# Ensure critical directories exist for Go embed
mkdir -p "$SCRIPT_DIR/src/core/web/dist"

# First pass: find --env flag so we can source env before dispatching.
DIALTONE_ENV_FILE=""
for arg in "$@"; do
    if [[ "$arg" == --env=* ]]; then
        DIALTONE_ENV_FILE="${arg#*=}"
    fi
done
for (( i=1; i<=$#; i++ )); do
    if [[ "${!i}" == "--env" ]]; then
        j=$((i+1))
        DIALTONE_ENV_FILE="${!j}"
    fi
done
[ -z "$DIALTONE_ENV_FILE" ] && DIALTONE_ENV_FILE="$SCRIPT_DIR/env/.env"

if [ -f "$DIALTONE_ENV_FILE" ]; then
    set -a
    # shellcheck disable=SC1090
    source "$DIALTONE_ENV_FILE"
    set +a
fi

# Parse global flags + command.
CMD=""
CMD_ARGS=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        --env=*|--env)
            if [[ "$1" == "--env" ]]; then
                shift 2
            else
                shift
            fi
            ;;
        --timeout=*)
            PROCESS_TIMEOUT="${1#*=}"
            shift
            ;;
        --timeout)
            if [ -z "${2:-}" ]; then
                echo "Error: --timeout requires a value in seconds"
                exit 1
            fi
            PROCESS_TIMEOUT="$2"
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
            if [ -z "$CMD" ]; then
                print_help
                exit 0
            fi
            CMD_ARGS+=("$1")
            shift
            ;;
        *)
            if [ -z "$CMD" ]; then
                CMD="$1"
            else
                CMD_ARGS+=("$1")
            fi
            shift
            ;;
    esac
done

if [ -z "$CMD" ]; then
    start_repl
    exit 0
fi

# Command families handled entirely by shell.
if [ "$CMD" = "ps" ]; then
    option="${CMD_ARGS[0]:-all}"
    case "$option" in
        all) ps_all ;;
        tracked) proc_ps ;;
        tree) ps_tree ;;
        help|-h|--help) ps_usage ;;
        *)
            echo "Unknown ps option: $option"
            ps_usage
            exit 1
            ;;
    esac
    exit $?
fi

if [ "$CMD" = "proc" ]; then
    subcmd="${CMD_ARGS[0]:-}"
    case "$subcmd" in
        ps) proc_ps ;;
        stop)
            key="${CMD_ARGS[1]:-}"
            [ -z "$key" ] && { echo "Usage: ./dialtone.sh proc stop <key>"; exit 1; }
            proc_stop "$key"
            ;;
        logs)
            key="${CMD_ARGS[1]:-}"
            [ -z "$key" ] && { echo "Usage: ./dialtone.sh proc logs <key>"; exit 1; }
            proc_logs "$key"
            ;;
        help|-h|--help|"") proc_usage ;;
        *)
            echo "Unknown proc command: $subcmd"
            proc_usage
            exit 1
            ;;
    esac
    exit $?
fi

if [ "$CMD" = "kill" ]; then
    target="${CMD_ARGS[0]:-}"
    case "$target" in
        all) kill_all_dialtone ;;
        help|-h|--help|"") kill_usage ;;
        *) kill_pid_tree "$target" ;;
    esac
    exit $?
fi

if [ "$CMD" = "install" ]; then
    installer="$SCRIPT_DIR/src/plugins/go/install.sh"
    if [ ! -f "$installer" ]; then
        echo "Error: installer not found: $installer"
        exit 1
    fi
    bash "$installer" "${CMD_ARGS[@]}"
    exit $?
fi

if [ "$CMD" = "__bootstrap_dev" ]; then
    echo "Installing latest Go runtime for managed ./dialtone.sh go commands..."
    installer="$SCRIPT_DIR/src/plugins/go/install.sh"
    if [ ! -f "$installer" ]; then
        echo "Installer missing: $installer"
        exit 1
    fi
    if ! bash "$installer" --latest; then
        echo "Install failed."
        exit 1
    fi

    if [[ "${DIALTONE_ENV:-}" == "~"* ]]; then
        DIALTONE_ENV="${DIALTONE_ENV/#\~/$HOME}"
    fi
    GO_BIN="${DIALTONE_ENV:-}/go/bin/go"
    if [ -x "$GO_BIN" ]; then
        echo "Bootstrap complete. Initializing dev.go scaffold..."
        (cd "$SCRIPT_DIR/src" && "$GO_BIN" run cmd/dev/main.go help) >/dev/null 2>&1 || true
        echo "Ready. You can now run plugin commands (install/build/test) via DIALTONE."
    else
        echo "Go runtime installed, but DIALTONE_ENV/go/bin/go was not found."
        exit 1
    fi
    exit 0
fi

# Everything else runs through managed Go toolchain.
if [[ "${DIALTONE_ENV:-}" == "~"* ]]; then
    DIALTONE_ENV="${DIALTONE_ENV/#\~/$HOME}"
fi

export DIALTONE_ENV
export DIALTONE_ENV_FILE

if [ -z "${DIALTONE_ENV:-}" ]; then
    echo "Error: DIALTONE_ENV is not set."
    echo "Set it in $DIALTONE_ENV_FILE or pass --env <path>."
    exit 1
fi

GO_BIN="$DIALTONE_ENV/go/bin/go"
if [ ! -x "$GO_BIN" ]; then
    echo "Error: Go not found in $DIALTONE_ENV/go."
    echo "Please run './dialtone.sh install' first to set up the environment."
    exit 1
fi

# Force managed Go runtime/tooling resolution.
export GOROOT="$DIALTONE_ENV/go"
export PATH="$DIALTONE_ENV/go/bin:$PATH"

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

    cat >"$meta_file" <<EOF_META
CMD=$cmdline
LOG=$log_file
STARTED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF_META

    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ") | INFO | EXEC] Start: $cmdline" >> "$LOG_FILE"

    (cd "$SCRIPT_DIR/src" && "$go_cmd" run cmd/dev/main.go "$@") \
        > >(tee -a "$log_file") \
        2> >(tee -a "$log_file" >&2) &
    child_pid=$!
    echo "$child_pid" > "$pid_file"

    local watchdog_pid=""
    if [ "$PROCESS_TIMEOUT" -gt 0 ] 2>/dev/null; then
        (
            sleep "$PROCESS_TIMEOUT"
            if kill -0 "$child_pid" 2>/dev/null; then
                echo "[dialtone] Timeout (${PROCESS_TIMEOUT}s) reached; killing process tree for pid=$child_pid"
                kill_pid_tree "$child_pid" || true
            fi
        ) &
        watchdog_pid=$!
    fi

    set +e
    wait "$child_pid"
    exit_code=$?
    set -e

    if [ -n "$watchdog_pid" ] && kill -0 "$watchdog_pid" 2>/dev/null; then
        kill "$watchdog_pid" 2>/dev/null || true
    fi

    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ") | INFO | EXEC] Exit $exit_code: $cmdline" >> "$LOG_FILE"
    rm -f "$pid_file" "$meta_file"
    return "$exit_code"
}

run_tool "$GO_BIN" "$CMD" "${CMD_ARGS[@]}"
exit $?
