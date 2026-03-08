#!/usr/bin/env bash
set -euo pipefail

HOST="${HOST:-legion}"
ROLE="${ROLE:-robot-test}"
BASE_URL="${BASE_URL:-http://127.0.0.1:3000}"
STAMP="$(date +%s)"
TARGET_URL="${BASE_URL}/?cli=${STAMP}#robot-three-stage"

echo "[robot-src_v2] driving Three -> System -> Arm on host=${HOST} role=${ROLE}"

./dialtone.sh chrome src_v3 goto --host "${HOST}" --role "${ROLE}" --url "${TARGET_URL}"
./dialtone.sh chrome src_v3 wait-aria --host "${HOST}" --role "${ROLE}" --label "Three Section" --timeout-ms 8000
./dialtone.sh chrome src_v3 click-aria --host "${HOST}" --role "${ROLE}" --label "Three Mode"
./dialtone.sh chrome src_v3 click-aria --host "${HOST}" --role "${ROLE}" --label "Three Thumb 1"
./dialtone.sh chrome src_v3 wait-log --host "${HOST}" --role "${ROLE}" --contains "Publishing rover.command cmd=arm" --timeout-ms 5000
./dialtone.sh chrome src_v3 get-url --host "${HOST}" --role "${ROLE}"

