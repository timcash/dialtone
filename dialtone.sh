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
    echo "Usage: ./dialtone.sh --env env/test.env"
    echo "Usage: ./dialtone.sh repl --env env/test.env"
    echo "Usage: ./dialtone.sh repl install"
    echo "Run './dialtone.sh install' to install managed Go."
}

dialtone_say() {
    local line="DIALTONE> $*"
    echo "$line"
    if [ -n "${REPL_SESSION_ID:-}" ]; then
        repl_log_event "$REPL_SESSION_ID" "DIALTONE" "$line" "OUTPUT"
    fi
}

dialtone_block() {
    printf "DIALTONE> %s\n" "$1"
    if [ -n "${REPL_SESSION_ID:-}" ]; then
        while IFS= read -r _line; do
            [ -z "$_line" ] && continue
            repl_log_event "$REPL_SESSION_ID" "DIALTONE" "$_line" "OUTPUT"
        done <<< "DIALTONE> $1"
    fi
}

role_say() {
    local role="${1:-USER-1}"
    shift
    local line="${role}> $*"
    echo "$line"
    if [ -n "${REPL_SESSION_ID:-}" ]; then
        repl_log_event "$REPL_SESSION_ID" "$role" "$line" "OUTPUT"
    fi
}

repl_async_emit() {
    local line="$1"
    # Async REPL output can arrive while the prompt is on screen; redraw cleanly.
    if [ "${REPL_AGENT_MODE:-0}" -eq 0 ]; then
        printf "\r\033[K%s\n" "$line"
        printf "%s> " "${REPL_ROLE:-USER-1}"
    else
        echo "$line"
    fi
}

llm_role_reply() {
    local role="$1"
    local text="$2"
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        role_say "$role" "$line"
    done <<< "$text"
}

repl_log_event() {
    local session_id="$1"
    local role="$2"
    local text="$3"
    local kind="${4:-INPUT}"
    local safe_text="$text"
    safe_text="${safe_text//$'\t'/ }"
    safe_text="${safe_text//$'\n'/ }"
    mkdir -p "$RUNTIME_DIR"
    printf "%s\t%s\t%s\t%s\t%s\n" "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "$session_id" "$kind" "$role" "$safe_text" >> "$RUNTIME_DIR/repl-events.log"
}

run_llm_ops_bridge() {
    local question="$1"
    local bridge_bin="${DIALTONE_LLM_OPS_BIN:-}"
    local output

    if [ -z "$question" ]; then
        dialtone_say "Usage: @LLM-OPS <question>"
        return 1
    fi
    if [ -z "$bridge_bin" ]; then
        if [ -x "$SCRIPT_DIR/scripts/llm_ops_demo_bridge.sh" ]; then
            bridge_bin="$SCRIPT_DIR/scripts/llm_ops_demo_bridge.sh"
        else
            dialtone_say "No LLM-OPS bridge configured. Set DIALTONE_LLM_OPS_BIN in your env file."
            return 1
        fi
    fi
    if [[ "$bridge_bin" != /* ]]; then
        bridge_bin="$SCRIPT_DIR/$bridge_bin"
    fi
    if [ ! -x "$bridge_bin" ]; then
        dialtone_say "LLM-OPS bridge is not executable: $bridge_bin"
        return 1
    fi

    set +e
    output="$("$bridge_bin" "$question" 2>&1)"
    bridge_exit=$?
    set -e
    if [ $bridge_exit -ne 0 ]; then
        dialtone_say "LLM-OPS bridge failed (exit $bridge_exit)."
        dialtone_say "$output"
        return $bridge_exit
    fi
    llm_role_reply "LLM-OPS" "$output"
    return 0
}

is_external_llm_ops_active() {
    local marker_file="$RUNTIME_DIR/repl-llm-ops.external"
    [ -f "$marker_file" ]
}

normalize_dialtone_env() {
    local env_path="$1"
    if [[ "$env_path" == "~"* ]]; then
        env_path="${env_path/#\~/$HOME}"
    fi
    if [ -n "$env_path" ] && [[ "$env_path" != /* ]]; then
        env_path="$SCRIPT_DIR/$env_path"
    fi
    echo "$env_path"
}

count_tracked_processes() {
    local count=0 pid_file pid
    mkdir -p "$RUNTIME_DIR"
    shopt -s nullglob
    for pid_file in "$RUNTIME_DIR"/*.pid; do
        pid="$(cat "$pid_file" 2>/dev/null || true)"
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            count=$((count + 1))
        fi
    done
    shopt -u nullglob
    echo "$count"
}

is_repl_session_alive() {
    local session_id="$1"
    local cmd
    if [[ ! "$session_id" =~ ^[0-9]+$ ]]; then
        return 1
    fi
    cmd="$(ps -p "$session_id" -o args= 2>/dev/null || true)"
    [[ "$cmd" == *"dialtone.sh"* ]]
}

count_repl_sessions() {
    local repl_fifos=() fifo session_id count=0
    mkdir -p "$RUNTIME_DIR"
    shopt -s nullglob
    repl_fifos=("$RUNTIME_DIR"/repl-control-*.fifo)
    shopt -u nullglob
    for fifo in "${repl_fifos[@]}"; do
        session_id="${fifo##*/repl-control-}"
        session_id="${session_id%.fifo}"
        if is_repl_session_alive "$session_id"; then
            count=$((count + 1))
        else
            rm -f "$fifo"
        fi
    done
    echo "$count"
}

list_repl_sessions() {
    local repl_fifos=() fifo session_id active_session="" count=0 output_lines=()
    mkdir -p "$RUNTIME_DIR"
    if [ -f "$RUNTIME_DIR/repl-active.session" ]; then
        active_session="$(cat "$RUNTIME_DIR/repl-active.session" 2>/dev/null || true)"
    fi

    shopt -s nullglob
    repl_fifos=("$RUNTIME_DIR"/repl-control-*.fifo)
    shopt -u nullglob

    for fifo in "${repl_fifos[@]}"; do
        session_id="${fifo##*/repl-control-}"
        session_id="${session_id%.fifo}"
        if is_repl_session_alive "$session_id"; then
            count=$((count + 1))
            if [ -n "$active_session" ] && [ "$session_id" = "$active_session" ]; then
                output_lines+=("- $session_id (active)")
            else
                output_lines+=("- $session_id")
            fi
        else
            rm -f "$fifo"
        fi
    done

    echo "Running REPL sessions: $count"
    if [ "$count" -eq 0 ]; then
        return 0
    fi
    for line in "${output_lines[@]}"; do
        echo "$line"
    done
}

repl_plugin_install() {
    local plugin_dir="$SCRIPT_DIR/src/plugins/repl"
    if [ ! -d "$plugin_dir" ]; then
        echo "REPL plugin directory not found: $plugin_dir"
        return 1
    fi
    if [ ! -f "$plugin_dir/pixi.toml" ]; then
        echo "REPL plugin pixi config not found: $plugin_dir/pixi.toml"
        return 1
    fi
    if ! command -v pixi >/dev/null 2>&1; then
        echo "pixi is not installed. Install pixi first, then re-run: ./dialtone.sh repl install"
        return 1
    fi
    (cd "$plugin_dir" && pixi install "$@")
}

generate_random_task_id() {
    local prefix="$1"
    if [ -n "${DIALTONE_TEST_TASK_ID:-}" ]; then
        echo "$DIALTONE_TEST_TASK_ID"
        return
    fi
    local suffix
    suffix="$(LC_ALL=C tr -dc 'a-z0-9' </dev/urandom | head -c 6)"
    [ -z "$suffix" ] && suffix="000000"
    echo "${prefix}-${suffix}"
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

    dialtone_say "Spawning subtone subprocess via PID $subtone_pid..."
    dialtone_say "Streaming stdout/stderr from subtone PID $subtone_pid."
    while IFS= read -r line; do
        stream_line="DIALTONE:${subtone_pid}:> $line"
        echo "$stream_line"
        if [ -n "${REPL_SESSION_ID:-}" ]; then
            repl_log_event "$REPL_SESSION_ID" "DIALTONE" "$stream_line" "OUTPUT"
        fi
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
    [ -z "${REPL_ROLE:-}" ] && REPL_ROLE="USER-1"
    local repl_session_id="$$"
    REPL_SESSION_ID="$repl_session_id"
    export REPL_SESSION_ID
    local pending_task_id=""
    local -a pending_subtone_cmd=()
    local env_path_display="$DIALTONE_ENV_FILE"
    local subtone_count=0

    if command -v realpath >/dev/null 2>&1; then
        env_path_display="$(realpath "$DIALTONE_ENV_FILE" 2>/dev/null || echo "$DIALTONE_ENV_FILE")"
    fi

    dialtone_block "$(cat <<'EOF'
Virtual Librarian online.
I can bootstrap dev tools, route commands through dev.go, and help install plugins.
EOF
)"
    repl_control_start "$repl_session_id"
    repl_bus_start "$repl_session_id"
    trap repl_runtime_cleanup EXIT
    if [ -f "$DIALTONE_ENV_FILE" ]; then
        dialtone_say "Using .env file: $(basename "$DIALTONE_ENV_FILE")"
        dialtone_say "Env path: $env_path_display"
    else
        dialtone_say "No .env file found at: $env_path_display"
    fi
    dialtone_block "$(cat <<EOF
Runtime report:
  Active REPL sessions: $(count_repl_sessions)
  Active tracked processes: $(count_tracked_processes)
EOF
)"
    dialtone_say "Type 'help' for commands, or 'exit' to quit."

    while true; do
        if [ "${REPL_AGENT_MODE:-0}" -eq 0 ]; then
            printf "%s> " "$REPL_ROLE"
            if ! IFS= read -r user_input; then
                echo
                dialtone_say "Session closed."
                break
            fi
        else
            if ! IFS= read -r user_input; then
                dialtone_say "Session closed."
                break
            fi
            if [ "${REPL_ECHO_INPUT:-0}" -eq 1 ]; then
                echo "$REPL_ROLE> $user_input"
            fi
        fi

        user_input="$(echo "$user_input" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
        [ -z "$user_input" ] && continue
        repl_log_event "$repl_session_id" "$REPL_ROLE" "$user_input" "INPUT"

        case "$user_input" in
            exit|quit)
                dialtone_say "Goodbye."
                break
                ;;
            help)
                dialtone_block "$(cat <<'EOF'
Commands:
- `@DIALTONE dev install` - install latest Go via a monitored subtone process
- `@DIALTONE repl install` - install REPL plugin Python env with pixi
- `@DIALTONE robot install src_v1` - queue robot install and return a random task id
- `@DIALTONE task --sign <task-id>` - sign the queued task and run it in a subtone
- `status` - show REPL process-monitor state
- `repls` - list active REPL sessions
- `role <name>` - switch speaker role (example: `role LLM-OPS`)
- `whoami` - show current speaker role
- `ps`, `proc ps`, `kill <pid|all>` - process monitor commands available immediately
- `<any command>` - forward to `./dialtone.sh <command>`
EOF
)"
                continue
                ;;
            status)
                dialtone_block "$(cat <<EOF
Session state:
  Active speaker role: $REPL_ROLE
  Subtones created this REPL session: $subtone_count
  Active tracked processes: $(count_tracked_processes)
  Active REPL sessions: $(count_repl_sessions)
  Pending sign task: ${pending_task_id:-none}
EOF
)"
                continue
                ;;
            repls)
                dialtone_block "$(list_repl_sessions)"
                continue
                ;;
            role\ *)
                new_role="${user_input#role }"
                new_role="$(echo "$new_role" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
                if [ -z "$new_role" ]; then
                    dialtone_say "Usage: role <name>"
                    continue
                fi
                REPL_ROLE="$new_role"
                export REPL_ROLE
                dialtone_say "Speaker role set to ${REPL_ROLE}>"
                role_say "$REPL_ROLE" "Joined REPL session."
                continue
                ;;
            whoami)
                dialtone_say "Current speaker role: ${REPL_ROLE}>"
                continue
                ;;
        esac

        if [[ "$user_input" == "@DIALTONE "* ]]; then
            dialtone_cmd="${user_input#@DIALTONE }"
            read -r -a dialtone_parts <<< "$dialtone_cmd"

            if [ "${dialtone_parts[0]:-}" = "dev" ] && [ "${dialtone_parts[1]:-}" = "install" ]; then
                dialtone_say "Starting monitored subtone for dev install (latest Go runtime)..."
                installer="$SCRIPT_DIR/src/plugins/go/install.sh"
                if [ ! -f "$installer" ]; then
                    dialtone_say "Installer missing: $installer"
                    continue
                fi
                subtone_count=$((subtone_count + 1))
                if ! run_subtone_stream install --latest; then
                    dialtone_say "Install failed."
                    continue
                fi

                if [[ "${DIALTONE_ENV:-}" == "~"* ]]; then
                    DIALTONE_ENV="${DIALTONE_ENV/#\~/$HOME}"
                fi
                GO_BIN="${DIALTONE_ENV:-}/go/bin/go"
                if [ -x "$GO_BIN" ]; then
                    dialtone_say "Bootstrap complete. Verifying dev.go command routing..."
                    subtone_count=$((subtone_count + 1))
                    run_subtone_stream go exec version || true
                    dialtone_say "Ready. You can now run plugin commands (install/build/test) via DIALTONE."
                    dialtone_say "Session subtone count: $subtone_count | Active tracked processes: $(count_tracked_processes)"
                else
                    dialtone_say "Go runtime installed, but DIALTONE_ENV/go/bin/go was not found."
                fi
                continue
            fi

            if [ "${dialtone_parts[0]:-}" = "repl" ] && [ "${dialtone_parts[1]:-}" = "install" ]; then
                dialtone_say "Starting monitored subtone for REPL plugin pixi install..."
                subtone_count=$((subtone_count + 1))
                if run_subtone_stream repl install; then
                    dialtone_say "REPL plugin pixi install completed."
                else
                    dialtone_say "REPL plugin pixi install failed."
                fi
                dialtone_say "Session subtone count: $subtone_count | Active tracked processes: $(count_tracked_processes)"
                continue
            fi

            if [ "${dialtone_parts[0]:-}" = "task" ] && [ "${dialtone_parts[1]:-}" = "--sign" ]; then
                sign_id="${dialtone_parts[2]:-}"
                if [ -z "$pending_task_id" ]; then
                    dialtone_say "No pending request to sign."
                    continue
                fi
                if [ -z "$sign_id" ]; then
                    dialtone_say "Missing task id. Use: @DIALTONE task --sign $pending_task_id"
                    continue
                fi
                if [ "$sign_id" != "$pending_task_id" ]; then
                    dialtone_say "Signature id mismatch. Expected: $pending_task_id"
                    continue
                fi

                dialtone_say "Signatures verified."
                subtone_count=$((subtone_count + 1))
                if run_subtone_stream "${pending_subtone_cmd[@]}"; then
                    dialtone_say "Subtone completed successfully."
                else
                    dialtone_say "Subtone reported a non-zero exit code."
                fi
                dialtone_say "Session subtone count: $subtone_count | Active tracked processes: $(count_tracked_processes)"
                pending_task_id=""
                pending_subtone_cmd=()
                continue
            fi

            if [ "${dialtone_parts[0]:-}" = "robot" ] && [ "${dialtone_parts[1]:-}" = "install" ] && [ "${dialtone_parts[2]:-}" = "src_v1" ]; then
                pending_task_id="$(generate_random_task_id "robot-install")"
                pending_subtone_cmd=("robot" "install" "src_v1")
                dialtone_say "Request received. Task created: \`$pending_task_id\`."
                dialtone_say "Sign with \`@DIALTONE task --sign $pending_task_id\` to run."
                continue
            fi

            dialtone_say "Unknown @DIALTONE request: $dialtone_cmd"
            continue
        fi

        if [[ "$user_input" == "@LLM-OPS "* ]]; then
            continue
        fi

        if [ "$user_input" = "dev install" ]; then
            dialtone_say "Use \`@DIALTONE dev install\` to run this command."
            continue
        fi

        read -r -a cmd_parts <<< "$user_input"
        dialtone_say "Running: ${cmd_parts[*]}"
        if "$SCRIPT_DIR/dialtone.sh" "${cmd_parts[@]}" > >(while IFS= read -r cmd_line; do
            echo "$cmd_line"
            repl_log_event "$repl_session_id" "DIALTONE" "$cmd_line" "OUTPUT"
        done) 2> >(while IFS= read -r cmd_err; do
            echo "$cmd_err" >&2
            repl_log_event "$repl_session_id" "DIALTONE" "$cmd_err" "OUTPUT"
        done); then
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

repl_usage() {
    cat <<EOF_REPL
Usage: ./dialtone.sh repl [options]

Options:
  --stdin            Read commands from stdin without interactive prompt.
  --script <file>    Read commands from a script file.
  --echo-input       Echo stdin/script lines as "USER-1> <line>".
  --role <name>      Set initial speaker role label (default: USER-1).
  -h, --help         Show this help.

Examples:
  ./dialtone.sh repl --env env/test.env
  ./dialtone.sh repl --stdin --env env/test.env <<'EOF'
@DIALTONE dev install
@DIALTONE robot install src_v1
EOF
  ./dialtone.sh repl --script ./scripts/repl-smoke.txt --echo-input
  ./dialtone.sh repl --role LLM-OPS
EOF_REPL
}

repl_send_usage() {
    cat <<EOF_REPL_SEND
Usage: ./dialtone.sh repl-send [options] <message>

Options:
  --role <name>       Speaker role label (default: LLM-OPS)
  --session <id>      Target specific REPL session id
  -h, --help          Show this help

Examples:
  ./dialtone.sh repl-send --role LLM-OPS "Hello from external controller."
  ./dialtone.sh repl-send --session 531583 "status check"
EOF_REPL_SEND
}

repl_control_start() {
    local session_id="$1"
    REPL_CONTROL_FIFO="$RUNTIME_DIR/repl-control-${session_id}.fifo"
    REPL_ACTIVE_FILE="$RUNTIME_DIR/repl-active.session"
    mkdir -p "$RUNTIME_DIR"
    rm -f "$REPL_CONTROL_FIFO"
    mkfifo "$REPL_CONTROL_FIFO"
    echo "$session_id" > "$REPL_ACTIVE_FILE"

    (
        while true; do
            if IFS= read -r control_line < "$REPL_CONTROL_FIFO"; then
                [ -z "$control_line" ] && continue
                repl_async_emit "$control_line"
            fi
        done
    ) &
    REPL_CONTROL_PID=$!
}

repl_control_stop() {
    if [ -n "${REPL_CONTROL_PID:-}" ]; then
        kill "$REPL_CONTROL_PID" 2>/dev/null || true
    fi
    if [ -n "${REPL_CONTROL_FIFO:-}" ]; then
        rm -f "$REPL_CONTROL_FIFO"
    fi
}

repl_bus_start() {
    local session_id="$1"
    local events_file="$RUNTIME_DIR/repl-events.log"
    local start_bytes
    mkdir -p "$RUNTIME_DIR"
    touch "$events_file"
    start_bytes="$(wc -c < "$events_file" 2>/dev/null || echo 0)"
    start_bytes="${start_bytes//[[:space:]]/}"
    [ -z "$start_bytes" ] && start_bytes=0

    (
        tail -c "+$((start_bytes + 1))" -F "$events_file" 2>/dev/null | while IFS=$'\t' read -r ts sid kind role text; do
            [ "$sid" = "$session_id" ] && continue
            [ -z "$text" ] && continue
            case "$kind" in
                INPUT)
                    [ -z "$role" ] && role="USER-1"
                    repl_async_emit "${role}> ${text}"
                    ;;
                OUTPUT)
                    repl_async_emit "$text"
                    ;;
            esac
        done
    ) &
    REPL_BUS_PID=$!
}

repl_bus_stop() {
    if [ -n "${REPL_BUS_PID:-}" ]; then
        kill "$REPL_BUS_PID" 2>/dev/null || true
    fi
}

repl_runtime_cleanup() {
    repl_control_stop
    repl_bus_stop
}

repl_send_message() {
    local role="LLM-OPS"
    local session_id=""
    local message=""
    local i=0
    while [ $i -lt ${#CMD_ARGS[@]} ]; do
        case "${CMD_ARGS[$i]}" in
            --role)
                i=$((i + 1))
                role="${CMD_ARGS[$i]:-}"
                ;;
            --session)
                i=$((i + 1))
                session_id="${CMD_ARGS[$i]:-}"
                ;;
            -h|--help|help)
                repl_send_usage
                return 0
                ;;
            *)
                if [ -n "$message" ]; then
                    message="$message ${CMD_ARGS[$i]}"
                else
                    message="${CMD_ARGS[$i]}"
                fi
                ;;
        esac
        i=$((i + 1))
    done

    if [ -z "$message" ]; then
        echo "Error: repl-send requires a message."
        repl_send_usage
        return 1
    fi

    if [ -z "$session_id" ]; then
        if [ -f "$RUNTIME_DIR/repl-active.session" ]; then
            session_id="$(cat "$RUNTIME_DIR/repl-active.session" 2>/dev/null || true)"
        fi
    fi
    if [ -z "$session_id" ]; then
        echo "Error: no active REPL session found. Start ./dialtone.sh first."
        return 1
    fi

    fifo="$RUNTIME_DIR/repl-control-${session_id}.fifo"
    if [ ! -p "$fifo" ]; then
        echo "Error: REPL control channel not found for session $session_id."
        echo "Restart REPL with latest dialtone.sh and try again."
        return 1
    fi

    printf "%s> %s\n" "$role" "$message" > "$fifo"
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
# Preserve inherited selection so subtones can reuse the same env file.
DIALTONE_ENV_FILE="${DIALTONE_ENV_FILE:-}"
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

DIALTONE_ENV="$(normalize_dialtone_env "${DIALTONE_ENV:-}")"

# Ensure nested dialtone.sh invocations (subtones) inherit selected env config.
export DIALTONE_ENV_FILE
export DIALTONE_ENV

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

if [ "$CMD" = "repl" ]; then
    if [ "${CMD_ARGS[0]:-}" = "install" ]; then
        repl_plugin_install "${CMD_ARGS[@]:1}"
        exit $?
    fi

    REPL_AGENT_MODE=0
    REPL_ECHO_INPUT=0
    REPL_SCRIPT_FILE=""
    REPL_ROLE="${REPL_ROLE:-USER-1}"

    idx=0
    while [ $idx -lt ${#CMD_ARGS[@]} ]; do
        arg="${CMD_ARGS[$idx]}"
        case "$arg" in
            --stdin|--agent)
                REPL_AGENT_MODE=1
                ;;
            --echo-input)
                REPL_ECHO_INPUT=1
                ;;
            --role)
                idx=$((idx + 1))
                REPL_ROLE="${CMD_ARGS[$idx]:-}"
                if [ -z "$REPL_ROLE" ]; then
                    echo "Error: --role requires a role name"
                    exit 1
                fi
                ;;
            --script)
                idx=$((idx + 1))
                REPL_SCRIPT_FILE="${CMD_ARGS[$idx]:-}"
                if [ -z "$REPL_SCRIPT_FILE" ]; then
                    echo "Error: --script requires a file path"
                    exit 1
                fi
                ;;
            -h|--help|help)
                repl_usage
                exit 0
                ;;
            *)
                echo "Unknown repl option: $arg"
                repl_usage
                exit 1
                ;;
        esac
        idx=$((idx + 1))
    done

    if [ -n "$REPL_SCRIPT_FILE" ]; then
        REPL_AGENT_MODE=1
        if [ ! -f "$REPL_SCRIPT_FILE" ]; then
            echo "Error: repl script not found: $REPL_SCRIPT_FILE"
            exit 1
        fi
        start_repl < "$REPL_SCRIPT_FILE"
    else
        start_repl
    fi
    exit 0
fi

if [ "$CMD" = "repl-send" ]; then
    repl_send_message
    exit $?
fi

if [ "$CMD" = "repls" ]; then
    list_repl_sessions
    exit $?
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

# Everything else runs through managed Go toolchain.
DIALTONE_ENV="$(normalize_dialtone_env "${DIALTONE_ENV:-}")"

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

    (
        cd "$SCRIPT_DIR/src" && "$go_cmd" run ./cmd/dev/main.go "$@"
    ) > >(tee -a "$log_file") \
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

    rm -f "$pid_file" "$meta_file"
    return "$exit_code"
}

run_tool "$GO_BIN" "$CMD" "${CMD_ARGS[@]}"
exit $?
