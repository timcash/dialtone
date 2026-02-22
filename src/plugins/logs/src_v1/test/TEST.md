# Test Report: logs-src-v1

- **Date**: Sun, 22 Feb 2026 10:19:39 PST
- **Total Duration**: 2.054660206s

## Summary

- **Steps**: 5 / 5 passed
- **Status**: PASSED

## Details

### 1. ✅ 01 Embedded NATS + topic publish

- **Duration**: 2.258845ms
- **Report**: NATS messages verified at nats://127.0.0.1:4222.

---

### 2. ✅ 02 Listener filtering (error.topic)

- **Duration**: 1.418728ms
- **Report**: Verified error-topic filtering via NATS

---

### 3. ✅ 04 Two-process pingpong via dialtone logs

- **Duration**: 1.128589514s
- **Report**: Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.

---

### 4. ✅ 05 Example plugin binary imports logs library

- **Duration**: 921.469898ms
- **Report**: Verified example plugin binary imports logs library and publishes expected messages.

---

### 5. ✅ 03 Finalize artifacts

- **Duration**: 904.282µs
- **Report**: Suite finalized. Verification transitioned to NATS topics.

---

