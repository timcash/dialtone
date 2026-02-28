# Chrome Plugin src_v1 Test Report

## Test Environment

```text
<empty>
```

**Generated at:** Sat, 28 Feb 2026 08:59:21 -0800
**Version:** `chrome-src-v1`
**Runner:** `test/src_v1`
**Status:** ✅ PASS
**Total Time:** `24.368740363s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| example-library-metadata-and-helpers | ✅ PASS | `2.330951ms` |
| setup-and-launch-dev-headed-gpu | ✅ PASS | `10.291753543s` |
| reuse-dev-and-attach-new-tab | ✅ PASS | `6.079005379s` |
| launch-test-headless-and-list-processes | ✅ PASS | `3.721041618s` |
| cleanup-test-preserve-dev | ✅ PASS | `1.778035208s` |
| cleanup-all | ✅ PASS | `2.028013938s` |

## Step Details

## example-library-metadata-and-helpers

### Results

```text
result: PASS
duration: 2.330951ms
report: library metadata helpers validated
```

### Logs

```text
logs:
INFO: wrote metadata file: .chrome_data/meta-1772297939020798928.json
INFO: report: library metadata helpers validated
PASS: [TEST][PASS] [STEP:example-library-metadata-and-helpers] report: library metadata helpers validated
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

## setup-and-launch-dev-headed-gpu

### Results

```text
result: PASS
duration: 10.291753543s
report: remote dev session attach succeeded; title mismatch tolerated
```

### Error-Ping Check

```text
INFO: ERROR_PING: start browser_subject=logs.test.chrome-src-v1.setup-and-launch-dev-headed-gpu.browser error_subject=logs.test.chrome-src-v1.error
INFO: ERROR_PING: browser-topic-ok marker=__DIALTONE_ERROR_PING__:1772297944433155620
INFO: ERROR_PING: error-topic-ok marker=__DIALTONE_ERROR_PING__:1772297944433155620:error
INFO: ERROR_PING: pass browser_topic=true error_topic=true
```

### Logs

```text
logs:
INFO: remote pre-launch role counts on legion: dev=0 test=0
INFO: ERROR_PING: start browser_subject=logs.test.chrome-src-v1.setup-and-launch-dev-headed-gpu.browser error_subject=logs.test.chrome-src-v1.error
INFO: ERROR_PING: browser-topic-ok marker=__DIALTONE_ERROR_PING__:1772297944433155620
INFO: ERROR_PING: error-topic-ok marker=__DIALTONE_ERROR_PING__:1772297944433155620:error
INFO: ERROR_PING: pass browser_topic=true error_topic=true
WARN: unexpected remote dev title on legion: "" (continuing)
INFO: report: remote dev session attach succeeded; title mismatch tolerated
PASS: [TEST][PASS] [STEP:setup-and-launch-dev-headed-gpu] report: remote dev session attach succeeded; title mismatch tolerated
```

### Errors

```text
errors:
<empty>
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
INFO: CONSOLE:log: "__DIALTONE_ERROR_PING__:1772297944433155620"
ERROR: CONSOLE:error: "__DIALTONE_ERROR_PING__:1772297944433155620:error"
```

## reuse-dev-and-attach-new-tab

### Results

```text
result: PASS
duration: 6.079005379s
report: reused remote dev session attach on remote node
```

### Logs

```text
logs:
INFO: remote pre-reuse role counts on legion: dev=0 test=0
INFO: remote post-reuse role counts on legion: dev=0 test=0
INFO: reused remote dev session on legion
INFO: report: reused remote dev session attach on remote node
PASS: [TEST][PASS] [STEP:reuse-dev-and-attach-new-tab] report: reused remote dev session attach on remote node
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

![auto_reuse-dev-and-attach-new-tab.png](screenshots/auto_reuse-dev-and-attach-new-tab.png)

## launch-test-headless-and-list-processes

### Results

```text
result: PASS
duration: 3.721041618s
report: launched remote test session and verified remote attach path
```

### Logs

```text
logs:
INFO: remote pre-test-launch role counts on legion: dev=0 test=0
INFO: remote post-test-launch role counts on legion: dev=0 test=0
WARN: remote test role count did not increase on legion after launch (dev=0 test=0); continuing due cross-shell process visibility limits
INFO: verified remote test session on legion
INFO: report: launched remote test session and verified remote attach path
PASS: [TEST][PASS] [STEP:launch-test-headless-and-list-processes] report: launched remote test session and verified remote attach path
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

![auto_launch-test-headless-and-list-processes.png](screenshots/auto_launch-test-headless-and-list-processes.png)

## cleanup-test-preserve-dev

### Results

```text
result: PASS
duration: 1.778035208s
report: remote mode cleanup removed test role while preserving dev role
```

### Logs

```text
logs:
INFO: remote pre-cleanup-test role counts on legion: dev=0 test=0
INFO: remote post-cleanup-test role counts on legion: dev=0 test=0
INFO: report: remote mode cleanup removed test role while preserving dev role
PASS: [TEST][PASS] [STEP:cleanup-test-preserve-dev] report: remote mode cleanup removed test role while preserving dev role
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

## cleanup-all

### Results

```text
result: PASS
duration: 2.028013938s
report: cleanup complete for remote chrome test mode
```

### Logs

```text
logs:
INFO: remote pre-cleanup-all role counts on legion: dev=0 test=0
INFO: remote final role counts on legion: dev=0 test=0
INFO: report: cleanup complete for remote chrome test mode
PASS: [TEST][PASS] [STEP:cleanup-all] report: cleanup complete for remote chrome test mode
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
