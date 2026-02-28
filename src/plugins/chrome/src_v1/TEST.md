# Chrome Plugin src_v1 Test Report

## Test Environment

```text
<empty>
```

**Generated at:** Sat, 28 Feb 2026 12:43:47 -0800
**Version:** `chrome-src-v1`
**Runner:** `test/src_v1`
**Status:** ✅ PASS
**Total Time:** `27.904093279s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| example-library-metadata-and-helpers | ✅ PASS | `4.109473ms` |
| setup-and-launch-dev-headed-gpu | ✅ PASS | `10.314511748s` |
| reuse-dev-and-attach-new-tab | ✅ PASS | `3.132499892s` |
| launch-test-headless-and-list-processes | ✅ PASS | `6.183450156s` |
| cleanup-test-preserve-dev | ✅ PASS | `3.210537398s` |
| cleanup-all | ✅ PASS | `5.05896293s` |

## Step Details

## example-library-metadata-and-helpers

### Results

```text
result: PASS
duration: 4.109473ms
report: library metadata helpers validated
```

### Logs

```text
logs:
INFO: wrote metadata file: .chrome_data/meta-1772311401305303634.json
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
duration: 10.314511748s
report: launched headed dev session with gpu and debug port ready
```

### Logs

```text
logs:
INFO: pre-launch role counts: dev=0 test=8
INFO: post-precleanup role counts: dev=0 test=0
INFO: post-dev-launch role counts: dev=2 test=0
INFO: launched dev session pid=1759749 port=9333 user_data_dir=C:\Users\timca\AppData\Local\Temp\dialtone-chrome-test-1772311406209575706-dev
INFO: report: launched headed dev session with gpu and debug port ready
PASS: [TEST][PASS] [STEP:setup-and-launch-dev-headed-gpu] report: launched headed dev session with gpu and debug port ready
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

## reuse-dev-and-attach-new-tab

### Results

```text
result: PASS
duration: 3.132499892s
report: reused dev session, reattached after disconnect, and confirmed no extra dev spawn
```

### Logs

```text
logs:
INFO: reused dev session and created new tab via chromedp
INFO: report: reused dev session, reattached after disconnect, and confirmed no extra dev spawn
PASS: [TEST][PASS] [STEP:reuse-dev-and-attach-new-tab] report: reused dev session, reattached after disconnect, and confirmed no extra dev spawn
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

## launch-test-headless-and-list-processes

### Results

```text
result: PASS
duration: 6.183450156s
report: launched headless test session and validated process listing metadata
```

### Logs

```text
logs:
INFO: post-test-launch role counts: dev=2 test=2
INFO: verified list shows dev/test roles, headed/headless, gpu and user-data-dir
INFO: report: launched headless test session and validated process listing metadata
PASS: [TEST][PASS] [STEP:launch-test-headless-and-list-processes] report: launched headless test session and validated process listing metadata
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

## cleanup-test-preserve-dev

### Results

```text
result: PASS
duration: 3.210537398s
report: cleaned test session while preserving dev session
```

### Logs

```text
logs:
INFO: post-cleanup-test role counts: dev=2 test=0
INFO: cleaned test role and preserved dev role
INFO: report: cleaned test session while preserving dev session
PASS: [TEST][PASS] [STEP:cleanup-test-preserve-dev] report: cleaned test session while preserving dev session
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
duration: 5.05896293s
report: cleanup complete for chrome role sessions
```

### Logs

```text
logs:
INFO: final role counts: dev=0 test=0 (pre-launch dev=0 test=8)
INFO: cleanup complete
INFO: report: cleanup complete for chrome role sessions
PASS: [TEST][PASS] [STEP:cleanup-all] report: cleanup complete for chrome role sessions
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

