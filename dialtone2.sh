#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

NIX_BIN=""
NIX_FLAGS=(--extra-experimental-features "nix-command")
NIXPKGS_URL="${NIXPKGS_URL:-https://channels.nixos.org/nixpkgs-unstable/nixexprs.tar.xz}"
NIX_PKGS=(bashInteractive openssh go git tmux tailscale codex)
SSH_COMMON_OPTS=( -F /dev/null
  -o BatchMode=yes
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
  -o LogLevel=ERROR
)
ENV_FILE="${SCRIPT_DIR}/env/.env"
ENV_FILE_EXPLICIT=0
REMOTE_HOST=""
REMOTE_HOST_SET=0
TMUX_SESSION_PREFIX="dialtone-"
DEFAULT_LOCAL_REPO_ROOT="${SCRIPT_DIR}"
DIALTONE_REPO_ROOT="${DIALTONE_REPO_ROOT:-$DEFAULT_LOCAL_REPO_ROOT}"
GO_SANITIZE_VARS=(GOROOT GOPATH GOMODCACHE GOCACHE GOENV GOMOD GOFLAGS GOOS GOARCH GOEXE GOTOOLCHAIN GOPROXY GOSUMDB GONOSUMDB GOPRIVATE GOSUMDB GOTOOLCHAIN CGO_ENABLED CGO_CFLAGS CGO_CPPFLAGS CGO_CXXFLAGS CGO_LDFLAGS CXXFLAGS CPPFLAGS CFLAGS CC CXX)

sanitize_system_go_env() {
  local var
  local path_entries
  local entry
  local filtered_path=""

  for var in "${GO_SANITIZE_VARS[@]}"; do
    unset "$var"
  done

  IFS=: read -r -a path_entries <<< "${PATH:-}"
  for entry in "${path_entries[@]}"; do
    [ -z "$entry" ] && continue
    case "$entry" in
      "/usr/local/go/bin" | "/usr/local/go/bin/")
        continue
        ;;
      *)
        if [ -z "$filtered_path" ]; then
          filtered_path="$entry"
        else
          filtered_path="${filtered_path}:$entry"
        fi
        ;;
    esac
  done
  PATH="$filtered_path"
  export PATH
}

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

mesh_config_path() {
  local candidate="${DIALTONE_MESH_CONFIG:-env/mesh.json}"
  if [ -z "$candidate" ]; then
    echo "${SCRIPT_DIR}/env/mesh.json"
    return 0
  fi
  if [ "${candidate#/}" != "$candidate" ]; then
    echo "$candidate"
  else
    echo "${SCRIPT_DIR}/${candidate}"
  fi
}

tailnet_alias_from_mesh() {
  local raw_host="${1:-}"
  local host
  host="$(normalize_host "$raw_host")"
  if [ -z "$host" ]; then
    return 1
  fi
  local mesh_file
  mesh_file="$(mesh_config_path)"
  if [ ! -f "$mesh_file" ]; then
    return 1
  fi

  if command -v python3 >/dev/null 2>&1; then
    local resolved=""
    resolved="$(python3 - "$mesh_file" "$host" <<'PY'
import json
import sys

path = sys.argv[1]
target = sys.argv[2].strip().lower().rstrip(".")

def norm(v):
    return str(v or "").strip().lower().rstrip(".")

with open(path, "r", encoding="utf-8") as fp:
    data = json.load(fp)

for node in data:
    name = norm(node.get("name"))
    aliases = [norm(a) for a in (node.get("aliases") or [])]
    if name == target or target in aliases:
        for a in aliases:
            if ".ts.net" in a:
                print(a)
                sys.exit(0)
        host = norm(node.get("host"))
        if ".ts.net" in host:
            print(host)
            sys.exit(0)

        continue

sys.exit(1)
PY
)"
    if [ -n "$resolved" ]; then
      echo "$resolved"
      return 0
    fi
  fi

  return 1
}

infer_tailnet_host() {
  local env_host
  local host
  local picked_host=""
  local from_mesh
  local candidates=()

  host="$(normalize_host "$(hostname -s 2>/dev/null || echo)")"
  if [ -n "$host" ]; then
    candidates+=("$host")
    candidates+=("${host%%.*}")
  fi

  host="$(normalize_host "${HOSTNAME:-}")"
  if [ -n "$host" ]; then
    candidates+=("$host")
    candidates+=("${host%%.*}")
  fi

  env_host="$(normalize_host "${DIALTONE_HOSTNAME:-}")"
  if [ -n "$env_host" ]; then
    candidates+=("$env_host")
  fi

  if [ "${#candidates[@]}" -eq 0 ]; then
    candidates+=("$(normalize_host "${DIALTONE_HOSTNAME:-$(hostname -s 2>/dev/null || echo dialtone)}")")
  fi

  for host in "${candidates[@]}"; do
    [ -z "$host" ] && continue
    if [ -z "$picked_host" ]; then
      picked_host="$host"
    fi
    if from_mesh="$(tailnet_alias_from_mesh "$host")" && [ -n "$from_mesh" ]; then
      echo "$from_mesh"
      return 0
    fi
  done

  local tailnet="${TS_TAILNET:-shad-artichoke.ts.net}"
  if [ -n "$picked_host" ] && [[ "$picked_host" == *".ts.net" ]]; then
    echo "$picked_host"
    return 0
  fi

  if [ -n "$picked_host" ] && [ -n "$tailnet" ] && [ "$tailnet" != "shad-artichoke.ts.net" ]; then
    echo "$picked_host.$tailnet"
    return 0
  fi

  # tailscale status from the local node often provides self DNSName.
  if command -v tailscale >/dev/null 2>&1; then
    local tail_dns
    tail_dns="$(tailscale status --json 2>/dev/null | tr -d '\n' | sed -n 's/.*"Self":[[:space:]]*{[^}]*"DNSName":"\\([^"]*\\)".*/\\1/p' | head -n1)"
    if [ -n "$tail_dns" ]; then
      echo "$tail_dns"
      return 0
    fi
  fi

  echo "$picked_host"
}

tmux_session_for_host() {
  local host="${1:-}"
  host="$(normalize_host "$host")"
  host="${host%%.*}"
  host="${host//./_}"
  echo "${TMUX_SESSION_PREFIX}${host}"
}

run_tsnet_bootstrap() {
  local host
  host="$(normalize_host "${DIALTONE_HOSTNAME:-}")"
  local args=(tsnet v1 bootstrap --host "$host")
  if [ -n "${ENV_FILE:-}" ]; then
    args+=(--env-file "$ENV_FILE")
  fi
  local tsnet_out
  if ! tsnet_out="$($NIX_BIN "${NIX_FLAGS[@]}" shell -f "$NIXPKGS_URL" "${NIX_PKGS[@]}" --command go run ./src/cli.go "${args[@]}" 2>&1)"; then
    echo "warning: tsnet bootstrap failed; continuing without automatic tsnet keepalive" >&2
    printf '%s\n' "$tsnet_out" >&2
    return 1
  fi
  if printf '%s\n' "$tsnet_out" | grep -q '^tsnet bootstrap:' && \
     ! printf '%s\n' "$tsnet_out" | grep -qiE 'native tailscale daemon|already running|already detected|already running'; then
    printf '%s\n' "$tsnet_out" | grep '^tsnet bootstrap:'
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
  local remote_repo_root=""
  local raw_remote_host

  local remote_host
  raw_remote_host="$(normalize_host "$host")"
  remote_host="${raw_remote_host}"

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
  remote_repo_root="$(resolve_mesh_repo_root "$host" || true)"
  if [ -z "${remote_repo_root}" ]; then
    remote_repo_root="${DIALTONE_REMOTE_REPO_ROOT:-}"
  fi
  if [ -z "${remote_repo_root}" ] && [ -n "${DIALTONE_REPO_ROOT:-}" ] && [ "${remote_host}" = "$(normalize_host "${DIALTONE_HOSTNAME:-}")" ]; then
    remote_repo_root="${DIALTONE_REPO_ROOT}"
  fi
  if [ -n "${remote_repo_root:-}" ]; then
    remote_cmd+="DIALTONE_REMOTE_REPO_ROOT=$(printf '%q' "${remote_repo_root}") "
  fi
  if [ "${ENV_FILE_EXPLICIT}" -eq 1 ]; then
    remote_cmd+="DIALTONE_ENV_FILE=$(printf '%q' "${ENV_FILE}") "
  elif [ -n "${DIALTONE_ENV_FILE:-}" ]; then
    remote_cmd+="DIALTONE_ENV_FILE=$(printf '%q' "${DIALTONE_ENV_FILE}") "
  fi
  remote_cmd+='set -e
repo_root="${DIALTONE_REMOTE_REPO_ROOT:-${DIALTONE_REPO_ROOT}}"
if [ -z "${repo_root}" ]; then
  repo_root="$HOME/dialtone"
fi
if [ -z "${repo_root}" ] || [ ! -d "${repo_root}" ]; then
  echo "remote repo root not found; set DIALTONE_REMOTE_REPO_ROOT" >&2
  exit 1
fi
cd "$repo_root"
./dialtone_mod'
  if [ -n "$arg_line" ]; then
    remote_cmd="${remote_cmd} ${arg_line}"
  fi

  run_remote_exec "$host" "$remote_cmd"
}

resolve_mesh_repo_root() {
  local raw_host="${1:-}"
  local host
  local mesh_file
  host="$(normalize_host "$raw_host")"
  if [ -z "$host" ]; then
    return 1
  fi

  mesh_file="$(mesh_config_path)"
  if [ ! -f "$mesh_file" ]; then
    return 1
  fi

  if command -v python3 >/dev/null 2>&1; then
    local resolved=""
    resolved="$(python3 - "$mesh_file" "$host" <<'PY'
import json
import sys

path = sys.argv[1]
target = sys.argv[2].strip().lower().rstrip(".")

with open(path, "r", encoding="utf-8") as fp:
    data = json.load(fp)

def norm(v):
    return str(v or "").strip().lower().rstrip(".")

for node in data:
    name = norm(node.get("name"))
    aliases = [norm(a) for a in (node.get("aliases") or [])]
    if name == target or target in aliases:
        if node.get("repo_candidates"):
            print(node["repo_candidates"][0])
            sys.exit(0)

print("")
PY
)"
    if [ -n "$resolved" ]; then
      echo "$resolved"
      return 0
    fi
  fi
  return 1
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

sanitize_system_go_env

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
  SESSION_NAME="$(tmux_session_for_host "$(infer_tailnet_host)")"
  TMUX_START_COMMAND='if command -v tmux >/dev/null 2>&1 && [ -z "${TMUX:-}" ]; then if ! tmux has-session -t "${TMUX_SESSION_NAME}" 2>/dev/null; then tmux new-session -ds "${TMUX_SESSION_NAME}" -n "${TMUX_SESSION_NAME}"; fi; exec tmux attach-session -t "${TMUX_SESSION_NAME}"; else exec bash -i; fi'
  exec "$NIX_BIN" "${NIX_FLAGS[@]}" shell -f "$NIXPKGS_URL" "${NIX_PKGS[@]}" --command env IN_NIX_SHELL=1 TMUX_SESSION_NAME="$SESSION_NAME" bash -lc "$TMUX_START_COMMAND"
fi

if [ "$1" = "tmux" ] && [ "${2:-}" = "v1" ]; then
  shift 2
  if [ "$#" -eq 0 ]; then
    echo "tmux command is required: ./dialtone_mod tmux v1 <command>" >&2
    exit 1
  fi
  exec "$NIX_BIN" "${NIX_FLAGS[@]}" shell -f "$NIXPKGS_URL" "${NIX_PKGS[@]}" --command env IN_NIX_SHELL=1 go run ./src/cli.go tmux v1 "$@"
  exit $?
fi

exec "$NIX_BIN" "${NIX_FLAGS[@]}" shell -f "$NIXPKGS_URL" "${NIX_PKGS[@]}" --command env IN_NIX_SHELL=1 go run ./src/cli.go "$@"
