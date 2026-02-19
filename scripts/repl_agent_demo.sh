#!/usr/bin/env bash
set -euo pipefail

# Demo: one live REPL session, reactive agent behavior.
# - Starts REPL once
# - Waits for responses
# - Intentionally sends a bad sign id
# - Reads error, then sends corrected sign id
# - Requests status and exits

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "AGENT> Starting live REPL session..."
coproc REPL_PROC { ./dialtone.sh repl --env env/test.env; }

repl_in="${REPL_PROC[1]}"
repl_out="${REPL_PROC[0]}"
task_id=""
sent_bad_sign=0
sent_good_sign=0
requested_status=0

send_line() {
    local line="$1"
    echo "AGENT> $line"
    printf "%s\n" "$line" >&"${repl_in}"
}

# Read loop: react to DIALTONE output in real time.
while IFS= read -r -t 2 line <&"${repl_out}"; do
    echo "$line"

    if [[ "$line" == *"Type 'help' for commands"* ]]; then
        sleep 1
        send_line "@DIALTONE robot install src_v1"
        continue
    fi

    if [[ "$line" == *"Task created:"* ]] && [[ -z "$task_id" ]]; then
        task_id="$(echo "$line" | awk -F'`' '{print $2}')"
        if [[ -n "$task_id" ]]; then
            sleep 1
            send_line "@DIALTONE task --sign wrong-id"
            sent_bad_sign=1
        fi
        continue
    fi

    if [[ "$line" == *"Signature id mismatch."* ]] && [[ "$sent_bad_sign" -eq 1 ]] && [[ "$sent_good_sign" -eq 0 ]]; then
        sleep 1
        send_line "@DIALTONE task --sign $task_id"
        sent_good_sign=1
        continue
    fi

    if [[ "$line" == *"Subtone completed successfully."* ]] && [[ "$requested_status" -eq 0 ]]; then
        sleep 1
        send_line "status"
        sleep 1
        send_line "exit"
        requested_status=1
        continue
    fi

    if [[ "$line" == *"Goodbye."* ]]; then
        break
    fi
done

# Close writer and wait for process.
exec {repl_in}>&-
wait "${REPL_PROC_PID}"
echo "AGENT> Demo complete."
