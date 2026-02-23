# Plugin Guide

This file is the short, LLM-first contract for working in `src/plugins`.

## Core Rules
1. Use versioned commands: `./dialtone.sh <plugin> src_vN <command> [args]`.
2. Keep `scaffold/main.go` thin; put real logic in `src_vN`.
3. Use `config` for runtime/env/path resolution; avoid hardcoded `src/plugins/...` joins.
4. Use `env/.env`
5. Use `logs` for operational output.
6. Use `test` for plugin verification with a single `src_vN/test/cmd/main.go` orchestrator.
7. Use managed toolchains from Dialtone (`go src_v1 ...`, `bun src_v1 ...`), not random system tools.
8. Define one path resolver per plugin/version (for example `src_vN/go/paths.go`) and reuse it everywhere.
9. Keep each plugin README aligned with actual CLI/env/test behavior.
10. Treat each `src_vN` as the source of truth for that version.

## Standard Plugin Layout
```text
src/plugins/<plugin>/
  README.md
  scaffold/main.go
  src_v1/
    go/
    cmd/
    test/
      cmd/main.go
      01_.../suite.go
      02_.../suite.go
```

## Foundation Libraries
- `logs`: `dialtone/dev/plugins/logs/src_v1/go`
- `test`: `dialtone/dev/plugins/test/src_v1/go`
- `config`: `dialtone/dev/plugins/config/src_v1/go`

Use `config` presets:
```go
rt, _ := configv1.ResolveRuntime("")
preset := configv1.NewPluginPreset(rt, "robot", "src_v1")
_ = preset.PluginVersionRoot
_ = preset.UI
_ = preset.Test
```

For detailed runtime/path usage, read:
- `src/plugins/config/README.md`

## NATS Topic Usage
Default URL: `nats://127.0.0.1:4222`

### Logging subjects
- `logs.>`: global log stream
- `logs.test.<suite>.<step>`: per-step test logs
- `logs.test.<suite>.browser`: browser console/error logs during tests
- `logs.test.<suite>.error`: error stream for suite
- `logs.test.<suite>.status.pass`
- `logs.test.<suite>.status.fail`

### REPL subjects
- `repl.<room>`: shared REPL room subject
- Frame types on that subject: `input`, `line`, `probe`, `server`, `heartbeat`, `join`, `left`

### Robot/telemetry subjects
- `mavlink.>`: MAVLink telemetry
- `mavlink.stats`: MAVLink health/stats
- `mavlink.attitude`: orientation data
- `rover.command`: robot control frames

### Misc coordination subjects
- `test.subject`: connectivity/smoke checks
- `ui.>`: mock UI state/telemetry topics

## Minimal New Plugin Workflow
```sh
mkdir -p src/plugins/my-plugin/{scaffold,src_v1/go,src_v1/test/cmd,src_v1/test/01_setup}
./dialtone.sh my-plugin src_v1 help
./dialtone.sh my-plugin src_v1 test
```
