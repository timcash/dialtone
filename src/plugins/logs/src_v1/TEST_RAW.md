# Test Report: logs-src-v1

- **Date**: Sat, 28 Feb 2026 12:36:54 PST
- **Total Duration**: 2.299169378s

## Summary

- **Steps**: 5 / 5 passed
- **Status**: PASSED

## Details

### 1. ✅ 01 Embedded NATS + topic publish

- **Duration**: 2.922695ms
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

- **Duration**: 1.026545ms
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

- **Duration**: 1.25906356s
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

- **Duration**: 1.034742881s
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

- **Duration**: 1.365045ms
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

