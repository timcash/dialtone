# Test Report: src-v1-self-check

- **Date**: Sat, 07 Mar 2026 16:19:22 PST
- **Total Duration**: 6.165517001s

## Summary

- **Steps**: 4 / 4 passed
- **Status**: PASSED

## Details

### 1. ✅ example-browser-stepcontext-api

- **Duration**: 134.578364ms
- **Report**: StepContext browser helpers ready (goto + aria + console via chrome src_v3 service)

#### Logs

```text
WARN: ERROR_PING: skipped for chrome src_v3 NATS-managed browser session
INFO: report: StepContext browser helpers ready (goto + aria + console via chrome src_v3 service)
PASS: [TEST][PASS] [STEP:example-browser-stepcontext-api] report: StepContext browser helpers ready (goto + aria + console via chrome src_v3 service)
```

#### Browser Logs

```text
INFO: CONSOLE:log: "clicked"
INFO: CONSOLE:log: "clicked-smoke"
INFO: CONSOLE:log: "coord-hit-1"
INFO: CONSOLE:log: "search-enter:dialtone"
```

#### Screenshots

![auto_example-browser-stepcontext-api.png](screenshots/auto_example-browser-stepcontext-api.png)

---

### 2. ✅ browser-stepcontext-aria-and-console

- **Duration**: 5.920962135s
- **Report**: StepContext browser API verified through chrome src_v3 service: aria wait timeout, goto, aria click, type+enter, screenshots, browser console waits

#### Logs

```text
INFO: report: StepContext browser API verified through chrome src_v3 service: aria wait timeout, goto, aria click, type+enter, screenshots, browser console waits
PASS: [TEST][PASS] [STEP:browser-stepcontext-aria-and-console] report: StepContext browser API verified through chrome src_v3 service: aria wait timeout, goto, aria click, type+enter, screenshots, browser console waits
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![auto_browser-stepcontext-aria-and-console.png](screenshots/auto_browser-stepcontext-aria-and-console.png)

---

### 3. ✅ auto-screenshot-uses-browser

- **Duration**: 104.39557ms
- **Report**: browser used through chrome src_v3 service; auto screenshot should be captured after step

#### Logs

```text
INFO: report: browser used through chrome src_v3 service; auto screenshot should be captured after step
PASS: [TEST][PASS] [STEP:auto-screenshot-uses-browser] report: browser used through chrome src_v3 service; auto screenshot should be captured after step
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![auto_auto-screenshot-uses-browser.png](screenshots/auto_auto-screenshot-uses-browser.png)

---

### 4. ✅ auto-screenshot-file-exists

- **Duration**: 155.309µs
- **Report**: auto screenshot file exists

#### Logs

```text
INFO: report: auto screenshot file exists
PASS: [TEST][PASS] [STEP:auto-screenshot-file-exists] report: auto screenshot file exists
```

#### Browser Logs

```text
<empty>
```

---

