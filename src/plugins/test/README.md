# Log 2026-02-16 13:36:14 -0800

- `test_v2` is the active browser test runtime for DAG `src_v3`.
- Attach semantics are domain-stable:
  - attach mode reuses the existing headed dev browser role/session
  - attach binds to an existing page target instead of creating new tabs/windows
- Screenshot sequence language is standardized:
  - each step may emit multiple screenshots
  - per-step `1xN` grid screenshots are generated for compact TEST.md storytelling
- `test_v2` now carries the session metadata accessor needed by dev orchestration (`pid`, debug websocket/url reporting).

# test_v2

`src/libs/test_v2` is a lightweight Go testing toolkit for browser-driven plugin tests.
It provides:

- Browser session startup/reuse via the Dialtone Chrome plugin.
- Per-step execution + timeout handling.
- Console/error capture from browser and runner.
- Markdown report generation with screenshots.
- Per-step screenshot-grid composition for sequence views.
- UI helper actions keyed by `aria-label`.
- Utility helpers for port waiting and PNG pixel assertions.

## Files

- `test.go`: suite runner, browser session, log capture, report writer.
- `browser_actions.go`: common chromedp actions/assertions for UI tests.
- `dev_server.go`: headed dev browser helpers (`role=dev`).
- `ports.go`: `WaitForPort`, `PickFreePort`.
- `pixel_assert.go`: PNG pixel color assertions.

## Core Types

- `BrowserOptions`
  - `Headless`, `Role`, `ReuseExisting`, `URL`, `LogWriter`, `LogPrefix`, `EmitProofOfLife`
- `BrowserSession`
  - wraps chromedp context + Chrome plugin session metadata (`isNew`).
- `Step`
  - `Name`, `Run` or `RunWithContext`, `SectionID`, `Screenshot`, `Screenshots`, `ScreenshotGrid`, `Timeout`.
- `StepContext`
  - `Name`, `Started` (per-step metadata passed into `RunWithContext`).
- `StepRunResult`
  - `Report`: markdown text added to the step section in `TEST.md` as "Step Story".
- `SuiteOptions`
  - `Version`, `ReportPath`, `LogPath`, `ErrorLogPath`.

## Browser Lifecycle Semantics

`StartBrowser` uses `chrome_app.StartSession(...)` with the provided `Role`/`ReuseExisting` behavior.

Important:

- `BrowserSession.Close()` always cancels chromedp context.
- `BrowserSession.Close()` only calls `chrome_app.CleanupSession(...)` if `isNew == true`.
- If the browser was reused (`ReuseExisting: true` and existing role/profile), `Close()` should not kill the shared Chrome process.

`StartDevBrowser` in `dev_server.go` is a convenience wrapper:

- `Headless: false`
- `Role: dev`
- `ReuseExisting: true`

## Suite Execution Model (`RunSuite`)

For each step:

1. Starts step log capture.
2. Runs `Step.Run()` or `Step.RunWithContext(...)` in a goroutine with timeout (default `30s`).
3. Captures stdout/stderr and prefixes lines with elapsed tags (`[T+0001]`).
4. Collects browser console entries during the step window.
5. On failure: writes report + returns immediately.
6. On success: proceeds to next step.

`RunSuite` itself does not own server/browser lifecycle for plugin-specific sessions; callers manage that.

## Report Outputs

Generated markdown includes:

- Suite metadata (version, status, duration).
- Step summary table.
- Per-step:
  - result/duration/section/error
  - optional step story text (from `StepRunResult.Report`)
  - runner output
  - filtered browser logs (by `SectionID` token `#<section>`)
  - browser errors block when present
  - screenshot grid embed when `Screenshots`/`ScreenshotGrid` is configured
- Artifacts list (`test.log`, `error.log`, screenshots)

## UI Action Helpers (`browser_actions.go`)

Common helpers:

- `ComposeSectionID` (`<plugin>-<subname>-<underlay>`)
- `NavigateToSection`
- `WaitForAriaLabel`
- `ClickAriaLabel`
- `TypeAriaLabel`
- `PressEnterAriaLabel`
- `TypeAndSubmitAriaLabel`
- `AssertAriaLabelTextContains`
- `AssertAriaLabelAttrEquals`
- `WaitForAriaLabelAttrEquals`
- `AssertElementHidden`
- `AssertAriaLabelInsideViewport`

Pattern: test UIs should expose stable `aria-label` hooks.

## Section + Overlay Conventions (From DAG `src_v3`)

`test_v2` assumes section UIs follow shared UIv2 conventions:

- Section id naming: `<plugin-name>-<subname>-<underlay-type>`
- Underlay kinds: `stage | table | docs | xterm | video`
- Overlay kinds: `menu | mode-form | legend | chatlog | status-bar`

Practical testing rule:

- Prefer `aria-label` selectors for all major UI elements (section controls, buttons, inputs, legends, tables, terminals).
- Keep section id stable in URL hash and pass section ids directly to `NavigateToSection(...)`.

## DevSession Helper (`dev_server.go`)

`DevSession` is optional convenience for dev UX:

- Creates `dev.log` in a version dir.
- `StartBrowserAttach()` waits for the dev port then opens headed Chrome.
- `Close()` closes attached browser and log file.

Note: this helper does not start/stop the dev server process itself.

## Port + Pixel Utilities

- `WaitForPort(port, timeout)` polls `127.0.0.1`.
- `PickFreePort()` reserves and returns an ephemeral local port.
- `AssertPNGPixelColorWithinTolerance(...)` validates screenshot pixels.

## Practical Notes

- `EmitProofOfLife` intentionally writes a console error marker.
- `SectionID` filtering depends on console messages containing `#<section>`.
- If you need a dev server to persist after test command exit, that persistence must be implemented by the caller/CLI orchestration, not by `RunSuite` itself.
