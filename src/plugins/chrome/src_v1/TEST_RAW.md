# Test Report: chrome-src-v1

- **Date**: Sun, 01 Mar 2026 12:53:28 PST
- **Total Duration**: 24.208277411s

## Summary

- **Steps**: 6 / 6 passed
- **Status**: PASSED

## Details

### 1. ✅ example-library-metadata-and-helpers

- **Duration**: 4.128185ms
- **Report**: library metadata helpers validated

#### Logs

```text
INFO: wrote metadata file: .chrome_data/meta-1772398385147893389.json
INFO: report: library metadata helpers validated
PASS: [TEST][PASS] [STEP:example-library-metadata-and-helpers] report: library metadata helpers validated
```

#### Browser Logs

```text
<empty>
```

---

### 2. ✅ setup-and-launch-dev-headed-gpu

- **Duration**: 7.184729304s
- **Report**: launched headed dev session with gpu and debug port ready

#### Logs

```text
INFO: pre-launch role counts: dev=0 test=0
INFO: post-precleanup role counts: dev=0 test=0
INFO: post-dev-launch role counts: dev=2 test=0
INFO: launched dev session pid=2322693 port=19333 user_data_dir=C:\Users\timca\AppData\Local\Temp\dialtone-chrome-test-1772398386952404533-dev
INFO: report: launched headed dev session with gpu and debug port ready
PASS: [TEST][PASS] [STEP:setup-and-launch-dev-headed-gpu] report: launched headed dev session with gpu and debug port ready
```

#### Browser Logs

```text
<empty>
```

---

### 3. ✅ reuse-dev-and-attach-new-tab

- **Duration**: 2.793912852s
- **Report**: reused dev session, reattached after disconnect, and confirmed no extra dev spawn

#### Logs

```text
INFO: reused dev session and created new tab via chromedp
INFO: report: reused dev session, reattached after disconnect, and confirmed no extra dev spawn
PASS: [TEST][PASS] [STEP:reuse-dev-and-attach-new-tab] report: reused dev session, reattached after disconnect, and confirmed no extra dev spawn
```

#### Browser Logs

```text
<empty>
```

---

### 4. ✅ launch-test-headless-and-list-processes

- **Duration**: 5.942887107s
- **Report**: launched headless test session and validated process listing metadata

#### Logs

```text
INFO: post-test-launch role counts: dev=2 test=2
INFO: verified list shows dev/test roles, headed/headless, gpu and user-data-dir
INFO: report: launched headless test session and validated process listing metadata
PASS: [TEST][PASS] [STEP:launch-test-headless-and-list-processes] report: launched headless test session and validated process listing metadata
```

#### Browser Logs

```text
<empty>
```

---

### 5. ✅ cleanup-test-preserve-dev

- **Duration**: 3.127769046s
- **Report**: cleaned test session while preserving dev session

#### Logs

```text
INFO: post-cleanup-test role counts: dev=2 test=0
INFO: cleaned test role and preserved dev role
INFO: report: cleaned test session while preserving dev session
PASS: [TEST][PASS] [STEP:cleanup-test-preserve-dev] report: cleaned test session while preserving dev session
```

#### Browser Logs

```text
<empty>
```

---

### 6. ✅ cleanup-all

- **Duration**: 5.154830459s
- **Report**: cleanup complete for chrome role sessions

#### Logs

```text
INFO: final role counts: dev=0 test=0 (pre-launch dev=0 test=0)
INFO: cleanup complete
INFO: report: cleanup complete for chrome role sessions
PASS: [TEST][PASS] [STEP:cleanup-all] report: cleanup complete for chrome role sessions
```

#### Browser Logs

```text
<empty>
```

---

