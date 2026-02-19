#!/usr/bin/env bash
set -euo pipefail

question="${1:-}"

if [ -z "$question" ]; then
  echo "Please send a question after @LLM-OPS."
  exit 0
fi

case "$question" in
  *status*|*state*)
    echo "I can help you inspect REPL state. Run: status"
    ;;
  *install*robot*)
    echo "Use: @DIALTONE robot install src_v1"
    echo "Then sign with: @DIALTONE task --sign <task-id>"
    ;;
  *tallest*building*)
    echo "The tallest building is the Burj Khalifa in Dubai (828 m)."
    ;;
  *)
    echo "I can help with that. Ask me for commands, troubleshooting, or quick facts."
    ;;
esac
