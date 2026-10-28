#!/bin/sh
# dialtone.sh: the dev tasks in xtask/ (`./dialtone.sh help`). Needs only cargo (rustup's bin is
# added to PATH, for shells that do not have it).
PATH="${CARGO_HOME:-$HOME/.cargo}/bin:$PATH"
cd "$(dirname "$0")" && exec cargo run -q -p xtask -- "$@"
