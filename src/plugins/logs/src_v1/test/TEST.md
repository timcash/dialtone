# Test Report: logs-src-v1

- **Date**: Mon, 23 Feb 2026 21:32:03 PST
- **Total Duration**: 2.309137039s

## Summary

- **Steps**: 5 / 5 passed
- **Status**: PASSED

## Details

### 1. ✅ 01 Embedded NATS + topic publish

- **Duration**: 2.167412ms
- **Report**: NATS messages verified at nats://127.0.0.1:4222.

---

### 2. ✅ 02 Listener filtering (error.topic)

- **Duration**: 1.244271ms
- **Report**: Verified error-topic filtering via NATS

---

### 3. ✅ 04 Two-process pingpong via dialtone logs

- **Duration**: 1.199787059s
- **Report**: Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic.

---

### 4. ✅ 05 Example plugin binary imports logs library

- **Duration**: 1.104239335s
- **Report**: Verified example plugin binary imports logs library and publishes expected messages.

---

### 5. ✅ 03 Finalize artifacts

- **Duration**: 1.683528ms
- **Report**: Suite finalized. Verification transitioned to NATS topics.

---

