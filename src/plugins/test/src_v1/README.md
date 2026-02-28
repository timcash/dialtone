# Test Plugin (`src/plugins/test`)

Reusable test runtime for plugin integration tests.

## Bash Workflows

```bash
# Run test plugin self-check suite (reference implementation)
./dialtone.sh test src_v1 test

# Run UI suite local/headless
./dialtone.sh ui src_v1 test

# Run UI suite attached to a remote headed browser
./dialtone.sh ui src_v1 test --attach chroma

# Stream all suite logs for one plugin
./dialtone.sh logs src_v1 stream --topic 'logs.test.ui.src-v1.>'

# Stream browser-only events for one suite
./dialtone.sh logs src_v1 stream --topic 'logs.test.ui.src-v1.*.browser'

# Stream pass/fail tags across suites
./dialtone.sh logs src_v1 stream --topic 'logfilter.tag.pass.>'
./dialtone.sh logs src_v1 stream --topic 'logfilter.tag.fail.>'
```

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
- browser: `EnsureBrowser`, `RunBrowser`, `RunBrowserWithTimeout`
- aria helpers: `WaitForAriaLabel`, `ClickAriaLabel`, `TypeAriaLabel`, `PressEnterAriaLabel`
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
- set `DIALTONE_TEST_BROWSER_NODE=<node>` or pass plugin-level `--attach <node>`
- when attach is active, browser console/error events are still routed through test logger + NATS

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
- `./dialtone.sh test src_v1 test`

## Notes

- This plugin is the shared test runtime. Plugin-specific tests live in each plugin's own `src_vN/test` folder.
- Logs from test steps are intentionally centralized through the logs plugin so behavior is consistent across plugins.
