#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status
set -e

# Usage: ./codex.sh [reasoning_level]
# Example: ./codex.sh high

REASONING=${1:-medium}

echo "Starting Codex CLI with gpt-5.4 (reasoning: $REASONING) and skipping confirmations..."
npx @openai/codex --model gpt-5.4 --reasoning "$REASONING" -c approval_mode="never" "${@:2}"
