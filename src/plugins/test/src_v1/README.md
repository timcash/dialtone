# Test Plugin (`src/plugins/test`)

Reusable test runtime for plugin integration tests.

## Bash Workflows

```bash
# Install test UI dependencies
./dialtone.sh test src_v1 install

# Format test Go code and UI sources
./dialtone.sh test src_v1 format

# Run test plugin Go vet and UI lint
./dialtone.sh test src_v1 lint

# Build test Go entrypoints and the Vite UI
./dialtone.sh test src_v1 build

# Run test plugin self-check suite (reference implementation)
./dialtone.sh test src_v1 test

# Run UI suite local/headless
./dialtone.sh ui src_v1 test

# Run UI suite attached to a remote headed browser
./dialtone.sh ui src_v1 test --attach legion

# Stream all suite logs for one plugin
./dialtone.sh logs src_v1 stream --topic 'logs.test.ui.src-v1.>'

# Stream browser-only events for one suite
./dialtone.sh logs src_v1 stream --topic 'logs.test.ui.src-v1.*.browser'

# Stream pass/fail tags across suites
./dialtone.sh logs src_v1 stream --topic 'logfilter.tag.pass.>'
./dialtone.sh logs src_v1 stream --topic 'logfilter.tag.fail.>'
```

Use the `dialtone.sh` wrapper for test plugin workflows. Do not run `go`, `npm`, or `vite` directly for normal install/build/format paths; the wrapper is the supported entrypoint.

Build outputs:
- Go binaries go to `bin/`, not `src/`
- current wrapped build artifacts are:
  - `bin/dialtone_test_v1`
  - `bin/dialtone_test_v1_runner`
  - `bin/dialtone_test_v1_mock_server`
- the UI build output stays in `src/plugins/test/src_v1/ui/dist`

Minimal plugin layout:

```text
src/plugins/<plugin>/src_v1/test/
  cmd/main.go
  01_smoke/suite.go
  02_browser/suite.go
```

Minimal orchestrator:

```go
reg := testv1.NewRegistry()
smoke.Register(reg)
browser.Register(reg)
err := reg.Run(testv1.SuiteOptions{
  Version:       "<plugin>-src-v1",
  NATSURL:       "nats://127.0.0.1:4222",
  NATSSubject:   "logs.test.<plugin>-src-v1",
  AutoStartNATS: true,
})
```

## StepContext

Use `StepContext` only; avoid plugin-local test context wrappers.

Common methods:
- logging: `Infof`, `Warnf`, `Errorf`, `TestPassf`, `TestFailf`
- wait helpers: `WaitForStepMessage*`, `WaitForBrowserMessage*`, `WaitForErrorMessage*`
- browser: `EnsureBrowser`, `Goto`, `SetHTML`, `CaptureScreenshot`
- aria helpers: `WaitForAriaLabel`, `ClickAriaLabel`, `TypeAriaLabel`, `PressEnterAriaLabel`, `WaitForAriaLabelAttrEquals`
- misc: `WaitForConsoleContains`, `ClickAt`, `TapAt`, `ResetStepLogClock`, `RepoRoot`
- default step timeout: `10s` when `Step.Timeout` is not set

UI-style step pattern:

```go
reg.Add(testv1.Step{
  Name:    "ui-menu-nav",
  Timeout: 20 * time.Second,
  RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
    _, err := ctx.EnsureBrowser(testv1.BrowserOptions{
      Headless: true,
      GPU:      false,
      Role:     "ui-test",
      URL:      "http://127.0.0.1:5177/#hero",
    })
    if err != nil {
      return testv1.StepRunResult{}, err
    }
    if err := ctx.ClickAriaLabelAfterWait("Toggle Global Menu", 8*time.Second); err != nil {
      return testv1.StepRunResult{}, err
    }
    if err := ctx.ClickAriaLabelAfterWait("Navigate Docs", 8*time.Second); err != nil {
      return testv1.StepRunResult{}, err
    }
    return testv1.StepRunResult{Report: "menu navigation verified"}, nil
  },
})
```

Remote browser behavior:
- default is local/headless unless caller passes explicit attach options
- `./dialtone.sh test src_v1 test` auto-attaches to the paired Windows browser node on WSL unless you pass `--force-local-browser`
- set `DIALTONE_TEST_BROWSER_NODE` in `env/dialtone.json`, pass `--default-attach <node>`, or pass `--attach <node>` to choose the test-browser host explicitly
- when attach is active, browser console/error events are still routed through test logger + NATS
- for `chrome src_v3` service sessions, treat the managed remote browser as exclusive for the duration of the run; do not run multiple attach suites against the same host at the same time

## Logs + NATS

`RunSuite(...)` wires `logs/src_v1` and NATS topics automatically.

Topic shape:
- suite base: `logs.test.<version-token>`
- step logs: `logs.test.<version-token>.<step-token>`
- step browser logs: `logs.test.<version-token>.<step-token>.browser`
- suite error topic: `logs.test.<version-token>.error`

Notes:
- `Infof/Warnf/Errorf` publish to step topic and are included in final report
- browser console/exception hooks publish to `.browser` topic and are included in final report
- pass/fail status tags emit for each step and can be streamed via `logfilter.tag.pass.*` / `logfilter.tag.fail.*`

Small wait pattern:

```go
err := ctx.WaitForStepMessageAfterAction("did-action", 5*time.Second, func() error {
  ctx.Infof("did-action")
  return nil
})
```

## TEST.md

`test/src_v1` owns report generation. Plugin tests should not hand-roll markdown.

Recommended `SuiteOptions` for template reports:
- `ReportPath`: final markdown path (for example `plugins/ui/src_v1/TEST.md`)
- `RawReportPath`: intermediate raw markdown path
- `ReportFormat`: `template`
- `ReportTitle`: report title
- `ReportRunner`: runner label

Generation flow:
1. suite run writes raw step status data
2. template renderer converts raw markdown to final `TEST.md`
3. each step includes:
- `### Results`
- `### Logs`
- `### Errors`
- `### Browser Logs`
- `### Screenshots`

Screenshot behavior (as used by UI tests):
- screenshot paths come from each step's `Screenshots` list
- when multiple screenshots are present for one step, they are rendered in a grid-style table
- links are normalized relative to the report file location

Example config:

```go
err := reg.Run(testv1.SuiteOptions{
  Version:       "ui-src-v1",
  ReportPath:    "plugins/ui/src_v1/TEST.md",
  RawReportPath: "plugins/ui/src_v1/TEST_RAW.md",
  ReportFormat:  "template",
  ReportTitle:   "UI Plugin src_v1 Test Report",
  ReportRunner:  "test/src_v1",
  NATSURL:       "nats://127.0.0.1:4222",
  NATSSubject:   "logs.test.ui.src-v1",
  AutoStartNATS: true,
})
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
- start or reuse a service-backed browser session (`EnsureBrowser`)
- navigate the managed tab (`Goto`)
- replace the managed tab document (`SetHTML`)
- wait/assert element presence and attributes by aria-label
- click/type/press-enter by aria-label
- capture screenshots that are written into `TEST.md`
- wait for browser console output and route it into test logs/NATS

Service-backed `StepContext` browser API reference:
- `EnsureBrowser(BrowserOptions) (*BrowserSession, error)`
- `Browser() (*BrowserSession, error)`
- `CloseBrowser()` (no-op for suite-owned shared browser)
- `Goto(url string) error`
- `SetHTML(markup string) error`
- `WaitForAriaLabel(label string, timeout time.Duration) error`
- `ClickAriaLabel(label string) error`
- `ClickAriaLabelAfterWait(label string, timeout time.Duration) error`
- `TypeAriaLabel(label, value string) error`
- `PressEnterAriaLabel(label string) error`
- `WaitForAriaLabelAttrEquals(label, attr, expected string, timeout time.Duration) error`
- `CaptureScreenshot(path string) error`
- `WaitForConsoleContains(substr string, timeout time.Duration) error`
- `WaitForBrowserMessage(pattern string, timeout time.Duration) error`
- `WaitForBrowserMessageAfterAction(pattern string, timeout time.Duration, action func() error) error`

Notes:
- These browser helpers now drive `chrome src_v3` over NATS through the service host.
- Direct `chromedp` attach/allocator workflows are retired for normal tests.
- `RunBrowser(...)` and `RunBrowserWithTimeout(...)` are only for legacy direct-browser tests and should not be the default API.

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
- `./dialtone.sh test src_v1 test`

## Notes

- This plugin is the shared test runtime. Plugin-specific tests live in each plugin's own `src_vN/test` folder.
- Logs from test steps are intentionally centralized through the logs plugin so behavior is consistent across plugins.
