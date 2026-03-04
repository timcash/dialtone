#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

NIX_BIN=""
NIX_FLAGS=(--extra-experimental-features "nix-command")
NIXPKGS_URL="${NIXPKGS_URL:-https://channels.nixos.org/nixpkgs-unstable/nixexprs.tar.xz}"
NIX_PKGS=(bashInteractive openssh go git tmux tailscale)
SSH_COMMON_OPTS=( -F /dev/null
  -o BatchMode=yes
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
  -o GSSAPIAuthentication=no
)
ENV_FILE="${SCRIPT_DIR}/env/.env"
ENV_FILE_EXPLICIT=0
REMOTE_HOST=""
REMOTE_HOST_SET=0
TMUX_SESSION_PREFIX="dialtone-"

normalize_host() {
  local host="${1:-}"
  host="$(printf '%s' "$host" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9._-')"
  if [ -z "$host" ]; then
    host="$(hostname -s 2>/dev/null || echo dialtone)"
  fi
  if [ -z "$host" ]; then
    host="dialtone"
  fi
  echo "$host"
}

tmux_session_for_host() {
  local host="${1:-}"
  host="$(normalize_host "$host")"
  echo "${TMUX_SESSION_PREFIX}${host}"
}

run_tsnet_bootstrap() {
  local host
  host="$(normalize_host "${DIALTONE_HOSTNAME:-}")"
  local args=(tsnet v1 bootstrap --host "$host")
  if [ -n "${ENV_FILE:-}" ]; then
    args+=(--env-file "$ENV_FILE")
  fi
  if ! "$NIX_BIN" "${NIX_FLAGS[@]}" shell -f "$NIXPKGS_URL" "${NIX_PKGS[@]}" --command go run ./src/cli.go "${args[@]}"; then
    echo "warning: tsnet bootstrap failed; continuing without automatic tsnet keepalive" >&2
    return 1
  fi
}

run_remote_exec() {
  local host="$1"
  shift
  local cmd="$*"
  if [ -z "$cmd" ]; then
    return 1
  fi

  local payload
  payload="$(printf '%s' "$cmd" | base64 | tr -d '\n')"
  local remote_cmd="printf '%s' '$payload' | base64 -d | bash -se"
  local mosh_ssh_cmd=(ssh)
  local mosh_ssh_arg=()
  local arg
  for arg in "${SSH_COMMON_OPTS[@]}"; do
    mosh_ssh_arg+=("$arg")
  done
  mosh_ssh_cmd+=("${mosh_ssh_arg[@]}")
  local mosh_ssh_string
  mosh_ssh_string="$(printf '%s ' "${mosh_ssh_cmd[@]}" | sed 's/ $//')"

  if command -v mosh >/dev/null 2>&1; then
    if mosh "$host" --ssh "$mosh_ssh_string" -- bash -lc "$remote_cmd"; then
      return 0
    fi
    echo "mosh failed for $host; falling back to ssh" >&2
  fi

  ssh "${SSH_COMMON_OPTS[@]}" "$host" "bash -lc $(printf '%q' "$remote_cmd")"
}

run_remote_dialtone_command() {
  local host="$1"
  shift
  local args=("$@")

  local remote_host
  remote_host="$(tmux_session_for_host "$host")"
  remote_host="${remote_host#${TMUX_SESSION_PREFIX}}"

  local quoted=()
  local i
  for i in "${args[@]}"; do
    quoted+=("$(printf '%q' "$i")")
  done
  local arg_line
  if [ "${#quoted[@]}" -eq 0 ]; then
    arg_line=""
  else
    arg_line="${quoted[*]}"
  fi

  local remote_cmd
  remote_cmd="DIALTONE_HOSTNAME=$(printf '%q' "${remote_host}") "
  if [ -n "${DIALTONE_REPO_ROOT:-}" ]; then
    remote_cmd+="DIALTONE_REPO_ROOT=$(printf '%q' "${DIALTONE_REPO_ROOT}") "
  fi
  if [ "${ENV_FILE_EXPLICIT}" -eq 1 ]; then
    remote_cmd+="DIALTONE_ENV_FILE=$(printf '%q' "${ENV_FILE}") "
  elif [ -n "${DIALTONE_ENV_FILE:-}" ]; then
    remote_cmd+="DIALTONE_ENV_FILE=$(printf '%q' "${DIALTONE_ENV_FILE}") "
  fi
  remote_cmd+='set -e
repo_root="${DIALTONE_REMOTE_REPO_ROOT:-${DIALTONE_REPO_ROOT}}"
if [ -z "${repo_root}" ]; then
  for p in "$HOME/dialtone" "/home/user/dialtone" "/Users/user/dialtone" "/home/tim/dialtone" "/Users/tim/dialtone"; do
    if [ -d "$p" ]; then
      repo_root="$p"
      break
    fi
  done
fi
if [ -z "${repo_root}" ] || [ ! -d "${repo_root}" ]; then
  echo "remote repo root not found; set DIALTONE_REMOTE_REPO_ROOT" >&2
  exit 1
fi
cd "$repo_root"
./dialtone2.sh'
  if [ -n "$arg_line" ]; then
    remote_cmd="${remote_cmd} ${arg_line}"
  fi

  run_remote_exec "$host" "$remote_cmd"
}

run_tmux_v1_logs() {
  local lines=10
  local args=("$@")

  local i=0
  while [ "$i" -lt "${#args[@]}" ]; do
    case "${args[$i]}" in
      --lines)
        i=$((i + 1))
        if [ "$i" -ge "${#args[@]}" ]; then
          echo "tmux logs requires a value for --lines" >&2
          exit 1
        fi
        lines="${args[$i]}"
        ;;
      --lines=*)
        lines="${args[$i]#--lines=}"
        ;;
      *)
        ;;
    esac
    i=$((i + 1))
  done

  if ! command -v tmux >/dev/null 2>&1; then
    echo "tmux is not available in this shell" >&2
    exit 1
  fi

  local session
  session="$(tmux_session_for_host "${DIALTONE_HOSTNAME:-}")"
  local target="${session}:0.0"
  if ! tmux has-session -t "$session" 2>/dev/null; then
    echo "tmux session not found: $session" >&2
    exit 1
  fi
  if ! tmux capture-pane -pt "$target" -S "-$lines"; then
    local first_pane
    first_pane="$(tmux list-panes -t "$session" -F '#{window_index}.#{pane_index}' | head -n1 || true)"
    if [ -n "$first_pane" ]; then
      target="${session}:${first_pane}"
      tmux capture-pane -pt "$target" -S "-$lines"
      return $?
    fi
    echo "tmux session $session has no panes" >&2
    exit 1
  fi
}

parse_args() {
  PARSED_ARGS=()
  local command_started=0
  while [ "$#" -gt 0 ]; do
    if [ "$command_started" -eq 1 ]; then
      PARSED_ARGS+=("$@")
      break
    fi
    case "$1" in
      --env)
        if [ "$#" -lt 2 ]; then
          echo "missing value for --env" >&2
          exit 1
        fi
        ENV_FILE="$2"
        ENV_FILE_EXPLICIT=1
        shift 2
        ;;
      --env=*)
        ENV_FILE="${1#--env=}"
        ENV_FILE_EXPLICIT=1
        shift
        ;;
      --host)
        if [ "$#" -lt 2 ]; then
          echo "missing value for --host" >&2
          exit 1
        fi
        REMOTE_HOST="$2"
        REMOTE_HOST_SET=1
        shift 2
        ;;
      --host=*)
        REMOTE_HOST="${1#--host=}"
        REMOTE_HOST_SET=1
        shift
        ;;
      --)
        shift
        if [ "$#" -gt 0 ]; then
          PARSED_ARGS+=("$@")
        fi
        break
        ;;
      *)
        PARSED_ARGS+=("$1")
        command_started=1
        shift
        ;;
    esac
  done
}

load_env_file() {
  local env_path="${1:-$ENV_FILE}"
  if [ ! -f "$env_path" ]; then
    return 0
  fi
  set -a
  . "$env_path"
  set +a
}

find_nix() {
  if command -v nix >/dev/null 2>&1; then
    command -v nix
    return 0
  fi

  local p=""
  for p in "/nix/var/nix/profiles/default/bin/nix" "$HOME/.nix-profile/bin/nix" "/run/current-system/sw/bin/nix"; do
    if [ -x "$p" ]; then
      echo "$p"
      return 0
    fi
  done

  p="$(ls -1d /nix/store/*-nix-*/bin/nix 2>/dev/null | sort | tail -n1 || true)"
  if [ -n "$p" ] && [ -x "$p" ]; then
    echo "$p"
    return 0
  fi

  return 1
}

PARSED_ARGS=()
parse_args "$@"
set -- "${PARSED_ARGS[@]}"
if [ "$ENV_FILE_EXPLICIT" -eq 0 ] && [ -n "${DIALTONE_ENV_FILE:-}" ]; then
  ENV_FILE="$DIALTONE_ENV_FILE"
fi
if [ "$ENV_FILE_EXPLICIT" -eq 0 ] && [ -n "${DIALTONE_ENV_PATH:-}" ]; then
  ENV_FILE="$DIALTONE_ENV_PATH"
fi
if [ "$ENV_FILE_EXPLICIT" -eq 1 ] && [ ! -f "$ENV_FILE" ]; then
  echo "env file not found: $ENV_FILE" >&2
  exit 1
fi
if [ -f "$ENV_FILE" ]; then
  load_env_file
  export DIALTONE_ENV_FILE="$ENV_FILE"
fi

if ! NIX_BIN="$(find_nix)"; then
  echo "nix is required" >&2
  exit 1
fi

if [ "${1:-}" != "tsnet" ] || [ "${2:-}" != "v1" ] || [ "${3:-}" != "bootstrap" ]; then
  run_tsnet_bootstrap || true
fi

if [ "$REMOTE_HOST_SET" -eq 1 ] && [ "$#" -eq 0 ]; then
  run_remote_dialtone_command "$REMOTE_HOST"
  exit $?
fi

if [ "$REMOTE_HOST_SET" -eq 1 ]; then
  run_remote_dialtone_command "$REMOTE_HOST" "$@"
  exit $?
fi

if [ $# -eq 0 ]; then
  SESSION_NAME="$(tmux_session_for_host "${DIALTONE_HOSTNAME:-}")"
  TMUX_START_COMMAND='if command -v tmux >/dev/null 2>&1 && [ -z "${TMUX:-}" ]; then if ! tmux has-session -t "${TMUX_SESSION_NAME}" 2>/dev/null; then tmux new-session -ds "${TMUX_SESSION_NAME}" -n "${TMUX_SESSION_NAME}"; fi; exec tmux attach-session -t "${TMUX_SESSION_NAME}"; else exec bash -i; fi'
  exec "$NIX_BIN" "${NIX_FLAGS[@]}" shell -f "$NIXPKGS_URL" "${NIX_PKGS[@]}" --command env IN_NIX_SHELL=1 TMUX_SESSION_NAME="$SESSION_NAME" bash -lc "$TMUX_START_COMMAND"
fi

if [ "$1" = "tmux" ] && [ "${2:-}" = "v1" ] && [ "${3:-}" = "logs" ]; then
  shift 3
  run_tmux_v1_logs "$@"
  exit $?
fi

exec "$NIX_BIN" "${NIX_FLAGS[@]}" shell -f "$NIXPKGS_URL" "${NIX_PKGS[@]}" --command env IN_NIX_SHELL=1 go run ./src/cli.go "$@"
