# Test Report: logs-src-v1

- **Date**: Thu, 26 Feb 2026 13:33:06 PST
- **Total Duration**: 2.102416885s

## Summary

- **Steps**: 5 / 5 passed
- **Status**: PASSED

## Details

### 1. ✅ 01 Embedded NATS + topic publish

- **Duration**: 1.446795ms
- **Report**: NATS messages verified at nats://127.0.0.1:4222.

---

### 2. ✅ 02 Listener filtering (error.topic)

- **Duration**: 600.957µs
- **Report**: Verified error-topic filtering via NATS

---

### 3. ✅ 04 Two-process pingpong via dialtone logs

- **Duration**: 1.159897387s
- **Report**: Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.

---

### 4. ✅ 05 Example plugin binary imports logs library

- **Duration**: 939.621422ms
- **Report**: Verified example plugin binary imports logs library and publishes expected messages.

---

### 5. ✅ 03 Finalize artifacts

- **Duration**: 834.179µs
- **Report**: Suite finalized. Verification transitioned to NATS topics.

---

