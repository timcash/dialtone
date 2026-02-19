#!/usr/bin/env bash
set -euo pipefail

QUESTION_FILE="${1:-/home/user/dialtone/question.md}"
REPLY_FILE="${2:-/home/user/dialtone/reply.md}"
INTERVAL_SECONDS="${3:-10}"

answer_for() {
  local q_raw="$1"
  local q
  q="$(printf "%s" "$q_raw" | tr '[:upper:]' '[:lower:]')"

  case "$q" in
    *"what is in a bannana cake"*|*"what is in a banana cake"*)
      echo "Typical banana cake ingredients are mashed bananas, flour, sugar, eggs, butter (or oil), baking powder, and milk."
      ;;
    *"how many people on earth"*|*"earth population"*)
      echo "Roughly 8.1 billion people (current estimate)."
      ;;
    *"who am i"*)
      echo "You are the user running this Dialtone workspace and editing question files."
      ;;
    *)
      echo "I do not have a specific answer for this yet."
      ;;
  esac
}

write_reply() {
  local ts line idx answer
  ts="$(date -u +"%Y-%m-%d %H:%M:%SZ")"

  {
    printf "# Replies\n"
    printf "_Updated: %s_\n\n" "$ts"
    idx=0
    while IFS= read -r line || [ -n "$line" ]; do
      line="$(printf "%s" "$line" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
      [ -z "$line" ] && continue
      [[ "$line" == \#* ]] && continue
      idx=$((idx + 1))
      answer="$(answer_for "$line")"
      printf "%d. Q: %s\n" "$idx" "$line"
      printf "   A: %s\n" "$answer"
    done < "$QUESTION_FILE"
  } > "$REPLY_FILE"
}

echo "[question-responder-sh] watching: $QUESTION_FILE"
echo "[question-responder-sh] writing:  $REPLY_FILE"
echo "[question-responder-sh] interval: ${INTERVAL_SECONDS}s"

last_hash=""
while true; do
  if [ -f "$QUESTION_FILE" ]; then
    current_hash="$(sha256sum "$QUESTION_FILE" | awk '{print $1}')"
    if [ "$current_hash" != "$last_hash" ]; then
      write_reply
      echo "[question-responder-sh] updated reply at $(date -u +"%Y-%m-%d %H:%M:%SZ")"
      last_hash="$current_hash"
    fi
  fi
  sleep "$INTERVAL_SECONDS"
done
