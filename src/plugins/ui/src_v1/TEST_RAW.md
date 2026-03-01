# Test Report: ui-src-v1

- **Date**: Sat, 28 Feb 2026 14:38:31 PST
- **Total Duration**: 10.056023833s

## Summary

- **Steps**: 0 / 1 passed
- **Status**: FAILED

## Details

### 1. ❌ ui-section-hero-via-menu

- **Duration**: 10.056019935s
- **Error**: `step ui-section-hero-via-menu timed out`

#### Logs

```text
INFO: STEP> begin ui-section-hero-via-menu
INFO: LOOKING FOR: ui fixture build at /home/user/dialtone/src/plugins/ui/src_v1/test/fixtures/app
INFO: LOOKING FOR: [/home/user/dialtone_dependencies/bun/bin/bun install --silent]
INFO: LOOKING FOR: [/home/user/dialtone_dependencies/bun/bin/bun run build]
INFO: LOOKING FOR: go ui backend at http://127.0.0.1:32823
INFO: ERROR_PING: start browser_subject=logs.test.ui.src-v1.ui-section-hero-via-menu.browser error_subject=logs.test.ui.src-v1.error
INFO: ERROR_PING: error-topic-ok marker=__DIALTONE_ERROR_PING__:1772318303912799316:error
INFO: ERROR_PING: browser-topic-ok marker=__DIALTONE_ERROR_PING__:1772318303912799316
INFO: ERROR_PING: pass browser_topic=true error_topic=true
```

#### Errors

```text
FAIL: [TEST][FAIL] [STEP:ui-section-hero-via-menu] timed out after 10s
```

#### Browser Logs

```text
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] LOADING #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] LOADED #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] START #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] RESUME #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] LOADING #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] LOADED #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] START #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] RESUME #ui-hero-stage"
INFO: CONSOLE:log: "__DIALTONE_ERROR_PING__:1772318303912799316"
ERROR: CONSOLE:error: "__DIALTONE_ERROR_PING__:1772318303912799316:error"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] LOADING #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] LOADED #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] START #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] RESUME #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] LOADING #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] LOADED #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] START #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #ui-hero-stage"
INFO: CONSOLE:log: "[SectionManager] RESUME #ui-hero-stage"
```

#### Screenshots

![auto_ui-section-hero-via-menu.png](screenshots/auto_ui-section-hero-via-menu.png)

---

