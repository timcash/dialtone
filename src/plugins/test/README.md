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

## Test Folder Naming

Use numbered entries under each plugin's `src_vN/test/` folder:
- format: `NN_name` (two-digit prefix)
- examples: `01_self_check`, `02_example_plugin_template`, `03_browser_smoke`
- keep shared docs/files at top-level only when they are not executable test cases

`src/plugins/test/src_v1/test/` is the reference layout for how to test a plugin that imports this test library:
- `src/plugins/test/src_v1/test/cmd/main.go`: single-process suite orchestrator
- `src/plugins/test/src_v1/test/01_self_check/suite.go`: self-check step registration
- `src/plugins/test/src_v1/test/02_example_plugin_template/suite.go`: copyable step registration template
- `src/plugins/test/src_v1/test/TEMPLATE.md`: template instructions for other plugins

Pattern:
- each `NN_name` folder exports registration functions (no `main.go` required in subfolders)
- one orchestrator in `test/cmd/main.go` imports each folder and runs all registered steps in one process
- plugin tests should not define their own `test_ctx` type; use `testv1.StepContext` from the test library

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
  - `SuiteSubject`, `StepSubject`, `ErrorSubject`
  - `Logf(format, ...)` (alias of `Infof`)
  - `Infof(format, ...)`
  - `Warnf(format, ...)`
  - `Debugf(format, ...)`
  - `Errorf(format, ...)`
  - `WaitForMessage(subject, pattern, timeout)`
  - `WaitForStepMessage(pattern, timeout)`
  - `WaitForErrorMessage(pattern, timeout)`
  - `WaitForErrorMessageAfterAction(pattern, timeout, action)`
  - `ResetStepLogClock()`
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

Inside step code, use only `ctx` methods (no direct `logs` import needed in test step files):
- `ctx.Infof("...")` / `ctx.Logf("...")`
- `ctx.Warnf("...")`
- `ctx.Errorf("...")`
- `ctx.WaitForStepMessage("expected text", 5*time.Second)`
- `ctx.Errorf(...)` also emits to `ctx.ErrorSubject` for suite-level error monitoring
- `ctx.WaitForErrorMessage("expected error", 5*time.Second)` to assert error-topic behavior

Rendered log lines follow the shared logs format:
- `[T+0000s|LEVEL|source-file.go] message`

## Template For Other Plugins / Agents

Copyable example:
- `src/plugins/test/src_v1/test/02_example_plugin_template/suite.go`
- `src/plugins/test/src_v1/test/TEMPLATE.md`

This template shows how another plugin can:
1. import the test library
2. define steps
3. use `ctx` logging methods (`Infof/Warnf/Errorf`) and wait helpers
4. run `RunSuite(...)` with NATS settings

Example step pattern (no `logs` import needed inside step files):

```go
{
  Name: "my-step",
  RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
    err := ctx.WaitForStepMessageAfterAction("did-action", 5*time.Second, func() error {
      ctx.Infof("did-action")
      return nil
    })
    if err != nil {
      return testv1.StepRunResult{}, err
    }
    return testv1.StepRunResult{Report: "verified"}, nil
  },
}
```

The self-check command verifies this template path end-to-end:
- `./dialtone.sh test test src_v1`

## Notes

- This plugin is the shared test runtime. Plugin-specific tests live in each plugin's own `src_vN/test` folder.
- Logs from test steps are intentionally centralized through the logs plugin so behavior is consistent across plugins.
