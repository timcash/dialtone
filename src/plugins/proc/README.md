# proc Plugin

Managed process/subtone library + CLI.

Provides:
- `proc src_v1 sleep <seconds>`
- `proc src_v1 emit <line>`
- `proc src_v1 test`
- `proc src_v1 list` (or `ps`) with CPU/memory/port metrics
- `proc src_v1 kill <pid>`

The `proc` library is REPL-agnostic:
- it tracks managed dialtone processes
- it exposes list/kill APIs with normalized process metrics
- callers (like REPL) own the user-facing stdout format/prefixes
