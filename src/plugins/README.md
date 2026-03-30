# Plugin Guide

This file is the short, LLM-first contract for working in `src/plugins`.

The canonical user-facing workflow now lives in the root [README.md](/C:/Users/timca/dialtone/README.md), especially the `Working With Plugins` and REPL sections. This file should stay short and mirror that command shape.

## Generic Shell Workflow
```bash
./dialtone.sh <plugin-name> <src_vN> <command> [args] [--flags]
./dialtone.sh <plugin-name> <src_vN> install
./dialtone.sh <plugin-name> <src_vN> format
./dialtone.sh <plugin-name> <src_vN> lint
./dialtone.sh <plugin-name> <src_vN> build
./dialtone.sh <plugin-name> <src_vN> test --filter <expr>
```

```bash
/plugin-name src_vN install
/plugin-name src_vN format
/plugin-name src_vN lint
/plugin-name src_vN build
/plugin-name src_vN test --filter <expr>
```

Important behavior learned from active plugin work:
- Scaffold `test` commands must forward extra CLI args to the real `src_vN/test/cmd/main.go` runner. If they do not, `--filter` and attach flags silently do nothing.
- Headed remote-browser plugins should prefer `chrome src_v3`, not `chrome src_v1`.
- For WSL-driven headed UI work, the stable pattern is usually: local server on WSL, remote Chrome on `legion`, backend/service on a third host if needed.
- Remote browser tests should prefer one long-lived managed tab and reuse it across steps. Create a new tab only for recovery.
- Shared config belongs in `env/dialtone.json`; if a temporary env override is unavoidable, prefix the one `./dialtone.sh ...` command instead of relying on exported shell state.
- Optional behavior should be controlled by `--flags`, not optional env vars.

## Core Plugins
- `logs`: [src/plugins/logs/src_v1/README.md](/home/user/dialtone/src/plugins/logs/src_v1/README.md)
- `test`: [src/plugins/test/src_v1/README.md](/home/user/dialtone/src/plugins/test/src_v1/README.md)
- `chrome`: [src/plugins/chrome/src_v3/README.md](/home/user/dialtone/src/plugins/chrome/src_v3/README.md)
- `ssh`: [src/plugins/ssh/src_v1/README.md](/home/user/dialtone/src/plugins/ssh/src_v1/README.md)
- `ui`: [src/plugins/ui/src_v1/README.md](/home/user/dialtone/src/plugins/ui/src_v1/README.md)

## Core Rules
1. Use versioned commands: `./dialtone.sh <plugin> src_vN <command> [args]`.
2. Keep `scaffold/main.go` thin; put real logic in `src_vN`.
3. Use `config` for runtime/env/path resolution; avoid hardcoded `src/plugins/...` joins.
4. Use `env/dialtone.json` for runtime configuration.
5. Use `--flags` for optional behavior; reserve env values for shared config and explicit one-command overrides.
6. Use `logs` for operational output.
7. Use `test` for plugin verification with a single `src_vN/test/cmd/main.go` orchestrator.
8. Use managed toolchains from Dialtone (`go src_v1 ...`, `bun src_v1 ...`), not random system tools.
9. Define one path resolver per plugin/version (for example `src_vN/go/paths.go`) and reuse it everywhere.
10. Keep each plugin README aligned with actual CLI/env/test behavior.
11. Treat each `src_vN` as the source of truth for that version.
12. If a plugin exposes `test --filter`, verify the scaffold forwards user args into the test runner.
13. If a plugin uses headed browser tests, document the expected remote browser role and host.

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
- `repl.<topic>`: shared REPL topic subject
- Frame types on that subject: `input`, `line`, `probe`, `server`, `heartbeat`, `join`, `left`

### Robot/telemetry subjects
- `mavlink.>`: MAVLink telemetry
- `mavlink.stats`: MAVLink health/stats
- `mavlink.attitude`: orientation data
- `rover.command`: robot control frames
- `robot.>`: robot service/runtime state
- `robot.autoswap.supervisor`: autoswap supervisor snapshot
- `robot.autoswap.runtime`: autoswap runtime/process snapshot

### Misc coordination subjects
- `test.subject`: connectivity/smoke checks
- `ui.>`: mock UI state/telemetry topics

### UI/browser debug subjects
- `logs.ui.robot`: browser UI logs published back into NATS

## Minimal New Plugin Workflow
```bash
mkdir -p src/plugins/my-plugin/{scaffold,src_v1/go,src_v1/test/cmd,src_v1/test/01_setup}
./dialtone.sh my-plugin src_v1 help
./dialtone.sh my-plugin src_v1 test
```
