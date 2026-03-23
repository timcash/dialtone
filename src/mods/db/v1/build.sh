#!/usr/bin/env bash
# Move the zig cache outside the repo to prevent it from crashing the context window
export ZIG_LOCAL_CACHE_DIR="/tmp/dialtone_zig_cache/local"
export ZIG_GLOBAL_CACHE_DIR="/tmp/dialtone_zig_cache/global"

# Run zig build passing any arguments
zig build "$@"
