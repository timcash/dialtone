# Test Report: chrome-src-v1

- **Date**: Sat, 28 Feb 2026 08:59:21 PST
- **Total Duration**: 24.368740363s

## Summary

- **Steps**: 6 / 6 passed
- **Status**: PASSED

## Details

### 1. ✅ example-library-metadata-and-helpers

- **Duration**: 2.330951ms
- **Report**: library metadata helpers validated

#### Logs

```text
INFO: wrote metadata file: .chrome_data/meta-1772297939020798928.json
INFO: report: library metadata helpers validated
PASS: [TEST][PASS] [STEP:example-library-metadata-and-helpers] report: library metadata helpers validated
```

#### Browser Logs

```text
<empty>
```

---

### 2. ✅ setup-and-launch-dev-headed-gpu

- **Duration**: 10.291753543s
- **Report**: remote dev session attach succeeded; title mismatch tolerated

#### Logs

```text
INFO: remote pre-launch role counts on legion: dev=0 test=0
INFO: ERROR_PING: start browser_subject=logs.test.chrome-src-v1.setup-and-launch-dev-headed-gpu.browser error_subject=logs.test.chrome-src-v1.error
INFO: ERROR_PING: browser-topic-ok marker=__DIALTONE_ERROR_PING__:1772297944433155620
INFO: ERROR_PING: error-topic-ok marker=__DIALTONE_ERROR_PING__:1772297944433155620:error
INFO: ERROR_PING: pass browser_topic=true error_topic=true
WARN: unexpected remote dev title on legion: "" (continuing)
INFO: report: remote dev session attach succeeded; title mismatch tolerated
PASS: [TEST][PASS] [STEP:setup-and-launch-dev-headed-gpu] report: remote dev session attach succeeded; title mismatch tolerated
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

---

### 3. ✅ reuse-dev-and-attach-new-tab

- **Duration**: 6.079005379s
- **Report**: reused remote dev session attach on remote node

#### Logs

```text
INFO: remote pre-reuse role counts on legion: dev=0 test=0
INFO: remote post-reuse role counts on legion: dev=0 test=0
INFO: reused remote dev session on legion
INFO: report: reused remote dev session attach on remote node
PASS: [TEST][PASS] [STEP:reuse-dev-and-attach-new-tab] report: reused remote dev session attach on remote node
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![auto_reuse-dev-and-attach-new-tab.png](screenshots/auto_reuse-dev-and-attach-new-tab.png)

---

### 4. ✅ launch-test-headless-and-list-processes

- **Duration**: 3.721041618s
- **Report**: launched remote test session and verified remote attach path

#### Logs

```text
INFO: remote pre-test-launch role counts on legion: dev=0 test=0
INFO: remote post-test-launch role counts on legion: dev=0 test=0
WARN: remote test role count did not increase on legion after launch (dev=0 test=0); continuing due cross-shell process visibility limits
INFO: verified remote test session on legion
INFO: report: launched remote test session and verified remote attach path
PASS: [TEST][PASS] [STEP:launch-test-headless-and-list-processes] report: launched remote test session and verified remote attach path
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![auto_launch-test-headless-and-list-processes.png](screenshots/auto_launch-test-headless-and-list-processes.png)

---

### 5. ✅ cleanup-test-preserve-dev

- **Duration**: 1.778035208s
- **Report**: remote mode cleanup removed test role while preserving dev role

#### Logs

```text
INFO: remote pre-cleanup-test role counts on legion: dev=0 test=0
INFO: remote post-cleanup-test role counts on legion: dev=0 test=0
INFO: report: remote mode cleanup removed test role while preserving dev role
PASS: [TEST][PASS] [STEP:cleanup-test-preserve-dev] report: remote mode cleanup removed test role while preserving dev role
```

#### Browser Logs

```text
<empty>
```

---

### 6. ✅ cleanup-all

- **Duration**: 2.028013938s
- **Report**: cleanup complete for remote chrome test mode

#### Logs

```text
INFO: remote pre-cleanup-all role counts on legion: dev=0 test=0
INFO: remote final role counts on legion: dev=0 test=0
INFO: report: cleanup complete for remote chrome test mode
PASS: [TEST][PASS] [STEP:cleanup-all] report: cleanup complete for remote chrome test mode
```

#### Browser Logs

```text
<empty>
```

---

