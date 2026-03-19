# UI Plugin src_v1 Test Report

## Test Environment

```text
<empty>
```

**Generated at:** Thu, 19 Mar 2026 15:51:03 -0700
**Version:** `ui-src-v1`
**Runner:** `test/src_v1`
**Status:** ✅ PASS
**Total Time:** `39.532556616s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| ui-build-and-go-serve | ✅ PASS | `38.81339405s` |

## Step Details

## ui-build-and-go-serve

### Results

```text
result: PASS
duration: 38.81339405s
report: fixture built, docs/home section loaded, text legend verified (attach=true)
```

### Error-Ping Check

```text
WARN: ERROR_PING: skipped for chrome src_v3 NATS-managed browser session
```

### Logs

```text
logs:
INFO: STEP> begin ui-build-and-go-serve
INFO: LOOKING FOR: starting persistent ui dev server in background at http://127.0.0.1:5177
INFO: LOOKING FOR: persistent ui dev server ready at http://127.0.0.1:5177
WARN: ERROR_PING: skipped for chrome src_v3 NATS-managed browser session
INFO: saved browser debug config: /home/user/dialtone/src/plugins/ui/src_v1/test/browser.debug.json
WARN: skipping JS assertion for service-managed chrome src_v3 session
INFO: report: fixture built, docs/home section loaded, text legend verified (attach=true)
PASS: [TEST][PASS] [STEP:ui-build-and-go-serve] report: fixture built, docs/home section loaded, text legend verified (attach=true)
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

### Screenshots

![auto_ui-build-and-go-serve.png](screenshots/auto_ui-build-and-go-serve.png)

<!-- DIALTONE_CHROME_REPORT_START -->

## Chrome Report

- hostnode: `legion`
- chrome_count: `13`

| PID | ROLE | PORT |
| --- | --- | --- |
| 384 | `unlabeled` | 19464 |
| 1148 | `unlabeled` | 21944 |
| 2996 | `cad-smoke` | 21944 |
| 3084 | `unlabeled` | 22602 |
| 4296 | `dev-isolated` | 22602 |
| 5956 | `unlabeled` | 21944 |
| 10076 | `unlabeled` | 21944 |
| 11952 | `unlabeled` | 22602 |
| 14156 | `dev` | 19464 |
| 17200 | `unlabeled` | 21944 |
| 17888 | `unlabeled` | 19464 |
| 18708 | `unlabeled` | 22602 |
| 19132 | `unlabeled` | 19464 |

<!-- DIALTONE_CHROME_REPORT_END -->
