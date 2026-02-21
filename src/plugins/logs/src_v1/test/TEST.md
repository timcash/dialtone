# Test Report: logs-src-v1

- **Date**: Sat, 21 Feb 2026 15:41:50 PST
- **Total Duration**: 2.075232601s

## Summary

- **Steps**: 5 / 5 passed
- **Status**: PASSED

## Details

### 1. ✅ 01 Embedded NATS + topic publish

- **Duration**: 2.836581ms
- **Report**: NATS messages verified at nats://127.0.0.1:4222.

---

### 2. ✅ 02 Listener filtering (error.topic)

- **Duration**: 811.361µs
- **Report**: Verified error-topic filtering via NATS

---

### 3. ✅ 04 Two-process pingpong via dialtone logs

- **Duration**: 1.12572863s
- **Report**: Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.

---

### 4. ✅ 05 Example plugin binary imports logs library

- **Duration**: 944.836535ms
- **Report**: Verified example plugin binary imports logs library and publishes expected messages.

---

### 5. ✅ 03 Finalize artifacts

- **Duration**: 980.895µs
- **Report**: Suite finalized. Verification transitioned to NATS topics.

---

