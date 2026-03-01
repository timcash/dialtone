# Test Report: robot-src-v2

- **Date**: Sat, 28 Feb 2026 15:51:51 PST
- **Total Duration**: 3.454937471s

## Summary

- **Steps**: 3 / 4 passed
- **Status**: FAILED

## Details

### 1. ✅ 01-build-robot-v2-binary

- **Duration**: 316.296344ms
- **Report**: binary build verified

#### Logs

```text
INFO: [ACTION] build robot src_v2 server binary
INFO: build complete
INFO: report: binary build verified
PASS: [TEST][PASS] [STEP:01-build-robot-v2-binary] report: binary build verified
```

#### Browser Logs

```text
<empty>
```

---

### 2. ✅ 02-server-health-and-root-behavior

- **Duration**: 160.493508ms
- **Report**: server runtime smoke verified

#### Logs

```text
INFO: [ACTION] probe /health on http://127.0.0.1:18082
INFO: health ok
INFO: [ACTION] probe / expecting 200 (ui dist present) or 503 (scaffold)
INFO: root behavior verified
INFO: [ACTION] probe /api/init scaffold payload
INFO: api init returned wsPath
INFO: [ACTION] websocket dial /natsws
INFO: natsws websocket connected
INFO: [ACTION] probe /stream scaffold behavior
INFO: stream returned 503
INFO: [ACTION] probe /api/integration-health scaffold payload
INFO: integration health reported degraded
INFO: report: server runtime smoke verified
PASS: [TEST][PASS] [STEP:02-server-health-and-root-behavior] report: server runtime smoke verified
```

#### Browser Logs

```text
<empty>
```

---

### 3. ✅ 03-manifest-has-required-sync-artifacts

- **Duration**: 740.369µs
- **Report**: manifest sync artifact contract verified

#### Logs

```text
INFO: manifest contains required artifact keys
INFO: report: manifest sync artifact contract verified
PASS: [TEST][PASS] [STEP:03-manifest-has-required-sync-artifacts] report: manifest sync artifact contract verified
```

#### Browser Logs

```text
<empty>
```

---

### 4. ❌ 04-local-ui-mock-e2e-smoke

- **Duration**: 2.977395333s
- **Error**: `page load error net::ERR_CONNECTION_REFUSED`

#### Logs

```text
INFO: ui build complete
INFO: ui root returned 200
```

#### Errors

```text
FAIL: [TEST][FAIL] [STEP:04-local-ui-mock-e2e-smoke] failed: page load error net::ERR_CONNECTION_REFUSED
```

#### Browser Logs

```text
<empty>
```

---

