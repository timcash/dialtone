# Test Report: cad-src-v1

- **Date**: Wed, 18 Mar 2026 21:46:39 PDT
- **Total Duration**: 25.020993794s

## Summary

- **Steps**: 0 / 1 passed
- **Status**: FAILED

## Details

### 1. ❌ cad-ui-browser-smoke-src-v1

- **Duration**: 25.008534645s
- **Error**: `cad model never reached ready state: timed out waiting for aria-label "CAD Stage" attr "data-model-state"="ready"`

#### Logs

```text
INFO: cad browser smoke server ready at http://127.0.0.1:43397
WARN: ERROR_PING: skipped for chrome src_v3 NATS-managed browser session
```

#### Errors

```text
FAIL: [TEST][FAIL] [STEP:cad-ui-browser-smoke-src-v1] failed: cad model never reached ready state: timed out waiting for aria-label "CAD Stage" attr "data-model-state"="ready"
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![auto_cad-ui-browser-smoke-src-v1.png](screenshots/auto_cad-ui-browser-smoke-src-v1.png)

---

<!-- DIALTONE_CHROME_REPORT_START -->

## Chrome Report

- hostnode: `legion`
- chrome_count: `10`

| PID | ROLE | PORT |
| --- | --- | --- |
| 2412 | `unlabeled` | 19464 |
| 6928 | `unlabeled` | 19464 |
| 9592 | `unlabeled` | 19464 |
| 10452 | `unlabeled` | 19464 |
| 11364 | `cad-smoke` | 19464 |
| 12412 | `unlabeled` | 19464 |
| 13336 | `unlabeled` | 19464 |
| 16608 | `unlabeled` | 19464 |
| 19084 | `unlabeled` | 19464 |
| 19736 | `dev` | 19464 |

<!-- DIALTONE_CHROME_REPORT_END -->
