# Test Plugin src_v1 Self-Check Report

## Test Environment

```text
<empty>
```

**Generated at:** Sat, 28 Feb 2026 12:43:26 -0800
**Version:** `src-v1-self-check`
**Runner:** `test/src_v1`
**Status:** ❌ FAIL
**Total Time:** `5m34.547522305s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| ctx-logging-and-waits | ✅ PASS | `4.419079ms` |
| ctx-subjects-populated | ✅ PASS | `468.873µs` |
| example-template-step | ✅ PASS | `1.84966ms` |
| example-browser-stepcontext-api | ❌ FAIL | `5m31.213793177s` |

## Step Details

## ctx-logging-and-waits

### Results

```text
result: PASS
duration: 4.419079ms
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
duration: 468.873µs
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
duration: 1.84966ms
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
result: FAIL
duration: 5m31.213793177s
error: step example-browser-stepcontext-api timed out
```

### Error-Ping Check

```text
INFO: ERROR_PING: start browser_subject=logs.test.src-v1-self-check.example-browser-stepcontext-api.browser error_subject=logs.test.src-v1-self-check.error
INFO: ERROR_PING: browser-topic-ok marker=__DIALTONE_ERROR_PING__:1772311100190808987
INFO: ERROR_PING: error-topic-ok marker=__DIALTONE_ERROR_PING__:1772311100190808987:error
INFO: ERROR_PING: pass browser_topic=true error_topic=true
```

### Logs

```text
logs:
INFO: ERROR_PING: start browser_subject=logs.test.src-v1-self-check.example-browser-stepcontext-api.browser error_subject=logs.test.src-v1-self-check.error
INFO: ERROR_PING: browser-topic-ok marker=__DIALTONE_ERROR_PING__:1772311100190808987
INFO: ERROR_PING: error-topic-ok marker=__DIALTONE_ERROR_PING__:1772311100190808987:error
INFO: ERROR_PING: pass browser_topic=true error_topic=true
```

### Errors

```text
errors:
FAIL: [TEST][FAIL] [STEP:example-browser-stepcontext-api] timed out after 20s
```

### Browser Logs

```text
browser_logs:
INFO: CONSOLE:debug: "[vite] connecting..."
INFO: CONSOLE:debug: "[vite] connected."
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #hero"
INFO: CONSOLE:log: "[SectionManager] LOADING #hero"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #hero"
INFO: CONSOLE:log: "[SectionManager] LOADED #hero"
INFO: CONSOLE:log: "[SectionManager] START #hero"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #hero"
INFO: CONSOLE:log: "[SectionManager] RESUME #hero"
INFO: CONSOLE:log: "__DIALTONE_ERROR_PING__:1772311100190808987"
ERROR: CONSOLE:error: "__DIALTONE_ERROR_PING__:1772311100190808987:error"
INFO: CONSOLE:log: "__DIALTONE_ERROR_PING__:1772311098525688207"
ERROR: CONSOLE:error: "__DIALTONE_ERROR_PING__:1772311098525688207:error"
```

<!-- DIALTONE_CHROME_REPORT_START -->

## Chrome Report

- hostnode: `legion`
- chrome_count: `unknown`
- error: `remote browser inventory on legion failed: powershell command failed: exit status 1`

<!-- DIALTONE_CHROME_REPORT_END -->
