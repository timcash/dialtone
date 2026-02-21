# Test Plugin (`src/plugins/test`)

The test plugin is a reusable Go test runtime for plugin-level integration tests.

It provides:
- suite/step orchestration (`RunSuite`)
- browser helpers used by UI plugin tests
- process/dev helpers
- built-in logging via the `logs` plugin

## CLI

```bash
./dialtone.sh test help
./dialtone.sh test test src_v1
```

`./dialtone.sh test test src_v1` runs the test plugin self-check suite at:
- `src/plugins/test/src_v1/test/cmd/main.go`

## Library Entry

Import path:
- `dialtone/dev/plugins/test/src_v1/go`

Core files:
- `src/plugins/test/src_v1/go/test.go`
- `src/plugins/test/src_v1/go/dev.go`
- `src/plugins/test/src_v1/go/ops.go`
- `src/plugins/test/src_v1/go/browser_actions.go`

## Core Types

- `Step`
  - `Name`
  - `RunWithContext func(*StepContext) (StepRunResult, error)`
  - `Timeout`
  - optional screenshot/section fields
- `StepContext`
  - `Name`, `Started`, `Session`, `LogWriter`
  - `Logf(format, ...)`
  - `Errorf(format, ...)`
- `StepRunResult`
  - `Report string`
- `SuiteOptions`
  - `Version`, `ReportPath`, `LogPath`, `ErrorLogPath`
  - `NATSURL`, `NATSSubject`, `AutoStartNATS`

## Logging + NATS Behavior

The test plugin now uses `logs` plugin (`src/plugins/logs/src_v1/go`) as its logging backend.

`RunSuite(...)` behavior:
- emits suite and step logs through `logs`
- sets up NATS-backed suite logging
- auto-starts embedded NATS when needed
- creates step-scoped topic publishing via `StepContext`

Topic pattern used by step context logs:
- `logs.test.<suite-version>.<step-name-token>`

Inside step code, use:
- `ctx.Logf("...")`
- `ctx.Errorf("...")`

## Template For Other Plugins / Agents

Copyable example:
- `src/plugins/test/src_v1/test/example_plugin_template/main.go`
- `src/plugins/test/src_v1/test/TEMPLATE.md`

This template shows how another plugin can:
1. import the test library
2. define steps
3. use `ctx.Logf` / `ctx.Errorf`
4. run `RunSuite(...)` with NATS settings

The self-check command verifies this template path end-to-end:
- `./dialtone.sh test test src_v1`

## Notes

- This plugin is the shared test runtime. Plugin-specific tests live in each plugin's own `src_vN/test` folder.
- Logs from test steps are intentionally centralized through the logs plugin so behavior is consistent across plugins.
