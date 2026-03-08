# Test Plugin src_v1 Self-Check Report

## Test Environment

```text
<empty>
```

**Generated at:** Sat, 07 Mar 2026 16:19:22 -0800
**Version:** `src-v1-self-check`
**Runner:** `test/src_v1`
**Status:** ✅ PASS
**Total Time:** `6.165517001s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| example-browser-stepcontext-api | ✅ PASS | `134.578364ms` |
| browser-stepcontext-aria-and-console | ✅ PASS | `5.920962135s` |
| auto-screenshot-uses-browser | ✅ PASS | `104.39557ms` |
| auto-screenshot-file-exists | ✅ PASS | `155.309µs` |

## Step Details

## example-browser-stepcontext-api

### Results

```text
result: PASS
duration: 134.578364ms
report: StepContext browser helpers ready (goto + aria + console via chrome src_v3 service)
```

### Error-Ping Check

```text
WARN: ERROR_PING: skipped for chrome src_v3 NATS-managed browser session
```

### Logs

```text
logs:
WARN: ERROR_PING: skipped for chrome src_v3 NATS-managed browser session
INFO: report: StepContext browser helpers ready (goto + aria + console via chrome src_v3 service)
PASS: [TEST][PASS] [STEP:example-browser-stepcontext-api] report: StepContext browser helpers ready (goto + aria + console via chrome src_v3 service)
```

### Errors

```text
errors:
<empty>
```

### Browser Logs

```text
browser_logs:
INFO: CONSOLE:log: "clicked"
INFO: CONSOLE:log: "clicked-smoke"
INFO: CONSOLE:log: "coord-hit-1"
INFO: CONSOLE:log: "search-enter:dialtone"
```

### Screenshots

![auto_example-browser-stepcontext-api.png](screenshots/auto_example-browser-stepcontext-api.png)

## browser-stepcontext-aria-and-console

### Results

```text
result: PASS
duration: 5.920962135s
report: StepContext browser API verified through chrome src_v3 service: aria wait timeout, goto, aria click, type+enter, screenshots, browser console waits
```

### Logs

```text
logs:
INFO: report: StepContext browser API verified through chrome src_v3 service: aria wait timeout, goto, aria click, type+enter, screenshots, browser console waits
PASS: [TEST][PASS] [STEP:browser-stepcontext-aria-and-console] report: StepContext browser API verified through chrome src_v3 service: aria wait timeout, goto, aria click, type+enter, screenshots, browser console waits
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

![auto_browser-stepcontext-aria-and-console.png](screenshots/auto_browser-stepcontext-aria-and-console.png)

## auto-screenshot-uses-browser

### Results

```text
result: PASS
duration: 104.39557ms
report: browser used through chrome src_v3 service; auto screenshot should be captured after step
```

### Logs

```text
logs:
INFO: report: browser used through chrome src_v3 service; auto screenshot should be captured after step
PASS: [TEST][PASS] [STEP:auto-screenshot-uses-browser] report: browser used through chrome src_v3 service; auto screenshot should be captured after step
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
duration: 155.309µs
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

<!-- DIALTONE_CHROME_REPORT_START -->

## Chrome Report

- hostnode: `legion`
- chrome_count: `unknown`
- error: `remote browser inventory on legion failed: powershell command failed: exit status 1`

<!-- DIALTONE_CHROME_REPORT_END -->
