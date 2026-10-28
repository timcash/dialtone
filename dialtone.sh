#!/bin/sh
# dialtone.sh: the dev tasks in xtask/ (`./dialtone.sh help`). Needs only cargo.
cd "$(dirname "$0")" && exec cargo run -q -p xtask -- "$@"
