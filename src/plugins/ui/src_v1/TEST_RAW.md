# Test Report: ui-src-v1

- **Date**: Sun, 08 Mar 2026 10:18:03 PDT
- **Total Duration**: 982.240806ms

## Summary

- **Steps**: 2 / 2 passed
- **Status**: PASSED

## Details

### 1. ✅ ui-build-and-go-serve

- **Duration**: 478.374324ms
- **Report**: fixture built, docs/home section loaded, text legend verified (attach=true)

#### Logs

```text
INFO: STEP> begin ui-build-and-go-serve
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
WARN: ERROR_PING: skipped for chrome src_v3 NATS-managed browser session
INFO: saved browser debug config: /home/user/dialtone/src/plugins/ui/src_v1/test/browser.debug.json
WARN: skipping JS assertion for service-managed chrome src_v3 session
INFO: report: fixture built, docs/home section loaded, text legend verified (attach=true)
PASS: [TEST][PASS] [STEP:ui-build-and-go-serve] report: fixture built, docs/home section loaded, text legend verified (attach=true)
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![auto_ui-build-and-go-serve.png](screenshots/auto_ui-build-and-go-serve.png)

---

### 2. ✅ ui-section-hero-via-menu

- **Duration**: 501.014814ms
- **Report**: section ui-home-docs navigation verified

#### Logs

```text
INFO: STEP> begin ui-section-hero-via-menu
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
WARN: skipping JS assertion for service-managed chrome src_v3 session
WARN: skipping overlay overlap detection for service-managed chrome src_v3 session
INFO: report: section ui-home-docs navigation verified
PASS: [TEST][PASS] [STEP:ui-section-hero-via-menu] report: section ui-home-docs navigation verified
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![auto_ui-section-hero-via-menu.png](screenshots/auto_ui-section-hero-via-menu.png)

---

