# Test Plugin src_v1 Self-Check Report

## Test Environment

```text
<empty>
```

**Generated at:** Mon, 09 Mar 2026 14:26:34 -0700
**Version:** `src-v1-self-check`
**Runner:** `test/src_v1`
**Status:** ✅ PASS
**Total Time:** `7.063625ms`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| ctx-logging-and-waits | ✅ PASS | `2.924167ms` |
| ctx-subjects-populated | ✅ PASS | `158.292µs` |
| example-template-step | ✅ PASS | `729.542µs` |
| example-browser-stepcontext-api | ✅ PASS | `350.25µs` |
| browser-stepcontext-aria-and-console | ✅ PASS | `790.917µs` |
| nats-step-wait-patterns | ✅ PASS | `1.256542ms` |
| browser-lifecycle-setup-options | ✅ PASS | `206.458µs` |
| browser-lifecycle-reuse-shared-session | ✅ PASS | `127.708µs` |
| auto-screenshot-uses-browser | ✅ PASS | `291µs` |
| auto-screenshot-file-exists | ✅ PASS | `151.625µs` |

## Step Details

## ctx-logging-and-waits

### Results

```text
result: PASS
duration: 2.924167ms
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
duration: 158.292µs
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
duration: 729.542µs
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
duration: 350.25µs
report: skipped browser helper example (chrome not installed)
```

### Logs

```text
logs:
WARN: browser provider not available; use --attach <node> for remote mode
INFO: report: skipped browser helper example (chrome not installed)
PASS: [TEST][PASS] [STEP:example-browser-stepcontext-api] report: skipped browser helper example (chrome not installed)
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

## browser-stepcontext-aria-and-console

### Results

```text
result: PASS
duration: 790.917µs
report: skipped browser ctx smoke (chrome not installed)
```

### Logs

```text
logs:
WARN: browser provider not available; use --attach <node> for remote mode
INFO: report: skipped browser ctx smoke (chrome not installed)
PASS: [TEST][PASS] [STEP:browser-stepcontext-aria-and-console] report: skipped browser ctx smoke (chrome not installed)
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

## nats-step-wait-patterns

### Results

```text
result: PASS
duration: 1.256542ms
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
duration: 206.458µs
report: skipped browser lifecycle options (chrome not installed)
```

### Logs

```text
logs:
WARN: browser provider not available; use --attach <node> for remote mode
INFO: report: skipped browser lifecycle options (chrome not installed)
PASS: [TEST][PASS] [STEP:browser-lifecycle-setup-options] report: skipped browser lifecycle options (chrome not installed)
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

## browser-lifecycle-reuse-shared-session

### Results

```text
result: PASS
duration: 127.708µs
report: skipped browser lifecycle reuse (chrome not installed)
```

### Logs

```text
logs:
INFO: report: skipped browser lifecycle reuse (chrome not installed)
PASS: [TEST][PASS] [STEP:browser-lifecycle-reuse-shared-session] report: skipped browser lifecycle reuse (chrome not installed)
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

## auto-screenshot-uses-browser

### Results

```text
result: PASS
duration: 291µs
report: skipped auto screenshot setup (browser unavailable)
```

### Logs

```text
logs:
WARN: browser provider not available; skipping auto screenshot test step
INFO: report: skipped auto screenshot setup (browser unavailable)
PASS: [TEST][PASS] [STEP:auto-screenshot-uses-browser] report: skipped auto screenshot setup (browser unavailable)
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

## auto-screenshot-file-exists

### Results

```text
result: PASS
duration: 151.625µs
report: skipped auto screenshot verification (browser step skipped)
```

### Logs

```text
logs:
INFO: report: skipped auto screenshot verification (browser step skipped)
PASS: [TEST][PASS] [STEP:auto-screenshot-file-exists] report: skipped auto screenshot verification (browser step skipped)
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

