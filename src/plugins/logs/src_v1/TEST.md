# Logs Plugin src_v1 Test Report

## Test Environment

```text
<empty>
```

**Generated at:** Tue, 03 Mar 2026 14:21:41 -0800
**Version:** `logs-src-v1`
**Runner:** `test/src_v1`
**Status:** ✅ PASS
**Total Time:** `2.326653543s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 Embedded NATS + topic publish | ✅ PASS | `3.068936ms` |
| 02 Listener filtering (error.topic) | ✅ PASS | `1.391416ms` |
| 04 Two-process pingpong via dialtone logs | ✅ PASS | `1.186771171s` |
| 05 Example plugin binary imports logs library | ✅ PASS | `1.133051513s` |
| 03 Finalize artifacts | ✅ PASS | `2.349304ms` |

## Step Details

## 01 Embedded NATS + topic publish

### Results

```text
result: PASS
duration: 3.068936ms
report: NATS messages verified at nats://127.0.0.1:4222.
```

### Logs

```text
logs:
INFO: report: NATS messages verified at nats://127.0.0.1:4222.
PASS: [TEST][PASS] [STEP:01 Embedded NATS + topic publish] report: NATS messages verified at nats://127.0.0.1:4222.
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

## 02 Listener filtering (error.topic)

### Results

```text
result: PASS
duration: 1.391416ms
report: Verified error-topic filtering via NATS
```

### Logs

```text
logs:
INFO: report: Verified error-topic filtering via NATS
PASS: [TEST][PASS] [STEP:02 Listener filtering (error.topic)] report: Verified error-topic filtering via NATS
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

## 04 Two-process pingpong via dialtone logs

### Results

```text
result: PASS
duration: 1.186771171s
report: Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.
```

### Logs

```text
logs:
INFO: report: Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.
PASS: [TEST][PASS] [STEP:04 Two-process pingpong via dialtone logs] report: Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.
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

## 05 Example plugin binary imports logs library

### Results

```text
result: PASS
duration: 1.133051513s
report: Verified example plugin binary imports logs library and publishes expected messages.
```

### Logs

```text
logs:
INFO: report: Verified example plugin binary imports logs library and publishes expected messages.
PASS: [TEST][PASS] [STEP:05 Example plugin binary imports logs library] report: Verified example plugin binary imports logs library and publishes expected messages.
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

## 03 Finalize artifacts

### Results

```text
result: PASS
duration: 2.349304ms
report: Suite finalized. Verification transitioned to NATS topics.
```

### Logs

```text
logs:
INFO: report: Suite finalized. Verification transitioned to NATS topics.
PASS: [TEST][PASS] [STEP:03 Finalize artifacts] report: Suite finalized. Verification transitioned to NATS topics.
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

