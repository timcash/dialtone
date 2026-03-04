#!/usr/bin/env bash
set -euo pipefail

HOST="${TMUX_PORTAL_HOST:-gold}"
SSH_CONFIG="${TMUX_PORTAL_SSH_CONFIG:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/env/ssh_config}"
TARGET="${TMUX_PORTAL_TARGET:-}"
LINES="${TMUX_PORTAL_LINES:-80}"

usage() {
  cat <<'EOF'
usage:
  tmux-portal.sh [--host HOST] [--target session:win.pane] list
  tmux-portal.sh [--host HOST] [--target session:win.pane] read [lines]
  tmux-portal.sh [--host HOST] [--target session:win.pane] send "<command>"
  tmux-portal.sh [--host HOST] [--target session:win.pane] clear
  tmux-portal.sh [--host HOST] [--target session:win.pane] interrupt

env:
  TMUX_PORTAL_HOST
  TMUX_PORTAL_SSH_CONFIG
  TMUX_PORTAL_TARGET
  TMUX_PORTAL_LINES
EOF
}

ssh_cmd() {
  if [[ -f "$SSH_CONFIG" ]]; then
    ssh -F "$SSH_CONFIG" "$HOST" "$@"
  else
    ssh "$HOST" "$@"
  fi
}

remote_tmux() {
  local remote="$1"
  ssh_cmd "TMUXBIN=\$(command -v tmux || ls -1d /nix/store/*-tmux-*/bin/tmux 2>/dev/null | sort | tail -n1); if [ -z \"\$TMUXBIN\" ]; then echo 'tmux not found' >&2; exit 1; fi; \"\$TMUXBIN\" $remote"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)
      HOST="$2"
      shift 2
      ;;
    --target)
      TARGET="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      break
      ;;
  esac
done

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

cmd="$1"
shift

case "$cmd" in
  list)
    remote_tmux "list-panes -a -F '#{session_name}:#{window_index}.#{pane_index} active=#{pane_active} path=#{pane_current_path} cmd=#{pane_current_command}'"
    ;;
  read)
    if [[ -n "${1:-}" ]]; then
      LINES="$1"
    fi
    if [[ -z "$TARGET" ]]; then
      TARGET="$(remote_tmux "list-panes -a -F '#{session_name}:#{window_index}.#{pane_index} #{pane_active}' | awk '\$2==1{print \$1; exit}'")"
    fi
    if [[ -z "$TARGET" ]]; then
      echo "no tmux pane found" >&2
      exit 1
    fi
    remote_tmux "capture-pane -pt '$TARGET' -S -$LINES"
    ;;
  send)
    if [[ $# -lt 1 ]]; then
      echo "send requires a command string" >&2
      exit 1
    fi
    if [[ -z "$TARGET" ]]; then
      TARGET="$(remote_tmux "list-panes -a -F '#{session_name}:#{window_index}.#{pane_index} #{pane_active}' | awk '\$2==1{print \$1; exit}'")"
    fi
    if [[ -z "$TARGET" ]]; then
      echo "no tmux pane found" >&2
      exit 1
    fi
    remote_tmux "send-keys -t '$TARGET' \"${1//\"/\\\"}\" Enter"
    ;;
  clear)
    if [[ -z "$TARGET" ]]; then
      TARGET="$(remote_tmux "list-panes -a -F '#{session_name}:#{window_index}.#{pane_index} #{pane_active}' | awk '\$2==1{print \$1; exit}'")"
    fi
    if [[ -z "$TARGET" ]]; then
      echo "no tmux pane found" >&2
      exit 1
    fi
    remote_tmux "send-keys -t '$TARGET' C-l"
    ;;
  interrupt)
    if [[ -z "$TARGET" ]]; then
      TARGET="$(remote_tmux "list-panes -a -F '#{session_name}:#{window_index}.#{pane_index} #{pane_active}' | awk '\$2==1{print \$1; exit}'")"
    fi
    if [[ -z "$TARGET" ]]; then
      echo "no tmux pane found" >&2
      exit 1
    fi
    remote_tmux "send-keys -t '$TARGET' C-c"
    ;;
  *)
    usage
    exit 1
    ;;
esac
