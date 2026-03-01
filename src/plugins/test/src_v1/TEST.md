# Test Plugin src_v1 Self-Check Report

## Test Environment

```text
<empty>
```

**Generated at:** Sun, 01 Mar 2026 12:24:52 -0800
**Version:** `src-v1-self-check`
**Runner:** `test/src_v1`
**Status:** ✅ PASS
**Total Time:** `21.35138071s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| ctx-logging-and-waits | ✅ PASS | `2.597488ms` |
| ctx-subjects-populated | ✅ PASS | `301.959µs` |
| example-template-step | ✅ PASS | `1.48486ms` |
| example-browser-stepcontext-api | ✅ PASS | `17.846941434s` |
| browser-stepcontext-aria-and-console | ✅ PASS | `2.067157831s` |
| nats-step-wait-patterns | ✅ PASS | `1.90532ms` |
| browser-lifecycle-setup-options | ✅ PASS | `1.17065113s` |
| browser-lifecycle-reuse-shared-session | ✅ PASS | `117.592329ms` |
| auto-screenshot-uses-browser | ✅ PASS | `139.212904ms` |
| auto-screenshot-file-exists | ✅ PASS | `464.066µs` |

## Step Details

## ctx-logging-and-waits

### Results

```text
result: PASS
duration: 2.597488ms
report: StepContext log methods + wait helpers verified
```

### Logs

```text
logs:
INFO: ctx info message
WARN: ctx warn message
INFO: ctx info format check
WARN: ctx warn format check
INFO: report: StepContext log methods + wait helpers verified
PASS: [TEST][PASS] [STEP:ctx-logging-and-waits] report: StepContext log methods + wait helpers verified
```

### Errors

```text
errors:
ERROR: ctx error message
ERROR: ctx error message
ERROR: ctx error format check
```

### Browser Logs

```text
browser_logs:
<empty>
```

## ctx-subjects-populated

### Results

```text
result: PASS
duration: 301.959µs
report: StepContext subjects available for plugin tests
```

### Logs

```text
logs:
INFO: report: StepContext subjects available for plugin tests
PASS: [TEST][PASS] [STEP:ctx-subjects-populated] report: StepContext subjects available for plugin tests
```

### Errors

```text
errors:
<empty>
```

### Browser Logs

```text
browser_logs:
<empty>
```

## example-template-step

### Results

```text
result: PASS
duration: 1.48486ms
report: template-style test step ran in shared process
```

### Logs

```text
logs:
INFO: template plugin info
INFO: report: template-style test step ran in shared process
PASS: [TEST][PASS] [STEP:example-template-step] report: template-style test step ran in shared process
```

### Errors

```text
errors:
ERROR: template plugin error
```

### Browser Logs

```text
browser_logs:
<empty>
```

## example-browser-stepcontext-api

### Results

```text
result: PASS
duration: 17.846941434s
report: skipped browser helper example (aria wait failed)
```

### Error-Ping Check

```text
INFO: ERROR_PING: start browser_subject=logs.test.src-v1-self-check.example-browser-stepcontext-api.browser error_subject=logs.test.src-v1-self-check.error
INFO: ERROR_PING: browser-topic-ok marker=__DIALTONE_ERROR_PING__:1772396677018606406
INFO: ERROR_PING: error-topic-ok marker=__DIALTONE_ERROR_PING__:1772396677018606406:error
INFO: ERROR_PING: pass browser_topic=true error_topic=true
```

### Logs

```text
logs:
INFO: ERROR_PING: start browser_subject=logs.test.src-v1-self-check.example-browser-stepcontext-api.browser error_subject=logs.test.src-v1-self-check.error
INFO: ERROR_PING: browser-topic-ok marker=__DIALTONE_ERROR_PING__:1772396677018606406
INFO: ERROR_PING: error-topic-ok marker=__DIALTONE_ERROR_PING__:1772396677018606406:error
INFO: ERROR_PING: pass browser_topic=true error_topic=true
WARN: browser aria wait failed: timed out waiting for aria-label "Do Thing" after 10s
INFO: report: skipped browser helper example (aria wait failed)
PASS: [TEST][PASS] [STEP:example-browser-stepcontext-api] report: skipped browser helper example (aria wait failed)
```

### Errors

```text
errors:
<empty>
```

### Browser Logs

```text
browser_logs:
INFO: CONSOLE:log: "__DIALTONE_ERROR_PING__:1772396677018606406"
ERROR: CONSOLE:error: "__DIALTONE_ERROR_PING__:1772396677018606406:error"
```

### Screenshots

![auto_example-browser-stepcontext-api.png](screenshots/auto_example-browser-stepcontext-api.png)

## browser-stepcontext-aria-and-console

### Results

```text
result: PASS
duration: 2.067157831s
report: StepContext browser API verified: aria wait timeout, aria click, type+enter, coordinate click/tap, browser console logs via NATS waits
```

### Logs

```text
logs:
INFO: report: StepContext browser API verified: aria wait timeout, aria click, type+enter, coordinate click/tap, browser console logs via NATS waits
PASS: [TEST][PASS] [STEP:browser-stepcontext-aria-and-console] report: StepContext browser API verified: aria wait timeout, aria click, type+enter, coordinate click/tap, browser console logs via NATS waits
```

### Errors

```text
errors:
<empty>
```

### Browser Logs

```text
browser_logs:
INFO: CONSOLE:log: "clicked-smoke"
INFO: CONSOLE:log: "coord-hit-1"
INFO: CONSOLE:log: "coord-hit-2"
INFO: CONSOLE:log: "search-enter:dialtone"
```

### Screenshots

![auto_browser-stepcontext-aria-and-console.png](screenshots/auto_browser-stepcontext-aria-and-console.png)

## nats-step-wait-patterns

### Results

```text
result: PASS
duration: 1.90532ms
report: StepContext NATS wait patterns verified (step/error/custom/all)
```

### Logs

```text
logs:
INFO: step-msg-one
INFO: multi-a
INFO: multi-b
INFO: direct-step-hit
INFO: report: StepContext NATS wait patterns verified (step/error/custom/all)
PASS: [TEST][PASS] [STEP:nats-step-wait-patterns] report: StepContext NATS wait patterns verified (step/error/custom/all)
```

### Errors

```text
errors:
ERROR: expected-step-error
```

### Browser Logs

```text
browser_logs:
<empty>
```

## browser-lifecycle-setup-options

### Results

```text
result: PASS
duration: 1.17065113s
report: browser options + aria-click helper verified
```

### Logs

```text
logs:
INFO: report: browser options + aria-click helper verified
PASS: [TEST][PASS] [STEP:browser-lifecycle-setup-options] report: browser options + aria-click helper verified
```

### Errors

```text
errors:
<empty>
```

### Browser Logs

```text
browser_logs:
INFO: CONSOLE:log: "option-clicked"
```

### Screenshots

![auto_browser-lifecycle-setup-options.png](screenshots/auto_browser-lifecycle-setup-options.png)

## browser-lifecycle-reuse-shared-session

### Results

```text
result: PASS
duration: 117.592329ms
report: shared suite browser session reuse verified across steps
```

### Logs

```text
logs:
INFO: report: shared suite browser session reuse verified across steps
PASS: [TEST][PASS] [STEP:browser-lifecycle-reuse-shared-session] report: shared suite browser session reuse verified across steps
```

### Errors

```text
errors:
<empty>
```

### Browser Logs

```text
browser_logs:
INFO: CONSOLE:log: "shared-session-ok"
```

### Screenshots

![auto_browser-lifecycle-reuse-shared-session.png](screenshots/auto_browser-lifecycle-reuse-shared-session.png)

## auto-screenshot-uses-browser

### Results

```text
result: PASS
duration: 139.212904ms
report: browser used; auto screenshot should be captured after step
```

### Logs

```text
logs:
INFO: report: browser used; auto screenshot should be captured after step
PASS: [TEST][PASS] [STEP:auto-screenshot-uses-browser] report: browser used; auto screenshot should be captured after step
```

### Errors

```text
errors:
<empty>
```

### Browser Logs

```text
browser_logs:
<empty>
```

### Screenshots

![auto_auto-screenshot-uses-browser.png](screenshots/auto_auto-screenshot-uses-browser.png)

## auto-screenshot-file-exists

### Results

```text
result: PASS
duration: 464.066µs
report: auto screenshot file exists
```

### Logs

```text
logs:
INFO: report: auto screenshot file exists
PASS: [TEST][PASS] [STEP:auto-screenshot-file-exists] report: auto screenshot file exists
```

### Errors

```text
errors:
<empty>
```

### Browser Logs

```text
browser_logs:
<empty>
```

