#!/usr/bin/env bash
set -u

ROVER_HOST="${ROVER_HOST:-rover-1.shad-artichoke.ts.net}"
ROVER_USER="${ROVER_USER:-tim}"
LOCAL_REPO="${LOCAL_REPO:-/home/user/dialtone}"
REMOTE_SRC="${REMOTE_SRC:-/home/tim/dialtone/src}"
INTERVAL="${INTERVAL:-2}"
ROBOT_VERSION="${ROBOT_VERSION:-src_v2}"

common_args=(
  -az
  --delete
  --exclude ".git"
  --exclude "node_modules"
  --exclude "dist"
  --exclude ".pixi"
  --exclude ".cache"
)

sync_one() {
  local rel="$1"
  rsync "${common_args[@]}" "$LOCAL_REPO/src/$rel" "$ROVER_USER@$ROVER_HOST:$REMOTE_SRC/${rel%/*}/"
}

while true; do
  {
    rsync "${common_args[@]}" "$LOCAL_REPO/src/go.mod" "$ROVER_USER@$ROVER_HOST:$REMOTE_SRC/"
    rsync "${common_args[@]}" "$LOCAL_REPO/src/go.sum" "$ROVER_USER@$ROVER_HOST:$REMOTE_SRC/"

    sync_one "plugins/robot/$ROBOT_VERSION/"
    sync_one "plugins/robot/scaffold/main.go"
    sync_one "plugins/mavlink/"
    sync_one "plugins/camera/"
    sync_one "plugins/logs/"
    sync_one "plugins/ui/src_v1/ui/"
  } || true

  sleep "$INTERVAL"
done
