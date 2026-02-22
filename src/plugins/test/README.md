# Test Plugin (`src/plugins/test`)

The test plugin is a reusable Go test runtime for plugin-level integration tests.

```sh
# 1) Run the canonical source-of-truth suite
./dialtone.sh test test src_v1

# 2) Stream all test logs (raw)
./dialtone.sh logs stream --topic 'logs.test.src-v1-self-check.>'

# 3) Stream only pass/fail status lines
./dialtone.sh logs stream --topic 'logfilter.tag.pass.test'
./dialtone.sh logs stream --topic 'logfilter.tag.fail.test'

# 4) Stream browser console/error events from test steps
./dialtone.sh logs stream --topic 'logs.test.src-v1-self-check.*.browser'
```

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

`test test src_v1` is the source-of-truth template suite for other plugins and LLM agents.
Each numbered test folder demonstrates one integration area:
- `01_self_check`: StepContext logging + NATS waits (`step`, `error`, format)
- `02_example_plugin_template`: minimal plugin template + browser helper entry path
- `03_browser_ctx`: local UI flow, aria waits, aria clicks, input type+enter, coordinate click/tap, browser-log waits via NATS
- `04_nats_wait_patterns`: `NATSConn/NATSURL`, `ResetStepLogClock`, `WaitForMessage`, `WaitForMessageAfterAction`, `WaitForAllMessagesAfterAction`, custom topics
- `05_browser_lifecycle_options`: browser options + shared suite browser lifecycle across steps

## Quick Integration For Another Plugin

Use this pattern in your plugin's `src_vN/test/`:

```text
src/plugins/<plugin>/src_v1/test/
  cmd/main.go
  01_smoke/suite.go
```

`cmd/main.go` (single process orchestrator):

```go
package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	smoke "dialtone/dev/plugins/<plugin>/src_v1/test/01_smoke"
)

func main() {
	logs.SetOutput(os.Stdout)
	reg := testv1.NewRegistry()
	smoke.Register(reg)

	err := reg.Run(testv1.SuiteOptions{
		Version:       "<plugin>-src-v1",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.<plugin>-src-v1",
		AutoStartNATS: true,
	})
	if err != nil {
		logs.Error("suite failed: %v", err)
		os.Exit(1)
	}
}
```

`01_smoke/suite.go` (action -> wait):

```go
package smoke

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name: "smoke-action-wait",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := ctx.WaitForStepMessageAfterAction("did-action", 5*time.Second, func() error {
				ctx.Infof("[SMOKE] did-action")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "smoke verified"}, nil
		},
	})
}
```

Run:

```bash
./dialtone.sh <plugin> test src_v1
```

## Test Folder Naming

Use numbered entries under each plugin's `src_vN/test/` folder:
- format: `NN_name` (two-digit prefix)
- examples: `01_self_check`, `02_example_plugin_template`, `03_browser_smoke`
- keep shared docs/files at top-level only when they are not executable test cases

`src/plugins/test/src_v1/test/` is the reference layout for how to test a plugin that imports this test library:
- `src/plugins/test/src_v1/test/cmd/main.go`: single-process suite orchestrator
- `src/plugins/test/src_v1/test/01_self_check/suite.go`: self-check step registration
- `src/plugins/test/src_v1/test/02_example_plugin_template/suite.go`: copyable step registration template
- `src/plugins/test/src_v1/test/03_browser_ctx/suite.go`: browser ctx + aria + console/NATS waits
- `src/plugins/test/src_v1/test/04_nats_wait_patterns/suite.go`: wait-pattern coverage
- `src/plugins/test/src_v1/test/05_browser_lifecycle_options/suite.go`: shared browser + options coverage
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
  - `WaitForBrowserMessage(pattern, timeout)`
  - `WaitForErrorMessage(pattern, timeout)`
  - `WaitForErrorMessageAfterAction(pattern, timeout, action)`
  - `WaitForBrowserMessageAfterAction(pattern, timeout, action)`
  - `EnsureBrowser(BrowserOptions)` / `AttachBrowserByPort(...)` / `AttachBrowserByWebSocket(...)`
  - `Browser()` / `CloseBrowser()`
  - `RunBrowser(actions...)` / `RunBrowserWithTimeout(timeout, actions...)`
  - `WaitForAriaLabel(...)` / `ClickAriaLabel(...)` / `TypeAriaLabel(...)` / `PressEnterAriaLabel(...)`
  - `ClickAriaLabelAfterWait(label, timeout)`
  - `ClickAt(x, y)` / `TapAt(x, y)` (coordinate interactions)
  - `WaitForAriaLabelAttrEquals(...)`
  - `WaitForConsoleContains(...)`
  - `TestPassf(format, ...)`
  - `TestFailf(format, ...)`
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
- `[T+0000s|LEVEL|src/path/file.go:line] message`

Status tags:
- framework emits `[TEST][PASS]` for successful step completion
- framework emits `[TEST][FAIL]` for failed/timed-out/panic steps
- these can be streamed via:
  - `./dialtone.sh logs stream --topic 'logfilter.tag.pass.>'`
  - `./dialtone.sh logs stream --topic 'logfilter.tag.fail.>'`

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

Browser-ready pattern (no plugin-local browser context needed):

```go
b, err := ctx.EnsureBrowser(testv1.BrowserOptions{
  Headless: true,
  GPU:      false,
  Role:     "test",
  URL:      "data:text/html,<button aria-label='Go' onclick=\"console.log('ok')\">Go</button>",
})
if err != nil {
  return testv1.StepRunResult{}, err
}

if err := ctx.WaitForAriaLabel("Go", 10*time.Second); err != nil {
  return testv1.StepRunResult{}, err
}
if err := ctx.ClickAriaLabel("Go"); err != nil {
  return testv1.StepRunResult{}, err
}
if err := ctx.WaitForConsoleContains("ok", 5*time.Second); err != nil {
  return testv1.StepRunResult{}, err
}
_ = b // direct chromedp access still available when needed
```

Browser lifecycle model:
- the test orchestrator owns one shared main browser tab for the full suite
- each step receives that browser via `StepContext`
- if a step opens extra tabs, the orchestrator closes extra tabs before/after the next step
- individual steps should not call `CloseBrowser()` for suite-owned sessions
- orchestrator closes the shared browser at suite end

Timeout behavior (fails step when aria-label is not found in time):

```go
if err := ctx.WaitForAriaLabel("Missing Button", 2*time.Second); err != nil {
  return testv1.StepRunResult{}, err // test step fails here
}
```

Coordinate click/tap pattern:

```go
var pt []float64
err := ctx.RunBrowser(chromedp.Evaluate(`(() => {
  const el = document.querySelector("[aria-label='Tap Area']");
  const r = el.getBoundingClientRect();
  return [Math.floor(r.left + r.width/2), Math.floor(r.top + r.height/2)];
})()`, &pt))
if err != nil {
  return testv1.StepRunResult{}, err
}
if err := ctx.ClickAt(pt[0], pt[1]); err != nil {
  return testv1.StepRunResult{}, err
}
if err := ctx.TapAt(pt[0], pt[1]); err != nil {
  return testv1.StepRunResult{}, err
}
```

Browser console logs through NATS (and wait for them):

```go
if err := ctx.WaitForBrowserMessageAfterAction("clicked-smoke", 5*time.Second, func() error {
  return ctx.ClickAriaLabel("Smoke Button")
}); err != nil {
  return testv1.StepRunResult{}, err
}
```

Browser subjects:
- step topic: `logs.test.<suite>.<step>`
- browser topic: `logs.test.<suite>.<step>.browser`
- error topic: `logs.test.<suite>.error`

What the StepContext chromedp API can do:
- start or attach browser sessions (`EnsureBrowser`, `AttachBrowserByPort`, `AttachBrowserByWebSocket`)
- reuse active session in step (`Browser`, `CloseBrowser`)
- run arbitrary chromedp actions (`RunBrowser`, `RunBrowserWithTimeout`)
- wait/assert element presence and attributes by aria-label
- click/type/press-enter by aria-label
- click/tap by absolute coordinates
- wait for browser console output and treat misses as test failures
- route browser console/errors into test logs and NATS subjects automatically

StepContext browser API reference:
- `EnsureBrowser(BrowserOptions) (*BrowserSession, error)`
- `AttachBrowserByPort(port int, role string) (*BrowserSession, error)`
- `AttachBrowserByWebSocket(webSocketURL string, role string) (*BrowserSession, error)`
- `Browser() (*BrowserSession, error)`
- `CloseBrowser()` (no-op for suite-owned shared browser)
- `RunBrowser(actions ...chromedp.Action) error`
- `RunBrowserWithTimeout(timeout time.Duration, actions ...chromedp.Action) error`
- `WaitForAriaLabel(label string, timeout time.Duration) error`
- `ClickAriaLabel(label string) error`
- `ClickAriaLabelAfterWait(label string, timeout time.Duration) error`
- `TypeAriaLabel(label, value string) error`
- `PressEnterAriaLabel(label string) error`
- `WaitForAriaLabelAttrEquals(label, attr, expected string, timeout time.Duration) error`
- `ClickAt(x, y float64) error`
- `TapAt(x, y float64) error`
- `WaitForConsoleContains(substr string, timeout time.Duration) error`
- `WaitForBrowserMessage(pattern string, timeout time.Duration) error`
- `WaitForBrowserMessageAfterAction(pattern string, timeout time.Duration, action func() error) error`

Local `index.html` smoke pattern (aria-label + console):

```go
func RunUISmoke(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
  srv := httptest.NewServer(http.FileServer(http.Dir("/abs/path/to/src/plugins/<plugin>/src_v1/test/03_browser_ctx")))
  defer srv.Close()

  _, err := ctx.EnsureBrowser(testv1.BrowserOptions{
    Headless: true,
    GPU:      false,
    Role:     "test",
    URL:      srv.URL + "/index.html",
  })
  if err != nil {
    return testv1.StepRunResult{}, err
  }
  defer ctx.CloseBrowser()

  if err := ctx.WaitForAriaLabel("Smoke Button", 10*time.Second); err != nil {
    return testv1.StepRunResult{}, err
  }
  if err := ctx.ClickAriaLabel("Smoke Button"); err != nil {
    return testv1.StepRunResult{}, err
  }
  if err := ctx.WaitForConsoleContains("clicked-smoke", 5*time.Second); err != nil {
    return testv1.StepRunResult{}, err
  }
  return testv1.StepRunResult{Report: "browser ctx smoke passed"}, nil
}
```

Reference implementation in this repo:
- `src/plugins/test/src_v1/test/03_browser_ctx/index.html`
- `src/plugins/test/src_v1/test/03_browser_ctx/suite.go`

The self-check command verifies this template path end-to-end:
- `./dialtone.sh test test src_v1`

## Notes

- This plugin is the shared test runtime. Plugin-specific tests live in each plugin's own `src_vN/test` folder.
- Logs from test steps are intentionally centralized through the logs plugin so behavior is consistent across plugins.
