# Test Report: logs-src-v1

- **Date**: Tue, 03 Mar 2026 14:21:41 PST
- **Total Duration**: 2.326653543s

## Summary

- **Steps**: 5 / 5 passed
- **Status**: PASSED

## Details

### 1. ✅ 01 Embedded NATS + topic publish

- **Duration**: 3.068936ms
- **Report**: NATS messages verified at nats://127.0.0.1:4222.

#### Logs

```text
INFO: report: NATS messages verified at nats://127.0.0.1:4222.
PASS: [TEST][PASS] [STEP:01 Embedded NATS + topic publish] report: NATS messages verified at nats://127.0.0.1:4222.
```

#### Browser Logs

```text
<empty>
```

---

### 2. ✅ 02 Listener filtering (error.topic)

- **Duration**: 1.391416ms
- **Report**: Verified error-topic filtering via NATS

#### Logs

```text
INFO: report: Verified error-topic filtering via NATS
PASS: [TEST][PASS] [STEP:02 Listener filtering (error.topic)] report: Verified error-topic filtering via NATS
```

#### Browser Logs

```text
<empty>
```

---

### 3. ✅ 04 Two-process pingpong via dialtone logs

- **Duration**: 1.186771171s
- **Report**: Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.

#### Logs

```text
INFO: report: Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.
PASS: [TEST][PASS] [STEP:04 Two-process pingpong via dialtone logs] report: Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.
```

#### Browser Logs

```text
<empty>
```

---

### 4. ✅ 05 Example plugin binary imports logs library

- **Duration**: 1.133051513s
- **Report**: Verified example plugin binary imports logs library and publishes expected messages.

#### Logs

```text
INFO: report: Verified example plugin binary imports logs library and publishes expected messages.
PASS: [TEST][PASS] [STEP:05 Example plugin binary imports logs library] report: Verified example plugin binary imports logs library and publishes expected messages.
```

#### Browser Logs

```text
<empty>
```

---

### 5. ✅ 03 Finalize artifacts

- **Duration**: 2.349304ms
- **Report**: Suite finalized. Verification transitioned to NATS topics.

#### Logs

```text
INFO: report: Suite finalized. Verification transitioned to NATS topics.
PASS: [TEST][PASS] [STEP:03 Finalize artifacts] report: Suite finalized. Verification transitioned to NATS topics.
```

#### Browser Logs

```text
<empty>
```

---

