#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_DIR="$ROOT_DIR/.dialtone/run"
EVENTS_FILE="$RUNTIME_DIR/repl-events.log"
BRIDGE_BIN="${DIALTONE_LLM_OPS_BIN:-$ROOT_DIR/scripts/llm_ops_demo_bridge.sh}"
SESSION_ID="${1:-}"
ACTIVE_SESSION_FILE="$RUNTIME_DIR/repl-active.session"
EXTERNAL_MARKER_FILE="$RUNTIME_DIR/repl-llm-ops.external"
FOLLOW_ACTIVE=0

echo "[repl-reactive-bridge] LLM-OPS auto-reply is disabled."
exit 0

if [ -z "$SESSION_ID" ]; then
  FOLLOW_ACTIVE=1
fi

if [ ! -x "$BRIDGE_BIN" ]; then
  echo "Bridge executable not found: $BRIDGE_BIN"
  exit 1
fi

mkdir -p "$RUNTIME_DIR"
touch "$EVENTS_FILE"
echo "active" > "$EXTERNAL_MARKER_FILE"
cleanup() {
  rm -f "$EXTERNAL_MARKER_FILE"
}
trap cleanup EXIT INT TERM

if [ "$FOLLOW_ACTIVE" -eq 1 ]; then
  echo "[repl-reactive-bridge] watching current active REPL session (auto-follow)."
else
  echo "[repl-reactive-bridge] watching session: $SESSION_ID"
fi
echo "[repl-reactive-bridge] bridge: $BRIDGE_BIN"

emit_llm_ops_output() {
  local line="$1"
  local now
  now="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  printf "%s\t%s\tOUTPUT\tLLM-OPS\tLLM-OPS> %s\n" "$now" "LLMOPS-BRIDGE" "$line" >> "$EVENTS_FILE"
}

has_local_llm_reply_after() {
  local sid="$1"
  local input_ts="$2"
  awk -F'\t' -v sid="$sid" -v input_ts="$input_ts" '
    $2 == sid && $3 == "OUTPUT" && $1 >= input_ts && $5 ~ /^LLM-OPS> / { found = 1 }
    END { exit(found ? 0 : 1) }
  ' "$EVENTS_FILE"
}

tail -n 0 -F "$EVENTS_FILE" | while IFS=$'\t' read -r ts sid kind role text; do
  [ "$kind" != "INPUT" ] && continue
  if [ "$FOLLOW_ACTIVE" -eq 1 ]; then
    [ -f "$ACTIVE_SESSION_FILE" ] || continue
    current_active_session="$(cat "$ACTIVE_SESSION_FILE" 2>/dev/null || true)"
    [ -n "$current_active_session" ] || continue
    [ "$sid" = "$current_active_session" ] || continue
  else
    [ "$sid" = "$SESSION_ID" ] || continue
  fi
  case "$text" in
    "@LLM-OPS "*)
      question="${text#@LLM-OPS }"
      [ -z "$question" ] && continue
      # If the local REPL handler already replied, do not duplicate it.
      sleep 0.5
      if has_local_llm_reply_after "$sid" "$ts"; then
        continue
      fi
      if output="$("$BRIDGE_BIN" "$question" 2>&1)"; then
        while IFS= read -r line; do
          [ -z "$line" ] && continue
          emit_llm_ops_output "$line"
          sleep 0.2
        done <<< "$output"
      else
        emit_llm_ops_output "Bridge error: $output"
      fi
      ;;
  esac
done
