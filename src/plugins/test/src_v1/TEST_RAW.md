# Test Report: src-v1-self-check

- **Date**: Sat, 28 Feb 2026 12:43:26 PST
- **Total Duration**: 5m34.547522305s

## Summary

- **Steps**: 3 / 4 passed
- **Status**: FAILED

## Details

### 1. ✅ ctx-logging-and-waits

- **Duration**: 4.419079ms
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

- **Duration**: 468.873µs
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

- **Duration**: 1.84966ms
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

### 4. ❌ example-browser-stepcontext-api

- **Duration**: 5m31.213793177s
- **Error**: `step example-browser-stepcontext-api timed out`

#### Logs

```text
INFO: ERROR_PING: start browser_subject=logs.test.src-v1-self-check.example-browser-stepcontext-api.browser error_subject=logs.test.src-v1-self-check.error
INFO: ERROR_PING: browser-topic-ok marker=__DIALTONE_ERROR_PING__:1772311100190808987
INFO: ERROR_PING: error-topic-ok marker=__DIALTONE_ERROR_PING__:1772311100190808987:error
INFO: ERROR_PING: pass browser_topic=true error_topic=true
```

#### Errors

```text
FAIL: [TEST][FAIL] [STEP:example-browser-stepcontext-api] timed out after 20s
```

#### Browser Logs

```text
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

---

