# UI Plugin src_v1 Test Report

## Test Environment

```text
<empty>
```

**Generated at:** Sun, 08 Mar 2026 10:18:03 -0700
**Version:** `ui-src-v1`
**Runner:** `test/src_v1`
**Status:** ✅ PASS
**Total Time:** `982.240806ms`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| ui-build-and-go-serve | ✅ PASS | `478.374324ms` |
| ui-section-hero-via-menu | ✅ PASS | `501.014814ms` |

## Step Details

## ui-build-and-go-serve

### Results

```text
result: PASS
duration: 478.374324ms
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
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
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

## ui-section-hero-via-menu

### Results

```text
result: PASS
duration: 501.014814ms
report: section ui-home-docs navigation verified
```

### Logs

```text
logs:
INFO: STEP> begin ui-section-hero-via-menu
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
WARN: skipping JS assertion for service-managed chrome src_v3 session
WARN: skipping overlay overlap detection for service-managed chrome src_v3 session
INFO: report: section ui-home-docs navigation verified
PASS: [TEST][PASS] [STEP:ui-section-hero-via-menu] report: section ui-home-docs navigation verified
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

![auto_ui-section-hero-via-menu.png](screenshots/auto_ui-section-hero-via-menu.png)

<!-- DIALTONE_CHROME_REPORT_START -->

## Chrome Report

- hostnode: `legion`
- chrome_count: `4`

| PID | ROLE | PORT |
| --- | --- | --- |
| 22548 | `dev` | 19464 |
| 23088 | `unlabeled` | 19464 |
| 24528 | `unlabeled` | 19464 |
| 27388 | `unlabeled` | 19464 |

<!-- DIALTONE_CHROME_REPORT_END -->
