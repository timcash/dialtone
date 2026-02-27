# Test Report: robot-src-v2

- **Date**: Fri, 27 Feb 2026 09:37:13 PST
- **Total Duration**: 37.703476112s

## Summary

- **Steps**: 3 / 4 passed
- **Status**: FAILED

## Details

### 1. ✅ 01-build-robot-v2-binary

- **Duration**: 228.538387ms
- **Report**: binary build verified

---

### 2. ✅ 02-server-health-and-root-behavior

- **Duration**: 162.381202ms
- **Report**: server runtime smoke verified

---

### 3. ✅ 03-manifest-has-required-sync-artifacts

- **Duration**: 3.256956ms
- **Report**: manifest sync artifact contract verified

---

### 4. ❌ 04-local-ui-mock-e2e-smoke

- **Duration**: 37.309291899s
- **Error**: `local navigate failed (could not dial "ws://127.0.0.1:56847/devtools/browser/638f3f1d-33d5-40b8-b1ca-c3086da06778": dial tcp 127.0.0.1:56847: connect: connection refused), remote fallback failed on darkmac (remote command on darkmac failed: ssh command failed on darkmac: command error: Process exited with status 1
Output: DIALTONE> Go runtime missing at /Users/tim/.dialtone_env/go
DIALTONE> Would you like to install it? [y/N]  output=DIALTONE> Go runtime missing at /Users/tim/.dialtone_env/go
DIALTONE> Would you like to install it? [y/N])`

---

