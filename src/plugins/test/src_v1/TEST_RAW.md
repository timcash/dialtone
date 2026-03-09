# Test Report: src-v1-self-check

- **Date**: Mon, 09 Mar 2026 14:26:34 PDT
- **Total Duration**: 7.063625ms

## Summary

- **Steps**: 10 / 10 passed
- **Status**: PASSED

## Details

### 1. ✅ ctx-logging-and-waits

- **Duration**: 2.924167ms
- **Report**: StepContext log methods + wait helpers verified

#### Logs

```text
INFO: ctx info message
WARN: ctx warn message
INFO: ctx info format check
WARN: ctx warn format check
INFO: report: StepContext log methods + wait helpers verified
PASS: [TEST][PASS] [STEP:ctx-logging-and-waits] report: StepContext log methods + wait helpers verified
```

#### Errors

```text
ERROR: ctx error message
ERROR: ctx error message
ERROR: ctx error format check
```

#### Browser Logs

```text
<empty>
```

---

### 2. ✅ ctx-subjects-populated

- **Duration**: 158.292µs
- **Report**: StepContext subjects available for plugin tests

#### Logs

```text
INFO: report: StepContext subjects available for plugin tests
PASS: [TEST][PASS] [STEP:ctx-subjects-populated] report: StepContext subjects available for plugin tests
```

#### Browser Logs

```text
<empty>
```

---

### 3. ✅ example-template-step

- **Duration**: 729.542µs
- **Report**: template-style test step ran in shared process

#### Logs

```text
INFO: template plugin info
INFO: report: template-style test step ran in shared process
PASS: [TEST][PASS] [STEP:example-template-step] report: template-style test step ran in shared process
```

#### Errors

```text
ERROR: template plugin error
```

#### Browser Logs

```text
<empty>
```

---

### 4. ✅ example-browser-stepcontext-api

- **Duration**: 350.25µs
- **Report**: skipped browser helper example (chrome not installed)

#### Logs

```text
WARN: browser provider not available; use --attach <node> for remote mode
INFO: report: skipped browser helper example (chrome not installed)
PASS: [TEST][PASS] [STEP:example-browser-stepcontext-api] report: skipped browser helper example (chrome not installed)
```

#### Browser Logs

```text
<empty>
```

---

### 5. ✅ browser-stepcontext-aria-and-console

- **Duration**: 790.917µs
- **Report**: skipped browser ctx smoke (chrome not installed)

#### Logs

```text
WARN: browser provider not available; use --attach <node> for remote mode
INFO: report: skipped browser ctx smoke (chrome not installed)
PASS: [TEST][PASS] [STEP:browser-stepcontext-aria-and-console] report: skipped browser ctx smoke (chrome not installed)
```

#### Browser Logs

```text
<empty>
```

---

### 6. ✅ nats-step-wait-patterns

- **Duration**: 1.256542ms
- **Report**: StepContext NATS wait patterns verified (step/error/custom/all)

#### Logs

```text
INFO: step-msg-one
INFO: multi-a
INFO: multi-b
INFO: direct-step-hit
INFO: report: StepContext NATS wait patterns verified (step/error/custom/all)
PASS: [TEST][PASS] [STEP:nats-step-wait-patterns] report: StepContext NATS wait patterns verified (step/error/custom/all)
```

#### Errors

```text
ERROR: expected-step-error
```

#### Browser Logs

```text
<empty>
```

---

### 7. ✅ browser-lifecycle-setup-options

- **Duration**: 206.458µs
- **Report**: skipped browser lifecycle options (chrome not installed)

#### Logs

```text
WARN: browser provider not available; use --attach <node> for remote mode
INFO: report: skipped browser lifecycle options (chrome not installed)
PASS: [TEST][PASS] [STEP:browser-lifecycle-setup-options] report: skipped browser lifecycle options (chrome not installed)
```

#### Browser Logs

```text
<empty>
```

---

### 8. ✅ browser-lifecycle-reuse-shared-session

- **Duration**: 127.708µs
- **Report**: skipped browser lifecycle reuse (chrome not installed)

#### Logs

```text
INFO: report: skipped browser lifecycle reuse (chrome not installed)
PASS: [TEST][PASS] [STEP:browser-lifecycle-reuse-shared-session] report: skipped browser lifecycle reuse (chrome not installed)
```

#### Browser Logs

```text
<empty>
```

---

### 9. ✅ auto-screenshot-uses-browser

- **Duration**: 291µs
- **Report**: skipped auto screenshot setup (browser unavailable)

#### Logs

```text
WARN: browser provider not available; skipping auto screenshot test step
INFO: report: skipped auto screenshot setup (browser unavailable)
PASS: [TEST][PASS] [STEP:auto-screenshot-uses-browser] report: skipped auto screenshot setup (browser unavailable)
```

#### Browser Logs

```text
<empty>
```

---

### 10. ✅ auto-screenshot-file-exists

- **Duration**: 151.625µs
- **Report**: skipped auto screenshot verification (browser step skipped)

#### Logs

```text
INFO: report: skipped auto screenshot verification (browser step skipped)
PASS: [TEST][PASS] [STEP:auto-screenshot-file-exists] report: skipped auto screenshot verification (browser step skipped)
```

#### Browser Logs

```text
<empty>
```

---

